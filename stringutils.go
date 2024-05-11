package main

import (
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/adrg/strutil"
	"github.com/adrg/strutil/metrics"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/unicode/norm"
)

func extractTitleAndYearFromRutrackerTitle(title string) (string, string, error) {
	// extract title before `(` and year (19xx or 20xx after `[`)
	pattern := regexp.MustCompile(`([^(]+)\s+\(?.*\[((?:19\d\d|20\d\d)).*`)

	// Find submatches using the regex pattern
	matches := pattern.FindStringSubmatch(title)

	// Extract title and year
	var year string
	if len(matches) >= 3 {
		extractedTitle := matches[1]
		year = matches[2]

		// Split the title by "/"
		parts := strings.Split(extractedTitle, "/")
		originalTitle := strings.TrimSpace(extractedTitle)
		// Trim the spaces from each part
		for i := range parts {
			title := strings.TrimSpace(parts[i])
			if len(title) > 3 && !containsCyrillicCharacters(title) {
				// Take the last fitting as the original title
				originalTitle = title
			}
		}

		return originalTitle, year, nil
	} else {
		return "", "", fmt.Errorf("no title and year matches found \"%s\"", title)
	}
}

func convertWindows1251ToUTF8(input string) (string, error) {
	// Create a Windows-1251 to UTF-8 decoder
	decoder := charmap.Windows1251.NewDecoder()
	// Decode the input string from Windows-1251 to UTF-8
	utf8Bytes, err := decoder.String(input)
	if err != nil {
		return "", err
	}
	return utf8Bytes, nil
}

var latinToCyrillicMap = map[string]string{
	"a": "а", "b": "б", "c": "ц", "d": "д", "e": "е", "f": "ф", "g": "г", "h": "х", "i": "и", "j": "й", "k": "к", "l": "л", "m": "м",
	"n": "н", "o": "о", "p": "п", "q": "к", "r": "р", "s": "с", "t": "т", "u": "у", "v": "в", "w": "в", "x": "кс", "y": "ы", "z": "з",
	"ch": "ч", "zh": "ж", "sh": "ш", "sch": "щ", "yo": "ё", "jo": "ё", "yu": "ю", "ju": "ю", "ya": "я", "ja": "я", "'": "ь",
	"'a": "я", "iy": "ий", "yy": "ый", "ii": "ий", "kh": "х",
}
var CyrillicToLatinMap = map[string]string{
	"а": "a", "б": "b", "в": "v", "г": "g", "д": "d", "е": "e", "ё": "yo", "ж": "zh", "з": "z", "и": "i", "й": "j", "к": "k", "л": "l",
	"м": "m", "н": "n", "о": "o", "п": "p", "р": "r", "с": "s", "т": "t", "у": "u", "ф": "f", "х": "h", "ц": "c", "ч": "ch", "ш": "sh",
	"щ": "sch", "ъ": "'", "ы": "y", "ь": "", "э": "e", "ю": "yu", "я": "ya",
}

func containsCyrillicCharacters(s string) bool {
	for _, r := range s {
		if (r >= 'а' && r <= 'я') || r >= 'А' && r <= 'Я' {
			return true
		}
	}
	return false
}

func TransliterateToCyrillic(str string) string {
	if containsCyrillicCharacters(str) {
		return str
	}
	var result strings.Builder

	strlen := len(str)
	skipChars := 0
	for idx, char := range str {
		if skipChars > 0 {
			skipChars -= 1
			continue
		}
		lowerChar := unicode.ToLower(char)

		if cyrillicChar, ok := latinToCyrillicMap[string(lowerChar)]; ok {
			complexFound := false
			if strlen-idx >= 3 {
				// Take the next three characters from the index if possible
				substr := strings.ToLower(str[idx : idx+3])

				if complexChar, ok := latinToCyrillicMap[substr]; ok {
					cyrillicChar = complexChar
					complexFound = true
					skipChars = 2
				}
			}
			if !complexFound && strlen-idx >= 2 {
				// Take the next two characters from the index if possible
				substr := strings.ToLower(str[idx : idx+2])

				if complexChar, ok := latinToCyrillicMap[substr]; ok {
					cyrillicChar = complexChar
					complexFound = true
					skipChars = 1
				}
			}
			if unicode.IsUpper(char) {
				cyrillicChar = strings.ToUpper(cyrillicChar)
			}

			result.WriteString(cyrillicChar)
		} else {
			result.WriteString(string(char))
		}
	}
	// Log("result:", result.String())
	return result.String()
}

