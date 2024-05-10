package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
)

type KinopoiskAPI struct {
	ApiKey      string
	TvShowsOnly bool
}

type KinopoiskResponse struct {
	Total      int              `json:"total"`
	Limit      int              `json:"limit"`
	Page       int              `json:"page"`
	PagesCount int              `json:"pages"`
	Results    []KinopoiskMovie `json:"docs"`
}

type KinopoiskMovie struct {
	Id int `json:"id"`

	Title            string           `json:"name,omitempty"`
	AlternativeTitle string           `json:"alternativeName,omitempty"`
	NameEN           string           `json:"enName,omitempty"`
	Names            []KinopoiskGenre `json:"names,omitempty"`
	Type             string           `json:"type"`
	Year             int              `json:"year,omitempty"`

	Description      string `json:"description"`
	ShortDescription string `json:"shortDescription"`

	Logo struct {
		Url string `json:"url"`
	} `json:"logo,omitempty"`

	Poster struct {
		Url        string `json:"url"`
		PreviewUrl string `json:"previewUrl"`
	} `json:"poster,omitempty"`

	Backdrop struct {
		Url        string `json:"url"`
		PreviewUrl string `json:"previewUrl"`
	} `json:"backdrop,omitempty"`

	Rating struct {
		Kp                 float64 `json:"kp"`                 //: 6.2,
		Imdb               float64 `json:"imdb"`               //: 8.4,
		Tmdb               float64 `json:"tmdb"`               //: 3.2,
		FilmCritics        float64 `json:"filmCritics"`        //: 10,
		RussianFilmCritics float64 `json:"russianFilmCritics"` //: 5.1,
		Await              float64 `json:"await"`              //: 6.1
	} `json:"rating,omitempty"`

	IsSeries bool `json:"isSeries"`

	Genres []KinopoiskGenre `json:"genres,omitempty"`

	ExternalId struct {
		KpHD string `json:"kpHD,omitempty"` //: "48e8d0acb0f62d8585101798eaeceec5",
		IMDb string `json:"imdb,omitempty"` //: "tt0232500",
		TMDb int    `json:"tmdb,omitempty"` //: 9799
	} `json:"externalId"`
}
type KinopoiskGenre struct {
	Name string `json:"name"`
}

// TODO: Implement https://www.kinopoisk.ru/index.php?kp_query=fallout website parsing
func (api KinopoiskAPI) FindMovies(title string, year string, page int) (MovieSearchResult, error) {
	query := url.QueryEscape(title)
	url := fmt.Sprintf("https://api.kinopoisk.dev/v1.4/movie/search?page=%d&limit=20&query=%s", page, query)

	p := ""
	if page > 1 {
		p = "page: " + strconv.Itoa(page)
	}
	Log("fetching kp", title, p, url)

	response, err := FetchURL(url, map[string]string{
		"Accept":    "application/json",
		"X-API-KEY": api.ApiKey,
	})

	if err != nil {
		return MovieSearchResult{}, err
	}
	// Log(resp1)

	var searchResults KinopoiskResponse
	if err := json.Unmarshal(response, &searchResults); err != nil {
		return MovieSearchResult{}, err
	}

	var results []MediaInfo
	for _, movie := range searchResults.Results {
		if movie.Type != "movie" && movie.Type != "tv-series" && movie.Type != "cartoon" && movie.Type != "anime" && movie.Type != "tv-show" && movie.Type != "animated-series" {
			continue
		}
		if api.TvShowsOnly && !movie.IsSeries {
			continue
		}

		// externalId": {
		// "kpHD": "48e8d0acb0f62d8585101798eaeceec5",
		// "imdb": "tt0232500",
		// "tmdb": 9799
		// },
		var id string
		var idType IdType
		var url string
		if movie.ExternalId.TMDb > 0 {
			id = strconv.Itoa(movie.ExternalId.TMDb)
			idType = TMDB
			url = fmt.Sprintf("https://themoviedb.org/movie/%s/", id)
		} else if movie.ExternalId.IMDb != "" {
			id = movie.ExternalId.IMDb
			idType = IMDB
			url = fmt.Sprintf("https://www.imdb.com/title/%s", id)
		} else {
			id = strconv.Itoa(movie.Id)
			idType = KPID
			if movie.IsSeries {
				url = fmt.Sprintf("https://www.kinopoisk.ru/series/%s/", id)
			} else {
				url = fmt.Sprintf("https://www.kinopoisk.ru/film/%s/", id)
			}
		}

		title := Coalesce3(movie.Title, movie.NameEN, movie.AlternativeTitle)
		origTitle := movie.AlternativeTitle
		if title == origTitle && movie.NameEN != movie.Title && movie.NameEN != "" {
			origTitle = movie.NameEN
		}
		alternativeTitle := ""
		for _, name := range movie.Names {
			if title == origTitle && name.Name != title && name.Name != "" {
				title = name.Name
			} else if title != name.Name && movie.AlternativeTitle != name.Name && name.Name != "" {
				alternativeTitle = name.Name
			}
		}
		genres := mapSlice(movie.Genres, func(genre KinopoiskGenre) string { return genre.Name })

		year := ""
		if movie.Year > 1900 {
			year = strconv.Itoa(movie.Year)
		}
		mediaInfo := MediaInfo{
			Id: MediaId{
				id:     id,
				idType: idType,
			},
			Title:            title,
			OriginalTitle:    origTitle,
			AlternativeTitle: alternativeTitle,
			Description:      movie.Description,
			Year:             year,
			IsTvShow:         movie.IsSeries,
			Url:              url,
			PosterUrl:        movie.Poster.Url,
			BackdropUrl:      movie.Backdrop.Url,
			Genres:           genres,
		}
		results = append(results, mediaInfo)
	}
	return MovieSearchResult{
		Results:   results,
		PageCount: searchResults.PagesCount,
	}, nil

}
