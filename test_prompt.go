package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"path/filepath"
	"regexp"
	"strings"
)

// getTestFilename prend le nom d'un fichier Go et retourne le nom du fichier de test associé
func (j *job) getTestFilename() (string, error) {
	// Vérifie si le fichier a l'extension .go
	if filepath.Ext(j.fileName) != ".go" {
		return "", fmt.Errorf("le fichier %s n'est pas un fichier Go", j.fileName)
	}

	// Construit le nom du fichier de test en ajoutant "_test" avant l'extension
	testFilename := strings.TrimSuffix(j.fileName, ".go") + "_test.go"
	return testFilename, nil
}

func (j *job) getFailedTests(output string) ([]string, error) {
	// Extraction des noms de tests ayant échoué depuis la sortie de `go test`
	re := regexp.MustCompile(`--- FAIL: ([\w\/]+)`)
	matches := re.FindAllStringSubmatch(output, -1)

	// Temporaire pour stocker les parents et enfants échoués
	failedTestsMap := make(map[string][]string)

	for _, match := range matches {
		if len(match) > 1 {
			fullTestName := match[1]
			segments := strings.Split(fullTestName, "/")
			parentTest := segments[0]

			// Vérifie si c'est un sous-test
			if len(segments) > 1 {
				// Ajoute le sous-test à la liste des sous-tests pour le test parent
				failedTestsMap[parentTest] = append(failedTestsMap[parentTest], fullTestName)
			} else {
				// Si aucun sous-test, initialise une entrée pour le parent
				if _, exists := failedTestsMap[parentTest]; !exists {
					failedTestsMap[parentTest] = nil
				}
			}
		}
	}

	// Construire la liste finale des tests ayant échoué
	var failedTests []string
	for parent, subTests := range failedTestsMap {
		if len(subTests) > 0 {
			// Ajoute uniquement les sous-tests si présents
			failedTests = append(failedTests, subTests...)
		} else {
			// Sinon, ajoute le parent lui-même
			failedTests = append(failedTests, parent)
		}
	}

	if len(failedTests) == 0 {
		return nil, nil
	}

	return failedTests, nil
}

func (j *job) getTestCode(failedTests []string) (string, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, j.fileDir+"/"+j.currentFileName, nil, parser.ParseComments)
	if err != nil {
		return "", fmt.Errorf("Error parsing file: %v\n", err)
	}

	var failedTestsCode strings.Builder

	for _, decl := range node.Decls {
		// Vérifier si la déclaration est une fonction
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}

		for _, fullTestName := range failedTests {
			// Extraire la dernière partie après le dernier "/"
			parts := strings.Split(fullTestName, "/")

			if funcDecl.Name.Name != parts[0] { // Vérifie le nom principal du test
				continue
			}

			var buf bytes.Buffer
			if err := printer.Fprint(&buf, fset, funcDecl); err != nil {
				fmt.Printf("Error printing function %s: %v\n", fullTestName, err)
			} else {
				failedTestsCode.Write(buf.Bytes())
				failedTestsCode.WriteString("\n\n")
			}
		}
	}

	return failedTestsCode.String(), nil
}
