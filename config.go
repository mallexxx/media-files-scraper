package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
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

	TMDbMovieGenres []TMDbGenre `json:"tmdb_movie_genres"`
	TMDbTvGenres    []TMDbGenre `json:"tmdb_tv_genres"`
}

type TransmissionConfig struct {
	Rpc string `json:"rpc,omitempty"`

	UnsortedDir Path `json:"unsorted_dir,omitempty"`

	SortingRules      []TorrentSortingRule `json:"sorting_rules,omitempty"`
	DefaultMoviesDest Path                 `json:"default_movies_destination,omitempty"`
	DefaultSeriesDest Path                 `json:"default_series_destination,omitempty"`
}

type TorrentSortingRule struct {
	GenreRegex  *regexp.Regexp `json:"genre_regex,omitempty"`
	Destination Path           `json:"destination,omitempty"`
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

	return &config, nil
}
