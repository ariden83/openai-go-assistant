package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/joho/godotenv"
	"go/ast"
	"go/format"
	"go/parser"
	"go/printer"
	"go/token"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const ErrorContextLines = 10 // Nombre de lignes de contexte avant et après

type job struct {
	apiKey      string
	maxAttempts int
	fileDir     string
	fileName    string
	step        step
}

func main() {
	// Chargement des variables d'environnement depuis le fichier .env
	if err := godotenv.Load(); err != nil {
		log.Fatal("Erreur de chargement du fichier .env")
	}

	j := job{
		apiKey:      os.Getenv("OPENAI_API_KEY"),
		maxAttempts: 2,
		fileDir:     "./test",
		fileName:    "generated_code.go",
		step:        stepStart,
	}

	// Instruction initiale pour l'API
	prompt := "Génère uniquement du code Golang pour une fonction qui affiche 'Hello, world!' sans commentaire ou explication."

	for attempt := 1; attempt <= j.maxAttempts; attempt++ {
		fmt.Println(fmt.Sprintf("prompt: %s", prompt))
		// Appel API pour générer du code
		code, err := j.generateGolangCode(prompt)
		if err != nil {
			fmt.Println("Erreur lors de la génération de code:", err)
			return
		}

		if j.step == stepStart {
			// Écriture du code dans un fichier
			if err = j.stepStart(code); err != nil {
				fmt.Println("Erreur lors de l'écriture du fichier:", err)
				return
			}
		} else if err = j.stepFixCode(code); err != nil {
			fmt.Println("Erreur lors de l'update du fichier:", err)
			return
		}

		// Exécuter go mod init et go mod tidy
		if err = setupGoMod(j.fileDir); err != nil {
			fmt.Println("Erreur lors de la configuration des modules Go:", err)
			return
		}

		// Exécution de goimports pour corriger les imports manquants
		if err = fixImports(j.fileDir + "/" + j.fileName); err != nil {
			fmt.Println("Erreur lors de la correction des imports:", err)
			return
		}

		// Exécution du fichier Go
		output, err := runGolangFile(j.fileDir + "/" + j.fileName)
		if err != nil {

			j.step = stepError

			errorLine, err := extractLineNumber(output)
			if err != nil {
				fmt.Println("Erreur:", err)
				return
			}
			fmt.Println("Numéro de ligne de l'erreur :", errorLine)

			funcCode, err := extractFunctionFromLine(j.fileDir+"/"+j.fileName, errorLine)
			if err != nil {
				log.Fatalf("Erreur lors de l'extraction de la fonction: %v", err)
			}

			fmt.Println("Erreur d'exécution:", output)
			// Mise à jour de l'instruction pour l'API en ajoutant le retour d'erreur
			prompt = "Corrige le code suivant qui a généré une erreur:\n\n" + funcCode + "\n\nErreur : " + output + "\n\nsans ajouter de commentaires ou explications"

		} else {
			if j.step != stepOptimize {
				attempt = 1
				j.step = stepOptimize
			}
			fmt.Println("Sortie du code:", output)
			break
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

// Fonction pour extraire toutes les fonctions dans un code donné
func (j *job) extractFunctionsFromCode(code string) ([]*ast.FuncDecl, error) {
	fs := token.NewFileSet()
	node, err := parser.ParseFile(fs, "generated_code.go", code, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de l'analyse du code: %v", err)
	}

	var funcs []*ast.FuncDecl
	for _, decl := range node.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if ok {
			funcs = append(funcs, funcDecl)
		}
	}
	return funcs, nil
}

// Fonction pour extraire les imports d'un code Go sous forme de chaîne
// Fonction pour extraire les imports d'un code Go sous forme de chaîne
func extractImportsFromCode(code string) ([]string, error) {
	// Parser le fichier Go pour en extraire l'AST
	fs := token.NewFileSet()
	node, err := parser.ParseFile(fs, "", []byte(code), parser.ImportsOnly)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de l'analyse des imports: %v", err)
	}

	var imports []string
	for _, imp := range node.Imports {
		imports = append(imports, strings.Trim(imp.Path.Value, `"`))
	}
	return imports, nil
}

// Fonction pour ajouter un import à un bloc d'import existant
func addImport(existingImports []string, newImport string) []string {
	// Vérifier si l'import existe déjà
	for _, imp := range existingImports {
		if imp == newImport {
			return existingImports // Ne rien faire si l'import existe déjà
		}
	}
	// Ajouter l'import à la liste
	return append(existingImports, newImport)
}

func (j *job) replaceCompleteFunctionsInFile(openAIResponse string) error {
	// Extraire toutes les fonctions du code OpenAI
	funcs, err := j.extractFunctionsFromCode(openAIResponse)
	if err != nil {
		return fmt.Errorf("erreur lors de l'extraction des fonctions: %v", err)
	}

	// Extraire les imports proposés par OpenAI
	openAIImports, err := extractImportsFromCode(openAIResponse)
	if err != nil {
		return fmt.Errorf("erreur lors de l'extraction des imports OpenAI: %v", err)
	}

	// Lire le fichier Go existant
	data, err := ioutil.ReadFile(j.fileDir + "/" + j.fileName)
	if err != nil {
		return fmt.Errorf("erreur lors de la lecture du fichier: %v", err)
	}

	// Extraire les imports existants dans le fichier
	existingImports, err := extractImportsFromCode(string(data))
	if err != nil {
		return fmt.Errorf("erreur lors de l'extraction des imports existants: %v", err)
	}

	// Ajouter les imports OpenAI manquants aux imports existants
	for _, newImport := range openAIImports {
		existingImports = addImport(existingImports, newImport)
	}

	// Créer un fichier tokeniseur
	fs := token.NewFileSet()
	node, err := parser.ParseFile(fs, j.fileDir+"/"+j.fileName, data, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("erreur lors de l'analyse du fichier: %v", err)
	}

	// Préparer un buffer pour le fichier modifié
	var modifiedFile bytes.Buffer

	// Récupérer et réutiliser le package existant
	packageDecl := node.Name
	fmt.Println("Package existant :", packageDecl.Name)

	// Ajouter la déclaration du package au fichier modifié
	fmt.Fprintf(&modifiedFile, "package %s", packageDecl.Name)
	modifiedFile.WriteString("\n\n")
	// Mettre à jour les imports dans le fichier
	// Recréer la section d'import avec les nouveaux imports
	importDecl := &ast.GenDecl{
		Tok:   token.IMPORT,
		Specs: make([]ast.Spec, 0, len(existingImports)),
	}

	for _, imp := range existingImports {
		importDecl.Specs = append(importDecl.Specs, &ast.ImportSpec{
			Path: &ast.BasicLit{Value: fmt.Sprintf("%q", imp)},
		})
	}

	// Remplacer la section des imports dans le fichier
	node.Decls[0] = importDecl // Remplacer la déclaration d'import existante

	modifiedFile.WriteString("\n\n")
	// Pour chaque déclaration dans le fichier, traiter les fonctions
	for _, decl := range node.Decls[1:] {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if ok {
			// Si la fonction est complète (pas de "..." dans son code), la remplacer
			if isCompleteFunction(funcDecl) {
				// Chercher la fonction correspondante dans la réponse d'OpenAI
				for _, openAIFunc := range funcs {
					// Comparer les noms des fonctions (ou utiliser d'autres critères d'identification)
					if funcDecl.Name.Name == openAIFunc.Name.Name {
						// Remplacer le corps de la fonction existante par celui d'OpenAI
						funcDecl.Body = openAIFunc.Body
						break
					}
				}
			}
		}
	}

	// Ajouter toutes les déclarations modifiées au fichier modifié
	for _, decl := range node.Decls {
		if err = printer.Fprint(&modifiedFile, fs, decl); err != nil {
			return fmt.Errorf("erreur lors de l'écriture de la déclaration modifiée: %v", err)
		}
		modifiedFile.WriteString("\n\n")
	}

	// Affichage du code généré juste avant le formatage
	fmt.Println("Code généré avant formatage:\n", modifiedFile.String())

	// Appliquer un formatage Go standard au code généré
	formattedCode, err := format.Source(modifiedFile.Bytes())
	if err != nil {
		return fmt.Errorf("erreur lors du formatage du fichier: %v", err)
	}

	// Sauvegarder le fichier modifié
	return ioutil.WriteFile(j.fileDir+"/"+j.fileName, formattedCode, 0644)
}

// Fonction pour envoyer une requête à l'API OpenAI
func (j *job) generateGolangCode(prompt string) (string, error) {

	requestBody := map[string]interface{}{
		"model": "gpt-3.5-turbo",
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"temperature": 0.2,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+j.apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var response APIResponse
	if err = json.Unmarshal(body, &response); err != nil {
		return "", err
	}

	response.Error = nil

	if j.step == stepStart {
		response.Choices = []Choice{
			{
				Message: Message{
					Content: `package main

import (
	"encoding/json"
	"fmt"
	"log"
)

// Définition des structures correspondant au JSON de réponse valide
type Choice struct {
	Index        int    ` + "`" + `json:"index"` + "`" + `
	Message      Message ` + "`" + `json:"message"` + "`" + `
	FinishReason string ` + "`" + `json:"finish_reason"` + "`" + `
}

type Message struct {
	Role    string ` + "`" + `json:"role"` + "`" + `
	Content string ` + "`" + `json:"content"` + "`" + `
}

type Usage struct {
	PromptTokens     int ` + "`" + `json:"prompt_tokens"` + "`" + `
	CompletionTokens int ` + "`" + `json:"completion_tokens"` + "`" + `
	TotalTokens      int ` + "`" + `json:"total_tokens"` + "`" + `
}

type APIResponse struct {
	ID      string   ` + "`" + `json:"id"` + "`" + `
	Object  string   ` + "`" + `json:"object"` + "`" + `
	Created int64    ` + "`" + `json:"created"` + "`" + `
	Model   string   ` + "`" + `json:"model"` + "`" + `
	Choices []Choice ` + "`" + `json:"choices"` + "`" + `
	Usage   Usage    ` + "`" + `json:"usage"` + "`" + `
}

func main() {
	// Exemple de JSON de réponse valide
	responseJSON := ` + "`" + `
			{
				"id": "chatcmpl-12345",
				"object": "chat.completion",
				"created": 1689200300,
				"model": "gpt-3.5-turbo",
				"choices": [
			{
				"index": 0,
				"message": {
				"role": "assistant",
				"content": "This is a test response from the assistant."
			},
				"finish_reason": "stop"
			}
			],
				"usage": {
				"prompt_tokens": 10,
				"completion_tokens": 20,
				"total_tokens": 30
			}
			}` + "`" + `

	// Parse le JSON de réponse
	var apiResponse APIResponse
	err := json.Unmarshal([]byte(responseJSON), &apiResponse)
	if err != nil {
		log.Fatalf("Erreur lors du parsing de la réponse JSON: %v", err)
	}

	data := bytes.NewBufferString(` + "`" + `{"hello":"world","answer":42}` + "`" + `)
	req, _ := http.NewRequest("PUT", "http://www.example.com/abc/def.ghi?jlk=mno&pqr=stu", data)
	req.Header.Set("Content-Type", "application/json")

	command, _ := http2curl.GetCurlCommand(req)
	fmt.Println(command)

	// Affichage des informations extraites
	fmt.Println("ID:", apiResponse.ID)
	fmt.Println("Model:", apiResponse.Model)
	fmt.Println("Contenu du message:", apiResponse.Choices[0].Message.Content)
	fmt.Println("Nombre total de tokens utilisés:", apiResponse.Usage.TotalTokens)

	writeTest()
}

func writeTest() {
		fmt.Println("toto")
}
`,
				},
			},
		}

	} else if j.step == stepError {
		response.Choices = []Choice{
			{
				Message: Message{
					Content: `package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/moul/http2curl" // Importation de http2curl
)

type APIResponse struct {
	ID       string ` + "`" + `json:"id"` + "`" + `
	Object   string ` + "`" + `json:"object"` + "`" + `
	Created  int    ` + "`" + `json:"created"` + "`" + `
	Model    string ` + "`" + `json:"model"` + "`" + `
	Choices  []struct {
		Index   int ` + "`" + `json:"index"` + "`" + `
		Message struct {
			Role    string ` + "`" + `json:"role"` + "`" + `
			Content string ` + "`" + `json:"content"` + "`" + `
		} ` + "`" + `json:"message"` + "`" + `
		FinishReason string ` + "`" + `json:"finish_reason"` + "`" + `
	} ` + "`" + `json:"choices"` + "`" + `
	Usage struct {
		PromptTokens     int ` + "`" + `json:"prompt_tokens"` + "`" + `
		CompletionTokens int ` + "`" + `json:"completion_tokens"` + "`" + `
		TotalTokens      int ` + "`" + `json:"total_tokens"` + "`" + `
	} ` + "`" + `json:"usage"` + "`" + `
}

func main() {
	// JSON de réponse simulée
	responseJSON := ` + "`" + `
				{
					"id": "chatcmpl-12345",
					"object": "chat.completion",
					"created": 1689200300,
					"model": "gpt-3.5-turbo",
					"choices": [
				{
					"index": 0,
					"message": {
					"role": "assistant",
					"content": "This is a test response from the assistant."
				},
					"finish_reason": "stop"
				}
				],
					"usage": {
					"prompt_tokens": 10,
					"completion_tokens": 20,
					"total_tokens": 30
				}
				}` + "`" + `

	// Parse le JSON de réponse
	var apiResponse APIResponse
	err := json.Unmarshal([]byte(responseJSON), &apiResponse)
	if err != nil {
		log.Fatalf("Erreur lors du parsing de la réponse JSON: %v", err)
	}

	// Préparer la requête HTTP
	data := bytes.NewBufferString(` + "`" + `{"hello":"world","answer":42}` + "`" + `)
	req, _ := http.NewRequest("PUT", "http://www.example.com/abc/def.ghi?jlk=mno&pqr=stu", data)
	req.Header.Set("Content-Type", "application/json")

	// Utiliser http2curl pour obtenir la commande cURL correspondante
	command, _ := http2curl.GetCurlCommand(req)
	fmt.Println(command)

	// Afficher les informations de la réponse API
	fmt.Println("ID:", apiResponse.ID)
	fmt.Println("Model:", apiResponse.Model)
	fmt.Println("Contenu du message:", apiResponse.Choices[0].Message.Content)
	fmt.Println("Nombre total de tokens utilisés:", apiResponse.Usage.TotalTokens)
}
`,
				},
			},
		}
	}

	if response.Error != nil {
		return "", fmt.Errorf("%s: %s", response.Error.Code, response.Error.Message)
	}

	if len(response.Choices) > 0 {
		return response.Choices[0].Message.Content, nil
	}

	return "", fmt.Errorf("could not parse API response")
}

// Fonction pour écrire le code dans un fichier .go
func (j *job) writeCodeToFile(code string) error {
	return ioutil.WriteFile(j.fileDir+"/"+j.fileName, []byte(code), 0644)
}

func fixImports(filename string) error {
	// Commande pour exécuter goimports
	cmd := exec.Command("goimports", "-w", filename)

	// Exécution de la commande
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("erreur lors de l'exécution de goimports: %v - %s", err, out.String())
	}

	return nil
}

// Fonction pour exécuter le fichier Go et capturer les erreurs
func runGolangFile(filename string) (string, error) {
	cmd := exec.Command("go", "run", filename)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return out.String(), err
}

// Fonction pour exécuter `go mod init` et `go mod tidy`
func setupGoMod(dir string) error {
	// Initialisation du module si le fichier go.mod n'existe pas
	goModPath := filepath.Join(dir, "go.mod")
	if _, err := os.Stat(goModPath); os.IsNotExist(err) {
		cmdInit := exec.Command("go", "mod", "init", "generated_code_module")
		cmdInit.Dir = dir // Définit le répertoire de travail pour `go mod init`
		if output, err := cmdInit.CombinedOutput(); err != nil {
			return fmt.Errorf("erreur lors de l'initialisation du module: %v - %s", err, output)
		}
	}

	// Exécution de go mod tidy pour installer les dépendances
	cmdTidy := exec.Command("go", "mod", "tidy")
	cmdTidy.Dir = dir // Définit le répertoire de travail pour `go mod tidy`
	if output, err := cmdTidy.CombinedOutput(); err != nil {
		return fmt.Errorf("erreur lors de l'exécution de go mod tidy: %v - %s", err, output)
	}

	cmdVendor := exec.Command("go", "mod", "vendor")
	cmdVendor.Dir = dir // Définit le répertoire de travail pour `go mod vendor`
	if output, err := cmdVendor.CombinedOutput(); err != nil {
		return fmt.Errorf("erreur lors de l'exécution de go mod vendor: %v - %s", err, output)
	}

	return nil
}

func extractLineNumber(errorMessage string) (int, error) {
	// Expression régulière pour capturer le numéro de ligne
	re := regexp.MustCompile(`:(\d+):\d+`)

	// Rechercher la correspondance dans le message d'erreur
	matches := re.FindStringSubmatch(errorMessage)
	if len(matches) < 2 {
		return 0, fmt.Errorf("numéro de ligne non trouvé dans le message d'erreur : %s", errorMessage)
	}

	// Convertir le numéro de ligne en entier
	lineNumber, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, fmt.Errorf("erreur de conversion du numéro de ligne: %v", err)
	}

	return lineNumber, nil
}

