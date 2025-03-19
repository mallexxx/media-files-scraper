package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/hekmon/transmissionrpc/v3"
	"github.com/kazhuravlev/go-rutracker/parser"
)

func getTorrentsByPath(transmissionURL string) (map[string]transmissionrpc.Torrent, error) {
	endpoint, err := url.Parse(transmissionURL)
	if err != nil {
		panic(err)
	}

	// Initialize Transmission RPC client
	client, err := transmissionrpc.New(endpoint, nil)
	if err != nil {
		return nil, err
	}

	// Fetch all torrents
	// torrents, err := client.TorrentGetAll(context.Background())
	torrents, err := client.TorrentGet(context.Background(), []string{"id", "downloadDir", "name", "comment"}, nil)

	if err != nil {
		return nil, err
	}

	// Create a map to store torrents by lowercased file paths
	torrentMap := make(map[string]transmissionrpc.Torrent)
	for _, torrent := range torrents {
		// Get the root directory of the torrent
		rootDir := filepath.Join(*torrent.DownloadDir, *torrent.Name)
		// Convert root directory to lower case for case-insensitive comparison
		lowerRootDir := strings.ToLower(rootDir)
		// Add the torrent to the map indexed by the lowercased root directory
		torrentMap[lowerRootDir] = torrent
	}
	return torrentMap, nil
}

func moveTorrent(id int64, newLocation Path, transmissionURL string) error {
	endpoint, err := url.Parse(transmissionURL)
	if err != nil {
		panic(err)
	}

	// Initialize Transmission RPC client
	client, err := transmissionrpc.New(endpoint, nil)
	if err != nil {
		return err
	}

	return client.TorrentSetLocation(context.Background(), id, string(newLocation), true)
}

func loadTitleYearIMDbIdFromRutracker(url string) (string, string, string, error) {
	_, err := parseTopicID(url)
	if err != nil {
		return "", "", "", err
	}

	// Generate a cache key for this URL
	cacheKey := ReplaceInvalidFilenameChars(url) + "_rutracker.json"
	cacheFilename := filepath.Join(CacheDir, cacheKey)

	// Check if the cached data exists
	if _, err := os.Stat(cacheFilename); err == nil {
		// If cached data exists, read and return it
		Log("🔄 Using cached Rutracker data for URL:", url)
		data, err := os.ReadFile(cacheFilename)
		if err != nil {
			return "", "", "", err
		}

		var cachedResponse struct {
			Title  string `json:"title"`
			Year   string `json:"year"`
			IMDbID string `json:"imdb_id"`
		}
		if err := json.Unmarshal(data, &cachedResponse); err != nil {
			return "", "", "", err
		}

		return cachedResponse.Title, cachedResponse.Year, cachedResponse.IMDbID, nil
	}

	// Cache miss - making a real API call
	if os.Getenv("TEST_MODE") == "true" {
		Log("💥💥💥 TEST MODE: Making real Rutracker request (cache miss) for:", url)
	}

	// Fetch HTML content
	resp, err := http.Get(url)
	if err != nil {
		return "", "", "", err
	}
	defer resp.Body.Close()

	p, _ := parser.NewParser()

	topic, err := p.ParseTopicPage(resp.Body)
	if err != nil {
		return "", "", "", err
	}

	title, err := convertWindows1251ToUTF8(topic.Title)
	if err != nil {
		return "", "", "", err
	}
	Logf("   Title: %s\n", title)

	title, year, err := extractTitleAndYearFromRutrackerTitle(title)
	if err != nil {
		return "", "", "", err
	}
	Logf("   Cleaned: %s\n", title)

	// Cache the results
	cachedResponse := struct {
		Title  string `json:"title"`
		Year   string `json:"year"`
		IMDbID string `json:"imdb_id"`
	}{
		Title:  title,
		Year:   year,
		IMDbID: topic.IMDbID,
	}

	data, err := json.Marshal(cachedResponse)
	if err != nil {
		Log("Error marshaling Rutracker response for cache:", err)
	} else {
		if err := os.WriteFile(cacheFilename, data, 0644); err != nil {
			Log("Error writing Rutracker cache file:", err)
		}
	}

	return title, year, topic.IMDbID, nil
}

func parseTopicID(comment string) (string, error) {
	// Define a regular expression pattern to match Rutracker topic URLs
	pattern := `https://rutracker.org/forum/viewtopic.php\?t=(\d+)`
	// Compile the regular expression
	re := regexp.MustCompile(pattern)
	// Find submatches in the comment using the regular expression
	matches := re.FindStringSubmatch(comment)
	// If no matches found, return an error
	if len(matches) < 2 {
		return "", fmt.Errorf("no topic ID found in comment in \"%s\"", comment)
	}
	// Return the topic ID (first captured group)
	return matches[1], nil
}
