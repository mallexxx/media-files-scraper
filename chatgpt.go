package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
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

func promptAI(prompt string, chatGptToken string, cacheKey string) (ChatGPTResponse, error) {
	// Use provided cacheKey for the cache file name
	cacheFilename := filepath.Join(CacheDir, "chatgpt", cacheKey+".json")
	chatgptCacheDir := filepath.Join(CacheDir, "chatgpt")

	// Ensure the chatgpt cache directory exists
	if !Path(chatgptCacheDir).isDirectory() {
		if err := os.MkdirAll(chatgptCacheDir, 0755); err != nil {
			Log("Error creating ChatGPT cache directory:", err)
		}
	}

	// Check if the cached data exists
	if _, err := os.Stat(cacheFilename); err == nil {
		// If cached data exists, read and return it
		if os.Getenv("TEST_MODE") == "true" {
			Log("🔄 TEST MODE: Using cached ChatGPT response for prompt", cacheKey)
		} else {
			Log("🔄 Using cached ChatGPT response for prompt", cacheKey)
		}

		data, err := os.ReadFile(cacheFilename)
		if err != nil {
			return ChatGPTResponse{}, err
		}

		var response ChatGPTResponse
		if err := json.Unmarshal(data, &response); err != nil {
			return ChatGPTResponse{}, err
		}

		return response, nil
	}

	// Cache miss - need to get a response
	// In test mode, create a simplified mock response instead of making API call
	if os.Getenv("TEST_MODE") == "true" {
		Log("💥💥💥 Making real ChatGPT API request (cache miss)")
	}

	// Real API call for production
	// Normal API call flow for non-test mode
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

	// Write the response to the cache file
	responseData, err := json.Marshal(response)
	if err != nil {
		Log("Error marshaling ChatGPT response for cache:", err)
	} else {
		if err := os.WriteFile(cacheFilename, responseData, 0644); err != nil {
			Log("Error writing ChatGPT cache file:", err)
		}
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

	// Create a cache key based on request type and movie name
	cacheKey := "movieNameAndYear-" + ReplaceInvalidFilenameChars(fileName)

	response, err := promptAI(prompt, chatGptToken, cacheKey)
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
	Provide the movie name in original Cyrillic/Russian encoding but with correct usage of the letter 'ё' where 'е' is used instead of it.
	If there is no letter 'е' to 'ё' conversion needed, leave the original name intact. Don't modify other letters.
	Answer in json format: { 
		"title": "correct movie title",
	 	"error": "provide null or error if you can't answer for whatever reason."
	}
	movie: "` + fileName + `"
	`

	// Create a cache key based on request type and movie name
	cacheKey := "correctYoUsage-" + ReplaceInvalidFilenameChars(fileName)

	response, err := promptAI(prompt, chatGptToken, cacheKey)
	if err != nil {
		return "", err
	}

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
