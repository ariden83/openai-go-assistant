package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/ariden/openai-go-assistant/secret"
	"github.com/joho/godotenv"
)

type job struct {
	apiKey             secret.String
	maxAttempts        int
	fileDir            string
	fileName           string
	fileWithVendor     bool
	currentFileName    string
	currentStep        step
	mockOpenAIResponse bool
	openAIModel        string
	openAIURL          string
	openAITemperature  float64
}

func main() {
	// Chargement des variables d'environnement depuis le fichier .env
	if err := godotenv.Load(); err != nil {
		log.Fatal("Erreur de chargement du fichier .env")
	}

	// Récupération du modèle OpenAI avec une valeur par défaut
	model := os.Getenv("OPENAI_MODEL")
	if model == "" {
		model = "gpt-3.5-turbo"
	}

	fileDir := os.Getenv("FILE_PATH")
	if fileDir == "" {
		fileDir = "./test"
	}

	j := job{
		apiKey:             secret.String(os.Getenv("OPENAI_API_KEY")),
		maxAttempts:        4,
		fileDir:            fileDir,
		fileName:           "main.go",
		currentStep:        stepStart,
		currentFileName:    "main.go",
		mockOpenAIResponse: true,
		openAIModel:        model,
		openAIURL:          "https://api.openai.com/v1/chat/completions",
		openAITemperature:  0.2,
	}

	log.Println("Configuration du job:", j)

	filesFound, err := j.loadFilesFromFolder()
	if err != nil {
		fmt.Println("No files found in the specified folder, a main.go file will be created", err)
		if err := j.promptNoFilesFoundCreateANewFile(); err != nil {
			return
		}
	} else {
		if err := j.promptSelectAFileOrCreateANewOne(filesFound); err != nil {
			return
		}
	}

	// Exécuter go mod init et go mod tidy
	if err := j.setupGoMod(); err != nil {
		fmt.Println("Erreur lors de la configuration des modules Go:", err)
		return
	}

	// Instruction initiale pour l'API
	prompt := "Génère uniquement du code Golang pour une fonction qui affiche 'Hello, world!', sans commentaire ou explication."

	for _, stepEntry := range stepsOrder {
		j.currentStep = stepEntry.ValidStep
		j.currentFileName = j.fileName

		if j.currentStep != stepStart {
			prompt = stepEntry.Prompt

			fileContent, err := j.readFileContent()
			if err != nil {
				fmt.Println("Erreur lors de la récupération du nom du contenu du fichier courant :", err)
				return
			}
			prompt += "\n\n" + fileContent
		}

		if j.currentStep == stepAddTest {
			testFileName, err := j.getTestFilename()
			if err != nil {
				fmt.Println("Erreur lors de la récupération du nom du fichier de test :", err)
				return
			}
			j.currentFileName = testFileName
		}

		for attempt := 1; attempt <= j.maxAttempts; attempt++ {

			fmt.Println(fmt.Sprintf("***************************************(start)\nprompt: %s\n\n*************************************** (end)", prompt))

			code, err := j.generateGolangCode(prompt)
			if err != nil {
				fmt.Println("Erreur lors de la génération de code:", err)
				return
			}

			if j.currentStep == stepAddTest {
				// Écriture du code dans un fichier
				if err = j.stepStart(code); err != nil {
					fmt.Println("Erreur lors de l'écriture du fichier:", err)
					return
				}
			}

			if err = j.stepFixCode(code); err != nil {
				fmt.Println("Erreur lors de l'update du fichier:", err)
				return
			}

			// Exécuter go mod init et go mod tidy
			if err = j.updateGoMod(); err != nil {
				fmt.Println("Erreur lors de la configuration des modules Go:", err)
				return
			}

			// Exécution de goimports pour corriger les imports manquants
			if err = j.fixImports(); err != nil {
				fmt.Println("Erreur lors de la correction des imports:", err)
				return
			}

			// Exécution du fichier Go
			output, err := j.runGolangFile()
			if err != nil {
				fmt.Println(fmt.Sprintf("------------------------------------ result (failed): \n\n %s", output))
				j.currentStep = stepEntry.ErrorStep

				if j.currentStep == stepAddTestError {

					getFailedTests, err := j.getFailedTests(output)
					if err != nil {
						fmt.Println(fmt.Sprintf("erreur lors de la récupération des tests échoués: %v", err))
						return
					}

					if getFailedTests == nil {
						fmt.Println("Aucun test n'a échoué")
						return
					}

					testCode, err := j.getTestCode(getFailedTests)
					if err != nil {
						fmt.Println(fmt.Sprintf("erreur lors de la récupération du code des tests échoués: %v", err))
						return
					}

					prompt = "Les tests suivants \n\n" + testCode + "\n\n ont retourné les erreurs suivantes: \n\nErreur : " + output + "\n\nrépond sans ajouter de commentaires ou explications"

				} else {
					funcCode, err := j.extractErrorForPrompt(output)
					if err != nil {
						fmt.Println("Erreur:", err)
						return
					}
					fmt.Println("Erreur d'exécution:", output)
					// Mise à jour de l'instruction pour l'API en ajoutant le retour d'erreur
					prompt = "Corrige le code suivant qui a généré une erreur:\n\n" + funcCode + "\n\nErreur : " + output + "\n\nrépond sans ajouter de commentaires ou explications"
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

				fmt.Println("Sortie du code: `", output, "`")
				break
			}
		}
	}
}

func (j *job) stepStart(code string) error {
	// Écriture du code dans un fichier
	return j.writeCodeToFile(code)
}

func (j *job) stepFixCode(code string) error {
	return j.replaceCompleteFunctionsInFile(code)
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
