package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

	log "github.com/sirupsen/logrus"

	"github.com/ariden/goia/secret"
)

// Conversation represents a conversation with the OpenAI API.
type Conversation struct {
	Messages    []map[string]string `json:"messages"`             // Historique des messages
	Model       string              `json:"model"`                // Modèle utilisé pour la conversation
	Temperature float32             `json:"temperature"`          // Paramètre de température utilisé pour la conversation
	MaxTokens   int                 `json:"max_tokens,omitempty"` // Nombre maximum de tokens à générer
	N           int                 `json:"n,omitempty"`          // The number of responses to generate.
}

type job struct {
	args                  *appArgs
	cache                 *ConfigCache
	fileDir               string
	fileDirSelected       string
	fileName              string
	fileWithVendor        bool
	conversation          Conversation
	listFiles             []string
	currentFileDir        string
	currentFileName       string
	currentSourceFileName string
	currentTestFileName   string
	currentStep           step
	currentSrcSource      []byte
	currentSrcTest        []byte
	repoStructure         string
	lang                  string
	listFunctionsCreated  []string
	listFunctionsUpdated  []string
	maxAttempts           int
	mockOpenAIResponse    bool
	modulePath            string
	openAIApiKey          secret.String
	openAIURL             string
	openAIMaxTokens       int
	source                fileSource
	trad                  Translations
	validateEachStep      bool
}

// newJob create a new job.
func newJob(cache *ConfigCache, fileDir string, args *appArgs) (*job, error) {
	j := job{
		cache:    cache,
		fileDir:  fileDir,
		fileName: "main.go",
		conversation: Conversation{
			// L'ID en soi n'a pas de signification pour l'API OpenAI (l'API ne comprend pas les IDs comme des sessions).
			// Ce qui importe, c'est de maintenir et d'envoyer l'historique des messages dans le champ messages.
			// Cependant, garder un ID constant côté client ou backend vous permet de :
			// Associer un historique à une conversation spécifique :
			// Vous pouvez identifier les messages pertinents pour une conversation donnée et les inclure dans chaque appel.
			// Gérer plusieurs conversations : Si vous avez plusieurs utilisateurs ou sujets, un ID unique pour
			// chaque conversation vous permet de distinguer et de gérer les différents contextes.
			// Assurer la continuité : Tant que vous envoyez l’historique complet des messages pertinents, le modèle
			// d’OpenAI peut répondre de manière contextuelle.
			Model:       cache.rootConfig.OpenAIModel,
			Temperature: cache.rootConfig.OpenAITemperature,
			// MaxTokens:   cache.rootConfig.OpenAIMaxTokens, // limite la taille de la réponse.
		},
		currentStep:           stepDefault,
		currentFileName:       "main.go",
		currentSourceFileName: "main.go",
		currentTestFileName:   "main_test.go",
		currentSrcSource:      []byte{},
		currentSrcTest:        []byte{},
		listFunctionsUpdated:  []string{},
		listFunctionsCreated:  []string{},
		maxAttempts:           cache.rootConfig.MaxAttempts,
		mockOpenAIResponse:    false,
		openAIApiKey:          secret.String(cache.rootConfig.OpenAIKey),
		openAIURL:             cache.rootConfig.OpenAIURL,
		lang:                  "en",
		args:                  args,
		validateEachStep:      cache.rootConfig.ValidateEachStep,
	}

	return &j, nil
}

