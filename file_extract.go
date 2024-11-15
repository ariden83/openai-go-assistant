package main

import (
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

// Fonction pour extraire toutes les fonctions dans un code donné
func (j *job) extractFunctionsFromCode(code string) ([]*ast.FuncDecl, error) {

	if !strings.HasPrefix(code, "package") {
		// Ajouter "package main" au début du code
		code = "package main\n\nimport \"fmt\"\n\n" + code
	}

	fs := token.NewFileSet()
	node, err := parser.ParseFile(fs, "", code, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf(j.t("error parsing code")+" : %v", err)
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

func (j *job) extractStructsFromCode(code string) ([]*ast.TypeSpec, error) {

	if !strings.HasPrefix(code, "package") {
		// Ajouter "package main" au début du code
		code = "package main\n\nimport \"fmt\"\n\n" + code
	}

	fs := token.NewFileSet()
	node, err := parser.ParseFile(fs, "", code, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de l'analyse du code : %v", err)
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

func (j *job) extractInterfacesFromCode(code string) ([]*ast.TypeSpec, error) {

	if !strings.HasPrefix(code, "package") {
		// Ajouter "package main" au début du code
		code = "package main\n\nimport \"fmt\"\n\n" + code
	}

	fs := token.NewFileSet()
	node, err := parser.ParseFile(fs, "", code, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de l'analyse du code : %v", err)
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

func (j *job) extractConstsFromCode(code string) ([]*ast.GenDecl, error) {

	if !strings.HasPrefix(code, "package") {
		// Ajouter "package main" au début du code
		code = "package main\n\nimport \"fmt\"\n\n" + code
	}

	fs := token.NewFileSet()
	node, err := parser.ParseFile(fs, "", code, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de l'analyse du code : %v", err)
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

func (j *job) extractVarsFromCode(code string) ([]*ast.GenDecl, error) {
	if !strings.HasPrefix(code, "package") {
		// Ajouter "package main" au début du code
		code = "package main\n\nimport \"fmt\"\n\n" + code
	}

	fs := token.NewFileSet()
	node, err := parser.ParseFile(fs, "", code, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de l'analyse du code : %v", err)
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

// Fonction pour extraire les imports d'un code Go sous forme de chaîne
func extractImportsFromCode(code string) ([]string, error) {

	if !strings.HasPrefix(code, "package") {
		// Ajouter "package main" au début du code
		code = "package main\n\nimport \"fmt\"\n\n" + code
	}

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

func (j *job) extractErrorForPrompt(output string) (string, error) {
	errorLine, err := extractLineNumber(output)
	if err != nil {
		return "", err
	}
	fmt.Println("Numéro de ligne de l'erreur :", errorLine)
	funcCode, err := j.extractFunctionFromLine(errorLine)
	if err != nil {
		return "", fmt.Errorf("erreur lors de l'extraction de la fonction: %v", err)
	}
	return funcCode, nil
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
