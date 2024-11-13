package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/manifoldco/promptui"
)

func (j *job) loadFilesFromFolder() ([]string, error) {
	var files []string
	err := filepath.Walk(j.fileDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

func (j *job) promptSelectAFileOrCreateANewOne(filesFound []string) error {
	confirmPrompt := promptui.Prompt{
		Label:     fmt.Sprintf("Files found in repo %s, create one", j.fileDir),
		IsConfirm: true,
	}
	_, err := confirmPrompt.Run()
	if err == nil {
		return j.promptCreateANewFile()
	}

	return j.promptSelectExistentFile(filesFound)
}

func (j *job) promptSelectExistentFile(filesFound []string) error {
	// Créer un prompt pour sélectionner un fichier
	filePrompt := promptui.Select{
		Label: "Select a file",
		Items: filesFound,
	}

	// Lire le fichier sélectionné
	_, selectedFile, err := filePrompt.Run()
	if err != nil {
		log.Fatalf("Erreur lors de la sélection du fichier: %v", err)
	}

	fmt.Println("Selected file:", selectedFile)

	// Extraire le chemin du dossier sans le fichier
	filePath := filepath.Dir(selectedFile)

	// Extraire uniquement le nom du fichier avec l'extension
	fileNameWithExt := filepath.Base(selectedFile)

	// Afficher le fichier et son chemin
	fmt.Println("Selected file:", selectedFile)
	fmt.Println("File path:", filePath)
	fmt.Println("File name with extension:", fileNameWithExt)

	// Mettre à jour les champs de la struct `job`
	j.fileName = fileNameWithExt        // Chemin complet du fichier sélectionné
	j.currentFileName = fileNameWithExt // Nom du fichier avec extension
	j.fileDir = filePath                // Chemin du dossier

	return nil
}

func (j *job) promptNoFilesFoundCreateANewFile() error {
	// Prompt de confirmation pour générer un fichier
	confirmPrompt := promptui.Prompt{
		Label:     "No files found, create one",
		IsConfirm: true,
	}

	// Lire la réponse de confirmation
	_, err := confirmPrompt.Run()
	if err != nil {
		fmt.Println("File generation canceled.")
		return errors.New("end")
	}
	return j.promptCreateANewFile()
}

func (j *job) getDirectories() ([]string, error) {
	var directories []string

	files, err := ioutil.ReadDir(j.fileDir)
	if err != nil {
		return directories, err
	}

	for _, file := range files {
		if file.IsDir() {
			directories = append(directories, file.Name())
		}
	}
	return directories, nil
}

// Fonction pour l'étape 1 : sélectionner un dossier ou en entrer un nouveau
func (j *job) promptSelectOrCreateDirectory() (string, error) {
	// Lire les dossiers existants dans le répertoire racine
	directories, err := j.getDirectories()
	if err != nil {
		return "", fmt.Errorf("erreur lors de la lecture des dossiers: %v", err)
	}

	// Ajouter l'option de création de nouveau chemin
	directories = append(directories, "Create a new folder")

	// Sélection de l'option
	selectPrompt := promptui.Select{
		Label: "Choose a folder",
		Items: directories,
	}

	_, selectedDir, err := selectPrompt.Run()
	if err != nil {
		return "", fmt.Errorf("error while selecting folder: %v", err)
	}

	// Si l'utilisateur choisit "Créer un nouveau dossier", demander le chemin
	if selectedDir == "Create a new folder" {
		pathPrompt := promptui.Prompt{
			Label: "Enter the path of the new folder",
			Validate: func(input string) error {
				if len(input) == 0 {
					return fmt.Errorf("the path cannot be empty")
				}
				return nil
			},
		}
		selectedDir, err = pathPrompt.Run()
		if err != nil {
			return "", fmt.Errorf("error entering path: %v", err)
		}
	}

	return selectedDir, nil
}

func (j *job) promptCreateANewFile() error {
	fmt.Println("Select a folder or enter a new path :")
	selectedDir, err := j.promptSelectOrCreateDirectory()
	if err != nil {
		return err
	}

	// Prompt pour entrer le nom du fichier si confirmation est "Oui"
	filenamePrompt := promptui.Prompt{
		Label:   "Enter the file name",
		Default: selectedDir,
		Validate: func(input string) error {
			if len(input) == 0 {
				return fmt.Errorf("file name cannot be empty")
			}
			return nil
		},
		Templates: &promptui.PromptTemplates{
			Invalid: "{{ . | red }}",
		},
	}

	// Lire le nom du fichier
	filename, err := filenamePrompt.Run()
	if err != nil {
		log.Fatalf("Erreur lors de la saisie du nom du fichier: %v", err)
	}

	// Ajouter l'extension .go si elle est absente
	if !strings.HasSuffix(filename, ".go") {
		filename += ".go"
	}

	// Extraire le chemin du dossier
	dir := filepath.Dir(filename)
	j.fileDir = j.fileDir + "/" + dir
	fmt.Println("Dir:", j.fileDir)
	// Créer les dossiers nécessaires si le chemin contient un sous-dossier
	if dir != "." {
		if err := os.MkdirAll(j.fileDir, 0755); err != nil {
			log.Fatalf("Erreur lors de la création des dossiers: %v", err)
		}
	}

	fileNameWithExt := filepath.Base(filename)

	// Créer le fichier avec le nom donné
	if err = j.createFileWithPackage(fileNameWithExt); err != nil {
		log.Fatalf("Erreur lors de la création du fichier: %v", err)
	}

	// Extraire uniquement le nom du fichier avec l'extension

	// Afficher le fichier et son chemin
	fmt.Println("Selected file:", filename)
	fmt.Println("File path:", j.fileDir)
	fmt.Println("File name with extension:", fileNameWithExt)

	// Mettre à jour les champs de la struct `job`
	j.fileName = fileNameWithExt        // Chemin complet du fichier sélectionné
	j.currentFileName = fileNameWithExt // Nom du fichier avec extension

	return nil
}

// createFile crée un fichier vide avec le nom spécifié
func (j *job) createFileWithPackage(filename string) error {
	file, err := os.Create(j.fileDir + "/" + filename)
	if err != nil {
		return err
	}
	defer file.Close()

	dirName := filepath.Base(j.fileDir)
	// Ajouter la ligne de package en haut du fichier
	_, err = file.WriteString(fmt.Sprintf("package %s\n\n", dirName))
	return err
}
