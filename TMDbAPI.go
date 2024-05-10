package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

type TMDbAPI struct {
	ApiKey       string
	Language     string
	TVShowSearch bool

	MovieGenres []TMDbGenre `json:"tmdb_movie_genres"`
	TvGenres    []TMDbGenre `json:"tmdb_tv_genres"`
}

// Movies

type TMDbMovie struct {
	Id            int    `json:"id"`
	Title         string `json:"title,omitempty"`
	Name          string `json:"name,omitempty"`
	OriginalTitle string `json:"original_title,omitempty"`
	OriginalName  string `json:"original_name,omitempty"`
	ReleaseDate   string `json:"release_date,omitempty"`
	FirstAirDate  string `json:"first_air_date,omitempty"`
	Overview      string `json:"overview,omitempty"`
	MediaType     string `json:"media_type,omitempty"`
	PosterPath    string `json:"poster_path,omitempty"`
	BackdropPath  string `json:"backdrop_path,omitempty"`
	GenreIDs      []int  `json:"genre_ids"`
}

type TMDbSearchResults struct {
	Results    []TMDbMovie `json:"results"`
	TotalPages int         `json:"total_pages"`
}

// TV Shows

type TMDbFindResponse struct {
	MovieResults     []interface{} `json:"movie_results"`
	PersonResults    []interface{} `json:"person_results"`
	TVResults        []TMDbSeries  `json:"tv_results"`
	TVEpisodeResults []interface{} `json:"tv_episode_results"`
	TVSeasonResults  []interface{} `json:"tv_season_results"`
}

type TMDbSeriesDetails struct {
	ID               int         `json:"id"`
	Name             string      `json:"name"`
	OriginalName     string      `json:"original_name"`
	FirstAirDate     string      `json:"first_air_date"`
	Overview         string      `json:"overview"`
	PosterPath       string      `json:"poster_path"`
	BackdropPath     string      `json:"backdrop_path"`
	NumberOfSeasons  int         `json:"number_of_seasons"`
	NumberOfEpisodes int         `json:"number_of_episodes"`
	OriginCountry    []string    `json:"origin_country"`
	OriginalLanguage string      `json:"original_language"`
	Genres           []TMDbGenre `json:"genres"`
}

type TMDbSeries struct {
	ID               int      `json:"id"`
	Name             string   `json:"name,omitempty"`
	Title            string   `json:"title,omitempty"`
	OriginalName     string   `json:"original_name,omitempty"`
	OriginalTitle    string   `json:"original_title,omitempty"`
	FirstAirDate     string   `json:"first_air_date"`
	Overview         string   `json:"overview"`
	PosterPath       string   `json:"poster_path"`
	BackdropPath     string   `json:"backdrop_path"`
	GenreIDs         []int    `json:"genre_ids"`
	OriginalLanguage string   `json:"original_language"`
	OriginCountry    []string `json:"origin_country"`
	Popularity       float64  `json:"popularity"`
	VoteCount        int      `json:"vote_count"`
	VoteAverage      float64  `json:"vote_average"`
}
type TMDbTVSearchResults struct {
	Results    []TMDbSeries `json:"results"`
	TotalPages int          `json:"total_pages"`
}

func (series TMDbSeries) Url() string {
	return fmt.Sprintf("https://www.themoviedb.org/tv/%d", series.ID)
}

type TMDbSeason struct {
	AirDate      string        `json:"air_date"`
	Episodes     []TMDbEpisode `json:"episodes"`
	Name         string        `json:"name"`
	Overview     string        `json:"overview"`
	ID           int           `json:"id"`
	PosterPath   string        `json:"poster_path"`
	SeasonNumber int           `json:"season_number"`
}

type TMDbEpisode struct {
	AirDate        string  `json:"air_date"`
	EpisodeNumber  int     `json:"episode_number"`
	ID             int     `json:"id"`
	Name           string  `json:"name"`
	Overview       string  `json:"overview"`
	ProductionCode string  `json:"production_code"`
	SeasonNumber   int     `json:"season_number"`
	ShowID         int     `json:"show_id"`
	StillPath      string  `json:"still_path"`
	VoteAverage    float64 `json:"vote_average"`
	VoteCount      int     `json:"vote_count"`
}

type TMDbGenre struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func (movie TMDbMovie) Url() string {
	if movie.MediaType == "tv" {
		return fmt.Sprintf("https://www.themoviedb.org/tv/%d", movie.Id)
	}
	return fmt.Sprintf("https://themoviedb.org/movie/%d/", movie.Id)
}

func (tmdb TMDbMovie) Year() string {
	var parts []string
	if tmdb.ReleaseDate != "" {
		parts = strings.Split(tmdb.ReleaseDate, "-")
	} else if tmdb.FirstAirDate != "" {
		parts = strings.Split(tmdb.FirstAirDate, "-")
	} else {
		return ""
	}

	return parts[0]
}

