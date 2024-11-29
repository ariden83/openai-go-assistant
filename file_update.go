package main

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// addImport ajoute un import à la liste des imports existants.
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

func (j *job) removeUnusedImports(unusedImports []string, currentFileName string) error {
	var data []byte
	if currentFileName == j.currentSourceFileName {
		data = j.currentSrcSource
	} else if currentFileName == j.currentTestFileName {
		data = j.currentSrcTest
	}

	// Créer une FileSet pour gérer le fichier source
	fs := token.NewFileSet()

	// Parser le fichier source pour obtenir son AST
	node, err := parser.ParseFile(fs, j.fileDir+"/"+currentFileName, data, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("impossible de parser le fichier: %w", err)
	}

	// Convertir la liste des imports inutilisés en un map pour faciliter la recherche
	unusedMap := make(map[string]struct{})
	for _, imp := range unusedImports {
		unusedMap[imp] = struct{}{}
	}

	// Filtrer les déclarations d'import
	var newDecls []ast.Decl
	for _, decl := range node.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.IMPORT {
			// Ajouter les déclarations qui ne sont pas des imports
			newDecls = append(newDecls, decl)
			continue
		}

		// Filtrer les spécifications d'import dans GenDecl
		var newSpecs []ast.Spec
		for _, spec := range genDecl.Specs {
			importSpec, ok := spec.(*ast.ImportSpec)
			if !ok {
				newSpecs = append(newSpecs, spec)
				continue
			}

			// Extraire le chemin de l'import (sans guillemets)
			importPath := strings.Trim(importSpec.Path.Value, "\"")
			if _, found := unusedMap[importPath]; !found {
				// Conserver l'import s'il n'est pas inutilisé
				newSpecs = append(newSpecs, spec)
			}
		}

		// Si des imports restent, conserver cette déclaration
		if len(newSpecs) > 0 {
			genDecl.Specs = newSpecs
			newDecls = append(newDecls, genDecl)
		}
	}

	// Mettre à jour les déclarations du fichier AST
	node.Decls = newDecls

	formattedCode, err := j.nodeToBytes(fs, node)
	if err != nil {
		return fmt.Errorf("impossible de formater le code: %w", err)
	}

	return j.writeFile(currentFileName, formattedCode)
}

