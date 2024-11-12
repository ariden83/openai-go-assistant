package main

import (
	"fmt"
	"path/filepath"
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
