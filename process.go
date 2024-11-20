package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"log"
	"os/exec"
	"regexp"
	"strings"

	"github.com/ariden/goia/secret"
)

type job struct {
	args                 *appArgs
	cache                *ConfigCache
	fileDir              string
	fileName             string
	fileWithVendor       bool
	currentFileName      string
	currentStep          step
	lang                 string
	listFunctionsCreated []string
	listFunctionsUpdated []string
	maxAttempts          int
	mockOpenAIResponse   bool
	openAIApiKey         secret.String
	openAIModel          string
	openAIURL            string
	openAITemperature    float64
	openAIMaxTokens      int
	trad                 Translations
	source               fileSource
}

func newJob(cache *ConfigCache, fileDir string, args *appArgs) (*job, error) {
	j := job{
		cache:                cache,
		fileDir:              fileDir,
		fileName:             "main.go",
		currentStep:          stepVerifyPrompt,
		currentFileName:      "main.go",
		listFunctionsUpdated: []string{},
		listFunctionsCreated: []string{},
		maxAttempts:          cache.rootConfig.MaxAttempts,
		mockOpenAIResponse:   false,
		openAIApiKey:         secret.String(cache.rootConfig.OpenAIKey),
		openAIModel:          cache.rootConfig.OpenAIModel,
		openAIURL:            cache.rootConfig.OpenAIURL,
		openAITemperature:    cache.rootConfig.OpenAITemperature,
		openAIMaxTokens:      cache.rootConfig.OpenAIMaxTokens,
		lang:                 "en",
		args:                 args,
	}

	return &j, nil
}

/*
je voudrais avoir un handler qui reçoit un prompt et qui execute le code golang associé dans un fichier local et je voudrais pouvoir estimer et retourner le cout d'execution du code en question
*/

// run exécute le job.
func (j *job) run() error {
	if err := j.updateCache(); err != nil {
		return err
	}

	src, err := j.readFileContent()
	if err != nil {
		return err
	}

	/*filesFound, err := j.loadFilesFromFolder()
	if err != nil {
		fmt.Println(j.t("No files found in the specified folder, a main.go file will be created"), err)
		if err := j.promptNoFilesFoundCreateANewFile(); err != nil {
			return
		}
	} else {
		if err := j.promptSelectAFileOrCreateANewOne(filesFound); err != nil {
			return
		}
	}*/

	// Exécuter go mod init et go mod tidy
	if err := j.setupGoMod(); err != nil {
		fmt.Println(j.t("Error configuring Go modules"), err)
		return err
	}

	prompt, err := j.promptForQuery()
	if err != nil {
		log.Fatalf("Erreur : %v", err)
	}

	prompt = j.prepareGoPrompt(prompt)

	for _, stepEntry := range stepsOrder {
		j.currentStep = stepEntry.ValidStep
		j.currentFileName = j.fileName

		if j.currentStep == stepVerifyPrompt {
			verifPrompt := fmt.Sprintf(j.t("Responds with true or false in JSON. Is the following question a request for Go code")+" : \"%s\" ?", prompt)
			fmt.Println(fmt.Sprintf("\nprompt: "+blue("%s")+"\n\n", verifPrompt))

			respContent, err := j.callIA(verifPrompt)
			if err != nil {
				fmt.Println(j.t("Error checking prompt"), err)
				return err
			}
			if !j.responseToBool(respContent) {
				fmt.Println(j.t("The question is not a request for Go code"))
				// restart the job
				return j.run()
			}
			continue

		} else if j.currentStep == stepStart {
			fileContent := string(src)
			if len(fileContent) > 50 {
				prompt += ".\n\n" + j.t("Here is the Golang code") + " :\n\n" + fileContent
			}

		} else if j.currentStep == stepAddTest {
			fileContent, err := j.readFileContent()
			if err != nil {
				fmt.Println(j.t("Error retrieving the name of the contents of the current file"), err)
				return err
			}

			prompt = j.t("I have some Golang code") + ":"
			prompt += "\n\n" + string(fileContent)
			prompt += "\n\n" + j.t("I would like to enrich these functions with unit tests") + ":"
			prompt += "\n\n" + j.printTestsFuncName()
			prompt += "\n\n" + j.t("Can you generate the tests for the nominal cases as well as the error cases, without comment or explanation? My goal is to ensure comprehensive coverage, particularly for:\n\nExpected success scenarios (nominal cases)\nError handling scenarios\nPlease structure the tests to be easily readable, using t.Run to name each test case.")

		} else {
			prompt = j.t(stepEntry.Prompt)

			fileContent, err := j.readFileContent()
			if err != nil {
				fmt.Println(j.t("Error retrieving the name of the contents of the current file"), err)
				return err
			}
			prompt += "\n\n" + string(fileContent)
		}

		if j.currentStep == stepAddTest {
			testFileName, err := j.getTestFilename()
			if err != nil {
				fmt.Println(j.t("Error retrieving test file name"), err)
				return err
			}
			j.currentFileName = testFileName
		}

		for attempt := 1; attempt <= j.maxAttempts; attempt++ {

			fmt.Println(fmt.Sprintf("\nprompt: "+blue("%s")+"\n\n", prompt))

			code, err := j.callIA(prompt)
			if err != nil {
				fmt.Println(j.t("Error generating code"), err)
				return err
			}

			if j.currentStep == stepAddTest {
				// Écriture du code dans un fichier
				src, err = j.createNewTestFile()
				if err != nil {
					fmt.Println(j.t("Error writing file"), err)
					return err
				}
			}

			codeModified, err := j.stepFixCode(code)
			if err != nil {
				fmt.Println(j.t("Error to get code modified"), err)
				return err
			}

			if err := j.writeFile(src, codeModified); err != nil {
				fmt.Println(j.t("Error updating file"), err)
				return err
			}

			// Exécuter go mod init et go mod tidy
			if err = j.updateGoMod(); err != nil {
				fmt.Println(j.t("Error configuring Go modules"), err)
				return err
			}

			// Exécution de goimports pour corriger les imports manquants
			if err = j.fixImports(); err != nil {
				fmt.Println(j.t("Error correcting imports"), err)
				return err
			}

			// Exécution du fichier Go
			output, err := j.runGolangFile()
			if err != nil {
				fmt.Println(fmt.Sprintf("------------------------------------ result (failed): \n\n %s", output))
				j.currentStep = stepEntry.ErrorStep

				if j.currentStep == stepAddTestError {

					getFailedTests, err := j.getFailedTests(output)
					if err != nil {
						fmt.Println(j.t("Error when recovering failed tests"), err)
						return err
					}

					if getFailedTests == nil {
						fmt.Println(j.t("No test failed"))
						return nil
					}

					testCode, err := j.getTestCode(getFailedTests)
					if err != nil {
						fmt.Println("Error retrieving failed test code", err)
						return err
					}

					prompt = j.t("The following tests") + " \n\n" + testCode + "\n\n " +
						j.t("returned the following errors") + ": \n\n" +
						j.t("Error") + " : " + output + "\n\n" +
						j.t("responds without adding comments or explanations")

				} else {
					funcCode, err := j.extractErrorForPrompt(output)
					if err != nil {
						fmt.Println("Erreur:", err)
						return err
					}
					fmt.Println(j.t("Runtime error"), output)
					// Mise à jour de l'instruction pour l'API en ajoutant le retour d'erreur
					prompt = j.t("Fix the following code that generated an error") + ":\n\n" + funcCode + "\n\n" +
						j.t("Error") + " : " + output + "\n\n" +
						j.t("responds without adding comments or explanations")
				}

			} else {
				fmt.Println(fmt.Sprintf("------------------------------------ result (ok): \n\n %s", output))
				/*unusedFuncs, err := j.findUnusedFunctions()
				if err != nil {
					fmt.Println("error lors de la recherche des fonctions inutilisées:", err)
					return
				}

				if err := j.commentUnusedFunctions(unusedFuncs); err != nil {
					fmt.Println("error lors de la mise en commentaire des fonctions:", err)
				}*/

				fmt.Println(j.t("Code output")+": `", output, "`")
				break
			}
		}
	}
	fmt.Println(j.t("End of the job") + "\n\n" + j.t("Restarting the job ?"))
	j.reinitJob()
	return j.run()
}

