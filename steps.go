package main

import (
	"fmt"
	"strings"
)

// step is a wizard step.
type step string

const (
	stepDefault             step = "default"
	stepVerifyGoPrompt      step = "verifyGoPrompt"
	stepVerifyTestPrompt    step = "verifyTestPrompt"
	stepVerifySwaggerPrompt step = "stepVerifySwaggerPrompt"
	// stepProjectStructuring is a step to structure the project if folder empty.
	stepProjectStructuring step = "projectStructuring"
	stepStart              step = "start"
	stepStartTest          step = "startTest"
	stepOptimize           step = "optimize"
	stepAddTest            step = "tests"
	stepFinish             step = "finish"

	stepStartError    step = "startError"
	stepOptimizeError step = "optimizeError"

	stepAddTestError step = "addTestsError"
)

// StepWithError is a step with an associated error step.
type StepWithError struct {
	ValidStep step
	ErrorStep step
	Prompt    string
}

// stepsOrderDefault is an ordered list of steps for default files.
var stepsOrderDefault = []StepWithError{
	{ValidStep: stepVerifyGoPrompt},
	{ValidStep: stepProjectStructuring},
	{ValidStep: stepStart, ErrorStep: stepStartError},
	// {ValidStep: stepOptimize, ErrorStep: stepOptimizeError, Prompt: "Optimize this Golang code taking into account readability, performance, and best practices. Only change behavior if it can be improved for more efficient or safer use cases. Return optimizations made, without comment or explanation. Here is the code: \nHere is the Golang code:\n\n"},
	// {ValidStep: stepAddTest, ErrorStep: stepAddTestError},
}

// stepsOrderTest is an ordered list of steps for test files.
var stepsOrderTest = []StepWithError{
	//{ValidStep: stepVerifyTestPrompt},
	{ValidStep: stepStartTest, ErrorStep: stepAddTestError},
}

// stepsOrderSwagger is an ordered list of steps for swagger files.
var stepsOrderSwagger = []StepWithError{
	{ValidStep: stepVerifySwaggerPrompt},
	{ValidStep: stepStart, ErrorStep: stepStartError},
}

// getStepFromFileName return the steps to follow based on the file name.
func (j *job) getStepFromFileName() ([]StepWithError, error) {
	stepChoose := stepsOrderDefault
	switch {
	case strings.HasSuffix(j.fileName, "_test.go"):
		j.currentTestFileName = j.fileName
		j.currentSourceFileName = j.getSourceFileName(j.fileName)
		j.currentFileName = j.currentSourceFileName
		stepChoose = stepsOrderTest

	case strings.Contains(j.fileName, "swagger"):
		return stepsOrderSwagger, nil

	default:
		j.currentFileName = j.fileName
		j.currentSourceFileName = j.fileName
		{
			testFileName, err := j.getTestFilename()
			if err != nil {
				return stepChoose, err
			}
			j.currentTestFileName = testFileName
		}
	}

	/*{
		src, err := j.readFileContent(j.currentSourceFileName)
		if err != nil {
			return stepChoose, err
		}
		j.currentSrcSource = src
	}
	{
		src, err := j.createNewTestFile()
		if err != nil {
			return stepChoose, err
		}
		j.currentSrcTest = src
	}*/
	return stepChoose, nil
}

// getPromptForVerifyPrompt returns a prompt to check if the question is a Go code request.
func (j *job) getPromptForVerifyPrompt(prompt string) string {
	switch j.currentStep {
	case stepVerifyTestPrompt:
		return fmt.Sprintf(j.t("Responds with true or false in JSON. Is the following question a request for an enhancement related to Golang unit tests")+" : \"%s\" ?", prompt)
	case stepVerifySwaggerPrompt:
		return fmt.Sprintf(j.t("Responds with true or false in JSON. Is the following question a request for generating or updating a Swagger interface")+" : \"%s\" ?", prompt)
	default:
		return fmt.Sprintf(j.t("Responds with true or false in JSON. Is the following question a request for Go code")+" : \"%s\" ?", prompt)
	}
}

