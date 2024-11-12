package main

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
		responseJSON := `
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
	}`

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
		responseJSON := `{"id": "chatcmpl-12345", "object": chat}` // JSON invalide
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
		data := bytes.NewBufferString(`{"hello":"world","answer":42}`)
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
	t.Run("Generate cURL command from HTTP request", func(t *testing.T) {
		data := bytes.NewBufferString(`{"hello":"world","answer":42}`)
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
	})
}