// stepFixCode met à jour le code Go existant avec les suggestions d'OpenAI.
func (j *job) stepFixCode(currentFileName, openAIResponse string) ([]byte, error) {

	openAIResponse = strings.TrimSpace(openAIResponse)

	// Extraire les imports proposés par OpenAI
	openAIImports, err := j.extractImportsFromCode("openAI", openAIResponse)
	if err != nil {
		return nil, fmt.Errorf(j.t("error extracting OpenAI imports")+": %v", err)
	}

	var data []byte
	if currentFileName == j.currentSourceFileName {
		data = j.currentSrcSource
	} else if currentFileName == j.currentTestFileName {
		data = j.currentSrcTest
	}

	// Extraire les imports existants dans le fichier
	existingImports, err := j.extractImportsFromCode("local", string(data))
	if err != nil {
		return nil, fmt.Errorf(j.t("error extracting existing imports")+": %v", err)
	}

	// Ajouter les imports OpenAI manquants aux imports existants
	for _, newImport := range openAIImports {
		existingImports = addImport(existingImports, newImport)
	}

	// Créer un fichier tokeniseur
	fs := token.NewFileSet()
	node, err := parser.ParseFile(fs, j.fileDir+"/"+currentFileName, data, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf(j.t("error parsing file")+": %v", err)
	}

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
	if len(node.Decls) > 0 {
		// Remplacer la première déclaration par importDecl
		node.Decls[0] = importDecl
	} else {
		// Si node.Decls est vide, ajoutez importDecl en tant que première déclaration
		node.Decls = append(node.Decls, importDecl)
	}

	interfaces, err := j.extractInterfacesFromCode(openAIResponse)
	if err != nil {
		return nil, fmt.Errorf(j.t("error extracting interfaces")+": %v", err)
	}

	for _, openAIInterface := range interfaces {
		found := false // Indique si l'interface OpenAI a été trouvée dans les déclarations existantes

		for _, decl := range node.Decls[1:] {
			genDecl, ok := decl.(*ast.GenDecl)
			if ok && genDecl.Tok == token.TYPE {
				for _, spec := range genDecl.Specs {
					typeSpec, ok := spec.(*ast.TypeSpec)
					if ok && typeSpec.Name.Name == openAIInterface.Name.Name {
						// Remplacer l'interface existante par celle d'OpenAI
						typeSpec.Type = openAIInterface.Type
						found = true
						break
					}
				}
			}
			if found {
				break
			}
		}

		// Si l'interface n'a pas été trouvée dans les déclarations existantes, l'ajouter
		if !found {
			genDecl := &ast.GenDecl{
				Tok: token.TYPE,
				Specs: []ast.Spec{&ast.TypeSpec{
					Name: openAIInterface.Name,
					Type: openAIInterface.Type,
				}},
			}
			node.Decls = append(node.Decls, genDecl)
		}
	}

	constants, err := j.extractConstsFromCode(openAIResponse)
	if err != nil {
		return nil, fmt.Errorf(j.t("error extracting constants")+": %v", err)
	}

	for _, genConst := range constants {
		found := false // Indique si le groupe de constantes a été trouvé dans les déclarations existantes

		// Rechercher si le groupe de constantes `genConst` est déjà dans les déclarations
		for _, decl := range node.Decls[1:] {
			existingGenDecl, ok := decl.(*ast.GenDecl)
			if ok && existingGenDecl.Tok == token.CONST {
				for _, openAISpec := range genConst.Specs {
					openAIConst, ok := openAISpec.(*ast.ValueSpec)
					if !ok {
						continue
					}

					for _, spec := range existingGenDecl.Specs {
						valueSpec, ok := spec.(*ast.ValueSpec)
						if ok && len(valueSpec.Names) > 0 && valueSpec.Names[0].Name == openAIConst.Names[0].Name {
							// Remplacer la valeur de la constante existante
							valueSpec.Values = openAIConst.Values
							found = true
							break
						}
					}

					if found {
						break
					}
				}
			}

			if found {
				break
			}
		}

		// Si aucun des noms de constantes dans `genConst` n'a été trouvé, ajouter le groupe entier
		if !found {
			node.Decls = append(node.Decls, genConst)
		}
	}

	vars, err := j.extractVarsFromCode(openAIResponse)
	if err != nil {
		return nil, fmt.Errorf(j.t("error extracting variables")+": %v", err)
	}

	for _, genVar := range vars {
		found := false // Indique si le groupe de constantes a été trouvé dans les déclarations existantes

		// Rechercher si le groupe de constantes `genConst` est déjà dans les déclarations
		for _, decl := range node.Decls[1:] {
			existingGenDecl, ok := decl.(*ast.GenDecl)
			if ok && existingGenDecl.Tok == token.VAR {
				for _, openAISpec := range genVar.Specs {
					openAIVar, ok := openAISpec.(*ast.ValueSpec)
					if !ok {
						continue
					}

					for _, spec := range existingGenDecl.Specs {
						valueSpec, ok := spec.(*ast.ValueSpec)
						if ok && len(valueSpec.Names) > 0 && valueSpec.Names[0].Name == openAIVar.Names[0].Name {
							// Remplacer la valeur de la constante existante
							valueSpec.Values = openAIVar.Values
							found = true
							break
						}
					}

					if found {
						break
					}
				}
			}

			if found {
				break
			}
		}

		if !found {
			node.Decls = append(node.Decls, genVar)
		}
	}

	structs, err := j.extractStructsFromCode(openAIResponse)
	if err != nil {
		return nil, fmt.Errorf(j.t("error extracting structures")+": %v", err)
	}

	for _, openAIStruct := range structs {
		found := false // Indique si la struct d'OpenAI a été trouvée dans les déclarations existantes

		for _, decl := range node.Decls[1:] {
			genDecl, ok := decl.(*ast.GenDecl)
			if ok && genDecl.Tok == token.TYPE {
				// Parcourir les spécifications de type dans la déclaration
				for _, spec := range genDecl.Specs {
					typeSpec, ok := spec.(*ast.TypeSpec)
					if ok {
						_, ok := typeSpec.Type.(*ast.StructType)
						// Vérifier si c'est bien une struct et que le nom correspond
						if ok && typeSpec.Name.Name == openAIStruct.Name.Name {
							// Remplacer la struct existante par celle d'OpenAI
							typeSpec.Type = openAIStruct.Type
							found = true
							break
						}
					}
				}
			}
			if found {
				break
			}
		}

		// Si la struct n'a pas été trouvée dans les déclarations existantes, l'ajouter.
		if !found {
			genDecl := &ast.GenDecl{
				Tok: token.TYPE,
				Specs: []ast.Spec{&ast.TypeSpec{
					Name: openAIStruct.Name,
					Type: openAIStruct.Type,
				}},
			}
			node.Decls = append(node.Decls, genDecl)
		}
	}

	funcs, err := j.extractFunctionsFromCode(openAIResponse)
	if err != nil {
		return nil, fmt.Errorf(j.t("error extracting functions")+": %v", err)
	}

	// Pour chaque déclaration dans le fichier, traiter les fonctions.
	for _, openAIFunc := range funcs {
		found := false // Indique si la fonction OpenAI a été trouvée dans les déclarations existantes.

		for _, decl := range node.Decls[1:] {
			funcDecl, ok := decl.(*ast.FuncDecl)
			if ok {
				// Si la fonction est complète, vérifier si elle correspond à celle d'OpenAI.
				if isCompleteFunction(funcDecl) && funcDecl.Name.Name == openAIFunc.Name.Name {
					fullFuncName := extractFunctionDetails(funcDecl)
					j.listFunctionsUpdated = append(j.listFunctionsUpdated, fullFuncName)
					// Remplacer le corps de la fonction existante par celui d'OpenAI.
					funcDecl.Body = openAIFunc.Body
					found = true
					break
				}
			}
		}

		// Si la fonction n'a pas été trouvée dans les déclarations existantes, l'ajouter.
		if !found {
			fullFuncName := extractFunctionDetails(openAIFunc)
			// Ajouter la fonction OpenAI à la liste des fonctions mises à jour.
			j.listFunctionsCreated = append(j.listFunctionsCreated, fullFuncName)
			node.Decls = append(node.Decls, openAIFunc)
		}
	}

	return j.nodeToBytes(fs, node)
}

