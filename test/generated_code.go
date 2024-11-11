package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/moul/http2curl"
)

// Définition des structures correspondant au JSON de réponse valide
type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type APIResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

func main() {
	responseJSON := `
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
				}`
	// Parse le JSON de réponse
	var apiResponse APIResponse
	err := json.
		Unmarshal([]byte(responseJSON), &apiResponse)
	if err !=
		nil {
		log.Fatalf("Erreur lors du parsing de la réponse JSON: %v",
			err)
	}
	data := bytes.NewBufferString(`{"hello":"world","answer":42}`)
	req, _ := http.NewRequest("PUT", "http://www.example.com/abc/def.ghi?jlk=mno&pqr=stu",
		data)
	req.Header.Set("Content-Type", "application/json")

	command, _ := http2curl.
		GetCurlCommand(req)
	fmt.Println(command)

	fmt.Println("ID:", apiResponse.ID)
	fmt.
		Println("Model:", apiResponse.Model)
	fmt.Println("Contenu du message:", apiResponse.
		Choices[0].Message.Content)
	fmt.Println("Nombre total de tokens utilisés:",

		apiResponse.
			Usage.TotalTokens)
}

func writeTest() {
	fmt.Println("toto")
}
