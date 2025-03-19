package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/hekmon/transmissionrpc/v3"
)

var testBaseDir = "testdata/mediadb_test"
var testMediaDir = filepath.Join(testBaseDir, "media")
var testCacheDir = filepath.Join(testBaseDir, "cache")
var testExpectedDir = filepath.Join(testBaseDir, "expected")
var testOutputDir = filepath.Join(testBaseDir, "out")
var testUnsortedDir = filepath.Join(testBaseDir, "unsorted")

// setupTestDirectories creates the test directory structure
func setupTestDirectories() error {
	dirs := []string{
		filepath.Join(testMediaDir, "Movies"),
		filepath.Join(testMediaDir, "Series"),
		testCacheDir,
		filepath.Join(testOutputDir, "movies"),
		filepath.Join(testOutputDir, "series"),
		filepath.Join(testExpectedDir, "movies"),
		filepath.Join(testExpectedDir, "series"),
		testUnsortedDir,
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return nil
}

// cleanOutputDirectory removes all files from the output directory
func cleanOutputDirectory() error {
	// Remove and recreate output directories to ensure they're clean
	if err := os.RemoveAll(testOutputDir); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Join(testOutputDir, "movies"), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(testOutputDir, "series"), 0755); err != nil {
		return err
	}

	return nil
}

// mockFindSuitableDirectoryForSymlink is a test version of findSuitableDirectoryForSymlink
// that ignores volume checks and always returns the first available directory
func mockFindSuitableDirectoryForSymlink(path Path, directories []Path) Path {
	if len(directories) == 0 {
		return Path("")
	}
	return directories[0]
}

// validateNfoContent validates the content of an NFO file
func validateNfoContent(t *testing.T, actualPath, expectedPath string) error {
	// Read both files
	actualContent, err := os.ReadFile(actualPath)
	if err != nil {
		return fmt.Errorf("❌ error reading actual NFO file: %v", err)
	}

	expectedContent, err := os.ReadFile(expectedPath)
	if err != nil {
		return fmt.Errorf("❌ error reading expected NFO file: %v", err)
	}

	// Parse both XML contents
	var actualMovie, expectedMovie struct {
		Title    string `xml:"title"`
		UniqueID struct {
			Type  string `xml:"type,attr"`
			Value string `xml:",chardata"`
		} `xml:"uniqueid"`
		OriginalTitle string   `xml:"originaltitle"`
		Plot          string   `xml:"plot"`
		Year          string   `xml:"year"`
		Genres        []string `xml:"genre"`
		TMDBURL       string   `xml:"tmdburl"`
	}

	if err := xml.Unmarshal(actualContent, &actualMovie); err != nil {
		return fmt.Errorf("❌ error parsing actual NFO XML: %v", err)
	}

	if err := xml.Unmarshal(expectedContent, &expectedMovie); err != nil {
		return fmt.Errorf("❌ error parsing expected NFO XML: %v", err)
	}

	// Compare fields
	if actualMovie.Title != expectedMovie.Title {
		return fmt.Errorf("❌ title mismatch: got %s, want %s", actualMovie.Title, expectedMovie.Title)
	}
	if actualMovie.UniqueID.Type != expectedMovie.UniqueID.Type {
		return fmt.Errorf("❌ uniqueid type mismatch: got %s, want %s", actualMovie.UniqueID.Type, expectedMovie.UniqueID.Type)
	}
	if actualMovie.UniqueID.Value != expectedMovie.UniqueID.Value {
		return fmt.Errorf("❌ uniqueid value mismatch: got %s, want %s", actualMovie.UniqueID.Value, expectedMovie.UniqueID.Value)
	}
	if actualMovie.OriginalTitle != expectedMovie.OriginalTitle {
		return fmt.Errorf("❌ originaltitle mismatch: got %s, want %s", actualMovie.OriginalTitle, expectedMovie.OriginalTitle)
	}
	if actualMovie.Plot != expectedMovie.Plot {
		return fmt.Errorf("❌ plot mismatch: got %s, want %s", actualMovie.Plot, expectedMovie.Plot)
	}
	if actualMovie.Year != expectedMovie.Year {
		return fmt.Errorf("❌ year mismatch: got %s, want %s", actualMovie.Year, expectedMovie.Year)
	}
	if len(actualMovie.Genres) != len(expectedMovie.Genres) {
		return fmt.Errorf("❌ genres count mismatch: got %d, want %d", len(actualMovie.Genres), len(expectedMovie.Genres))
	}
	if actualMovie.TMDBURL != expectedMovie.TMDBURL {
		return fmt.Errorf("❌ tmdburl mismatch: got %s, want %s", actualMovie.TMDBURL, expectedMovie.TMDBURL)
	}

	return nil
}

