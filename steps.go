package main

import (
	"fmt"
	"strings"
)

// step est une étape de l'assistant.
type step string

const (
	stepDefault             step = "default"
	stepVerifyGoPrompt      step = "verifyGoPrompt"
	stepVerifyTestPrompt    step = "verifyTestPrompt"
	stepVerifySwaggerPrompt step = "stepVerifySwaggerPrompt"
	stepStart               step = "start"
	stepOptimize            step = "optimize"
	stepAddTest             step = "tests"
	stepFinish              step = "finish"

	stepStartError    step = "startError"
	stepOptimizeError step = "optimizeError"

	stepAddTestError step = "addTestsError"
)

// StepWithError est une étape avec une étape d'erreur associée.
type StepWithError struct {
	ValidStep step
	ErrorStep step
	Prompt    string
}

// stepsOrderDefault est une liste ordonnée des étapes pour les fichiers par défaut.
var stepsOrderDefault = []StepWithError{
	{ValidStep: stepVerifyGoPrompt},
	{ValidStep: stepStart, ErrorStep: stepStartError},
	{ValidStep: stepOptimize, ErrorStep: stepOptimizeError, Prompt: "Optimize this Golang code taking into account readability, performance, and best practices. Only change behavior if it can be improved for more efficient or safer use cases. Return optimizations made, without comment or explanation. Here is the code: \nHere is the Golang code:\n\n"},
	{ValidStep: stepAddTest, ErrorStep: stepAddTestError},
}

// stepsOrderTest est une liste ordonnée des étapes pour les fichiers de test.
var stepsOrderTest = []StepWithError{
	{ValidStep: stepVerifyTestPrompt},
	{ValidStep: stepStart, ErrorStep: stepStartError},
}

// stepsOrderSwagger est une liste ordonnée des étapes pour les fichiers swagger.
var stepsOrderSwagger = []StepWithError{
	{ValidStep: stepVerifySwaggerPrompt},
	{ValidStep: stepStart, ErrorStep: stepStartError},
}

// getStepFromFileName retourne les étapes à suivre en fonction du nom du fichier.
func (j *job) getStepFromFileName() []StepWithError {
	switch {
	case strings.Contains(j.fileName, "test"):
		return stepsOrderTest
	case strings.Contains(j.fileName, "swagger"):
		return stepsOrderSwagger
	default:
		return stepsOrderDefault
	}
}

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
