package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

func FetchURL(url string, headers map[string]string) ([]byte, error) {
	// Replace invalid characters in the URL with underscores
	validFilename := ReplaceInvalidFilenameChars(url) + ".txt"

	// Generate a cache filename based on the URL
	exePath, _ := os.Executable()
	cacheDir := filepath.Join(filepath.Dir(exePath), "cache")
	if !Path(cacheDir).isDirectory() {
		err := os.MkdirAll(cacheDir, 0755)
		if err != nil {
			return nil, err
		}
	}
	cacheFilename := filepath.Join(cacheDir, validFilename)

	// Check if the cached data exists
	if _, err := os.Stat(cacheFilename); err == nil {
		// If cached data exists, read and return it
		return os.ReadFile(cacheFilename)
	}

	// Otherwise, make an HTTP request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	// Set headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// Perform the HTTP request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP request %s failed with status: %d", url, resp.StatusCode)
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Write the response body to the cache file
	err = os.WriteFile(cacheFilename, body, 0644)
	if err != nil {
		fmt.Println("Error writing cache file:", err)
	}

	return body, nil
}