// run executes the job.
func (j *job) run() error {
	if err := j.updateCache(); err != nil {
		return err
	}

	if err := j.findReposAndSubRepos(); err != nil {
		log.WithError(err).Error("Error finding repos and subrepos: %v", err)
	}

	userPrompt, err := j.promptForQuery()
	if err != nil {
		return err
	}

	prompt := userPrompt

	stepsOrder, err := j.getStepFromFileName()
	if err != nil {
		return err
	}

	for _, stepEntry := range stepsOrder {
		log.Println("current step:", stepEntry.ValidStep)

		j.currentStep = stepEntry.ValidStep
		j.currentFileName = j.fileName

		if j.currentStep == stepVerifyGoPrompt ||
			j.currentStep == stepVerifyTestPrompt ||
			j.currentStep == stepVerifySwaggerPrompt {

			verifyPrompt := j.getPromptForVerifyPrompt(userPrompt)

			log.Infof("\nprompt: "+blue("%s")+"\n\n", verifyPrompt)

			respContent, err := j.callIA(verifyPrompt)
			if err != nil {
				log.WithError(err).Error(j.t("Error checking prompt"))
				return err
			}
			if !j.responseToBool(respContent) {
				log.Info(red(j.t("The question is not a request for Go code")))
				return j.run()
			}
			continue

		} else if j.currentStep == stepProjectStructuring {
			log.Infof("j.currentStep == stepProjectStructuring")
			prompt = j.getPromptToAskProjectStructuring(userPrompt)

		} else if j.currentStep == stepStart {

			prompt = j.prepareGoPrompt(userPrompt)

			fileContent := string(j.currentSrcSource)
			if len(fileContent) > 50 {
				prompt += ".\n\n" + j.t("Here is the Golang code") + " :\n\n" + fileContent
			}

		} else if j.currentStep == stepAddTest {

			prompt = j.getPromptToAskTestsCreation()

		} else if j.currentStep == stepStartTest {

			prompt = j.getPromptToAskTestCorrection()

		} else {
			prompt = j.t(stepEntry.Prompt)

			fileContent := string(j.currentSrcSource) // ? a modifier ?
			prompt += "\n\n" + string(fileContent)
		}

		if j.currentStep == stepAddTest {
			j.currentFileName = j.currentTestFileName
		}

		for attempt := 1; attempt <= j.maxAttempts; attempt++ {
			log.Println("attempt:", attempt)
			log.Infof("\nprompt: "+blue("%s")+"\n\n", prompt)

			code, err := j.callIA(prompt)
			if err != nil {
				log.WithError(err).Error(j.t("Error generating code"))
				return err
			}

			log.Infof("API response:\n\n"+green("\"%s\"")+"\n\n", code)

			var output string

			if j.currentStep == stepProjectStructuring {
				log.Println("project structuring")
				parseListFolders := j.parseListFolders(code)
				j.createFoldersFromList(parseListFolders)
				if err := j.findReposAndSubRepos(); err != nil {
					log.WithError(err).Error("Error finding repos and subrepos: %v", err)
				}
				break

			} else if j.currentStep == stepStart {

				var mustBreak, mustContinue bool
				prompt, output, mustBreak, mustContinue, err = j.runStepStart(stepEntry, code)
				if err != nil {
					return err
				}
				if mustBreak {
					break
				}
				if mustContinue {
					continue
				}

			} else {

				fileToModify, codeReceived := j.splitFileNameAndCode(code)
				log.Infof(j.t("file to modify") + ": " + green(fileToModify) + "\n\n")

				if err := j.fixCodeAndWriteFile(fileToModify, codeReceived); err != nil {
					log.WithError(err).Error(j.t("Error fixing code and writing file"))
					return err
				}

				var mustBreak, mustContinue bool
				prompt, output, mustBreak, mustContinue, err = j.runContentForFile(stepEntry)
				if err != nil {
					return err
				}
				if mustBreak {
					break
				}
				if mustContinue {
					continue
				}

				log.Infof("------------------------------------ result (ok): \n\n %s", output)

				fmt.Println(j.t("Code output")+": `", output, "`")
			}

			log.Infof("------------------------------------ result (ok): \n\n %s", output)
			/*unusedFuncs, err := j.findUnusedFunctions()
			if err != nil {
				fmt.Println("error lors de la recherche des fonctions inutilisées:", err)
				return
			}

			if err := j.commentUnusedFunctions(unusedFuncs); err != nil {
				fmt.Println("error lors de la mise en commentaire des fonctions:", err)
			}*/

			log.Info(j.t("Code output")+": `", output, "`")
			break

		}
	}

	log.Info(j.t("End of the job") + "\n\n" + j.t("Restarting the job ?"))
	j.reinJob()
	return j.run()
}

