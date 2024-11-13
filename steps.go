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
	{ValidStep: stepAddTest, ErrorStep: stepAddTestError, Prompt: "J'ai un code Golang que j'aimerais enrichir avec des tests unitaires. Pouvez-vous me générer les tests pour les cas nominaux ainsi que pour les cas d'erreurs, sans commentaire ou explication ? Mon objectif est d'assurer une couverture complète, en particulier pour :\n\nLes scénarios de succès attendus (cas nominaux)\nLes scénarios de gestion des erreurs\nVeuillez structurer les tests pour être facilement lisibles, en utilisant t.Run pour nommer chaque cas de test. Fournissez des commentaires succincts pour expliquer chaque test. \nVoici le code Golang :\n\n"},
}