func (j *job) getPromptToAskProjectStructuring(prompt string) string {
	return j.t("You are an assistant specialized in software architecture") + ". " +
		j.t("A user wants to get code to fulfill a specific request which I will provide to you") + ". " +
		j.t("Before generating the solution, focus only on creating the project folder architecture based on best practices for this request") + "." + "\n\n" +
		j.t("Here is the user's request") + " :" + "\n\n" + prompt + "\n\n" +
		j.t("Here is the current structure of the project repository, given by the user") + " :" + "\n\n" + j.repoStructure + "\n\n" +
		j.t("Your answer must") + " : \n\n" +
		"- " + j.t("Suggest only folders and files to add or modify in the existing structure to meet the request") + "." + "\n\n" +
		"- " + j.t("Respect and complement the conventions already in place in the existing structure") + "." + "\n\n" +
		"- " + j.t("Follow recognized best practices for the language and type of project requested") + "." + "\n\n" +
		"- " + j.t("Be presented in tree form") + "." + "\n\n" +
		"- " + j.t("Example of expected format") + " :" + "\n\n" +
		"```bash" + `
	- /cmd             
	- /pkg 
	- /internal 
	- /configs
	- /scripts
	` + "```" + "\n\n" +
		j.t("Reply without comment or explanation")
}

// getPromptToAskTestsCreation returns a prompt to start the process.
func (j *job) getPromptToAskTestsCreation() string {

	fileContent := string(j.currentSrcTest)

	prompt := j.t("I have some Golang code") + ":"
	prompt += "\n\n" + string(fileContent)
	prompt += "\n\n" + j.t("I would like to enrich these functions with unit tests") + ":"
	prompt += "\n\n" + j.printTestsFuncName()
	prompt += "\n\n" + j.t("Can you generate the tests for the nominal cases as well as the error cases? My goal is to ensure comprehensive coverage, particularly for:\n\nExpected success scenarios (nominal cases)\nError handling scenarios\nPlease structure the tests to be easily readable, using t.Run to name each test case.")
	prompt += "\n\n" + j.t("Reply without comment or explanation")
	return prompt
}

func (j *job) getPromptToAskTestCorrection() string {
	prompt := j.t("Determines whether the problem is in the test file or the source file. Generates a concise response that specifies the file to modify in the form") +
		": \"MODIFY: <function or section name> (source <folder/filename.go>, not test file)\" or \"MODIFY: <function or section name> (test file)\"." +
		j.t("Then provide the corrected code in the form") + ": \"CODE: <corrected code>\".\n\n"

	fileContent := string(j.currentSrcTest)
	if len(fileContent) > 50 {
		prompt += ".\n\n" + j.t("Here is the Golang code") + " :\n\n" + fileContent
	}
	return prompt
}

// stepAddTestErrorProcessPrompt adds a prompt to handle errors when adding tests.
func (j *job) stepAddTestErrorProcessPrompt(output string) (string, error) {
	getFailedTests, err := j.getFailedTests(output)
	if err != nil {
		fmt.Println(j.t("Error when recovering failed tests"), err)
		return "", err
	}

	if getFailedTests == nil {
		fmt.Println(j.t("No test failed"))
		return "", nil
	}

	testCode, err := j.getTestCode(getFailedTests)
	if err != nil {
		fmt.Println("Error retrieving failed test code", err)
		return "", err
	}

	prompt := j.t("The following tests") + " \n\n" + testCode + "\n\n " +
		j.t("returned the following errors") + ": \n\n" +
		j.t("Error") + " : " + output + "\n\n" +
		j.t("Determines whether the problem is in the test file or the source file. Generates a concise response that specifies the file to modify in the form: \"MODIFY: <function or section name> (source file, not test file)\" or \"MODIFY: <function or section name> (test file)\"") + "." +
		j.t("Then provide the corrected code in the form: \"CODE: <corrected code>\"") + "." +
		j.t("responds without adding comments or explanations")

	return prompt, nil
}
