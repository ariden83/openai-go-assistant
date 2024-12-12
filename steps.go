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
	stepStart               step = "start"
	stepStartTest           step = "startTest"
	stepOptimize            step = "optimize"
	stepAddTest             step = "tests"
	stepFinish              step = "finish"

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
	{ValidStep: stepStart, ErrorStep: stepStartError},
	{ValidStep: stepOptimize, ErrorStep: stepOptimizeError, Prompt: "Optimize this Golang code taking into account readability, performance, and best practices. Only change behavior if it can be improved for more efficient or safer use cases. Return optimizations made, without comment or explanation. Here is the code: \nHere is the Golang code:\n\n"},
	{ValidStep: stepAddTest, ErrorStep: stepAddTestError},
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

	{
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
	}
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
