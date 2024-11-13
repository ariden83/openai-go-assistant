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
	{ValidStep: stepOptimize, ErrorStep: stepOptimizeError, Prompt: "Optimize this Golang code taking into account readability, performance, and best practices. Only change behavior if it can be improved for more efficient or safer use cases. Return optimizations made, without comment or explanation. Here is the code: \nHere is the Golang code:\n\n"},
	{ValidStep: stepAddTest, ErrorStep: stepAddTestError, Prompt: "I have some Golang code that I would like to enrich with unit tests. Can you generate the tests for the nominal cases as well as the error cases, without comment or explanation? My goal is to ensure comprehensive coverage, particularly for:\n\nExpected success scenarios (nominal cases)\nError handling scenarios\nPlease structure the tests to be easily readable, using t.Run to name each test case. Provide succinct comments to explain each test. \nHere is the Golang code:"},
}
