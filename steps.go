package main

// Type et constantes pour les étapes et erreurs
type step string

const (
	stepStart    step = "start"
	stepOptimize step = "optimize"
	stepAddTest  step = "tests"
	stepFinish   step = "finish"

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
	{ValidStep: stepStart, ErrorStep: stepStartError, Prompt: ""},
	{ValidStep: stepOptimize, ErrorStep: stepOptimizeError, Prompt: "peux-tu optimiser le code suivant "},
	{ValidStep: stepAddTest, ErrorStep: stepAddTestError, Prompt: "peux-tu ajouter des tests associés au code suivant "},
}
