package main

// Type et constantes pour les étapes et erreurs
type step string

const (
	stepVerifyPrompt step = "verifyPrompt"
	stepStart        step = "start"
	stepOptimize     step = "optimize"
	stepAddTest      step = "tests"
	stepFinish       step = "finish"

	stepStartError    step = "startError"
	stepOptimizeError step = "optimizeError"

	stepAddTestError step = "addTestsError"
)

// Définir la structure des étapes et leurs états d'erreur associés
type StepWithError struct {
	ValidStep step
	ErrorStep step
	Prompt    string
}

// Liste ordonnée des étapes et leurs erreurs associées
var stepsOrder = []StepWithError{
	{ValidStep: stepVerifyPrompt},
	{ValidStep: stepStart, ErrorStep: stepStartError},
	{ValidStep: stepOptimize, ErrorStep: stepOptimizeError, Prompt: "Optimize this Golang code taking into account readability, performance, and best practices. Only change behavior if it can be improved for more efficient or safer use cases. Return optimizations made, without comment or explanation. Here is the code: \nHere is the Golang code:\n\n"},
	{ValidStep: stepAddTest, ErrorStep: stepAddTestError},
}