func (j *job) nodeToBytes(fs *token.FileSet, node *ast.File) ([]byte, error) {
	var modifiedFile bytes.Buffer

	// Écrire le package
	fmt.Fprintf(&modifiedFile, "package %s", node.Name.Name)
	modifiedFile.WriteString("\n\n")

	// Écrire les déclarations
	for _, decl := range node.Decls {
		if err := printer.Fprint(&modifiedFile, fs, decl); err != nil {
			return nil, fmt.Errorf(j.t("error writing modified declaration")+": %v", err)
		}
		modifiedFile.WriteString("\n\n")
	}

	// Appliquer un formatage Go standard au code généré
	formattedCode, err := format.Source(modifiedFile.Bytes())
	if err != nil {
		return nil, fmt.Errorf(j.t("error while formatting file")+": %v", err)
	}

	return formattedCode, nil
}

// writeFile écrit le contenu du fichier modifié dans le fichier d'origine, stdout ou un fichier de destination.
func (j *job) writeFile(currentFileName string, res []byte) error {

	var src []byte
	if currentFileName == j.currentSourceFileName {
		src = j.currentSrcSource
		j.currentSrcSource = res
	} else if currentFileName == j.currentTestFileName {
		src = j.currentSrcTest
		j.currentSrcTest = res
	}

	out := os.Stdout
	if !bytes.Equal(src, res) {
		if j.args.listOnly {
			_, _ = fmt.Fprintln(out, j.fileDir+"/"+currentFileName)
		}

		if j.args.write {
			if j.source == fileSourceStdin {
				return errors.New("can't use -w on stdin")
			}
			return os.WriteFile(j.fileDir+"/"+currentFileName, res, 0o644)
		}

		if j.args.diffOnly {
			if j.source == fileSourceStdin {
				currentFileName = "stdin.go"
				j.currentFileName = "stdin.go" // because <standard input>.orig looks silly
			}

			data, err := diff(src, res, j.fileDir+"/"+currentFileName)
			if err != nil {
				return fmt.Errorf("computing diff: %v", err)
			}

			_, _ = out.Write(data)
		}
	}

	if !j.args.listOnly && !j.args.write && !j.args.diffOnly {
		if _, err := out.Write(res); err != nil {
			return err
		}
	}
	return nil
}

