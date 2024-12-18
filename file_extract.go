package main

import (
	"bufio"
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/ioutil"
	"regexp"
	"strconv"
	"strings"
)

// getSourceFileName returns the source file name from the test file name.
func (j *job) getSourceFileName(testFileName string) string {
	if strings.HasSuffix(testFileName, "_test.go") {
		return strings.TrimSuffix(testFileName, "_test.go") + ".go"
	}
	return testFileName
}

// isTestFile checks if a file is a test file.
func (j *job) isTestFile(testFileName string) bool {
	return strings.HasSuffix(testFileName, "_test.go")
}

var regFindNameAndCode = regexp.MustCompile("\\*\\*(.*?)\\*\\*\\s```go\\s*(?s)(.*?)\\s*```")

func (j *job) splitFilesAndCode(response string) map[string]string {
	var filesNameAndCode = map[string]string{}

	matches := regFindNameAndCode.FindAllStringSubmatch(response, -1)

	for _, match := range matches {
		filesNameAndCode[match[1]] = match[2]
	}

	return filesNameAndCode
}

func (j *job) parseListFolders(input string) []string {
	input = strings.ReplaceAll(input, "```bash", "")
	input = strings.ReplaceAll(input, "```", "")

	var paths []string
	stack := []string{} // Pile pour gérer les niveaux d'indentation

	scanner := bufio.NewScanner(strings.NewReader(input))
	for scanner.Scan() {
		line := scanner.Text()

		// Ignorer les lignes vides ou les commentaires
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}

		// Déterminer le niveau d'indentation
		indentLevel := 0
		for _, char := range line {
			if char == ' ' || char == '\t' {
				indentLevel++
			} else {
				break
			}
		}
		indentLevel /= 2 // En supposant une indentation de 2 espaces par niveau

		// Supprimer la pile jusqu'au bon niveau d'indentation
		if len(stack) > indentLevel {
			stack = stack[:indentLevel]
		}

		// Extraire le chemin sans les commentaires
		line = strings.Split(line, "#")[0]
		line = strings.TrimSpace(line)
		line = strings.TrimLeft(line, "- ")
		line = strings.Trim(line, "/") // Supprime les barres obliques autour

		// Construire le chemin complet
		fullPath := strings.Join(append(stack, line), "/")
		paths = append(paths, fullPath)

		// Ajouter à la pile si c'est un dossier (pas un fichier avec extension)
		if !strings.Contains(line, ".") {
			stack = append(stack, line)
		}
	}

	return paths
}

// splitFileNameAndCode separates the file name and the code of a response.
func (j *job) splitFileNameAndCode(response string) (fileName string, code string) {
	parts := strings.SplitN(response, "CODE:", 2)
	if len(parts) != 2 {
		fmt.Println("response has no 2 parts")
		return j.currentFileName, response
	}

	// Extract the file to modify
	fileLine := strings.TrimSpace(parts[0])
	isTestFile := strings.Contains(fileLine, "(test file)")
	code = strings.TrimSpace(parts[1])

	code = j.extractBackticks(code)

	if !isTestFile {
		return j.getSourceFileName(j.currentFileName), code
	}
	return j.currentFileName, code
}

// extractBackticks extracts Go code from a string surrounded by backticks.
func (j *job) extractBackticks(code string) string {
	// Si la chaîne commence et se termine par des backticks, on les supprime.
	if strings.HasPrefix(code, "```go") && strings.HasSuffix(code, "```") {
		// Supprimer les backticks au début et à la fin
		return code[5 : len(code)-3]
	}
	if strings.HasPrefix(code, "```") && strings.HasSuffix(code, "```") {
		// Supprimer les backticks au début et à la fin
		return code[3 : len(code)-3]
	}
	// Sinon, retourner la chaîne telle quelle
	return code
}

