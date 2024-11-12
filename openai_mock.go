package main

func (j *job) mockOpenAI(response *APIResponse) {
	response.Error = nil
	if j.currentStep == stepStart {
		response.Choices = []Choice{
			{
				Message: Message{
					Content: j.mockOpenAIStepStart(),
				},
			},
		}

	} else if j.currentStep == stepStartError {
		response.Choices = []Choice{
			{
				Message: Message{
					Content: j.mockOpenAIStepStartError(),
				},
			},
		}

	} else if j.currentStep == stepOptimize {
		response.Choices = []Choice{
			{
				Message: Message{
					Content: j.mockOpenAIStepOptimize(),
				},
			},
		}
	} else if j.currentStep == stepAddTest {
		response.Choices = []Choice{
			{
				Message: Message{
					Content: j.mockOpenAIStepAddTest(),
				},
			},
		}
	} else if j.currentStep == stepAddTestError {
		response.Choices = []Choice{
			{
				Message: Message{
					Content: j.mockOpenAIStepAddTestError(),
				},
			},
		}
	}
}

func (j *job) mockOpenAIStepStart() string {
	return `package main

import (
	"encoding/json"
	"fmt"
	"log"
)

// Définition des structures correspondant au JSON de réponse valide
type Choice struct {
	Index        int    ` + "`" + `json:"index"` + "`" + `
	Message      Message ` + "`" + `json:"message"` + "`" + `
	FinishReason string ` + "`" + `json:"finish_reason"` + "`" + `
}

type Message struct {
	Role    string ` + "`" + `json:"role"` + "`" + `
	Content string ` + "`" + `json:"content"` + "`" + `
}

type Usage struct {
	PromptTokens     int ` + "`" + `json:"prompt_tokens"` + "`" + `
	CompletionTokens int ` + "`" + `json:"completion_tokens"` + "`" + `
	TotalTokens      int ` + "`" + `json:"total_tokens"` + "`" + `
}

type APIResponse struct {
	ID      string   ` + "`" + `json:"id"` + "`" + `
	Object  string   ` + "`" + `json:"object"` + "`" + `
	Created int64    ` + "`" + `json:"created"` + "`" + `
	Model   string   ` + "`" + `json:"model"` + "`" + `
	Choices []Choice ` + "`" + `json:"choices"` + "`" + `
	Usage   Usage    ` + "`" + `json:"usage"` + "`" + `
}

func main() {
	if err := godotenv.Load(); err != nil {
		fmt.Println("Erreur de chargement du fichier .env")
	}

	// Exemple de JSON de réponse valide
	responseJSON := ` + "`" + `
			{
				"id": "chatcmpl-12345",
				"object": "chat.completion",
				"created": 1689200300,
				"model": "gpt-3.5-turbo",
				"choices": [
			{
				"index": 0,
				"message": {
				"role": "assistant",
				"content": "This is a test response from the assistant."
			},
				"finish_reason": "stop"
			}
			],
				"usage": {
				"prompt_tokens": 10,
				"completion_tokens": 20,
				"total_tokens": 30
			}
			}` + "`" + `

	// Parse le JSON de réponse
	var apiResponse APIResponse
	err := json.Unmarshal([]byte(responseJSON), &apiResponse)
	if err != nil {
		log.Fatalf("Erreur lors du parsing de la réponse JSON: %v", err)
	}

	data := bytes.NewBufferString(` + "`" + `{"hello":"world","answer":42}` + "`" + `)
	req, _ := http.NewRequest("PUT", "http://www.example.com/abc/def.ghi?jlk=mno&pqr=stu", data)
	req.Header.Set("Content-Type", "application/json")

	command, _ := http2curl.GetCurlCommand(req)
	fmt.Println(command)

	// Affichage des informations extraites
	fmt.Println("ID:", apiResponse.ID)
	fmt.Println("Model:", apiResponse.Model)
	fmt.Println("Contenu du message:", apiResponse.Choices[0].Message.Content)
	fmt.Println("Nombre total de tokens utilisés:", apiResponse.Usage.TotalTokens)

	writeTest()
}

func writeTest() {
		fmt.Println("toto")
}

func unusedFunc() {
		fmt.Println("toto")
}
`
}

