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
			fmt.Printf("found %s \"%s\": %d \n", mediaType, bestMatch.Title, score)

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

	if bestScore > 0 {
		fmt.Println("✅ taking", bestMatch.Title, bestScore)
		return bestMatch, bestScore, nil
	} else {
		return MediaInfo{}, 0, fmt.Errorf("movie matching %s not found", title)
	}
}

func findBestMatchingMediaInfo(movies []MediaInfo, query string, year string) ( /*bestMatchIndex*/ int /*score*/, int) {
	bestScore := -1
	bestMatch := -1

	for idx, movie := range movies {
		var score int

		origTitle := movie.OriginalTitle
		title := movie.Title
		altTitle := movie.AlternativeTitle
		queryTitle := query

		if title == "" && origTitle != "" {
			title = origTitle
		}
		if title == "" && origTitle == "" {
			fmt.Println("no title!", movie)
			continue
		}

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
		}
		// fmt.Println(movie.Id, "➡️ checking", origTitle, title, "⬅", queryTitle)
		if origTitle != "" && origTitle != title {
			score = max(computeSimilarityScore(title, queryTitle),
				computeSimilarityScore(origTitle, queryTitle))
		} else {
			score = computeSimilarityScore(title, queryTitle)
		}
		if altTitle != "" && altTitle != title && altTitle != origTitle {
			score = max(score, computeSimilarityScore(altTitle, queryTitle))
		}

		if score < 100 {
			origTitle = TransliterateToCyrillic(origTitle)
			title = TransliterateToCyrillic(title)
			queryTranslit := TransliterateToCyrillic(queryTitle)

			scoreT1 := 0
			if origTitle != "" && origTitle != title {
				// fmt.Println("checking t1", origTitle, "⬅", queryTranslit)
				scoreT1 = computeSimilarityScore(origTitle, queryTranslit)
			}
			// fmt.Println("checking t2", title, "⬅", queryTranslit)
			scoreT2 := computeSimilarityScore(title, queryTranslit)
			if scoreT1 > scoreT2 && score < scoreT1 && scoreT1 > 80 {
				// fmt.Println("taking transliterated match: ", origTitle)
				score = scoreT1
			} else if scoreT2 > score && scoreT2 > 80 {
				// fmt.Println("taking transliterated match: ", title)
				score = scoreT2
			}
			if year == movie.Year && year != "" {
				score = min(100, score+20)
			}
		}
		if year != "" && movie.Year != "" {
			y1, _ := strconv.Atoi(year)
			y2, _ := strconv.Atoi(movie.Year)

			// consider totally different year if more than 2 years difference
			if max(y1, y2)-min(y1, y2) > 2 {
				score = min(0, score-20)
			}
		}
		// fmt.Println("⬅️ score:", score)

		if score > bestScore {
			bestScore = score
			bestMatch = idx
		}
	}

	return bestMatch, bestScore
}
