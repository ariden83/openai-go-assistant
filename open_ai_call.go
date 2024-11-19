package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"moul.io/http2curl"
)

// Fonction pour envoyer une requête à l'API OpenAI
func (j *job) generateGolangCode(prompt string) (string, error) {

	requestBody := map[string]interface{}{
		"model": j.openAIModel,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"temperature": j.openAITemperature,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", j.openAIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+string(j.apiKey))

	client := &http.Client{}

	command, _ := http2curl.GetCurlCommand(req)
	fmt.Println(command)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var response APIResponse
	if err = json.Unmarshal(body, &response); err != nil {
		return "", err
	}

	if j.mockOpenAIResponse {
		j.mockOpenAI(&response)
	}

	if response.Error != nil {
		return "", fmt.Errorf("%s: %s", response.Error.Code, response.Error.Message)
	}

	// fmt.Println(fmt.Sprintf("openAI response : %+v", response))
	if len(response.Choices) > 0 {
		// fmt.Println(fmt.Sprintf("openAI response details : %+v", response.Choices[0].Message.Content))
		return response.Choices[0].Message.Content, nil
	}

	return "", fmt.Errorf(j.t("could not parse API response"))
}

func (j *job) responseToBool(messContent string) bool {
	responseText := strings.TrimSpace(messContent)
	return strings.ToLower(responseText) == "true"
}