func (j *job) runStepStart(stepEntry StepWithError, code string) (prompt, output string, mustBreak, mustContinue bool, err error) {
	filesAndCode := j.splitFilesAndCode(code)

	for file, codeReceived := range filesAndCode {
		j.currentFileName = file
		j.currentSourceFileName = file
		j.fileName = file

		if err = j.createAndWriteFile(file); err != nil {
			log.WithError(err).Error(j.t("Error updating file"))
			return
		}

		if err = j.fixCodeAndWriteFile(file, codeReceived); err != nil {
			log.WithError(err).Error(j.t("Error fixing code and writing file"))
			return
		}
	}

	j.waitingPrompt()

	// we move on to executing the files.
	for file, codeReceived := range filesAndCode {
		j.listFiles = append(j.listFiles, file)
		j.currentFileName = file
		j.currentSourceFileName = file
		j.fileName = file

		var testFileName string
		testFileName, err = j.getTestFilename()
		if err != nil {
			return
		}

		log.Infof("create or update test fileName: %s", testFileName)
		j.currentTestFileName = testFileName

		for attempt := 1; attempt <= j.maxAttempts; attempt++ {
			if attempt != 1 {
				log.Infof("\nprompt: "+blue("%s")+"\n\n", prompt)

				codeReceived, err = j.callIA(prompt)
				if err != nil {
					log.WithError(err).Error(j.t("Error generating code"))
					return
				}

				log.Infof("API response:\n\n"+green("\"%s\"")+"\n\n", codeReceived)

				if err = j.fixCodeAndWriteFile(file, codeReceived); err != nil {
					log.WithError(err).Error(j.t("Error fixing code and writing file"))
					return
				}
			}

			prompt, output, mustBreak, mustContinue, err = j.runContentForFile(stepEntry)
			if err != nil {
				return "", "", false, false, err
			}
			if mustBreak {
				break
			}
			if mustContinue {
				continue
			}

			// dans le cas ou tout se passe bien avec les tests, on ne fait plus rien
			if j.currentFileName == j.currentTestFileName {
				log.Println(j.t("Code output")+": `", output, "`")
				break
			}

			// on lance la génération des tests
			j.currentFileName = j.currentTestFileName
			attempt = 0

			src, err := j.createNewTestFile()
			if err != nil {
				log.WithError(err).Error(j.t("Error creating test file"))
				break
			}
			j.currentSrcTest = src

			prompt = j.getPromptToAskTestsCreation()
		}
	}
	return
}

// fixCodeAndWriteFile fixes the code and writes the file.
func (j *job) fixCodeAndWriteFile(fileToModify, code string) (err error) {
	var codeModified []byte
	codeModified, err = j.stepFixCode(fileToModify, code)
	if err != nil {
		log.WithError(err).Error(j.t("Error to get code modified"))
		return
	}

	if err = j.writeFile(fileToModify, codeModified); err != nil {
		log.WithError(err).Error(j.t("Error updating file"))
		return
	}

	return
}

// createAndWriteFile creates and writes the file.
func (j *job) createFoldersFromList(paths []string) {
	for _, path := range paths {
		fullPath := filepath.Join(j.fileDir, path)
		if filepath.Ext(path) == "" { // Vérifie si c'est un dossier
			err := os.MkdirAll(fullPath, os.ModePerm)
			if err != nil {
				fmt.Printf("Erreur lors de la création du dossier %s : %v\n", fullPath, err)
				continue
			}
			fmt.Printf("Dossier créé : %s\n", fullPath)
		} else { // Si c'est un fichier
			// S'assurer que le dossier parent existe
			parentDir := filepath.Dir(fullPath)
			err := os.MkdirAll(parentDir, os.ModePerm)
			if err != nil {
				fmt.Printf("Erreur lors de la création du dossier parent %s : %v\n", parentDir, err)
				continue
			}

			// Créer le fichier si nécessaire
			file, err := os.Create(fullPath)
			if err != nil {
				fmt.Printf("Erreur lors de la création du fichier %s : %v\n", fullPath, err)
				continue
			}
			if err := file.Close(); err != nil {
				fmt.Printf("Erreur lors de la fermeture du fichier %s : %v\n", fullPath, err)
			}
			fmt.Printf("Fichier créé : %s\n", fullPath)
			j.listFiles = append(j.listFiles, fullPath)
		}
	}
}