// compareDirectories compares the content of two directories
func compareDirectories(t *testing.T, expected, actual string) {
	t.Logf("Comparing directories: %s and %s", expected, actual)

	// Check if expected directory exists
	expectedInfo, err := os.Stat(expected)
	if os.IsNotExist(err) {
		t.Logf("Expected directory %s doesn't exist, skipping comparison", expected)
		return
	}
	if err != nil {
		t.Errorf("Error checking expected directory: %v", err)
		return
	}
	if !expectedInfo.IsDir() {
		t.Errorf("Expected path is not a directory: %s", expected)
		return
	}

	// Check if actual directory exists
	actualInfo, err := os.Stat(actual)
	if os.IsNotExist(err) {
		t.Errorf("Output directory %s doesn't exist", actual)
		return
	}
	if err != nil {
		t.Errorf("Error checking output directory: %v", err)
		return
	}
	if !actualInfo.IsDir() {
		t.Errorf("Output path is not a directory: %s", actual)
		return
	}

	// Walk through expected directory and check if files/directories exist in actual
	filepath.Walk(expected, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			t.Errorf("Error walking expected directory: %v", err)
			return nil
		}

		// skip .DS_Store
		if info.Name() == ".DS_Store" {
			return nil
		}

		// Get the path relative to the expected directory
		relPath, err := filepath.Rel(expected, path)
		if err != nil {
			t.Errorf("Error getting relative path: %v", err)
			return nil
		}

		if relPath == "." {
			return nil // Skip the root directory
		}

		// Check if the file/directory exists in the actual directory
		actualPath := filepath.Join(actual, relPath)
		actualInfo, err := os.Stat(actualPath)
		if os.IsNotExist(err) {
			t.Errorf("Expected file/directory doesn't exist in output: %s", relPath)
			return nil
		}
		if err != nil {
			t.Errorf("Error checking output file/directory: %v", err)
			return nil
		}

		// If it's a directory, just check existence
		if info.IsDir() {
			if !actualInfo.IsDir() {
				t.Errorf("Expected directory is a file in output: %s", relPath)
			}
			return nil
		}

		// If it's a file, we can optionally check the content
		if actualInfo.IsDir() {
			t.Errorf("Expected file is a directory in output: %s", relPath)
		}

		// Validate NFO file content if it's an NFO file
		if strings.HasSuffix(relPath, ".nfo") {
			if err := validateNfoContent(t, actualPath, path); err != nil {
				t.Errorf("NFO content validation failed for %s: %v", relPath, err)
			}
		}

		return nil
	})

	// Optionally, check for extra files in the actual directory
	// that aren't in the expected directory
	if t.Failed() {
		return // Skip this if we already have errors
	}

	filepath.Walk(actual, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		// Get the path relative to the actual directory
		relPath, err := filepath.Rel(actual, path)
		if err != nil {
			return nil
		}

		if relPath == "." {
			return nil // Skip the root directory
		}

		// Check if the file/directory exists in the expected directory
		expectedPath := filepath.Join(expected, relPath)
		_, err = os.Stat(expectedPath)
		if os.IsNotExist(err) {
			// This is not necessarily an error, but we can log it
			t.Logf("Extra file/directory in output (not in expected): %s", relPath)
		}

		return nil
	})
}

