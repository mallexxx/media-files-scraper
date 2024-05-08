package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
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
		// torrentJSON, err := json.MarshalIndent(torrent, "", "  ")
		// if err != nil {
		// 	fmt.Println("Error encoding torrent to JSON:", err)
		// 	return nil, err
		// }
		// fmt.Println(string(torrentJSON))

		// Get the root directory of the torrent
		rootDir := filepath.Join(*torrent.DownloadDir, *torrent.Name)
		// Convert root directory to lower case for case-insensitive comparison
		lowerRootDir := strings.ToLower(rootDir)
		// Add the torrent to the map indexed by the lowercased root directory
		torrentMap[lowerRootDir] = torrent
	}
	return torrentMap, nil
}

func loadTitleYearIMDbIdFromRutracker(url string) (string, string, string, error) {
	_, err := parseTopicID(url)
	if err != nil {
		return "", "", "", err
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
	fmt.Printf("   Title: %s\n", title)

	title, year, err := extractTitleAndYearFromRutrackerTitle(title)
	if err != nil {
		return "", "", "", err
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
