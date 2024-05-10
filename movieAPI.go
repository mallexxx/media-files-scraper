package main

import (
	"fmt"
	"strconv"
)

type MovieSearchResult struct {
	Results   []MediaInfo
	PageCount int
}

type MovieAPI interface {
	TMDbAPI | IMDbAPI | KinopoiskAPI

	FindMovies(title string, year string, page int) (MovieSearchResult, error)
}

func findMovieByTitle[API MovieAPI](api API, title string, year string) (MediaInfo, int /*score*/, error) {
	var bestMatch MediaInfo
	bestScore := 0

	page := 1
	totalPages := -1
	for {
		result, err := api.FindMovies(title, year, page)
		if err != nil {
			return MediaInfo{}, 0, err
		}

		movieIdx, score := findBestMatchingMediaInfo(result.Results, title, year)
		if movieIdx >= 0 && score > bestScore {
			bestMatch = result.Results[movieIdx]
			bestScore = score
			mediaType := "movie"
			if bestMatch.IsTvShow {
				mediaType = "tv show"
			}
			Logf("found %s \"%s\": %d \n", mediaType, bestMatch.Title, score)

		} else if page > 4 && bestScore >= 80 {
			// if better (than 80%) match not found on current page > 4 - take the best result so far
			break
		}
		if bestScore >= 90 {
			break
		}

		if result.PageCount != totalPages {
			totalPages = result.PageCount
		}
		if totalPages <= page {
			break
		}
		page += 1
	}

	if bestScore > 70 {
		Logf("taking '%s' score: %d\n", bestMatch.Title, bestScore)
		return bestMatch, bestScore, nil
	} else if bestScore > 0 {
		Logf("found '%s' but not taking as score is too low: %d", bestMatch.Title, bestScore)
		return MediaInfo{}, 0, fmt.Errorf("movie matching %s not found", title)
	} else {
		return MediaInfo{}, 0, fmt.Errorf("movie matching %s not found", title)
	}
}

func findBestMatchingMediaInfo(movies []MediaInfo, query string, year string) ( /*bestMatchIndex*/ int /*score*/, int) {
	bestScore := -1
	bestMatch := -1

	for idx, movie := range movies {
		var score int

		origTitle := TransliterateToLatin(movie.OriginalTitle)
		title := TransliterateToLatin(movie.Title)
		altTitle := TransliterateToLatin(movie.AlternativeTitle)
		queryTitle := TransliterateToLatin(query)

		if title == "" && origTitle != "" {
			title = origTitle
		}
		if title == "" && origTitle == "" {
			Log("no title!", movie)
			continue
		}

		useJaroWinkler := false
		if year != "" && year != movie.Year {
			if movie.Year != "" {
				queryTitle += " (" + year + ")"
			}
			if origTitle != "" {
				origTitle += " (" + movie.Year + ")"
			}
			if altTitle != "" {
				altTitle += " (" + movie.Year + ")"
			}
			title += " (" + movie.Year + ")"
		} else if year == movie.Year {
			// give more points if the titles have common beginning
			useJaroWinkler = true
		}

		// Log(movie.Id, "➡️ checking", origTitle, title, "⬅", queryTitle)
		if origTitle != "" && origTitle != title {
			score = max(computeSimilarityScore(title, queryTitle, useJaroWinkler),
				computeSimilarityScore(origTitle, queryTitle, useJaroWinkler))
		} else {
			score = computeSimilarityScore(title, queryTitle, useJaroWinkler)
		}
		if altTitle != "" && altTitle != title && altTitle != origTitle {
			score = max(score, computeSimilarityScore(altTitle, queryTitle, useJaroWinkler))
		}

		if year != "" && movie.Year != "" {
			y1, _ := strconv.Atoi(year)
			y2, _ := strconv.Atoi(movie.Year)

			// consider totally different year if more than 2 years difference
			if max(y1, y2)-min(y1, y2) > 2 {
				score = max(0, score-20)
			}
		}

		if score > bestScore {
			bestScore = score
			bestMatch = idx
		}

		if score == 100 {
			break
		}

		// Log("⬅️ score:", score)
	}

	return bestMatch, bestScore
}