// TestRunMediaSyncSimple tests the mediaSync functionality with a simplified approach
func TestRunMediaSyncSimple(t *testing.T) {
	// Initialize logger for the test
	logFile, err := os.OpenFile("test.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("Failed to open log file: %v", err)
	}
	defer logFile.Close()

	logger = log.New(io.MultiWriter(os.Stdout, logFile), "", log.LstdFlags)

	// Load real config to get API keys
	realConfig, err := LoadConfig("")
	if err != nil {
		t.Fatalf("Failed to load real config: %v", err)
	}

	// Set up test environment
	testDir := "testdata/mediadb_test"
	testCacheDir := filepath.Join(testDir, "cache")

	// Create test directories
	err = os.MkdirAll(filepath.Join(testDir, "media"), 0755)
	if err != nil {
		t.Fatal(err)
	}
	err = os.MkdirAll(filepath.Join(testDir, "out/movies"), 0755)
	if err != nil {
		t.Fatal(err)
	}
	err = os.MkdirAll(filepath.Join(testDir, "out/series"), 0755)
	if err != nil {
		t.Fatal(err)
	}
	err = os.MkdirAll(filepath.Join(testDir, "expected/movies"), 0755)
	if err != nil {
		t.Fatal(err)
	}
	err = os.MkdirAll(filepath.Join(testDir, "expected/series"), 0755)
	if err != nil {
		t.Fatal(err)
	}
	err = os.MkdirAll(filepath.Join(testDir, "unsorted"), 0755)
	if err != nil {
		t.Fatal(err)
	}

	// Set up test cache directory
	err = os.MkdirAll(testCacheDir, 0755)
	if err != nil {
		t.Fatal(err)
	}

	// Create necessary subdirectories in the cache
	for _, subdir := range []string{"chatgpt"} {
		err = os.MkdirAll(filepath.Join(testCacheDir, subdir), 0755)
		if err != nil {
			t.Fatal(err)
		}
	}

	// Copy cache files from real cache directory
	realCacheDir := "cache"
	err = filepath.WalkDir(realCacheDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		relPath, err := filepath.Rel(realCacheDir, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(testCacheDir, relPath)
		err = os.MkdirAll(filepath.Dir(dstPath), 0755)
		if err != nil {
			return err
		}
		return copyFile(path, dstPath)
	})
	if err != nil {
		t.Fatal(err)
	}

	// Set the cache directory for testing
	originalCacheDir := CacheDir
	CacheDir = testCacheDir
	defer func() {
		CacheDir = originalCacheDir
	}()

	// Set up test environment variables
	os.Setenv("TEST_MODE", "true")

	// Skip test in short mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup test directories
	if err := setupTestDirectories(); err != nil {
		t.Fatalf("Failed to set up test directories: %v", err)
	}

	// Clean output directory
	if err := cleanOutputDirectory(); err != nil {
		t.Fatalf("Failed to clean output directory: %v", err)
	}

	// Create absolute paths for media and output to ensure they're on the same volume
	absTestMediaDir, err := filepath.Abs(testMediaDir)
	if err != nil {
		t.Fatalf("Failed to get absolute path for media dir: %v", err)
	}

	absTestOutputMoviesDir, err := filepath.Abs(filepath.Join(testOutputDir, "movies"))
	if err != nil {
		t.Fatalf("Failed to get absolute path for output movies dir: %v", err)
	}

	absTestOutputSeriesDir, err := filepath.Abs(filepath.Join(testOutputDir, "series"))
	if err != nil {
		t.Fatalf("Failed to get absolute path for output series dir: %v", err)
	}

	absTestUnsortedDir, err := filepath.Abs(testUnsortedDir)
	if err != nil {
		t.Fatalf("Failed to get absolute path for unsorted dir: %v", err)
	}

	absTestCacheDir, err := filepath.Abs(testCacheDir)
	if err != nil {
		t.Fatalf("Failed to get absolute path for cache dir: %v", err)
	}

	// Log directories for debugging
	mediaVolume := filepath.VolumeName(absTestMediaDir)
	outputVolume := filepath.VolumeName(absTestOutputMoviesDir)
	t.Logf("Media volume: %s, Output volume: %s", mediaVolume, outputVolume)
	t.Logf("Using cache directory: %s", absTestCacheDir)

	// Create a sorting rule for the test
	sortingRule := TorrentSortingRule{
		GenreRegexStr: "Animation|Kids|мультфильм|детский",
		Destination:   Path(filepath.Join(absTestMediaDir, "Movies")),
	}
	sortingRule.GenreRegex, _ = regexp.Compile(sortingRule.GenreRegexStr)

	// Create test configuration with absolute paths to ensure same volume
	config := Config{
		Directories: []Path{Path(absTestMediaDir)},
		Output: struct {
			Movies []Path `json:"movies"`
			Series []Path `json:"series"`
		}{
			Movies: []Path{Path(absTestOutputMoviesDir)},
			Series: []Path{Path(absTestOutputSeriesDir)},
		},
		Transmission: TransmissionConfig{
			Rpc:               realConfig.Transmission.Rpc,
			UnsortedDir:       Path(absTestUnsortedDir),
			SortingRules:      []TorrentSortingRule{sortingRule},
			DefaultMoviesDest: Path(filepath.Join(absTestMediaDir, "Movies")),
			DefaultSeriesDest: Path(filepath.Join(absTestMediaDir, "Series")),
		},
		// Use real API keys from the loaded config
		TMDbApiKey:      realConfig.TMDbApiKey,
		OpenAiApiKey:    realConfig.OpenAiApiKey,
		KinopoiskApiKey: realConfig.KinopoiskApiKey,
		// Use genre mappings from the real config
		TMDbMovieGenres: realConfig.TMDbMovieGenres,
		TMDbTvGenres:    realConfig.TMDbTvGenres,
		GenresMap:       realConfig.GenresMap,
	}

	// Verify required API keys are present
	if config.TMDbApiKey == "" {
		t.Fatal("TMDB_API_KEY is not set in the config")
	}
	if config.OpenAiApiKey == "" {
		t.Fatal("OPENAI_API_KEY is not set in the config")
	}
	if config.KinopoiskApiKey == "" {
		t.Fatal("KINOPOISK_API_KEY is not set in the config")
	}

	// Adjust torrent cache file paths to match test environment
	modifyTransmissionCacheForTestPaths(absTestMediaDir, t)

	// Run the sync
	t.Logf("Running media sync...")
	if err := runMediaSync(config); err != nil {
		t.Fatalf("runMediaSync failed: %v", err)
	}

	// Wait a moment for file operations to complete
	time.Sleep(100 * time.Millisecond)

	// Compare output with expected output
	compareDirectories(t, filepath.Join(testExpectedDir, "movies"), filepath.Join(testOutputDir, "movies"))
	compareDirectories(t, filepath.Join(testExpectedDir, "series"), filepath.Join(testOutputDir, "series"))

	// Clean up environment
	os.Unsetenv("TEST_MODE")
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

// modifyTransmissionCacheForTestPaths creates or modifies the transmission cache file
// to make sure all paths in the torrent data match the test environment paths
func modifyTransmissionCacheForTestPaths(testMediaDir string, t *testing.T) {
	cacheFilePath := filepath.Join(CacheDir, "transmission_torrents.json")

	// Check if cache file already exists
	if _, err := os.Stat(cacheFilePath); os.IsNotExist(err) {
		// If the cache doesn't exist yet, there's nothing to modify
		t.Logf("No transmission cache file exists yet at %s", cacheFilePath)
		return
	}

	// Read the existing cache file
	data, err := os.ReadFile(cacheFilePath)
	if err != nil {
		t.Logf("Error reading transmission cache file: %v", err)
		return
	}

	var torrentMap map[string]transmissionrpc.Torrent
	if err := json.Unmarshal(data, &torrentMap); err != nil {
		t.Logf("Error unmarshaling transmission cache data: %v", err)
		return
	}

	// Create a new map with modified paths
	modifiedMap := make(map[string]transmissionrpc.Torrent)
	for key, torrent := range torrentMap {
		// Create a new key with the test media directory
		newKey := strings.Replace(strings.ToLower(key), strings.ToLower(*torrent.DownloadDir), strings.ToLower(testMediaDir), 1)

		// Create a copy of the torrent with the modified download directory
		newDownloadDir := testMediaDir
		newTorrent := torrent
		newTorrent.DownloadDir = &newDownloadDir

		// Add to the modified map
		modifiedMap[newKey] = newTorrent
	}

	// Write the modified data back to the cache file
	modifiedData, err := json.MarshalIndent(modifiedMap, "", "  ")
	if err != nil {
		t.Logf("Error marshaling modified transmission cache data: %v", err)
		return
	}

	// Create the test cache directory if it doesn't exist
	testCacheFilePath := filepath.Join(testCacheDir, "transmission_torrents.json")

	// Set environment variable for the transmission wrapper to use
	os.Setenv("TEST_TRANSMISSION_CACHE_PATH", testCacheFilePath)
	t.Logf("Set TEST_TRANSMISSION_CACHE_PATH to %s", testCacheFilePath)

	// Create the directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(testCacheFilePath), 0755); err != nil {
		t.Fatalf("Error creating directory for test transmission cache: %v", err)
		return
	}

	// Write the modified data to the test cache file
	if err := os.WriteFile(testCacheFilePath, modifiedData, 0644); err != nil {
		t.Fatalf("Error writing test transmission cache file: %v", err)
		return
	}

	t.Logf("Successfully created test transmission cache file with mock data")

	t.Logf("Successfully modified transmission cache file with test paths")
}