func (j *job) mockOpenAIStepStartError() string {
	return `package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"moul.io/http2curl"
	"github.com/joho/godotenv"
)

type APIResponse struct {
	ID       string ` + "`" + `json:"id"` + "`" + `
	Object   string ` + "`" + `json:"object"` + "`" + `
	Created  int    ` + "`" + `json:"created"` + "`" + `
	Model    string ` + "`" + `json:"model"` + "`" + `
	Choices  []struct {
		Index   int ` + "`" + `json:"index"` + "`" + `
		Message struct {
			Role    string ` + "`" + `json:"role"` + "`" + `
			Content string ` + "`" + `json:"content"` + "`" + `
		} ` + "`" + `json:"message"` + "`" + `
		FinishReason string ` + "`" + `json:"finish_reason"` + "`" + `
	} ` + "`" + `json:"choices"` + "`" + `
	Usage struct {
		PromptTokens     int ` + "`" + `json:"prompt_tokens"` + "`" + `
		CompletionTokens int ` + "`" + `json:"completion_tokens"` + "`" + `
		TotalTokens      int ` + "`" + `json:"total_tokens"` + "`" + `
	} ` + "`" + `json:"usage"` + "`" + `
}

func main() {
	if err := godotenv.Load(); err != nil {
		fmt.Println("Erreur de chargement du fichier .env")
	}

	// JSON de réponse simulée
	responseJSON := ` + "`" + `
				{
					"id": "chatcmpl-12345",
					"object": "chat.completion",
					"created": 1689200300,
					"model": "gpt-3.5-turbo",
					"choices": [
				{
					"index": 0,
					"message": {
					"role": "assistant",
					"content": "This is a test response from the assistant."
				},
					"finish_reason": "stop"
				}
				],
					"usage": {
					"prompt_tokens": 10,
					"completion_tokens": 20,
					"total_tokens": 30
				}
				}` + "`" + `

	// Parse le JSON de réponse
	var apiResponse APIResponse
	err := json.Unmarshal([]byte(responseJSON), &apiResponse)
	if err != nil {
		log.Fatalf("Erreur lors du parsing de la réponse JSON: %v", err)
	}

	// Préparer la requête HTTP
	data := bytes.NewBufferString(` + "`" + `{"hello":"world","answer":42}` + "`" + `)
	req, _ := http.NewRequest("PUT", "http://www.example.com/abc/def.ghi?jlk=mno&pqr=stu", data)
	req.Header.Set("Content-Type", "application/json")

	// Utiliser http2curl pour obtenir la commande cURL correspondante
	command, _ := http2curl.GetCurlCommand(req)
	fmt.Println(command)

	// Afficher les informations de la réponse API
	fmt.Println("ID:", apiResponse.ID)
	fmt.Println("Model:", apiResponse.Model)
	fmt.Println("Contenu du message:", apiResponse.Choices[0].Message.Content)
	fmt.Println("Nombre total de tokens utilisés:", apiResponse.Usage.TotalTokens)
}
`
}

func (j *job) mockOpenAIStepOptimize() string {
	return `package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"moul.io/http2curl"
	"github.com/joho/godotenv"
)

// Structures correspondant au JSON de réponse
type Choice struct {
	Index        int     ` + "`" + `json:"index"` + "`" + `
	Message      Message ` + "`" + `json:"message"` + "`" + `
	FinishReason string  ` + "`" + `json:"finish_reason"` + "`" + `
}

type Message struct {
	Role    string ` + "`" + `json:"role"` + "`" + `
	Content string ` + "`" + `json:"content"` + "`" + `
}

type Usage struct {
	PromptTokens     int ` + "`" + `json:"prompt_tokens"` + "`" + `
	CompletionTokens int ` + "`" + `json:"completion_tokens"` + "`" + `
	TotalTokens      int ` + "`" + `json:"total_tokens"` + "`" + `
}

type APIResponse struct {
	ID      string   ` + "`" + `json:"id"` + "`" + `
	Object  string   ` + "`" + `json:"object"` + "`" + `
	Created int64    ` + "`" + `json:"created"` + "`" + `
	Model   string   ` + "`" + `json:"model"` + "`" + `
	Choices []Choice ` + "`" + `json:"choices"` + "`" + `
	Usage   Usage    ` + "`" + `json:"usage"` + "`" + `
}

func main() {
	if err := godotenv.Load(); err != nil {
		fmt.Println("Erreur de chargement du fichier .env")
	}

	// Exemple de réponse JSON
	responseJSON := ` + "`" + `
	{
		"id": "chatcmpl-12345",
		"object": "chat.completion",
		"created": 1689200300,
		"model": "gpt-3.5-turbo",
		"choices": [
	{
	"index": 0,
	"message": {
	"role": "assistant",
	"content": "This is a test response from the assistant."
	},
	"finish_reason": "stop"
	}
	],
	"usage": {
	"prompt_tokens": 10,
	"completion_tokens": 20,
	"total_tokens": 30
	}
	}` + "`" + `

	// Parse le JSON de réponse
	var apiResponse APIResponse
	err := json.Unmarshal([]byte(responseJSON), &apiResponse)
	if err != nil {
		log.Fatalf("Erreur lors du parsing de la réponse JSON: %v", err)
	}

	// Préparation de la requête HTTP
	data := bytes.NewBufferString(` + "`" + `{"hello":"world","answer":42}` + "`" + `)
	req, err := http.NewRequest("PUT", "http://www.example.com/abc/def.ghi?jlk=mno&pqr=stu", data)
	if err != nil {
		log.Fatalf("Erreur lors de la création de la requête HTTP: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Génération de la commande cURL
	command, err := http2curl.GetCurlCommand(req)
	if err != nil {
		log.Fatalf("Erreur lors de la génération de la commande cURL: %v", err)
	}
	fmt.Println("Commande cURL générée:", command)

	// Affichage des données extraites du JSON
	fmt.Println("ID:", apiResponse.ID)
	fmt.Println("Model:", apiResponse.Model)
	fmt.Println("Contenu du message:", apiResponse.Choices[0].Message.Content)
	fmt.Println("Nombre total de tokens utilisés:", apiResponse.Usage.TotalTokens)
}
`
}

