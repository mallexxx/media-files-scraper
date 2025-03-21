package main

import (
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

func logPath() string {
	// Get the current directory of the executable
	exePath, err := os.Executable()
	if err != nil {
		panic(err)
	}
	dir := filepath.Dir(exePath)

	return filepath.Join(dir, "log.txt")
}

// Log writes a message to both the console and the log file
func Log(args ...interface{}) {
	logger.Println(args...)
}

// Logf writes a formatted message to both the console and the log file
func Logf(format string, args ...interface{}) {
	logger.Printf(format, args...)
}

func videoExistsInOutDirs(filePath Path, config Config) *Path {
	name := filePath.lastPathComponent()
	moviesDir := findSuitableDirectoryForSymlink(filePath, config.Output.Movies)
	moviesPath := moviesDir.appendingPathComponent(name)
	if moviesPath.exists() {
		return &moviesPath
	}

	seriesDir := findSuitableDirectoryForSymlink(filePath, config.Output.Series)
	seriesPath := seriesDir.appendingPathComponent(name)
	if seriesPath.exists() {
		return &seriesPath
	}
	return nil
}

func movieFileNameWithoutExtension(videoFiles []Path) string {
	if len(videoFiles) == 1 {
		return string(videoFiles[0].removingPathExtension().lastPathComponent())
	} else if len(videoFiles) == 2 {
		commonPrefix := commonPrefix(videoFiles[0].lastPathComponent(), videoFiles[1].lastPathComponent())
		regex := regexp.MustCompile(`(?i)\s*[_.,-]?(?:part|pt)\s*$`)
		name := regex.ReplaceAllString(commonPrefix, "")
		if name == "" {
			return videoFiles[0].removingLastPathComponent().lastPathComponent()
		}
		return name

	} else {
		panic(fmt.Sprintf("unexpected number of video files: %d", len(videoFiles)))
	}
}

func writeMovieNfo(mediaInfo MediaFilesInfo, output Path) error {
	fileName := movieFileNameWithoutExtension(mediaInfo.VideoFiles) + ".nfo"
	filePath := output.appendingPathComponent(fileName)
	Log("Writing Movie Nfo to", filePath)
	// Create or truncate the .nfo file
	file, err := os.Create(string(filePath))
	if err != nil {
		return err
	}
	defer file.Close()

	writeMovieNfoXML(file, mediaInfo.Info)

	return nil
}

func writeMovieNfoXML(w io.Writer, mediaInfo MediaInfo) {
	enc := xml.NewEncoder(w)
	enc.Indent("", "    ")

	enc.EncodeToken(xml.StartElement{Name: xml.Name{Local: "movie"}})

	enc.EncodeElement(mediaInfo.Title, xml.StartElement{Name: xml.Name{Local: "title"}})
	enc.EncodeElement(mediaInfo.Id.id, xml.StartElement{Name: xml.Name{Local: "uniqueid"}, Attr: []xml.Attr{{Name: xml.Name{Local: "type"}, Value: mediaInfo.Id.getType()}, {Name: xml.Name{Local: "default"}, Value: "true"}}})
	urlName := "url"
	if mediaInfo.Id.idType == IMDB {
		urlName = "imdburl"
	} else if mediaInfo.Id.idType == TMDB {
		urlName = "tmdburl"
	} else if mediaInfo.Id.idType == KPID {
		urlName = "kpurl"
	}
	// TODO: Write ratings ?
	if mediaInfo.OriginalTitle != "" {
		enc.EncodeElement(mediaInfo.OriginalTitle, xml.StartElement{Name: xml.Name{Local: "originaltitle"}})
	}
	if mediaInfo.Description != "" {
		enc.EncodeElement(mediaInfo.Description, xml.StartElement{Name: xml.Name{Local: "plot"}})
	}
	if mediaInfo.Year != "" {
		enc.EncodeElement(mediaInfo.Year, xml.StartElement{Name: xml.Name{Local: "year"}})
	}
	for _, genre := range mediaInfo.Genres {
		enc.EncodeElement(genre, xml.StartElement{Name: xml.Name{Local: "genre"}})
	}
	enc.EncodeElement(mediaInfo.Url, xml.StartElement{Name: xml.Name{Local: urlName}})

	enc.EncodeToken(xml.EndElement{Name: xml.Name{Local: "movie"}})
	enc.Flush()
}

func writeTVShowNfo(mediaInfo MediaInfo, nfoPath Path) error {
	Log("Writing TVShow Nfo to", nfoPath)
	// Create or truncate the .nfo file
	file, err := os.Create(string(nfoPath))
	if err != nil {
		return err
	}
	defer file.Close()

	writeTVShowNfoXML(file, mediaInfo)

	return nil
}

func writeTVShowNfoXML(w io.Writer, mediaInfo MediaInfo) {
	enc := xml.NewEncoder(w)
	enc.Indent("", "    ")

	enc.EncodeToken(xml.StartElement{Name: xml.Name{Local: "tvshow"}})

	enc.EncodeElement(mediaInfo.Title, xml.StartElement{Name: xml.Name{Local: "title"}})
	enc.EncodeElement(mediaInfo.Id.id, xml.StartElement{Name: xml.Name{Local: "uniqueid"}, Attr: []xml.Attr{{Name: xml.Name{Local: "type"}, Value: mediaInfo.Id.getType()}, {Name: xml.Name{Local: "default"}, Value: "true"}}})
	urlName := "url"
	if mediaInfo.Id.idType == IMDB {
		urlName = "imdburl"
	} else if mediaInfo.Id.idType == TMDB {
		urlName = "tmdburl"
	} else if mediaInfo.Id.idType == KPID {
		urlName = "kpurl"
	}
	// TODO: write ratings ?
	if mediaInfo.OriginalTitle != "" {
		enc.EncodeElement(mediaInfo.OriginalTitle, xml.StartElement{Name: xml.Name{Local: "originaltitle"}})
	}
	if mediaInfo.Description != "" {
		enc.EncodeElement(mediaInfo.Description, xml.StartElement{Name: xml.Name{Local: "plot"}})
	}
	if mediaInfo.Year != "" {
		enc.EncodeElement(mediaInfo.Year, xml.StartElement{Name: xml.Name{Local: "year"}})
	}
	for _, genre := range mediaInfo.Genres {
		enc.EncodeElement(genre, xml.StartElement{Name: xml.Name{Local: "genre"}})
	}
	enc.EncodeElement(mediaInfo.Url, xml.StartElement{Name: xml.Name{Local: urlName}})

	enc.EncodeToken(xml.EndElement{Name: xml.Name{Local: "tvshow"}})
	enc.Flush()
}

func writeEpisodeNfo(season int, episode int, title string, originalTitle string, mediaInfo MediaInfo, nfoPath Path) error {
	Log("Writing Episode Nfo to", nfoPath)
	// Create or truncate the .nfo file
	file, err := os.Create(string(nfoPath))
	if err != nil {
		return err
	}
	defer file.Close()

	writeEpisodeNfoXML(file, season, episode, title, originalTitle, mediaInfo)

	return nil
}

func writeEpisodeNfoXML(w io.Writer, season int, episode int, title string, originalTitle string, mediaInfo MediaInfo) {
	enc := xml.NewEncoder(w)
	enc.Indent("", "    ")

	enc.EncodeToken(xml.StartElement{Name: xml.Name{Local: "episodedetails"}})

	enc.EncodeElement(title, xml.StartElement{Name: xml.Name{Local: "title"}})
	enc.EncodeElement(originalTitle, xml.StartElement{Name: xml.Name{Local: "originaltitle"}})

	enc.EncodeElement(mediaInfo.Title, xml.StartElement{Name: xml.Name{Local: "showtitle"}})

	enc.EncodeElement(season, xml.StartElement{Name: xml.Name{Local: "season"}})
	enc.EncodeElement(episode, xml.StartElement{Name: xml.Name{Local: "episode"}})

	enc.EncodeToken(xml.EndElement{Name: xml.Name{Local: "episodedetails"}})
	enc.Flush()
}

type TVShow struct {
	UniqueIds []UniqueId `xml:"uniqueid"`
	URL       string     `xml:"url"`
	TMDBURL   string     `xml:"tmdburl"`
}

type UniqueId struct {
	Type  string `xml:"type,attr"`
	Value string `xml:",chardata"`
}

func readTVShowNfo(path Path) (MediaId, error) {
	// Read the XML file
	xmlFile, err := os.Open(string(path))
	if err != nil {
		return MediaId{}, err
	}
	defer xmlFile.Close()

	// Read the XML content
	byteValue, err := ioutil.ReadAll(xmlFile)
	if err != nil {
		return MediaId{}, err
	}

	// Parse the XML content
	var tvShow TVShow
	err = xml.Unmarshal(byteValue, &tvShow)
	if err != nil {
		return MediaId{}, err
	}

	imdbID := ""
	for _, uniqueId := range tvShow.UniqueIds {
		if uniqueId.Type == "tmdb" {
			return MediaId{id: uniqueId.Value, idType: TMDB}, nil
		} else if uniqueId.Type == "imdb" {
			imdbID = uniqueId.Value
		} else if uniqueId.Type == "kinopoisk" {
			return MediaId{id: uniqueId.Value, idType: KPID}, nil
		}
	}
	if imdbID != "" {
		return MediaId{id: imdbID, idType: IMDB}, nil
	}

	Log("could not id from TV Show NFO: uniqueIds", tvShow.UniqueIds)
	return MediaId{}, nil
}

// Function to get video contents at a specified path
func getVideoFiles(path Path) []Path {
	if strings.HasPrefix(path.lastPathComponent(), ".") {
		return nil
	}

	info, err := os.Stat(string(path))
	if err != nil {
		Log("Error:", err)
		return nil
	}

	if info.IsDir() {
		// If it's a directory, find all video files recursively
		return path.findVideoFilesInFolder()
	} else if path.isVideoFile() {
		// If it's a video file, return the single file path
		return []Path{path}
	}

	// Return empty slice if not a video file or directory
	return nil
}

func getSeasonEpisodeFromPath(filePath Path, videoFiles []Path) (int /*season*/, int /*episode*/) {
	// Get the file name without extension
	fileName := filePath.removingPathExtension().lastPathComponent()

	// Regular expressions to match different formats
	seasonEpisodeRE := regexp.MustCompile(`(?i)(?:s(?:eason)?)[\s\W]*(\d{1,2})[\s\W]*(?:e(?:p(?:isode)?)?)\s*(\d{1,3})`)
	seasonRE := regexp.MustCompile(`(?i)(?:s(?:eason)?)[\s\W]*(\d{1,2})`)
	seasonFolderRE := regexp.MustCompile(`(?i)(?:[^0-9]|^)(?:s(?:eason)?|сезон)[\s\W]*(\d{1,2})\b`)

	var seasonNumber, episodeNumber int

	// Try to match season and episode numbers
	if match := seasonEpisodeRE.FindStringSubmatch(fileName); len(match) == 3 {
		seasonNumber, _ = strconv.Atoi(match[1])
		episodeNumber, _ = strconv.Atoi(match[2])
	} else if match := seasonRE.FindStringSubmatch(fileName); len(match) == 2 {
		seasonNumber, _ = strconv.Atoi(match[1])
	} else {
		// If no explicit season number in file name, check parent folder
		dir := filePath.removingLastPathComponent()
		parentFolder := dir.lastPathComponent()
		if match := seasonFolderRE.FindStringSubmatch(parentFolder); len(match) == 2 {
			seasonNumber, _ = strconv.Atoi(match[1])
		} else {
			// If no season number found anywhere, set to 1
			seasonNumber = 1
		}
	}

	// If episode number is still zero, check if it's in the format "01 name"
	if episodeNumber == 0 {
		episodeRE := regexp.MustCompile(`^(\d{1,3})(?:\D|$)`)
		if match := episodeRE.FindStringSubmatch(fileName); len(match) == 2 {
			episodeNumber, _ = strconv.Atoi(match[1])
		}
		episodeRE = regexp.MustCompile(`(?i)e(?:p(?:isode)?)?\s*(\d{1,3})`)
		if match := episodeRE.FindStringSubmatch(fileName); len(match) == 2 {
			episodeNumber, _ = strconv.Atoi(match[1])
		}
	}
	if episodeNumber == 0 {
		videoFileStrings := mapSlice(videoFiles, func(path Path) string { return string(path) })
		sort.Strings(videoFileStrings)
		idx := findIndex(videoFileStrings, string(filePath))
		episodeNumber = idx + 1
	}

	return seasonNumber, episodeNumber
}

func linkVideoFileAndRelatedItems(videoFile Path, output Path, targetNameWithoutExtension string, multipart bool) error {
	name := strings.ToLower(videoFile.removingPathExtension().lastPathComponent())
	dir := videoFile.removingLastPathComponent()
	contents, err := dir.getDirectoryContents()
	if err != nil {
		return err
	}
	for _, filePath := range contents {
		if !strings.HasPrefix(strings.ToLower(filePath.lastPathComponent()), name+".") {
			// Log("skipping", filePath.lastPathComponent(), "noprefix", name+".")
			continue
		}

		prefixLen := len(name)
		ext := strings.TrimPrefix(filePath.lastPathComponent()[prefixLen:], ".")

		if multipart && filePath.isVideoFile() {
			regexBegin := regexp.MustCompile(`^(\d\d?)`)
			matches := regexBegin.FindStringSubmatch(filePath.lastPathComponent())
			if len(matches) == 2 {
				ext = "part" + matches[1] + "." + ext
			} else {

				regex := regexp.MustCompile(`[^\d]+(\d\d?)[^\d]*$`)
				matches = regex.FindStringSubmatch(filePath.lastPathComponent())
				if len(matches) == 2 {
					ext = "part" + matches[1] + "." + ext
				} else {
					panic(fmt.Sprint("could not extract part number from", filePath))
				}
			}
		}

		outName := targetNameWithoutExtension + "." + ext
		outPath := output.appendingPathComponent(outName)

		if outPath.exists() {
			// Log(outPath, "exists")
			continue
		}
		Log("creating link for", filePath.lastPathComponent(), "at", outPath)

		err := os.Symlink(string(filePath), string(outPath))
		if err != nil {
			return err
		}
	}

	return nil
}

func downloadImage(url string, filepath Path) error {
	// In test mode, just create an empty file instead of downloading
	if os.Getenv("TEST_MODE") == "true" {
		Log("🧪 TEST MODE: Creating empty image file instead of downloading:", filepath)

		// Ensure the directory exists
		dir := Path(string(filepath.removingLastPathComponent()))
		if !dir.exists() {
			if err := os.MkdirAll(string(dir), 0755); err != nil {
				return err
			}
		}

		// Create an empty file
		out, err := os.Create(string(filepath))
		if err != nil {
			return err
		}
		defer out.Close()

		Logf("Empty image placeholder created at %s\n", filepath)
		return nil
	}

	// Standard download for non-test mode
	// Send GET request to fetch the image
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Create the output file
	out, err := os.Create(string(filepath))
	if err != nil {
		return err
	}
	defer out.Close()

	// Copy the image data from the HTTP response body to the output file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	Logf("Image downloaded to %s\n", filepath)
	return nil
}

func findSuitableDirectoryForSymlink(path Path, directories []Path) Path {
	volumeName := filepath.VolumeName(string(path))
	for _, directory := range directories {
		if filepath.VolumeName(string(directory)) == volumeName {
			return directory
		}
	}
	return Path("")
}
