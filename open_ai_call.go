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

// callIA calls the OpenAI API with the given prompt and returns the response.
func (j *job) callIA(prompt string) (string, error) {

	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return "", fmt.Errorf(j.t("empty prompt"))
	}

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
	req.Header.Set("Authorization", "Bearer "+string(j.openAIApiKey))

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

	if len(response.Choices) > 0 {
		// fmt.Println(fmt.Sprintf("openAI response details : %+v", response.Choices[0].Message.Content))
		code := j.extractBackticks(response.Choices[0].Message.Content)
		return strings.TrimSpace(code), nil
	}

	return "", fmt.Errorf(j.t("could not parse API response"))
}

func (j *job) responseToBool(messContent string) bool {
	responseText := strings.TrimSpace(messContent)
	return strings.ToLower(responseText) == "true"
}