func (tmdb TMDbMovie) PosterURL() string {
	if tmdb.PosterPath == "" {
		return ""
	}
	baseURL := "https://image.tmdb.org/t/p/original"

	return fmt.Sprintf("%s%s", baseURL, tmdb.PosterPath)
}
func (tmdb TMDbMovie) BackdropURL() string {
	if tmdb.BackdropPath == "" {
		return ""
	}
	baseURL := "https://image.tmdb.org/t/p/original"

	return fmt.Sprintf("%s%s", baseURL, tmdb.BackdropPath)
}

func (tmdb TMDbSeries) Year() string {
	if tmdb.FirstAirDate == "" {
		return ""
	}

	parts := strings.Split(tmdb.FirstAirDate, "-")

	return parts[0]
}

func (tmdb TMDbSeries) PosterURL() string {
	if tmdb.PosterPath == "" {
		return ""
	}
	baseURL := "https://image.tmdb.org/t/p/original"

	return fmt.Sprintf("%s%s", baseURL, tmdb.PosterPath)
}

func (tmdb TMDbSeries) BackdropURL() string {
	if tmdb.BackdropPath == "" {
		return ""
	}
	baseURL := "https://image.tmdb.org/t/p/original"

	return fmt.Sprintf("%s%s", baseURL, tmdb.BackdropPath)
}

// - API

func (api TMDbAPI) FindMovies(title string, year string, page int) (MovieSearchResult, error) {
	if api.TVShowSearch {
		return api.PerformFindSeries(title, year, page)
	} else {
		return api.PerformFindMovies(title, year, page)
	}
}

func (api TMDbAPI) PerformFindMovies(title string, year string, page int) (MovieSearchResult, error) {
	query := url.QueryEscape(title)
	url := fmt.Sprintf("https://api.themoviedb.org/3/search/multi?api_key=%s&page=%d&query=%s&language=%s", api.ApiKey, page, query, api.Language)

	y := ""
	// If year is provided, add it to the URL
	if year != "" {
		url += "&year=" + year
		y = "y: " + year
	}
	p := ""
	if page > 1 {
		p = "page: " + strconv.Itoa(page)
	}
	Log("fetching tmdb", title, y, p, url)

	// Log(url)
	response, err := FetchURL(url, map[string]string{})

	if err != nil {
		return MovieSearchResult{}, err
	}

	var searchResults TMDbSearchResults
	if err := json.Unmarshal(response, &searchResults); err != nil {
		return MovieSearchResult{}, err
	}

	var results []MediaInfo
	for _, movie := range searchResults.Results {
		isTvShow := movie.MediaType == "tv"
		if !isTvShow && movie.MediaType != "movie" {
			continue
		}
		genres := mapSlice(movie.GenreIDs, func(genreId int) string {
			if isTvShow {
				return api.FindTvGenreById(genreId)
			}
			return api.FindMovieGenreById(genreId)
		})

		mediaInfo := MediaInfo{
			Id:            MediaId{id: strconv.Itoa(movie.Id), idType: TMDB},
			Title:         Coalesce(movie.Name, movie.Title),
			OriginalTitle: Coalesce(movie.OriginalName, movie.OriginalTitle),
			Year:          movie.Year(),
			Description:   movie.Overview,
			IsTvShow:      isTvShow,
			Url:           movie.Url(),
			PosterUrl:     movie.PosterURL(),
			BackdropUrl:   movie.BackdropURL(),
			Genres:        genres,
		}
		results = append(results, mediaInfo)
	}
	// take Movies first
	sort.SliceStable(results, func(i, j int) bool {
		return !results[i].IsTvShow && results[j].IsTvShow
	})

	return MovieSearchResult{
		Results:   results,
		PageCount: searchResults.TotalPages,
	}, nil
}

func (api TMDbAPI) FindMovieGenreById(genreId int) string {
	for _, genre := range api.MovieGenres {
		if genre.ID == genreId {
			return genre.Name
		}
	}
	return ""
}

func (api TMDbAPI) FindTvGenreById(genreId int) string {
	for _, genre := range api.TvGenres {
		if genre.ID == genreId {
			return genre.Name
		}
	}
	return ""
}

