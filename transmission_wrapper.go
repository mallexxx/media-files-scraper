package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/hekmon/transmissionrpc/v3"
)

// getTorrentsByPathCached is a wrapper around getTorrentsByPath that uses caching in test mode
func getTorrentsByPathCached(transmissionURL string) (map[string]transmissionrpc.Torrent, error) {
	// In production mode, directly call the real function
	if os.Getenv("TEST_MODE") != "true" {
		return getTorrentsByPath(transmissionURL)
	}

	// In test mode, check for the test cache path first
	testCachePath := os.Getenv("TEST_TRANSMISSION_CACHE_PATH")
	if testCachePath != "" && Path(testCachePath).exists() {
		Log("🔄 Using test transmission cache from " + testCachePath)
		data, err := os.ReadFile(testCachePath)
		if err != nil {
			return nil, err
		}

		var torrentMap map[string]transmissionrpc.Torrent
		if err := json.Unmarshal(data, &torrentMap); err != nil {
			return nil, err
		}

		return torrentMap, nil
	}

	// Otherwise, use the regular cache file
	cacheFileName := filepath.Join(CacheDir, "transmission_torrents.json")

	// Check if the cached data exists
	if _, err := os.Stat(cacheFileName); err == nil {
		// If cached data exists, read and return it
		Log("🔄 Using cached Transmission torrents data")
		data, err := os.ReadFile(cacheFileName)
		if err != nil {
			return nil, err
		}

		var torrentMap map[string]transmissionrpc.Torrent
		if err := json.Unmarshal(data, &torrentMap); err != nil {
			return nil, err
		}

		return torrentMap, nil
	}

	// If no cache exists, call the real function
	Log("💥💥💥 TEST MODE: Making real Transmission API call (cache miss)")
	torrentMap, err := getRealTorrentsByPath(transmissionURL)
	if err != nil {
		return nil, err
	}

	// Cache the result
	data, err := json.MarshalIndent(torrentMap, "", "  ")
	if err != nil {
		return nil, err
	}

	if err := os.WriteFile(cacheFileName, data, 0644); err != nil {
		Log("Error writing transmission cache file:", err)
	}

	return torrentMap, nil
}

// getRealTorrentsByPath is the real implementation of getTorrentsByPath
func getRealTorrentsByPath(transmissionURL string) (map[string]transmissionrpc.Torrent, error) {
	endpoint, err := url.Parse(transmissionURL)
	if err != nil {
		return nil, err
	}

	// Initialize Transmission RPC client
	client, err := transmissionrpc.New(endpoint, nil)
	if err != nil {
		return nil, err
	}

	// Fetch all torrents
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

// moveTorrentCached is a wrapper around moveTorrent that uses caching in test mode
func moveTorrentCached(id int64, newLocation Path, transmissionURL string) error {
	// In production mode, directly call the real function
	if os.Getenv("TEST_MODE") != "true" {
		Log("🌐 Making real Transmission moveTorrent API call in production mode")
		return moveTorrent(id, newLocation, transmissionURL)
	}

	// In test mode, just log the action and return success
	Log(fmt.Sprintf("🧪 TEST MODE: Simulating moving torrent %d to %s (no real API call)", id, newLocation))
	return nil
}