func (j *job) mockOpenAIStepAddTest() string {
	return `package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"moul.io/http2curl"
)

// Test de l'analyse JSON avec des données valides
func TestParseValidJSONResponse(t *testing.T) {
	t.Run("Parse valid JSON response", func(t *testing.T) {
		responseJSON := ` + "`" + `
	{
		"id": "chatcmpl-12345",
		"object": "chat.completion",
		"created": 1689200300,
		"model": "gpt-3.5-turbo",
		"choices": [{
	"index": 0,
	"message": {"role": "assistant", "content": "This is a test response from the assistant."},
	"finish_reason": "stop"
	}],
	"usage": {"prompt_tokens": 10, "completion_tokens": 20, "total_tokens": 30}
	}` + "`" + `
		
		var apiResponse APIResponse
		err := json.Unmarshal([]byte(responseJSON), &apiResponse)
		
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if apiResponse.ID != "chatcmpl-12345" {
			t.Errorf("Expected ID to be 'chatcmpl-12345', got %s", apiResponse.ID)
		}
		if apiResponse.Choices[0].Message.Content != "This is a test response from the assistant." {
			t.Errorf("Expected message content to match, got %s", apiResponse.Choices[0].Message.Content)
		}
		if apiResponse.Usage.TotalTokens != 30 {
			t.Errorf("Expected total tokens to be 30, got %d", apiResponse.Usage.TotalTokens)
		}
	})
}

// Test de l'analyse JSON avec des données invalides
func TestParseInvalidJSONResponse(t *testing.T) {
	t.Run("Parse invalid JSON response", func(t *testing.T) {
		responseJSON := ` + "`" + `{"id": "chatcmpl-12345", "object": chat}` + "`" + ` // JSON invalide
		var apiResponse APIResponse
		err := json.Unmarshal([]byte(responseJSON), &apiResponse)

		if err == nil {
			t.Fatalf("Expected error for invalid JSON, got none")
		}
	})
}

// Test de la génération de requêtes HTTP
func TestHTTPRequestGeneration(t *testing.T) {
	t.Run("Generate valid HTTP request", func(t *testing.T) {
		data := bytes.NewBufferString(` + "`" + `{"hello":"world","answer":42}` + "`" + `)
		req, err := http.NewRequest("PUT", "http://www.example.com/abc/def.ghi?jlk=mno&pqr=stu", data)

		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if req.Method != http.MethodPut {
			t.Errorf("Expected method PUT, got %s", req.Method)
		}
		if req.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type 'application/json', got %s", req.Header.Get("Content-Type"))
		}
	})
}

// Test de génération de commande cURL à partir d'une requête HTTP
func TestCurlCommandGeneration(t *testing.T) {
		data := bytes.NewBufferString(` + "`" + `{"hello":"world","answer":42}` + "`" + `)
		req, _ := http.NewRequest("PUT", "http://www.example.com/abc/def.ghi?jlk=mno&pqr=stu", data)
		req.Header.Set("Content-Type", "application/json")

		command, err := http2curl.GetCurlCommand(req)
		if err != nil {
			t.Fatalf("Expected no error generating cURL command, got %v", err)
		}

		expectedSnippet := "curl -X PUT http://www.example.com/abc/def.ghi?jlk=mno&pqr=stu"
		if command.String()[:len(expectedSnippet)] != expectedSnippet {
			t.Errorf("Expected command to start with %q, got %q", expectedSnippet, command)
		}
}
`
}

func (j *job) mockOpenAIStepAddTestError() string {
	return `func TestHTTPRequestGeneration(t *testing.T) {
	t.Run("Generate valid HTTP request", func(t *testing.T) {
		data := bytes.NewBufferString(` + "`" + `{"hello":"world","answer":42}` + "`" + `)
		req, err := http.NewRequest("PUT", "http://www.example.com/abc/def.ghi?jlk=mno&pqr=stu", data)
		req.Header.Set("Content-Type", "application/json")

		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if req.Method != http.MethodPut {
			t.Errorf("Expected method PUT, got %s", req.Method)
		}
		if req.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type 'application/json', got %s", req.Header.Get("Content-Type"))
		}
	})
}

func TestCurlCommandGeneration(t *testing.T) {
		data := bytes.NewBufferString(` + "`" + `{"hello":"world","answer":42}` + "`" + `)
		req, _ := http.NewRequest("PUT", "http://www.example.com/abc/def.ghi?jlk=mno&pqr=stu", data)
		req.Header.Set("Content-Type", "application/json")

		command, err := http2curl.GetCurlCommand(req)
		if err != nil {
			t.Fatalf("Expected no error generating cURL command, got %v", err)
		}

		expectedSnippet := "curl -X 'PUT'"
		if command.String()[:len(expectedSnippet)] != expectedSnippet {
			t.Errorf("Expected command to start with %q, got %q", expectedSnippet, command)
		}
}
`
}