// extractFunctionsFromCode extracts function declarations from Go code as a string.
func (j *job) extractFunctionsFromCode(code string) ([]*ast.FuncDecl, error) {

	if !strings.HasPrefix(code, "package") {
		// Ajouter "package main" au début du code
		code = "package main\n\nimport \"fmt\"\n\n" + code
	}

	fs := token.NewFileSet()
	node, err := parser.ParseFile(fs, "", code, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf(j.t("error parsing functions from code")+": %v", err)
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

// extractStructsFromCode extracts struct declarations from Go code as a string.
func (j *job) extractStructsFromCode(code string) ([]*ast.TypeSpec, error) {

	if !strings.HasPrefix(code, "package") {
		// Ajouter "package main" au début du code
		code = "package main\n\nimport \"fmt\"\n\n" + code
	}

	fs := token.NewFileSet()
	node, err := parser.ParseFile(fs, "", code, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf(j.t("error parsing structs from code")+": %v", err)
	}

	var structs []*ast.TypeSpec
	for _, decl := range node.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if ok {
			for _, spec := range genDecl.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if ok {
					if _, isStruct := typeSpec.Type.(*ast.StructType); isStruct {
						structs = append(structs, typeSpec)
					}
				}
			}
		}
	}
	return structs, nil
}

// extractInterfacesFromCode extracts interface declarations from Go code as a string.
func (j *job) extractInterfacesFromCode(code string) ([]*ast.TypeSpec, error) {
	if !strings.HasPrefix(code, "package") {
		// Ajouter "package main" au début du code
		code = "package main\n\nimport \"fmt\"\n\n" + code
	}

	fs := token.NewFileSet()
	node, err := parser.ParseFile(fs, "", code, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf(j.t("error parsing interfaces from code : ")+red(code)+": %v", err)
	}

	var interfaces []*ast.TypeSpec
	for _, decl := range node.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if ok {
			for _, spec := range genDecl.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if ok {
					if _, isInterface := typeSpec.Type.(*ast.InterfaceType); isInterface {
						interfaces = append(interfaces, typeSpec)
					}
				}
			}
		}
	}
	return interfaces, nil
}

// extractConstsFromCode extracts const declarations from Go code as a string.
func (j *job) extractConstsFromCode(code string) ([]*ast.GenDecl, error) {

	if !strings.HasPrefix(code, "package") {
		// Ajouter "package main" au début du code
		code = "package main\n\nimport \"fmt\"\n\n" + code
	}

	fs := token.NewFileSet()
	node, err := parser.ParseFile(fs, "", code, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf(j.t("error parsing consts from code")+": %v", err)
	}

	var consts []*ast.GenDecl
	for _, decl := range node.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if ok && genDecl.Tok == token.CONST {
			consts = append(consts, genDecl)
		}
	}
	return consts, nil
}

// extractVarsFromCode extracts var declarations from Go code as a string.
func (j *job) extractVarsFromCode(code string) ([]*ast.GenDecl, error) {
	if !strings.HasPrefix(code, "package") {
		// Ajouter "package main" au début du code
		code = "package main\n\nimport \"fmt\"\n\n" + code
	}

	fs := token.NewFileSet()
	node, err := parser.ParseFile(fs, "", code, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf(j.t("error parsing variables from code")+": %v", err)
	}

	var vars []*ast.GenDecl
	for _, decl := range node.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if ok && genDecl.Tok == token.VAR {
			vars = append(vars, genDecl)
		}
	}
	return vars, nil
}

// extractImportsFromCode extracts import statements from a Go code as a string.
func (j *job) extractImportsFromCode(fromFile, code string) ([]string, error) {
	if !strings.HasPrefix(code, "package") {
		// Ajouter "package main" au début du code
		code = "package main\n\nimport \"fmt\"\n\n" + code
	}

	fmt.Println(fmt.Sprintf("code from: %s", fromFile), magenta(code), "end")
	// Parser le fichier Go pour en extraire l'AST
	fs := token.NewFileSet()
	node, err := parser.ParseFile(fs, "", []byte(code), parser.ImportsOnly)
	if err != nil {
		return nil, fmt.Errorf(j.t("error parsing imports")+": %v", err)
	}

	var imports []string
	for _, imp := range node.Imports {
		imports = append(imports, strings.Trim(imp.Path.Value, `"`))
	}
	return imports, nil
}

// extractLineNumber extracts the line number from an error message.
func (j *job) extractLineNumber(errorMessage string) (int, error) {
	// Expression régulière pour capturer le numéro de ligne
	re := regexp.MustCompile(`:(\d+):\d+`)

	// Rechercher la correspondance dans le message d'erreur
	matches := re.FindStringSubmatch(errorMessage)
	if len(matches) < 2 {
		return 0, fmt.Errorf(j.t("line number not found in error message")+" : %s", errorMessage)
	}

	// Convertir le numéro de ligne en entier
	lineNumber, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, fmt.Errorf(j.t("line number conversion error")+": %v", err)
	}

	return lineNumber, nil
}

