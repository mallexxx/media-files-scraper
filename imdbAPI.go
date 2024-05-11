package main

import (
	"bytes"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

type IMDbAPI struct {
	GenresMap map[string]string
}

func (api IMDbAPI) FindMovies(titlestr string, year string, page int) (MovieSearchResult, error) {
	title := strings.ReplaceAll(titlestr, "'", "")
	query := url.QueryEscape(title)
	searchURL := fmt.Sprintf("https://www.imdb.com/find?q=%s&s=tt|accept-language=ru-ru", query)

	// If year is provided, add it to the URL
	if year != "" {
		searchURL += "&year=" + year
	}
	Log("fetching imdb", title, searchURL)

	response, err := FetchURL(searchURL, map[string]string{
		"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.36",
	})
	if err != nil {
		return MovieSearchResult{}, err
	}

	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(response))
	if err != nil {
		return MovieSearchResult{}, err
	}

	// Log("response:", response.Body)

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
			Logf("could not extract id from %s for \"%s\" - skipping\n", imdbID, title)
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

func (api IMDbAPI) LoadMediaInfo(id string, tmdbApi TMDbAPI) (MediaInfo, error) {
	// try loading from tmdb by imdb id first
	mediaInfo, err := tmdbApi.findTMDbByIMDbID(id)
	if err == nil && mediaInfo.PosterUrl != "" {
		return mediaInfo, nil
	}

	// Prepare the IMDb URL
	imdbURL := fmt.Sprintf("https://www.imdb.com/title/%s", id)

	Log("fetching imdb", id, imdbURL)

	response, err := FetchURL(imdbURL, map[string]string{
		"User-Agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.36",
		"Accept-Language": "ru-RU,ru;q=0.9",
	})
	if err != nil {
		return MediaInfo{}, err
	}

	// Load the HTML document
	html := string(response)
	// Log(html)

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(response))
	if err != nil {
		return MediaInfo{}, err
	}

	// Parse the media information
	titleSpan := doc.Find(".hero__primary-text")
	titleHeader := titleSpan.Parent().Parent()

	title := titleSpan.Text()

	// find year
	var year string
	isTvShow := false

	yearRegex := regexp.MustCompile(`(?:19|20)\d\d`)
	titleHeader.Find("li.ipc-inline-list__item").Each(func(i int, span *goquery.Selection) {
		text := span.Text()
		if len(yearRegex.FindStringSubmatch(text)) > 0 {
			year = strings.TrimSpace(text)
		}
		if text == "TV Series" || text == "TV Mini Series" {
			isTvShow = true
		}
	})
	if year == "" || !isTvShow {
		titleHeader.Find("span.ipc-metadata-list-summary-item__li").Each(func(i int, span *goquery.Selection) {
			text := span.Text()
			if len(yearRegex.FindStringSubmatch(text)) > 0 {
				year = strings.TrimSpace(text)
			}
			if text == "TV Series" || text == "TV Mini Series" {
				isTvShow = true
			}
		})
	}

	description := strings.TrimSpace(doc.Find("p[data-testid=plot]").Text())

	// find genre
	genreRegex := regexp.MustCompile(`"id":"(\w+)","__typename":"Genre"`)
	matches := genreRegex.FindAllStringSubmatch(html, -1)
	var genres []string
	for _, match := range matches {
		var genre string
		if mappedGenre, ok := api.GenresMap[strings.ToLower(match[1])]; ok {
			genre = mappedGenre
		} else {
			genre = match[1]
			Logf("❗️ IMDb genre \"%s\" not mapped!", genre)
		}
		if genre != "" {
			genres = append(genres, genre)
		}
	}

	// find poster image
	imgRegex := regexp.MustCompile(`<img.*?srcSet="([^"]+)"`)
	var posterUrl string
	if match := imgRegex.FindStringSubmatch(html); len(match) > 1 {
		posterUrl = strings.Split(match[1], ",")[0]
	}

	var mediaId MediaId
	if mediaInfo.Id != (MediaId{}) {
		mediaId = mediaInfo.Id
	} else {
		mediaId = MediaId{
			id:     id,
			idType: IMDB,
		}
	}

	result := MediaInfo{
		Title:            Coalesce(mediaInfo.Title, title),
		OriginalTitle:    mediaInfo.OriginalTitle,
		AlternativeTitle: mediaInfo.AlternativeTitle,
		Year:             year,
		Id:               mediaId,
		Url:              Coalesce(mediaInfo.Url, imdbURL),
		IsTvShow:         isTvShow,
		Description:      description,
		Genres:           genres,
		PosterUrl:        Coalesce(mediaInfo.PosterUrl, posterUrl),
		BackdropUrl:      mediaInfo.BackdropUrl,
	}

	return result, nil
}
