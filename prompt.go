package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/manifoldco/promptui"
	log "github.com/sirupsen/logrus"
)

var (
	blue    func(a ...interface{}) string
	green   func(a ...interface{}) string
	magenta func(a ...interface{}) string
	red     func(a ...interface{}) string
)

// init initializes the blue function to display text in blue.
func init() {
	blue = color.New(color.FgBlue).SprintFunc()
	green = color.New(color.FgGreen).SprintFunc()
	magenta = color.New(color.FgMagenta).SprintFunc()
	red = color.New(color.FgRed).SprintFunc()
}

// waitingPrompt displays a prompt to ask the user if they want to continue.
func (j *job) waitingPrompt() {
	/*if !j.validateEachStep {
		return
	}*/
	prompt := promptui.Select{
		Label: j.t("Continue ?"),
		Items: []string{j.t("Yes"), j.t("No")},
	}

	_, result, err := prompt.Run()
	if err != nil {
		log.Fatalf(j.t("Error while entering")+" : %v", err)
	}

	switch result {
	case j.t("Yes"):
	case j.t("No"):
		fmt.Println(j.t("You chose to stop"))
		log.Fatal(j.t("Stopping the script"))
	default:
		fmt.Println(j.t("Option inconnue"))
	}
}

func (j *job) archiPrompt() map[string]string {
	return map[string]string{
		"role": "system",
		"content": j.t("Here is the current project tree") + ": " + j.repoStructure + ".\n\n" +
			j.t("Here is the main import path to use from root") + ": " + j.modulePath + ".",
	}
}

// prepareGoPrompt adds context to specify that the question is about Go code.
func (j *job) prepareGoPrompt(userPrompt string) string {
	goContextPrefix := j.t("Write code in Go to solve the following problem") + " :\n\n"

	goContextSuffix := ".\n\n" +
		j.t("In your response, for each part of the code returned, specify in which folder or file the code should be added (for example: `usecase/`, `model/`, `handler/`, etc.)") + ".\n\n" +
		j.t("Strictly use the following format for each part") + ": `**<folder/file.go>** <code ici>`.\n\n" +
		j.t("Reply without comment or explanation, only the code needed")

	return goContextPrefix + userPrompt + goContextSuffix
}

// loadFilesFromFolder reads files in the given directory.
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

// promptSelectAFileOrCreateANewOne asks the user to select an existing file or create a new file.
func (j *job) promptSelectAFileOrCreateANewOne(filesFound []string) error {
	confirmPrompt := promptui.Prompt{
		Label:     fmt.Sprintf(j.t("Files found in repo %s, create one"), j.fileDir),
		IsConfirm: true,
	}
	_, err := confirmPrompt.Run()
	if err == nil {
		return j.promptCreateANewFile()
	}

	return j.promptSelectExistentFile(filesFound)
}

// promptSelectExistentFile asks the user to select an existing file.
func (j *job) promptSelectExistentFile(filesFound []string) error {
	filePrompt := promptui.Select{
		Label: "Select a file",
		Items: filesFound,
	}

	_, selectedFile, err := filePrompt.Run()
	if err != nil {
		log.Fatalf(j.t("Error selecting file")+": %v", err)
	}

	log.Println("Selected file:", selectedFile)

	filePath := filepath.Dir(selectedFile)
	fileNameWithExt := filepath.Base(selectedFile)

	log.Println("Selected file:", selectedFile)
	log.Println("File path:", filePath)
	log.Println("File name with extension:", fileNameWithExt)

	j.fileName = fileNameWithExt        // Chemin complet du fichier sélectionné
	j.currentFileName = fileNameWithExt // Nom du fichier avec extension
	j.fileDir = filePath                // Chemin du dossier

	if err := j.setupGoMod(); err != nil {
		log.WithError(err).Error(j.t("Error configuring Go modules"))
		return err
	}

	j.fileName = strings.TrimPrefix(filePath, j.fileDir) + "/" + fileNameWithExt
	j.currentFileName = j.fileName

	log.Println("Filename:", j.fileName)

	return nil
}

// promptNoFilesFoundCreateANewFile asks the user to create a new file.
func (j *job) promptNoFilesFoundCreateANewFile() error {
	confirmPrompt := promptui.Prompt{
		Label:     j.t("No files found, create one"),
		IsConfirm: true,
	}

	_, err := confirmPrompt.Run()
	if err != nil {
		log.Println(j.t("File generation canceled"))
		return errors.New("end")
	}
	return j.promptCreateANewFile()
}

