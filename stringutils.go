package main

import (
	"fmt"
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
		// Trim the spaces from each part
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		// Take the last part as the original title
		originalTitle := parts[len(parts)-1]

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
	"A": "А", "B": "Б", "C": "Ц", "D": "Д", "E": "Е", "F": "Ф", "G": "Г", "H": "Х", "I": "И", "J": "Й", "K": "К", "L": "Л", "M": "М",
	"N": "Н", "O": "О", "P": "П", "Q": "К", "R": "Р", "S": "С", "T": "Т", "U": "У", "V": "В", "W": "В", "X": "КС", "Y": "Ы", "Z": "З",
	"ch": "ч", "zh": "ж", "sh": "ш", "sch": "щ", "yo": "ё", "jo": "ё", "yu": "ю", "ju": "ю", "ya": "я", "ja": "я",
	"CH": "Ч", "ZH": "Ж", "SH": "Ш", "SCH": "", "YO": "Ë", "JO": "Ë", "YU": "Ю", "JU": "Ю", "YA": "Я", "JA": "Я",
	"'": "ь",
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
	var result strings.Builder

	strlen := len(str)
	idx := 0
	for {
		if idx >= strlen {
			break
		}

		char := str[idx]
		if cyrillicChar, ok := latinToCyrillicMap[string(char)]; ok {
			complexFound := false
			if strlen-idx >= 3 {
				// Take the next three characters from the index if possible
				substr := str[idx : idx+3]
				if unicode.IsUpper(rune(char)) {
					substr = strings.ToUpper(substr)
				} else {
					substr = strings.ToLower(substr)
				}
				if complexChar, ok := latinToCyrillicMap[substr]; ok {
					cyrillicChar = complexChar
					complexFound = true
					idx += 2
				}
			}
			if !complexFound && strlen-idx >= 2 {
				// Take the next two characters from the index if possible
				substr := str[idx : idx+2]
				if unicode.IsUpper(rune(char)) {
					substr = strings.ToUpper(substr)
				} else {
					substr = strings.ToLower(substr)
				}
				if complexChar, ok := latinToCyrillicMap[substr]; ok {
					cyrillicChar = complexChar
					complexFound = true
					idx += 1
				}
			}

			result.WriteString(cyrillicChar)
		} else {
			result.WriteString(string(char))
		}
		idx += 1
	}
	// fmt.Println("result:", result.String())
	return result.String()
}

func computeSimilarityScore(title1, title2 string) int {
	title1 = norm.NFC.String(title1)
	title2 = norm.NFC.String(title2)

	nonAlphaNumRegex := regexp.MustCompile(`[^\p{L}\p{N}]+`)
	title1 = strings.Trim(nonAlphaNumRegex.ReplaceAllString(title1, " "), " ")
	title2 = strings.Trim(nonAlphaNumRegex.ReplaceAllString(title2, " "), " ")

	// metric := &metrics.JaroWinkler{CaseSensitive: false}
	metric := &metrics.SorensenDice{CaseSensitive: false, NgramSize: 2}
	similarity := strutil.Similarity(title1, title2, metric)

	return int(similarity * 100)
}

func cleanupMovieFileName(fileName string) (string, string) {
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
	// fmt.Println("1", movieName) // TODO: make debug levels: verbose, debug, info..

	seasonFolderRE := regexp.MustCompile(`(?:[Ss](?:eason)?)\s*(\d{1,2})\b`)
	seasonMatches := seasonFolderRE.FindStringSubmatch(movieName)
	if len(seasonMatches) > 0 {
		seasonIndex := seasonFolderRE.FindStringIndex(movieName)[0]
		movieName = movieName[:seasonIndex]
	}
	// fmt.Println("2", movieName)
	if movieName == "" {
		movieName = strings.TrimSuffix(fileName, filepath.Ext(fileName))
	}
	// fmt.Println("3", movieName)
	mediaInfoRE := regexp.MustCompile(`(?i)[^a-z0-9]([a-z]+rip|ts|avc|hdr|sdr|uhd|dvd|mvo|matroska|web|dub|Сериал)(?:[^a-z0-9]|$)`)
	mediaInfoMatches := mediaInfoRE.FindStringSubmatch(movieName)
	if len(mediaInfoMatches) > 0 {
		mediaInfoIndex := mediaInfoRE.FindStringIndex(movieName)[0]
		movieName = movieName[:mediaInfoIndex]
	}
	// fmt.Println("4", movieName)
	if movieName == "" {
		movieName = strings.TrimSuffix(fileName, filepath.Ext(fileName))
	}
	// fmt.Println("5", movieName)

	// Regular expression to match parentheses and extract text before it
	parenRegex := regexp.MustCompile(`^([^(\[\{]*?)\s*[(\[{].+`)
	parenMatches := parenRegex.FindStringSubmatch(movieName)
	if len(parenMatches) > 1 {
		// Extract the movie name from the part before the parentheses
		// fmt.Printf("extracting movie name before paren: %s\n", parenMatches[1])
		movieName = parenMatches[1]
	}
	// fmt.Println("6", movieName)

	if strings.HasPrefix(strings.ToLower(movieName), "bbc") && len(movieName) > 4 {
		movieName = movieName[3:]
	}
	// fmt.Println("8", movieName)

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
	// fmt.Println("8", movieName)

	movieName = norm.NFC.String(movieName)
	nonAlphaNumRegex := regexp.MustCompile(`[^\p{L}\p{N}]+`)
	cleanedMovieName := strings.Trim(nonAlphaNumRegex.ReplaceAllString(movieName, " "), " ")

	y := ""
	if year != "" {
		y = "(" + year + ")"
	}
	fmt.Println("initial:", movieName, "clean:", cleanedMovieName, y)
	return cleanedMovieName, year
}

func commonPrefix(str1, str2 string) string {
	minLength := len(str1)
	if len(str2) < minLength {
		minLength = len(str2)
	}

	var prefix string
	for i := 0; i < minLength; i++ {
		if str1[i] != str2[i] {
			break
		}
		prefix += string(str1[i])
	}

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