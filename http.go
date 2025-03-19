package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// CacheDir is the directory where API responses are cached
var CacheDir string

func init() {
	// Set default cache directory to be relative to the executable
	exePath, _ := os.Executable()
	CacheDir = filepath.Join(filepath.Dir(exePath), "cache")
}

func FetchURL(url string, headers map[string]string) ([]byte, error) {
	// Replace invalid characters in the URL with underscores
	validFilename := ReplaceInvalidFilenameChars(url) + ".txt"

	// Generate a cache filename based on the URL
	if !Path(CacheDir).isDirectory() {
		err := os.MkdirAll(CacheDir, 0755)
		if err != nil {
			return nil, err
		}
	}
	cacheFilename := filepath.Join(CacheDir, validFilename)

	// Check if the cached data exists
	if _, err := os.Stat(cacheFilename); err == nil {
		// If cached data exists, read and return it
		if os.Getenv("TEST_MODE") == "true" {
			Log("🔄 TEST MODE: Using cached data:", validFilename)
		} else {
			Log("🔄 Using cached data:", validFilename)
		}
		return os.ReadFile(cacheFilename)
	}

	// Cache miss - making a real HTTP request
	if os.Getenv("TEST_MODE") == "true" {
		Log("💥💥💥 TEST MODE: Making real HTTP request (cache miss) for:", url)
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

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// If status is not 200, return the error
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP request %s failed with status: %d", url, resp.StatusCode)
	}

	// Write the response body to the cache file
	err = os.WriteFile(cacheFilename, body, 0644)
	if err != nil {
		Log("Error writing cache file:", err)
	}

	// Cache successful response
	if os.Getenv("TEST_MODE") == "true" {
		Log("📥 TEST MODE: Cached response for future test runs:", url)
	}

	return body, nil
}