func extractFunctionFromLine(filename string, lineNumber int) (string, error) {
	// Lire le fichier Go
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return "", fmt.Errorf("erreur lors de la lecture du fichier: %v", err)
	}

	// Créer un fichier tokeniseur
	fs := token.NewFileSet()
	node, err := parser.ParseFile(fs, filename, data, parser.ParseComments)
	if err != nil {
		return "", fmt.Errorf("erreur lors de l'analyse du fichier: %v", err)
	}

	// Parcours de l'AST (arbre syntaxique abstrait) pour trouver la fonction
	for _, decl := range node.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}

		// Vérifier si la ligne d'erreur est dans cette fonction
		funcStartLine := fs.Position(funcDecl.Pos()).Line
		funcEndLine := fs.Position(funcDecl.End()).Line

		// Si la ligne d'erreur est dans cette fonction, retourner le code de la fonction
		if lineNumber >= funcStartLine && lineNumber <= funcEndLine {
			var functionCode bytes.Buffer
			// Utiliser le printer Go pour imprimer le code de la fonction
			err := printer.Fprint(&functionCode, fs, funcDecl)
			if err != nil {
				return "", fmt.Errorf("erreur lors de l'impression de la fonction: %v", err)
			}
			return functionCode.String(), nil
		}
	}

	return "", fmt.Errorf("fonction contenant la ligne %d non trouvée", lineNumber)
}
