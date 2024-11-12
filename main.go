package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/printer"
	"go/token"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/joho/godotenv"

	"github.com/ariden/openai-go-assistant/secret"
)

type job struct {
	apiKey             secret.String
	maxAttempts        int
	fileDir            string
	fileName           string
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

	j := job{
		apiKey:             secret.String(os.Getenv("OPENAI_API_KEY")),
		maxAttempts:        4,
		fileDir:            "./test",
		fileName:           "generated_code.go",
		currentStep:        stepStart,
		currentFileName:    "generated_code.go",
		mockOpenAIResponse: true,
		openAIModel:        model,
		openAIURL:          "https://api.openai.com/v1/chat/completions",
		openAITemperature:  0.2,
	}

	log.Println("Configuration du job:", j)

	// Exécuter go mod init et go mod tidy
	if err := j.setupGoMod(); err != nil {
		fmt.Println("Erreur lors de la configuration des modules Go:", err)
		return
	}

	// Instruction initiale pour l'API
	prompt := "Génère uniquement du code Golang pour une fonction qui affiche 'Hello, world!' sans commentaire ou explication."

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

			fmt.Println(fmt.Sprintf("***************************************\nprompt: %s", prompt))

			// Appel API pour générer du code
			code, err := j.generateGolangCode(prompt)
			if err != nil {
				fmt.Println("Erreur lors de la génération de code:", err)
				return
			}

			if j.currentStep == stepStart || j.currentStep == stepAddTest {
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
				j.currentStep = stepEntry.ErrorStep

				errorLine, err := extractLineNumber(output)
				if err != nil {
					fmt.Println("Erreur:", err)
					return
				}
				fmt.Println("Numéro de ligne de l'erreur :", errorLine)

				funcCode, err := j.extractFunctionFromLine(errorLine)
				if err != nil {
					log.Fatalf("Erreur lors de l'extraction de la fonction: %v", err)
				}

				fmt.Println("Erreur d'exécution:", output)
				// Mise à jour de l'instruction pour l'API en ajoutant le retour d'erreur
				prompt = "Corrige le code suivant qui a généré une erreur:\n\n" + funcCode + "\n\nErreur : " + output + "\n\nsans ajouter de commentaires ou explications"

			} else {

				unusedFuncs, err := j.findUnusedFunctions()
				if err != nil {
					fmt.Println("Erreur lors de la recherche des fonctions inutilisées:", err)
					return
				}

				if err := j.commentUnusedFunctions(unusedFuncs); err != nil {
					fmt.Println("Erreur lors de la mise en commentaire des fonctions:", err)
				}

				fmt.Println("Sortie du code: `", output, "`")
				break
			}
		}
	}
}

// ReadFileContent lit le contenu d'un fichier et le retourne sous forme de chaîne de caractères.
func (j *job) readFileContent() (string, error) {
	// Lire tout le contenu du fichier
	data, err := ioutil.ReadFile(j.fileDir + "/" + j.fileName)
	if err != nil {
		return "", fmt.Errorf("erreur lors de la lecture du fichier %s: %v", j.fileDir+"/"+j.fileName, err)
	}
	// Retourner le contenu sous forme de chaîne
	return string(data), nil
}

func (j *job) findUnusedFunctions() ([]string, error) {
	fmt.Println(fmt.Sprintf("Analyse de : %s", j.fileDir+"/"+j.currentFileName))

	// Construire la commande staticcheck avec le chemin complet
	cmd := exec.Command("staticcheck", j.currentFileName)

	// Spécifier le répertoire de travail pour staticcheck
	cmd.Dir = j.fileDir

	var out bytes.Buffer
	cmd.Stderr = &out
	cmd.Stdout = &out

	// Exécuter la commande et capturer les avertissements
	err := cmd.Run()
	output := out.String()

	// Si une erreur de statut de sortie est retournée, vérifier si la sortie contient des avertissements U1000
	if err != nil {
		fmt.Println("Avertissement ou erreur:", err) // Affiche l'erreur pour diagnostic
	}

	// Regex pour détecter les fonctions non utilisées signalées par U1000
	re := regexp.MustCompile(`func (\w+) is unused`)
	matches := re.FindAllStringSubmatch(output, -1)

	var unusedFuncs []string
	for _, match := range matches {
		if len(match) > 1 {
			unusedFuncs = append(unusedFuncs, match[1])
		}
	}

	// Retourner les fonctions inutilisées même si staticcheck a généré une erreur de statut
	return unusedFuncs, nil // Ignorer l'erreur pour continuer normalement
}