func (j *job) reinitJob() {
	j.listFunctionsUpdated = []string{}
	j.listFunctionsCreated = []string{}
}

func (j *job) printTestsFuncName() string {
	var funcs string
	j.listFunctionsCreated = removeDuplicates(j.listFunctionsCreated)
	for _, name := range j.listFunctionsCreated {
		funcs += name + "\n\n"
	}
	return funcs
}

// Fonction pour vérifier si une fonction est complète (pas de "..." dans son corps)
func isCompleteFunction(funcDecl *ast.FuncDecl) bool {
	// Utiliser un simple modèle pour vérifier si la fonction est complète
	// Cela peut être ajusté selon le besoin
	if funcDecl.Body != nil {
		// Rechercher les "..." dans le corps de la fonction
		bodyStr := fmt.Sprintf("%#v", funcDecl.Body)
		if !regexp.MustCompile(`\.\.\.`).MatchString(bodyStr) {
			return true
		}
	}
	return false
}

// Fonction pour exécuter le fichier Go et capturer les erreurs.
func (j *job) runGolangFile() (string, error) {
	var cmd *exec.Cmd

	// Vérifier si le fichier est un fichier de test
	if strings.HasSuffix(j.currentFileName, "_test.go") {
		// Si c'est un fichier de test, utiliser "go test"
		cmd = exec.Command("go", "test", "./...")
	} else {
		// Sinon, utiliser "go build" pour vérifier la compilation
		// Cela ne nécessite pas que le package soit main ou qu'il y ait une fonction main
		cmd = exec.Command("go", "build", "-o", "temp_binary")
		// Vous pouvez spécifier le fichier ou le package à construire
		cmd.Args = append(cmd.Args, j.currentFileName)
	}

	// Spécifier le répertoire contenant le fichier et le go.mod
	cmd.Dir = j.fileDir

	// Préparer le buffer pour capturer la sortie
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	// Exécuter la commande
	err := cmd.Run()

	// Nettoyer le binaire temporaire si "go build" a réussi
	if err == nil && !strings.HasSuffix(j.currentFileName, "_test.go") {
		// Supprimer le binaire temporaire généré par "go build"
		removeCmd := exec.Command("rm", "temp_binary")
		removeCmd.Dir = j.fileDir
		removeCmd.Run() // Ignorer les erreurs de suppression
	}

	// Retourner la sortie de la commande et l'erreur éventuelle
	return out.String(), err
}