// createNewTestFile crée un nouveau fichier s'il n'existe pas déjà.
func (j *job) createNewTestFile() ([]byte, error) {
	// Construit le chemin complet du fichier.
	fullPath := filepath.Join(j.fileDir, j.currentTestFileName)

	// Vérifie si le fichier existe déjà.
	if _, err := os.Stat(fullPath); err == nil {
		fmt.Println(j.t("The file already exists"), fullPath)
		return j.readFileContent(j.currentTestFileName)

	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf(j.t("error when checking for existence of file")+": %v", err)
	}

	// Crée un fichier vide
	file, err := os.Create(fullPath)
	if err != nil {
		return nil, fmt.Errorf(j.t("error creating file")+": %v", err)
	}
	defer file.Close()

	fmt.Println(j.t("File created successfully"), fullPath)
	dirName := filepath.Base(j.fileDir)
	packageName := sanitizePackageName(dirName)
	if _, err = file.WriteString(fmt.Sprintf("package %s\n\n", packageName)); err != nil {
		return nil, fmt.Errorf(j.t("error writing file")+": %v", err)
	}

	return j.readFileContent(j.currentTestFileName)
}

// setupGoMod initialise le module Go si le fichier go.mod n'existe pas.
func (j *job) setupGoMod() error {
	// Initialisation du module si le fichier go.mod n'existe pas
	goModPath := filepath.Join(j.fileDir, "go.mod")
	if _, err := os.Stat(goModPath); os.IsNotExist(err) {
		cmdInit := exec.Command("go", "mod", "init", "generated_code_module")
		cmdInit.Dir = j.fileDir // Définit le répertoire de travail pour `go mod init`
		if output, err := cmdInit.CombinedOutput(); err != nil {
			return fmt.Errorf(j.t("error during module initialization")+": %v - %s", err, output)
		}
	}
	return nil
}

// updateGoMod met à jour le fichier go.mod et le dossier vendor si nécessaire.
func (j *job) updateGoMod() error {
	cmdTidy := exec.Command("go", "mod", "tidy")
	cmdTidy.Dir = j.fileDir // Définit le répertoire de travail pour `go mod tidy`
	if output, err := cmdTidy.CombinedOutput(); err != nil {
		return fmt.Errorf(j.t("error running go mod tidy")+": %v - %s", err, output)
	}

	if j.fileWithVendor {
		cmdVendor := exec.Command("go", "mod", "vendor")
		cmdVendor.Dir = j.fileDir // Définit le répertoire de travail pour `go mod vendor`
		if output, err := cmdVendor.CombinedOutput(); err != nil {
			return fmt.Errorf(j.t("error running go mod vendor")+": %v - %s", err, output)
		}
	}
	return nil
}