// getDirectories reads folders in the root directory.
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

// promptSelectOrCreateDirectory asks the user to select an existing folder or create a new folder.
func (j *job) promptSelectOrCreateDirectory() (string, error) {
	directories, err := j.getDirectories()
	if err != nil {
		return "", fmt.Errorf(j.t("error when reading folders")+": %v", err)
	}

	directories = append(directories, j.t("Create a new folder"))

	selectPrompt := promptui.Select{
		Label: j.t("Choose a folder"),
		Items: directories,
	}

	_, selectedDir, err := selectPrompt.Run()
	if err != nil {
		return "", fmt.Errorf(j.t("error while selecting folder")+": %v", err)
	}

	if selectedDir == j.t("Create a new folder") {
		pathPrompt := promptui.Prompt{
			Label: j.t("Enter the path of the new folder"),
			Validate: func(input string) error {
				if len(input) == 0 {
					return fmt.Errorf(j.t("the path cannot be empty"))
				}
				return nil
			},
		}
		selectedDir, err = pathPrompt.Run()
		if err != nil {
			return "", fmt.Errorf(j.t("error entering path")+": %v", err)
		}
	}

	return selectedDir, nil
}

// promptCreateANewFile asks the user to select a folder or create a new file.
func (j *job) promptCreateANewFile() error {
	log.Println(j.t("Select a folder or enter a new path") + " :")
	selectedDir, err := j.promptSelectOrCreateDirectory()
	if err != nil {
		return err
	}

	filenamePrompt := promptui.Prompt{
		Label: j.t("Enter the file name"),
		Validate: func(input string) error {
			if len(input) == 0 {
				return fmt.Errorf(j.t("file name cannot be empty"))
			}
			return nil
		},
		Templates: &promptui.PromptTemplates{
			Invalid: "{{ . | red }}",
		},
	}

	filename, err := filenamePrompt.Run()
	if err != nil {
		log.Fatalf(j.t("Error entering file name")+": %v", err)
	}

	filename = selectedDir + "/" + filename
	if !strings.HasSuffix(filename, ".go") {
		filename += ".go"
	}

	dir := filepath.Dir(filename)
	j.fileDir = "./" + strings.Trim(j.fileDir+"/"+dir, "/.")
	j.fileDirSelected = j.fileDir

	if dir != "." {
		if err := os.MkdirAll(j.fileDir, 0755); err != nil {
			log.Fatalf(j.t("Error creating folders")+": %v", err)
		}
	}

	// update j.fileDir
	if err := j.setupGoMod(); err != nil {
		log.WithError(err).Error(j.t("Error configuring Go modules"))
		return err
	}

	j.fileName = strings.TrimPrefix("./"+filename, j.fileDir)
	j.currentFileName = j.fileName

	if err = j.createFileWithPackage(j.fileName); err != nil {
		log.Fatalf(j.t("Error creating file")+": %v", err)
	}

	log.Println(j.t("Selected file")+":", filename)
	log.Println(j.t("File path")+":", j.fileDir)
	log.Println(j.t("Current file path")+":", j.fileDirSelected)
	log.Println(j.t("File name with extension")+":", j.currentFileName)

	// j.fileName = fileNameWithExt
	// j.currentFileName = fileNameWithExt

	return nil
}

// createFolders creates the necessary folders if the path contains a sub folder.
func (j *job) createFolders(file string) error {
	dir := filepath.Dir(file)
	if dir != "." {
		return os.MkdirAll(dir, 0755)
	}
	return nil
}

// createFileWithPackage creates a file with the given name and adds the package line.
func (j *job) createFileWithPackage(filename string) error {
	log.Infof("Creating file with package given filename %s %s", j.fileDir, filename)
	file, err := os.Create(j.fileDir + "/" + filename)
	if err != nil {
		return err
	}

	defer func() {
		if err = file.Close(); err != nil {
			log.Fatalf(j.t("Error closing file")+": %v", err)
		}
	}()

	packageName := sanitizePackageName(j.fileDir + "/" + filename)

	log.Infof("Creating file with package give packageName %s", packageName)
	_, err = file.WriteString(fmt.Sprintf("package %s\n\n", packageName))
	return err
}

// promptForQuery asks the user to enter a question or query.
func (j *job) promptForQuery() (string, error) {
	prompt := promptui.Prompt{
		Label: j.t("Enter your question or request to the OpenAI API"),
	}

	query, err := prompt.Run()
	if err != nil {
		return "", fmt.Errorf(j.t("error when entering question")+" : %v", err)
	}
	return query, nil
}
