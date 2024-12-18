package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
)

// setupGoMod initializes the Go module if necessary.
func (j *job) setupGoMod() error {

	goModPath, err := j.findGoMod()
	if err != nil {
		log.WithError(err).Info("error finding go.mod file")

		goModPath := filepath.Join(j.fileDir, "go.mod")
		if _, err := os.Stat(goModPath); os.IsNotExist(err) {
			if err := j.createGoModName(); err != nil {
				return fmt.Errorf(j.t("error creating module name")+": %v", err)
			}
			cmdInit := exec.Command("go", "mod", "init", j.modulePath)
			cmdInit.Dir = j.fileDir
			if output, err := cmdInit.CombinedOutput(); err != nil {
				return fmt.Errorf(j.t("error during module initialization")+": %v - %s", err, output)
			}
		}

	} else {
		j.fileDir = "./" + filepath.Dir(goModPath)
	}

	if err := j.getModulePath(); err != nil {
		return fmt.Errorf(j.t("error getting module path")+": %v", err)
	}

	return nil
}

// updateGoMod updates the go.mod file and vendor directory.
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

// findGoMod find the go.mod file in the current directory or one of its parents.
func (j *job) findGoMod() (string, error) {
	startPath := j.fileDir + "/" + j.fileName

	goPath := os.Getenv("GOPATH")
	if goPath == "" {
		goPath = filepath.Join(os.Getenv("HOME"), "go") // Valeur par défaut si GOPATH n'est pas défini
	}

	// Construire le srcPath
	srcPath := filepath.Join(goPath, "src")

	// Démarrer la recherche
	currentPath := startPath
	for {
		// Vérifier si le fichier go.mod existe dans le répertoire courant
		goModPath := filepath.Join(currentPath, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			return goModPath, nil
		}

		// Si le dossier courant est égal ou contient "/go/src/", arrêter la recherche
		if strings.HasPrefix(currentPath, srcPath) {
			return "", errors.New("no go.mod file found up to the /go/src/ boundary")
		}

		// Remonter d'un niveau
		parentDir := filepath.Dir(currentPath)
		if parentDir == currentPath { // Si on atteint la racine
			break
		}
		currentPath = parentDir
	}

	return "", errors.New("no go.mod file found")
}

// findGoModPath finds the path of the go.mod file in the current directory or one of its parents.
func (j *job) findGoModPath() (string, error) {
	var goModPath string
	err := filepath.Walk(j.fileDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Name() == "go.mod" {
			goModPath = path
			return filepath.SkipDir // Stop dès qu'on trouve un go.mod
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("error while searching for go.mod: %w", err)
	}
	if goModPath == "" {
		return "", fmt.Errorf("no go.mod file found in or above %s", j.fileDir)
	}
	return goModPath, nil
}

// getModulePath returns the path of the current module.
func (j *job) getModulePath() error {
	cmd := exec.Command("go", "list", "-m")
	cmd.Dir = j.fileDir

	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return err
	}

	j.modulePath = strings.TrimSpace(out.String())
	// return j.adjustModulePath()
	return nil
}

// createGoModName creates a Go module name based on the current directory.
func (j *job) createGoModName() error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf(j.t("Error retrieving current directory")+":", err)
	}

	cwd = cwd + string(filepath.Separator) + j.fileDir

	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		gopath = filepath.Join(os.Getenv("HOME"), "go")
	}

	srcPrefix := filepath.Join(gopath, "src") + string(filepath.Separator)

	relativePath := strings.TrimPrefix(cwd, srcPrefix)
	relativePath = strings.TrimSuffix(relativePath, "/go.mod")
	relativePath = strings.ReplaceAll(relativePath, "/./", "/")

	// Déterminer le dernier répertoire à inclure basé sur fileDir
	// Exemple : si fileDir = "/./adevinta", garder jusqu'à "adevinta"
	fileDir := strings.Trim(j.fileDir, "/.") // Nettoyer les `/` ou `.`
	lastDir := filepath.Base(fileDir)        // Récupérer le dernier répertoire

	if strings.Contains(relativePath, lastDir) {
		parts := strings.Split(relativePath, string(filepath.Separator))
		for i, part := range parts {
			if part == lastDir {
				j.modulePath = strings.Join(parts[:i+1], "/")
				return nil
			}
		}
	}

	return fmt.Errorf("could not find %s in path %s", lastDir, relativePath)
}

// countDirectories counts the number of directories in the current directory.
func (j *job) countDirectories() (int, error) {
	var count int

	err := filepath.Walk(j.fileDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			count++
		}
		return nil
	})

	if err != nil {
		return 0, err
	}

	return count - 1, nil
}