// extractFunctionFromLine extracts the code of a function from the line number.
func (j *job) extractFunctionFromLine(lineNumber int) (string, error) {
	data, err := ioutil.ReadFile(j.fileDir + "/" + j.currentSourceFileName)
	if err != nil {
		return "", fmt.Errorf(j.t("error reading file")+": %v", err)
	}

	// Créer un fichier tokeniseur
	fs := token.NewFileSet()
	node, err := parser.ParseFile(fs, j.fileDir+"/"+j.currentSourceFileName, data, parser.ParseComments)
	if err != nil {
		return "", fmt.Errorf(j.t("error parsing file")+": %v", err)
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
				return "", fmt.Errorf(j.t("error printing function")+": %v", err)
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

// extractUnusedImports extracts unused imports from an error message.
func (j *job) extractUnusedImports(errorMessage string) ([]string, error) {
	// Regex pour trouver les imports inutilisés dans les erreurs
	re := regexp.MustCompile(`\./.*:\d+:\d+: "([^"]+)" imported and not used`)
	matches := re.FindAllStringSubmatch(errorMessage, -1)

	if len(matches) == 0 {
		return nil, fmt.Errorf("aucun import inutilisé trouvé")
	}

	// Extraire les imports inutilisés
	var unusedImports []string
	for _, match := range matches {
		if len(match) > 1 {
			unusedImports = append(unusedImports, match[1])
		}
	}

	return unusedImports, nil
}

// extractErrorForPrompt extracts the code from the function containing the error to display in the prompt.
func (j *job) extractErrorForPrompt(output string) (string, error) {
	errorLine, err := j.extractLineNumber(output)
	if err != nil {
		return "", err
	}
	fmt.Println(j.t("Error line number")+": ", errorLine)
	funcCode, err := j.extractFunctionFromLine(errorLine)
	if err != nil {
		return "", fmt.Errorf(j.t("error extracting function")+": %v", err)
	}
	return funcCode, nil
}

// ReadFileContent reads the contents of a file and returns it as a string.
func (j *job) readFileContent(file string) ([]byte, error) {
	if j.source == fileSourceStdin {
		return []byte{}, nil
	}
	// Lire tout le contenu du fichier
	data, err := ioutil.ReadFile(j.fileDir + "/" + file)
	if err != nil {
		return nil, fmt.Errorf(j.t("error reading file")+" %s: %v", j.fileDir+"/"+file, err)
	}
	// Retourner le contenu sous forme de chaîne
	return data, nil
}

// extractFunctionDetails extracts the details of a function from its declaration.
func extractFunctionDetails(funcDecl *ast.FuncDecl) string {
	var builder strings.Builder
	builder.WriteString("func ")

	// Ajouter le nom de la struct si c'est une méthode
	if funcDecl.Recv != nil {
		for _, recv := range funcDecl.Recv.List {
			// Type du receveur (struct)
			builder.WriteString(fmt.Sprintf("(%s) ", exprToString(recv.Type)))
		}
	}

	// Ajouter le nom de la fonction
	builder.WriteString(funcDecl.Name.Name)

	// Ajouter les paramètres
	builder.WriteString("(")
	if funcDecl.Type.Params != nil {
		params := []string{}
		for _, param := range funcDecl.Type.Params.List {
			paramType := exprToString(param.Type)
			for range param.Names {
				params = append(params, paramType)
			}
			// Si aucun nom, on ajoute juste le type
			if len(param.Names) == 0 {
				params = append(params, paramType)
			}
		}
		builder.WriteString(strings.Join(params, ", "))
	}
	builder.WriteString(")")

	// Ajouter les résultats
	if funcDecl.Type.Results != nil {
		results := []string{}
		for _, result := range funcDecl.Type.Results.List {
			results = append(results, exprToString(result.Type))
		}
		builder.WriteString("(")
		builder.WriteString(strings.Join(results, ", "))
		builder.WriteString(") { ... }")
	}

	return builder.String()
}

// exprToString Utility function to convert expression to string.
func exprToString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return fmt.Sprintf("%s.%s", exprToString(t.X), t.Sel.Name)
	case *ast.StarExpr:
		return fmt.Sprintf("*%s", exprToString(t.X))
	case *ast.ArrayType:
		return fmt.Sprintf("[]%s", exprToString(t.Elt))
	case *ast.FuncType:
		return "func" // Simplifié ici
	default:
		return fmt.Sprintf("%T", t)
	}
}