// Met en commentaire les fonctions inutilisées dans le fichier
func (j *job) commentUnusedFunctions(funcs []string) error {
	// Lire le fichier ligne par ligne
	content, err := os.ReadFile(j.fileDir + "/" + j.currentFileName)
	if err != nil {
		return err
	}

	// Transformation du contenu en lignes pour faciliter la manipulation
	lines := bytes.Split(content, []byte("\n"))

	// On crée une liste de lignes modifiées pour reconstruire le fichier avec les commentaires ajoutés
	var updatedLines [][]byte
	commentMode := false
	openBraces := 0

	for _, line := range lines {
		lineStr := string(line)

		// Si on est en mode commentaire, on ajoute `//` au début de la ligne
		if commentMode {
			updatedLines = append(updatedLines, append([]byte("// "), line...))

			// Compter les accolades ouvrantes et fermantes pour détecter la fin de la fonction
			openBraces += bytes.Count(line, []byte("{"))
			openBraces -= bytes.Count(line, []byte("}"))

			// Fin du bloc de fonction
			if openBraces == 0 {
				commentMode = false
			}
			continue
		}

		// Détection du début de la fonction non utilisée
		for _, fn := range funcs {
			if match, _ := regexp.MatchString(fmt.Sprintf(`^func %s\(`, fn), lineStr); match {
				commentMode = true
				openBraces = 1 // Initialiser le compte d'accolades pour cette fonction
				updatedLines = append(updatedLines, append([]byte("// "), line...))
				break
			}
		}

		// Si on n'est pas en mode commentaire, ajouter la ligne telle quelle
		if !commentMode {
			updatedLines = append(updatedLines, line)
		}
	}

	// Reconstruire le fichier avec les lignes mises à jour
	return os.WriteFile(j.fileDir+"/"+j.currentFileName, bytes.Join(updatedLines, []byte("\n")), 0644)
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
	data, err := ioutil.ReadFile(j.fileDir + "/" + j.currentFileName)
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
	node, err := parser.ParseFile(fs, j.fileDir+"/"+j.currentFileName, data, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("erreur lors de l'analyse du fichier: %v", err)
	}

	// Préparer un buffer pour le fichier modifié
	var modifiedFile bytes.Buffer

	// Récupérer et réutiliser le package existant
	packageDecl := node.Name

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
	// fmt.Println("Code généré avant formatage:\n", modifiedFile.String())

	// Appliquer un formatage Go standard au code généré
	formattedCode, err := format.Source(modifiedFile.Bytes())
	if err != nil {
		return fmt.Errorf("erreur lors du formatage du fichier: %v", err)
	}

	// Sauvegarder le fichier modifié
	return ioutil.WriteFile(j.fileDir+"/"+j.currentFileName, formattedCode, 0644)
}

// Fonction pour écrire le code dans un fichier .go
func (j *job) writeCodeToFile(code string) error {
	return ioutil.WriteFile(j.fileDir+"/"+j.currentFileName, []byte(code), 0644)
}

func (j *job) fixImports() error {
	// Commande pour exécuter goimports
	cmd := exec.Command("goimports", "-w", j.currentFileName)
	cmd.Dir = j.fileDir

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

// Fonction pour exécuter le fichier Go et capturer les erreurs.
func (j *job) runGolangFile() (string, error) {
	var cmd *exec.Cmd

	// Vérifier si le fichier est un fichier de test
	if strings.HasSuffix(j.currentFileName, "_test.go") {
		// Si c'est un fichier de test, utiliser "go test"
		cmd = exec.Command("go", "test", j.currentFileName)
	} else {
		// Sinon, utiliser "go run" pour exécuter le fichier
		cmd = exec.Command("go", "run", j.currentFileName)
	}

	// Spécifier le répertoire contenant le fichier et le go.mod
	cmd.Dir = j.fileDir

	// Préparer le buffer pour capturer la sortie
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	// Exécuter la commande
	err := cmd.Run()

	// Retourner la sortie de la commande et l'erreur éventuelle
	return out.String(), err
}

// Fonction pour exécuter `go mod init` et `go mod tidy`
func (j *job) setupGoMod() error {
	// Initialisation du module si le fichier go.mod n'existe pas
	goModPath := filepath.Join(j.fileDir, "go.mod")
	if _, err := os.Stat(goModPath); os.IsNotExist(err) {
		cmdInit := exec.Command("go", "mod", "init", "generated_code_module")
		cmdInit.Dir = j.fileDir // Définit le répertoire de travail pour `go mod init`
		if output, err := cmdInit.CombinedOutput(); err != nil {
			return fmt.Errorf("erreur lors de l'initialisation du module: %v - %s", err, output)
		}
	}
	return nil
}

func (j *job) updateGoMod() error {
	cmdTidy := exec.Command("go", "mod", "tidy")
	cmdTidy.Dir = j.fileDir // Définit le répertoire de travail pour `go mod tidy`
	if output, err := cmdTidy.CombinedOutput(); err != nil {
		return fmt.Errorf("erreur lors de l'exécution de go mod tidy: %v - %s", err, output)
	}

	cmdVendor := exec.Command("go", "mod", "vendor")
	cmdVendor.Dir = j.fileDir // Définit le répertoire de travail pour `go mod vendor`
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

func (j *job) extractFunctionFromLine(lineNumber int) (string, error) {
	// Lire le fichier Go
	data, err := ioutil.ReadFile(j.fileDir + "/" + j.currentFileName)
	if err != nil {
		return "", fmt.Errorf("erreur lors de la lecture du fichier: %v", err)
	}

	// Créer un fichier tokeniseur
	fs := token.NewFileSet()
	node, err := parser.ParseFile(fs, j.fileDir+"/"+j.currentFileName, data, parser.ParseComments)
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

	// Si la ligne ne fait pas partie d'une fonction, retourner les 10 lignes avant et après
	lines := bytes.Split(data, []byte("\n"))
	startLine := max(0, lineNumber-10)
	endLine := min(len(lines), lineNumber+10)

	var surroundingCode bytes.Buffer
	for i := startLine; i < endLine; i++ {
		surroundingCode.Write(lines[i])
		surroundingCode.WriteString("\n")
	}

	return surroundingCode.String(), nil
}

// Fonction utilitaire pour obtenir le maximum entre deux entiers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Fonction utilitaire pour obtenir le minimum entre deux entiers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