// runContentForFile executes the content of the file.
func (j *job) runContentForFile(stepEntry StepWithError) (prompt, output string, mustBreak, mustContinue bool, err error) {

	if err = j.updateGoMod(); err != nil {
		log.WithError(err).Error(j.t("Error configuring Go modules"))
		return
	}

	if err = j.fixImports(); err != nil {
		log.WithError(err).Error(j.t("Error correcting imports"))
		return
	}

	output, err = j.runGolangFile()
	if err != nil {
		log.Infof("------------------------------------ code result (failed): \n\n %s", output)
		j.currentStep = stepEntry.ErrorStep

		var unusedImports []string
		unusedImports, err = j.extractUnusedImports(output)
		if err != nil {
			log.WithError(err).Error(j.t("Error when extract unused imports"))
			return
		}

		if len(unusedImports) > 0 {
			log.Infof("------------------------------------ fix unused imports: \n\n%v", magenta(unusedImports))
			err = j.removeUnusedImports(unusedImports, j.currentSourceFileName)
			if err != nil {
				fmt.Println(j.t("Error deleting imports")+":", err)
				return
			}
			log.Info("------------------------------------ imports fixed")
			output, err = j.runGolangFile()
		}

		if err != nil {
			log.Infof("------------------------------------ code result (failed): \n\n %s", output)

			var funcCode string
			funcCode, err = j.extractErrorForPrompt(output)
			if err != nil {
				log.WithError(err).Error(j.t("Error when extract errors from prompt"))
				return
			}
			fmt.Println(j.t("Runtime error"), output)
			// Mise à jour de l'instruction pour l'API en ajoutant le retour d'erreur.
			prompt = j.t("Fix the following code that generated an error") + ":\n\n" + funcCode + "\n\n" +
				j.t("Error") + " : " + output + "\n\n" +
				j.t("responds without adding comments or explanations") + "\n\n" +
				j.t("Generates a concise response that specifies the file to modify in the form: \"MODIFY: <function or section name> (source file, not test file)\"") + "." +
				j.t("Then provide the corrected code in the form: \"CODE: <corrected code>\"") + "."

			mustContinue = true
			return
		}
	}

	if j.isTestFile(j.currentFileName) {

		output, err = j.runGolangTestFile()
		if err != nil {
			fmt.Println(fmt.Sprintf("------------------------------------ test result (failed): \n\n %s", output))
			j.currentStep = stepEntry.ErrorStep

			if j.currentStep == stepAddTestError {
				prompt, err = j.stepAddTestErrorProcessPrompt(output)
				if err != nil {
					return
				}

			} else {
				var funcCode string
				funcCode, err = j.extractErrorForPrompt(output)
				if err != nil {
					fmt.Println("Erreur:", err)
					return
				}
				fmt.Println(j.t("Runtime error"), output)
				// Mise à jour de l'instruction pour l'API en ajoutant le retour d'erreur
				prompt = j.t("Fix the following code that generated an error") + ":\n\n" + funcCode + "\n\n" +
					j.t("Error") + " : " + output + "\n\n" +
					j.t("responds without adding comments or explanations")
			}
			mustContinue = true
			return
		}
	}
	return
}

// reinJob resets the job values.
func (j *job) reinJob() {
	j.listFunctionsUpdated = []string{}
	j.listFunctionsCreated = []string{}
}

// printTestsFuncName returns the names of the functions to test.
func (j *job) printTestsFuncName() string {
	var funcs string
	j.listFunctionsCreated = removeDuplicates(j.listFunctionsCreated)
	for _, name := range j.listFunctionsCreated {
		funcs += name + "\n\n"
	}
	return funcs
}

// isCompleteFunction checks if the function is complete.
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

// runGolangFile executes the Go file.
func (j *job) runGolangFile() (string, error) {
	var cmd *exec.Cmd
	// Sinon, utiliser "go build" pour vérifier la compilation
	// Cela ne nécessite pas que le package soit main ou qu'il y ait une fonction main
	cmd = exec.Command("go", "build", "-o", "temp_binary")
	// Vous pouvez spécifier le fichier ou le package à construire
	cmd.Args = append(cmd.Args, j.currentSourceFileName)

	// Spécifier le répertoire contenant le fichier et le go.mod
	cmd.Dir = j.fileDir

	// Préparer le buffer pour capturer la sortie
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	// Exécuter la commande
	err := cmd.Run()

	// Nettoyer le binaire temporaire si "go build" a réussi
	if err == nil {
		// Supprimer le binaire temporaire généré par "go build"
		removeCmd := exec.Command("rm", "temp_binary")
		removeCmd.Dir = j.fileDir
		removeCmd.Run()
	}

	return out.String(), err
}

// runGolangTestFile runs the Go test file.
func (j *job) runGolangTestFile() (string, error) {
	if j.currentTestFileName == "" {
		return "", nil
	}

	cmd := exec.Command("go", "test", "./...")

	cmd.Dir = j.fileDir

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	err := cmd.Run()

	return out.String(), err
}
