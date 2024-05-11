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
	MovieResults     []TMDbMovie   `json:"movie_results"`
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

func (series TMDbSeriesDetails) Url() string {
	return fmt.Sprintf("https://www.themoviedb.org/tv/%d", series.ID)
}

func (series TMDbSeries) MediaInfo(api TMDbAPI) MediaInfo {
	var genres []string
	for _, genreId := range series.GenreIDs {

		genre := api.FindTvGenreById(genreId)
		if genre != "" {
			genres = append(genres, genre)
		}
	}
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
}

func (series TMDbSeriesDetails) MediaInfo(api TMDbAPI) MediaInfo {
	var genres []string
	for _, tmdbGenre := range series.Genres {
		genre := api.FindTvGenreById(tmdbGenre.ID)
		if genre != "" {
			genres = append(genres, genre)
		}
	}
	return MediaInfo{
		Id:            MediaId{id: strconv.Itoa(series.ID), idType: TMDB},
		Title:         series.Name,
		OriginalTitle: series.OriginalName,
		Year:          series.Year(),
		Description:   series.Overview,
		IsTvShow:      true,
		Url:           series.Url(),
		PosterUrl:     series.PosterURL(),
		BackdropUrl:   series.BackdropURL(),
		Genres:        genres,
	}
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

func (movie TMDbMovie) MediaInfo(api TMDbAPI) MediaInfo {
	isTvShow := movie.MediaType == "tv"
	var genres []string
	for _, genreId := range movie.GenreIDs {
		var genre string
		if isTvShow {
			genre = api.FindTvGenreById(genreId)
		} else {
			genre = api.FindMovieGenreById(genreId)
		}
		if genre != "" {
			genres = append(genres, genre)
		}
	}

	return MediaInfo{
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
}

func (tmdb TMDbSeries) Year() string {
	if tmdb.FirstAirDate == "" {
		return ""
	}

	parts := strings.Split(tmdb.FirstAirDate, "-")

	return parts[0]
}

func (tmdb TMDbSeriesDetails) Year() string {
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

func (tmdb TMDbSeriesDetails) PosterURL() string {
	if tmdb.PosterPath == "" {
		return ""
	}
	baseURL := "https://image.tmdb.org/t/p/original"

	return fmt.Sprintf("%s%s", baseURL, tmdb.PosterPath)
}

func (tmdb TMDbSeriesDetails) BackdropURL() string {
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

func (api TMDbAPI) PerformFindMovies(titlestr string, year string, page int) (MovieSearchResult, error) {
	title := strings.ReplaceAll(titlestr, "'", "")
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
		if movie.MediaType != "tv" && movie.MediaType != "movie" {
			continue
		}
		mediaInfo := movie.MediaInfo(api)
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
	panic(fmt.Sprintf("Movie Genre with id %d not found", genreId))
}

func (api TMDbAPI) FindTvGenreById(genreId int) string {
	for _, genre := range api.TvGenres {
		if genre.ID == genreId {
			return genre.Name
		}
	}
	// fallback to movie genres
	for _, genre := range api.MovieGenres {
		if genre.ID == genreId {
			return genre.Name
		}
	}
	panic(fmt.Sprintf("TV Genre with id %d not found", genreId))
}

func (api TMDbAPI) LoadMovieDetails(id string) (MediaInfo, error) {
	url := fmt.Sprintf("https://api.themoviedb.org/3/movie/%s?api_key=%s&language=ru-RU", id, api.ApiKey)

	Log("fetching tmdb movie details", id, url)

	response, err := FetchURL(url, map[string]string{})
	if err != nil {
		return MediaInfo{}, err
	}

	var result TMDbMovie
	if err := json.Unmarshal(response, &result); err != nil {
		return MediaInfo{}, err
	}

	return result.MediaInfo(api), nil
}

func (api TMDbAPI) PerformFindSeries(titlestr string, year string, page int) (MovieSearchResult, error) {
	title := strings.ReplaceAll(titlestr, "'", "")
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
		return series.MediaInfo(api)
	})
	return MovieSearchResult{
		Results:   results,
		PageCount: searchResults.TotalPages,
	}, nil
}

func (api TMDbAPI) getSeriesEpisodes(id MediaId) ([]TMDbEpisode, error) {
	tmdbID := 0
	if id.idType == IMDB {
		tvShow, err := api.findTMDbByIMDbID(id.id)
		if err != nil {
			return nil, err
		}
		tmdbID, _ = strconv.Atoi(tvShow.Id.id)
	} else if id.idType == TMDB {
		id, err := strconv.Atoi(id.id)
		if err != nil {
			return nil, err
		}
		tmdbID = id
	} else {
		return nil, nil
	}
	return api.getTMDbSeriesEpisodes(tmdbID)
}

func (api TMDbAPI) findTMDbByIMDbID(imdbID string) (MediaInfo, error) {
	url := fmt.Sprintf("https://api.themoviedb.org/3/find/%s?api_key=%s&external_source=imdb_id&language=ru-RU", imdbID, api.ApiKey)

	// Send HTTP GET request
	response, err := FetchURL(url, map[string]string{})
	if err != nil {
		return MediaInfo{}, err
	}

	// Parse the JSON response
	var result TMDbFindResponse
	if err := json.Unmarshal(response, &result); err != nil {
		return MediaInfo{}, err
	}

	// Check if there are any TV show results
	if len(result.TVResults) > 0 {
		return result.TVResults[0].MediaInfo(api), nil
	} else if len(result.MovieResults) > 0 {
		return result.MovieResults[0].MediaInfo(api), nil
	} else {
		return MediaInfo{}, fmt.Errorf("no TMDb item found for IMDb ID %s", imdbID)
	}
}

func (api TMDbAPI) getTMDbSeriesEpisodes(seriesID int) ([]TMDbEpisode, error) {
	// Fetch details of the TV series
	seriesDetails, err := api.LoadSeriesDetails(seriesID)
	if err != nil {
		return nil, err
	}

	var allEpisodes []TMDbEpisode

	// Iterate over each season and fetch episodes
	for seasonNumber := 1; seasonNumber <= seriesDetails.NumberOfSeasons; seasonNumber++ {
		// Fetch episodes for the current season
		episodes, err := getTMDbSeasonEpisodes(seriesID, seasonNumber, api.ApiKey)
		if err != nil {
			return nil, err
		}
		allEpisodes = append(allEpisodes, episodes...)
	}

	return allEpisodes, nil
}

func (api TMDbAPI) LoadSeriesDetails(seriesID int) (TMDbSeriesDetails, error) {
	url := fmt.Sprintf("https://api.themoviedb.org/3/tv/%d?api_key=%s&language=ru-RU", seriesID, api.ApiKey)

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

func (api TMDbAPI) LoadSeriesMediaInfo(seriesID string) (MediaInfo, error) {
	id, err := strconv.Atoi(seriesID)
	if err != nil {
		return MediaInfo{}, err
	}
	details, err := api.LoadSeriesDetails(id)
	if err != nil {
		return MediaInfo{}, err
	}
	return details.MediaInfo(api), nil
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
