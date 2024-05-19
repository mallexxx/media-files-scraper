package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/text/unicode/norm"
)

// Config represents the configuration structure.
type Config struct {
	Transmission TransmissionConfig `json:"transmission,omitempty"`

	TMDbApiKey      string `json:"tmdb_api_key,omitempty"`
	OpenAiApiKey    string `json:"openai_api_key,omitempty"`
	KinopoiskApiKey string `json:"kinopoisk_api_key,omitempty"`

	Directories []Path `json:"directories"`
	Output      struct {
		Movies []Path `json:"movies"`
		Series []Path `json:"series"`
	} `json:"output"`

	TMDbMovieGenres []TMDbGenre       `json:"tmdb_movie_genres"`
	TMDbTvGenres    []TMDbGenre       `json:"tmdb_tv_genres"`
	GenresMap       map[string]string `json:"genres_map"`
}

type TransmissionConfig struct {
	Rpc string `json:"rpc,omitempty"`

	UnsortedDir Path `json:"unsorted_dir,omitempty"`

	SortingRules      []TorrentSortingRule `json:"sorting_rules,omitempty"`
	DefaultMoviesDest Path                 `json:"default_movies_destination,omitempty"`
	DefaultSeriesDest Path                 `json:"default_series_destination,omitempty"`
}

type TorrentSortingRule struct {
	GenreRegexStr string `json:"genre_regex,omitempty"`
	GenreRegex    *regexp.Regexp
	Destination   Path `json:"destination,omitempty"`
}

// ConfigPath returns the path to the configuration file.
func ConfigPath(path Path) Path {
	if path == "" {
		exePath, _ := os.Executable()
		return Path(filepath.Dir(exePath)).appendingPathComponent("config.json")
	}
	if path.isDirectory() {
		return path.appendingPathComponent("config.json")
	}
	return path
}

// LoadConfig loads configuration from the specified path.
func LoadConfig(configFile Path) (*Config, error) {
	configFile = ConfigPath(configFile)
	file, err := os.Open(string(configFile))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var config Config
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return nil, err
	}

	if config.TMDbApiKey == "" {
		config.TMDbApiKey = os.Getenv("TMDB_API_KEY")
	}
	if config.OpenAiApiKey == "" {
		config.OpenAiApiKey = os.Getenv("OPENAI_API_KEY")
	}
	if config.KinopoiskApiKey == "" {
		config.KinopoiskApiKey = os.Getenv("KINOPOISK_API_KEY")
	}
	for idx, rule := range config.Transmission.SortingRules {
		config.Transmission.SortingRules[idx].GenreRegex, err = regexp.Compile(rule.GenreRegexStr)
		if err != nil {
			return nil, fmt.Errorf("could not compile regex `%s`: %s", rule.GenreRegexStr, err)
		}
	}

	return &config, nil
}

func (c Config) sourceDirectoryForVideoSymlink(symlink Path) (Path, Path, error) {
	target, err := os.Readlink(string(symlink))
	if err != nil {
		return "", "", err
	}

	targetPath := Path(target)
	fileName := targetPath.lastPathComponent()
	if norm.NFC.String(fileName) != norm.NFC.String(symlink.lastPathComponent()) {
		return "", "", fmt.Errorf("symlink valid but filename differs from %s", fileName)
	}

	targetLower := strings.ToLower(target)
	for _, path := range c.Directories {
		if strings.HasPrefix(targetLower, strings.ToLower(strings.TrimSuffix(string(path.appendingPathComponent("a")), "a"))) {
			if path.exists() && !targetPath.exists() {
				return "", "", fmt.Errorf("file does not exist")
			}
			return targetPath, path, nil
		}
	}
	return "", "", nil
}