func TransliterateToLatin(str string) string {
	if !containsCyrillicCharacters(str) {
		return str
	}

	var result strings.Builder
	for _, char := range str {
		lowerChar := unicode.ToLower(char)
		if transliterated, ok := CyrillicToLatinMap[string(lowerChar)]; ok {
			if transliterated == "" {
				continue
			}
			if unicode.IsUpper(char) {
				transliterated = strings.ToUpper(transliterated)
			}
			result.WriteString(transliterated)
		} else {
			result.WriteString(string(char))
		}
	}
	return result.String()
}

func computeSimilarityScore(title1, title2 string, useJaroWinkler bool) int {
	title1 = norm.NFC.String(title1)
	title2 = norm.NFC.String(title2)

	nonAlphaNumRegex := regexp.MustCompile(`[^\p{L}\p{N}]+`)
	title1 = strings.Trim(nonAlphaNumRegex.ReplaceAllString(title1, " "), " ")
	title2 = strings.Trim(nonAlphaNumRegex.ReplaceAllString(title2, " "), " ")

	var similarity float64
	if useJaroWinkler {
		metric := &metrics.JaroWinkler{CaseSensitive: false}
		similarity = strutil.Similarity(title1, title2, metric)
	} else {
		metric := &metrics.SorensenDice{CaseSensitive: false, NgramSize: 2}
		similarity = strutil.Similarity(title1, title2, metric)
	}

	return int(similarity * 100)
}

func cleanupMovieFileName(fileName string, multipleVideoFiles bool) (string, string) {
	// Regular expression to match the release year in the file name
	yearRegex := regexp.MustCompile(`\b((?:19\d\d|20\d\d))\b`)

	// Find the release year in the file name
	yearMatches := yearRegex.FindStringSubmatch(fileName)
	var year string
	if len(yearMatches) > 0 {
		year = yearMatches[0]
	}

	// Extract the movie name by removing everything after the release year
	movieName := ""
	if len(yearMatches) > 0 {
		yearIndex := yearRegex.FindStringIndex(fileName)[0]
		movieName = fileName[:yearIndex]
	} else {
		movieName = fileName
	}
	// Log("1", movieName) // TODO: make debug levels: verbose, debug, info..

	seasonFolderRE := regexp.MustCompile(`(?:[Ss](?:eason)?)\s*(\d{1,2})\b`)
	seasonMatches := seasonFolderRE.FindStringSubmatch(movieName)
	if len(seasonMatches) > 0 {
		seasonIndex := seasonFolderRE.FindStringIndex(movieName)[0]
		movieName = movieName[:seasonIndex]
	}

	if multipleVideoFiles {
		leadingNumberRE := regexp.MustCompile(`^(\d{1,3})(?:[\. _]|$)`)
		trimmed := leadingNumberRE.ReplaceAllString(movieName, "")
		if trimmed != "" {
			movieName = trimmed
		}
	}

	episodeRE := regexp.MustCompile(`(?i)e(?:p(?:isode)?)?\s*(\d{1,3})`)
	if match := episodeRE.FindStringSubmatch(movieName); len(match) == 2 {
		epIndex := episodeRE.FindStringIndex(movieName)[0]
		trimmed := movieName[:epIndex]
		if trimmed != "" {
			movieName = trimmed
		}
	}

	// Log("2", movieName)
	if movieName == "" {
		movieName = strings.TrimSuffix(fileName, filepath.Ext(fileName))
	}
	// Log("3", movieName)
	mediaInfoRE := regexp.MustCompile(`(?i)[^a-z0-9]([a-z]+rip|ts|avc|hdr|sdr|uhd|dvd|mvo|matroska|web|dub|Сериал)(?:[^a-z0-9]|$)`)
	mediaInfoMatches := mediaInfoRE.FindStringSubmatch(movieName)
	if len(mediaInfoMatches) > 0 {
		mediaInfoIndex := mediaInfoRE.FindStringIndex(movieName)[0]
		movieName = movieName[:mediaInfoIndex]
	}
	// Log("4", movieName)
	if movieName == "" {
		movieName = strings.TrimSuffix(fileName, filepath.Ext(fileName))
	}
	// Log("5", movieName)

	// Regular expression to match parentheses and extract text before it
	parenRegex := regexp.MustCompile(`^([^(\[\{]*?)\s*[(\[{].+`)
	parenMatches := parenRegex.FindStringSubmatch(movieName)
	if len(parenMatches) > 1 {
		// Extract the movie name from the part before the parentheses
		// Logf("extracting movie name before paren: %s\n", parenMatches[1])
		movieName = parenMatches[1]
	}
	// Log("6", movieName)

	if strings.HasPrefix(strings.ToLower(movieName), "bbc") && len(movieName) > 4 {
		movieName = movieName[3:]
	}
	// Log("8", movieName)

	// try extracting year from the movie name (again)
	if year == "" {
		yearRegex = regexp.MustCompile(`^.*[^\d]((?:19\d\d|20\d\d))$`)
		// Find the release year in the file name
		yearMatches = yearRegex.FindStringSubmatch(movieName)
		if len(yearMatches) > 1 {
			year = yearMatches[1]
			movieName = strings.TrimSuffix(movieName, year)
		}
	}
	// Log("8", movieName)

	movieName = norm.NFC.String(movieName)
	nonAlphaNumRegex := regexp.MustCompile(`[^'\p{L}\p{N}]+`)
	cleanedMovieName := strings.Trim(nonAlphaNumRegex.ReplaceAllString(movieName, " "), " ")

	y := ""
	if year != "" {
		y = "(" + year + ")"
	}
	Log("initial:", movieName, "clean:", cleanedMovieName, y)
	return cleanedMovieName, year
}