func (api TMDbAPI) PerformFindSeries(title string, year string, page int) (MovieSearchResult, error) {
	query := url.QueryEscape(title)
	url := fmt.Sprintf("https://api.themoviedb.org/3/search/tv?api_key=%s&page=%d&query=%s&language=ru-RU", api.ApiKey, page, query)

	y := ""
	// If year is provided, add it to the URL
	// if year != "" {
	// }
	if year != "" {
		// 	url += "&first_air_date_year=" + year
		y = "y: " + year
	}
	p := ""
	if page > 1 {
		p = "page: " + strconv.Itoa(page)
	}
	Log("fetching tmdb series", title, y, p, url)

	response, err := FetchURL(url, map[string]string{})
	if err != nil {
		return MovieSearchResult{}, err
	}

	var searchResults TMDbTVSearchResults
	if err := json.Unmarshal(response, &searchResults); err != nil {
		return MovieSearchResult{}, err
	}

	results := mapSlice(searchResults.Results, func(series TMDbSeries) MediaInfo {
		genres := mapSlice(series.GenreIDs, func(genreId int) string {
			return api.FindTvGenreById(genreId)
		})
		return MediaInfo{
			Id:            MediaId{id: strconv.Itoa(series.ID), idType: TMDB},
			Title:         Coalesce(series.Name, series.Title),
			OriginalTitle: Coalesce(series.OriginalName, series.OriginalTitle),
			Year:          series.Year(),
			Description:   series.Overview,
			IsTvShow:      true,
			Url:           series.Url(),
			PosterUrl:     series.PosterURL(),
			BackdropUrl:   series.BackdropURL(),
			Genres:        genres,
		}
	})
	return MovieSearchResult{
		Results:   results,
		PageCount: searchResults.TotalPages,
	}, nil

}

func getSeriesEpisodes(id MediaId, TMDbApiKey string) ([]TMDbEpisode, error) {
	tmdbID := 0
	if id.idType == IMDB {
		tvShow, err := findTMDbTVShowByIMDbID(id.id, TMDbApiKey)
		tmdbID = tvShow.ID
		if err != nil {
			return nil, err
		}
	} else if id.idType == TMDB {
		id, err := strconv.Atoi(id.id)
		if err != nil {
			return nil, err
		}
		tmdbID = id
	} else {
		return nil, nil
	}
	return getTMDbSeriesEpisodes(tmdbID, TMDbApiKey)
}

func findTMDbTVShowByIMDbID(imdbID string, TMDbApiKey string) (TMDbSeries, error) {
	url := fmt.Sprintf("https://api.themoviedb.org/3/find/%s?api_key=%s&external_source=imdb_id", imdbID, TMDbApiKey)

	// Send HTTP GET request
	response, err := FetchURL(url, map[string]string{})
	if err != nil {
		return TMDbSeries{}, err
	}

	// Parse the JSON response
	var result TMDbFindResponse
	if err := json.Unmarshal(response, &result); err != nil {
		return TMDbSeries{}, err
	}

	// Check if there are any TV show results
	if len(result.TVResults) == 0 {
		return TMDbSeries{}, fmt.Errorf("no TV show found for IMDb ID: %s", imdbID)
	}

	// Return the first TV show result
	return result.TVResults[0], nil
}

func getTMDbSeriesEpisodes(seriesID int, TMDbApiKey string) ([]TMDbEpisode, error) {
	// Fetch details of the TV series
	seriesDetails, err := getTMDbSeriesDetails(seriesID, TMDbApiKey)
	if err != nil {
		return nil, err
	}

	var allEpisodes []TMDbEpisode

	// Iterate over each season and fetch episodes
	for seasonNumber := 1; seasonNumber <= seriesDetails.NumberOfSeasons; seasonNumber++ {
		// Fetch episodes for the current season
		episodes, err := getTMDbSeasonEpisodes(seriesID, seasonNumber, TMDbApiKey)
		if err != nil {
			return nil, err
		}
		allEpisodes = append(allEpisodes, episodes...)
	}

	return allEpisodes, nil
}

func getTMDbSeriesDetails(seriesID int, TMDbApiKey string) (TMDbSeriesDetails, error) {
	url := fmt.Sprintf("https://api.themoviedb.org/3/tv/%d?api_key=%s&language=ru-RU", seriesID, TMDbApiKey)

	response, err := FetchURL(url, map[string]string{})
	if err != nil {
		return TMDbSeriesDetails{}, err
	}

	var seriesDetails TMDbSeriesDetails
	if err := json.Unmarshal(response, &seriesDetails); err != nil {
		return TMDbSeriesDetails{}, err
	}

	return seriesDetails, nil
}

func getTMDbSeasonEpisodes(seriesID, seasonNumber int, TMDbApiKey string) ([]TMDbEpisode, error) {
	url := fmt.Sprintf("https://api.themoviedb.org/3/tv/%d/season/%d?api_key=%s&language=ru-RU", seriesID, seasonNumber, TMDbApiKey)

	response, err := FetchURL(url, map[string]string{})
	if err != nil {
		return nil, err
	}

	var seasonDetails TMDbSeason
	if err := json.Unmarshal(response, &seasonDetails); err != nil {
		return nil, err
	}

	return seasonDetails.Episodes, nil
}
