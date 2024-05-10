package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatGPTResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int      `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens,omitempty"`
	TotalTokens      int `json:"total_tokens"`
}

func promptAI(prompt string, chatGptToken string) (ChatGPTResponse, error) {
	requestData := map[string]interface{}{
		"messages":    []Message{{Role: "user", Content: prompt}},
		"model":       "gpt-3.5-turbo",
		"max_tokens":  80,
		"temperature": 0,
	}

	jsonData, err := json.Marshal(requestData)
	if err != nil {
		return ChatGPTResponse{}, err
	}

	apiKey := chatGptToken
	apiUrl := "https://api.openai.com/v1/chat/completions"
	req, err := http.NewRequest("POST", apiUrl, bytes.NewBuffer(jsonData))
	if err != nil {
		return ChatGPTResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return ChatGPTResponse{}, err
	}
	defer resp.Body.Close()

	// Read and parse the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ChatGPTResponse{}, err
	}

	var response ChatGPTResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return ChatGPTResponse{}, err
	}

	return response, nil
}

type GPTMovieInfo struct {
	Title string  `json:"title"`
	Year  string  `json:"year,omitempty"`
	Error *string `json:"error"`
}

func promptAiForMovieNameAndYear(fileName string, chatGptToken string) (string, string, error) {
	prompt := `
	I need you to provide the corrected movie name that could be scraped by IMDb/TMDb and year if it's present or known.
	The movie name may appear in Russian transliterated to latin form - in this case transliterate it to Russian/cyrillic.
	Try to maintain sanity, e.g. don't transliterate Padal to Падаль, it should be Падал instead.
	Do not translate an original Russian name to english or original english to russian.
	Or there may be the original movie name but with some spelling mistakes - correct the mistakes in such case.
	Answer in json format: { 
		"title": "correct movie title",
		"year": "1999", // or empty string if not provided
	 	"error": "provide null or error if movie title can't be determined." // don't return error if only year is not provided.
	}
	movie: "` + fileName + `"
	`

	response, err := promptAI(prompt, chatGptToken)
	if err != nil {
		return "", "", err
	}
	// Log("Raw Response:", response)

	// Extract the guessed movie name and year from the response
	message := response.Choices[0].Message.Content

	var movieInfo GPTMovieInfo
	if err := json.Unmarshal([]byte(message), &movieInfo); err != nil {
		return "", "", err
	}
	if movieInfo.Error != nil {
		return "", "", fmt.Errorf("%s", *movieInfo.Error)
	}

	return movieInfo.Title, movieInfo.Year, nil
}

func promptAiForCorrectedYoLetterUsage(fileName string, chatGptToken string) (string, error) {
	prompt := `
	I need you to proved the corrected movie name in original Cyrillic/Russian encoding but with correct usage of the letter 'ё' where 'е' is used instead of it
	Answer in json format: { 
		"title": "correct movie title",
	 	"error": "provide null or error if you can‘t answer for whatever reason."
	}
	movie: "` + fileName + `"
	`

	response, err := promptAI(prompt, chatGptToken)
	if err != nil {
		return "", err
	}
	// Log("Raw Response:", response)

	// Extract the guessed movie name from the response
	message := response.Choices[0].Message.Content

	var movieInfo GPTMovieInfo
	if err := json.Unmarshal([]byte(message), &movieInfo); err != nil {
		return "", err
	}
	if movieInfo.Error != nil {
		return "", fmt.Errorf("%s", *movieInfo.Error)
	}

	return movieInfo.Title, nil
}