func commonPrefix(str1, str2 string) string {
	fileName1 := norm.NFC.String(str1)
	fileName2 := norm.NFC.String(str2)

	runes1 := []rune(fileName1)
	runes2 := []rune(fileName2)

	var prefix string
	for idx, rune1 := range runes1 {
		if idx >= len(runes2) {
			break
		}
		rune2 := runes2[idx]
		if rune1 != rune2 {
			break
		}

		prefix += string(rune1)
	}

	trimmedSuffixRegex := regexp.MustCompile(`(?:_|\W)+$`)
	prefix = trimmedSuffixRegex.ReplaceAllString(prefix, "")

	return prefix
}

func Coalesce(str1, str2 string) string {
	if str1 == "" {
		return str2
	}
	return str1
}

func Coalesce3(str1, str2, str3 string) string {
	return Coalesce(str1, Coalesce(str2, str3))
}

func Coalesce4(str1, str2, str3, str4 string) string {
	return Coalesce(str1, Coalesce(str2, Coalesce(str3, str4)))
}

// ReplaceInvalidFilenameChars replaces invalid characters in a string
// that cannot be used in filenames with underscores.
func ReplaceInvalidFilenameChars(s string) string {
	name, err := url.QueryUnescape(s)
	if err != nil {
		Log("could not query-unescape", s)
		name = s
	}

	re1 := regexp.MustCompile(`(?i)(https:\/\/|api_key=[0-9a-z]+&?)`)
	name = re1.ReplaceAllString(name, "")

	re2 := regexp.MustCompile(`(?i)(api.themoviedb.org)`)
	name = re2.ReplaceAllString(name, "tmdb_")

	re3 := regexp.MustCompile(`(?i)((?:www\.)kinopoisk.ru)`)
	name = re3.ReplaceAllString(name, "kp_")

	re4 := regexp.MustCompile(`(?i)((?:www\.)imdb.com)`)
	name = re4.ReplaceAllString(name, "imdb_")

	re5 := regexp.MustCompile(`[^\%\.\-\p{L}\p{N}]+`)
	name = re5.ReplaceAllString(name, "_")

	return name
}

func FindCommonItems(arr1 []string, arr2 []string, caseSensitive bool) int {
	result := 0
	for _, itm1 := range arr1 {
		for _, itm2 := range arr2 {
			if !caseSensitive {
				itm1 = strings.ToLower(itm1)
				itm2 = strings.ToLower(itm2)
			}
			if itm1 == itm2 {
				result += 1
			}
		}
	}
	return result
}
