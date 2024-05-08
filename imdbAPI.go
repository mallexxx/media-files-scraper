package main

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

type IMDbAPI struct {
}

type ImdbResult struct {
	Title       string `json:"Title,omitempty"`
	Year        string `json:"Year,omitempty"`
	IMDbID      string `json:"IMDbID,omitempty"`
	isTvShow    bool
	Description string `json:"Description,omitempty"`
}

func (r ImdbResult) Url() string {
	return fmt.Sprintf("https://www.imdb.com/title/%s", r.IMDbID)
}

func (api IMDbAPI) FindMovies(title string, year string, page int) (MovieSearchResult, error) {
	query := url.QueryEscape(title)
	searchURL := fmt.Sprintf("https://www.imdb.com/find?q=%s&s=tt|accept-language=ru-ru", query)

	// If year is provided, add it to the URL
	if year != "" {
		searchURL += "&year=" + year
	}
	fmt.Println("fetching imdb", title, searchURL)

	client := &http.Client{}
	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return MovieSearchResult{}, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.36")

	// Send HTTP GET request
	response, err := client.Do(req)
	if err != nil {
		return MovieSearchResult{}, err
	}
	defer response.Body.Close()

	if response.StatusCode != 200 {
		return MovieSearchResult{}, fmt.Errorf("HTTP request %s failed with status: %d", searchURL, response.StatusCode)
	}

	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(response.Body)
	if err != nil {
		return MovieSearchResult{}, err
	}

	// fmt.Println("response:", response.Body)

	// Parse the search results
	var results []MediaInfo
	doc.Find(".find-result-item").Each(func(i int, s *goquery.Selection) {
		title := strings.TrimSpace(s.Find("a.ipc-metadata-list-summary-item__t").First().Text())
		year := strings.TrimSpace(s.Find("li.ipc-inline-list__item").First().Text())
		url, _ := s.Find("a.ipc-metadata-list-summary-item__t").First().Attr("href")
		img := s.Find("img").First()
		var posterUrl string
		if img != nil {
			posterUrl, _ = img.Attr("srcset")
			if posterUrl != "" {
				posterUrl = strings.Split(posterUrl, ",")[0]
			} else {
				posterUrl, _ = img.Attr("src")
			}
		}

		// Extract IMDb ID from URL
		pattern := `/title/(tt\d+)/?`
		re := regexp.MustCompile(pattern)
		var imdbID string
		match := re.FindStringSubmatch(url)
		if len(match) > 1 {
			imdbID = match[1]
		} else {
			fmt.Printf("could not extract id from %s for \"%s\" - skipping\n", imdbID, title)
			return
		}

		// Item type: movie or series
		isTvShow := false
		s.Find("span.ipc-metadata-list-summary-item__li").Each(func(i int, span *goquery.Selection) {
			text := span.Text()
			if text == "TV Series" || text == "TV Mini Series" {
				isTvShow = true
			}
		})

		result := MediaInfo{
			Id:        MediaId{id: imdbID, idType: IMDB},
			Title:     title,
			Url:       fmt.Sprintf("https://www.imdb.com/title/%s", imdbID),
			Year:      year,
			IsTvShow:  isTvShow,
			PosterUrl: posterUrl,
		}
		results = append(results, result)
	})

	return MovieSearchResult{
		Results:   results,
		PageCount: 1,
	}, nil
}

func loadIMDbItem(id string) (ImdbResult, error) {
	// Prepare the IMDb URL
	imdbURL := fmt.Sprintf("https://www.imdb.com/title/%s", id)

	// Send HTTP GET request
	response, err := http.Get(imdbURL)
	if err != nil {
		return ImdbResult{}, err
	}
	defer response.Body.Close()

	if response.StatusCode != 200 {
		return ImdbResult{}, fmt.Errorf("HTTP request %s failed with status: %d", imdbURL, response.StatusCode)
	}

	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(response.Body)
	if err != nil {
		return ImdbResult{}, err
	}

	// Parse the media information
	title := doc.Find(".title_wrapper h1").Text()
	year := strings.TrimSpace(doc.Find(".title_wrapper .subtext a").First().Text())
	description := strings.TrimSpace(doc.Find(".summary_text").Text())

	// Determine the type of media based on the presence of episode information
	isTvShow := false
	if doc.Find(".title_wrapper .subtext .tv_series").Length() > 0 {
		isTvShow = true
	}

	result := ImdbResult{
		Title:       title,
		Year:        year,
		IMDbID:      id,
		isTvShow:    isTvShow,
		Description: description,
	}

	return result, nil
}

func loadIMDbMediaInfo(id string) (MediaInfo, error) {
	movie, err := loadIMDbItem(id)
	if err != nil {
		return MediaInfo{}, err
	}
	return MediaInfo{
		Title: movie.Title,
		Year:  movie.Year,
		Id: MediaId{
			id:     id,
			idType: IMDB,
		},
		IsTvShow: movie.isTvShow,
		Url:      movie.Url(),
	}, nil
}
