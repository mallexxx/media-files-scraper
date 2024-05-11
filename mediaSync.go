package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/hekmon/transmissionrpc/v3"
)

type MediaFilesInfo struct {
	Info       MediaInfo
	VideoFiles []Path
	Path       Path
}

// process all media folders and sync media items
func runMediaSync(config Config) error {
	dirs := config.Directories
	if config.Transmission.UnsortedDir != "" && config.Transmission.UnsortedDir.isDirectory() {
		dirs = append(dirs, config.Transmission.UnsortedDir)
	}

	var matchedItems []Path
	for _, dir := range dirs {
		output, err := runMediaSyncForDir(dir, config)
		if err != nil {
			return err
		}
		matchedItems = append(matchedItems, output...)
	}

	// remove orphaned output items against matchedItems
	var existingItems map[string]bool = make(map[string]bool)
	for _, path := range matchedItems {
		existingItems[strings.ToLower(string(path))] = true
		existingItems[strings.ToLower(string(path.removingPathExtension().appendingPathExtension("nfo")))] = true
		existingItems[strings.ToLower(string(path.removingPathExtension())+"-poster.jpg")] = true
		existingItems[strings.ToLower(string(path.removingPathExtension())+"-fanart.jpg")] = true
	}
	outputDirsArrays := [][]Path{config.Output.Movies, config.Output.Series}
	for _, outputDirs := range outputDirsArrays {
		for _, output := range outputDirs {
			contents, err := output.getDirectoryContents()
			if err != nil {
				return err
			}
			for _, path := range contents {
				if _, ok := existingItems[strings.ToLower(string(path))]; !ok {
					if videoSymlink := path.findRelatedVideoSymlink(); videoSymlink != "" {
						sourceDir, err := config.sourceDirectoryForVideoSymlink(videoSymlink)
						if err != nil {
							Log("❌ Error reading symlink:", err)
						} else /*if sourceDir != ""*/ {
							if !sourceDir.exists() {
								Log("⏏️ Symlink points to an unavailable source:", sourceDir)
								existingItems[strings.ToLower(string(videoSymlink))] = true
								existingItems[strings.ToLower(string(videoSymlink.removingPathExtension().appendingPathExtension("nfo")))] = true
								existingItems[strings.ToLower(string(videoSymlink.removingPathExtension())+"-poster.jpg")] = true
								existingItems[strings.ToLower(string(videoSymlink.removingPathExtension())+"-fanart.jpg")] = true
								continue
							} else {
								Log("🚢 Symlink points to source:", sourceDir)
								continue
							}
						}
					}

					Log("🪓 removing orphaned item", path)
					err := path.removeItem()
					if err != nil {
						Log("❌", err)
					}
				}
			}
		}
	}

	return nil
}

// process one media folder and sync its media items
func runMediaSyncForDir(directory Path, config Config) ([]Path, error) {
	if !directory.exists() {
		Log("⏏️ directory not available:", directory)
		return nil, nil
	}
	Log("🍋 scanning", directory)

	directoryContents, err := directory.getDirectoryContents()
	if err != nil {
		return []Path{}, err
	}
	var matchedItems []Path
	var torrents map[string]transmissionrpc.Torrent
	for _, item := range directoryContents {
		output, err := processMediaItem(item, config, &torrents)
		if _, ok := err.(*NoMediaItemsError); ok {
			continue
		} else if err != nil {
			return []Path{}, err
		}
		matchedItems = append(matchedItems, output...)
	}

	return matchedItems, nil
}

// process one media item in a folder
// returns paths in output directory matched against the original items
func processMediaItem(path Path, config Config, torrents *map[string]transmissionrpc.Torrent) ([]Path, error) {
	if outDir := videoExistsInOutDirs(path, config); outDir != nil {
		Log(path, "already processed")
		output := []Path{*outDir}
		seriesDir := findSuitableDirectoryForSymlink(path, config.Output.Series)
		if outDir.removingLastPathComponent() == seriesDir {
			// sync TV Show media files if missing
			_, err := syncTvShow(MediaFilesInfo{Path: path, Info: MediaInfo{}}, seriesDir, config)
			if err != nil {
				return []Path{}, nil
			}
		} else if outDir.isDirectory() {
			contents, err := outDir.getDirectoryContents()
			if err != nil {
				return []Path{}, err
			}
			// it‘s a fake (empty) directory, movies from the original dir are placed nearby
			if len(contents) == 0 {
				videoFiles := getVideoFiles(path)
				moviesDir := findSuitableDirectoryForSymlink(path, config.Output.Movies)
				for _, path := range videoFiles {
					outputPath := moviesDir.appendingPathComponent(path.lastPathComponent())
					// Log("🟠 taking nearby file", outputPath)
					output = append(output, outputPath)
				}
			}
		}
		return output, nil
	}
	Log("➡️ Updating metadata for:", path)

	mediaInfo, err := getMediaInfo(path, torrents, config)

	if multiMoviesErr, ok := err.(*FolderSeemsContainingMultipleMoviesError); ok {
		tmpMediaInfo := MediaFilesInfo{
			Path:       path,
			VideoFiles: multiMoviesErr.videoFiles,
			Info: MediaInfo{
				IsTvShow: false,
			},
		}
		if err := moveMediaItemFromUnsortedIfNeeded(&path, torrents, &tmpMediaInfo, config); err != nil {
			Log("❌ move from unsorted failed", err)
			return []Path{}, err
		}

		var output []Path
		// it seems the media item folder contains separate movie files, process them individually
		for _, videoFile := range tmpMediaInfo.VideoFiles {
			output, err = processMediaItem(videoFile, config, torrents)
			if err != nil {
				return nil, err
			}
		}
		// create folder at output path to ignore the item in future
		moviesDir := findSuitableDirectoryForSymlink(path, config.Output.Movies)
		if moviesDir == "" {
			return []Path{}, fmt.Errorf("no same-volume directory suitable for %s found in config.Output.Series", path)
		}
		dirPath := moviesDir.appendingPathComponent(path.lastPathComponent())
		err = os.MkdirAll(string(dirPath), 0755)
		output = append(output, dirPath)
		Log("🌕 independent proc", output)
		return output, err
	}
	if err != nil {
		return []Path{}, err
	}
	Log("✅", mediaInfo.Info.Title, "/", mediaInfo.Info.OriginalTitle, mediaInfo.Info.Year)

	if err := moveMediaItemFromUnsortedIfNeeded(&path, torrents, &mediaInfo, config); err != nil {
		Log("❌ move from unsorted failed", err)
		return []Path{}, err
	}

	// If no poster found for TMDB item
	if mediaInfo.Info.Id.idType == TMDB && mediaInfo.Info.PosterUrl == "" {
		kpApi := KinopoiskAPI{ApiKey: config.KinopoiskApiKey, GenresMap: config.KinopoiskGenres}
		Log("fetching posters for", mediaInfo.Info.OriginalTitle)
		if movie, score, err := findMovieByTitle(kpApi, mediaInfo.Info.OriginalTitle, mediaInfo.Info.Year); err == nil && score > 80 {
			mediaInfo = MediaFilesInfo{Info: movie, Path: mediaInfo.Path, VideoFiles: mediaInfo.VideoFiles}
		}
	}

	output, err := syncMediaItemFiles(mediaInfo, config)
	if err != nil {
		return nil, err
	}

	return []Path{output}, nil
}

func moveMediaItemFromUnsortedIfNeeded(path *Path, torrents *map[string]transmissionrpc.Torrent, mediaInfo *MediaFilesInfo, config Config) error {
	unsortedDir := strings.ToLower(strings.TrimSuffix(string(config.Transmission.UnsortedDir.appendingPathComponent("a")), "a")) // get path with trailing [back]slash
	if !strings.HasPrefix(strings.ToLower(string(*path)), unsortedDir) {
		return nil
	}

	var outDir Path
	if mediaInfo.Info.IsTvShow && config.Transmission.DefaultSeriesDest != "" {
		outDir = config.Transmission.DefaultSeriesDest
	} else if config.Transmission.DefaultMoviesDest != "" {
		outDir = config.Transmission.DefaultMoviesDest
	}
	// find matching rules
RuleLoop:
	for _, rule := range config.Transmission.SortingRules {
		for _, genre := range mediaInfo.Info.Genres {
			if rule.GenreRegex.MatchString(genre) {
				outDir = rule.Destination
				break RuleLoop
			}
		}
	}
	if outDir == "" {
		return fmt.Errorf("could not determine destination path for %s", mediaInfo.Info.Title)
	}

	torrent, ok := (*torrents)[strings.ToLower(string(mediaInfo.Path))]
	if !ok {
		return fmt.Errorf("torrent not found for %s", string(mediaInfo.Path))
	}
	Log("moving torrent", *torrent.Name, "to", outDir)
	err := moveTorrent(*torrent.ID, outDir, config.Transmission.Rpc)
	if err != nil {
		return err
	}

	*path = outDir.appendingPathComponent(string(*path)[len(unsortedDir):])
	mediaInfo.Path = outDir.appendingPathComponent(string(mediaInfo.Path)[len(unsortedDir):])
	for idx, path := range mediaInfo.VideoFiles {
		mediaInfo.VideoFiles[idx] = outDir.appendingPathComponent(string(path)[len(unsortedDir):])
	}

	return nil
}

type FolderSeemsContainingMultipleMoviesError struct {
	videoFiles []Path
}
type NoMediaItemsError struct {
}

func (ce *FolderSeemsContainingMultipleMoviesError) Error() string {
	return "The folder seems containing multiple movie files"
}

func (ce *NoMediaItemsError) Error() string {
	return "No media files found for the item"
}

// get MediaInfo for a media item
func getMediaInfo(path Path, torrents *map[string]transmissionrpc.Torrent, config Config) (MediaFilesInfo, error) {
	var title string
	var year string
	var imdbId string
	var err error

	videoFiles := getVideoFiles(path)
	if len(videoFiles) == 0 {
		Log("🚫 no video files found, skipping")
		return MediaFilesInfo{}, &NoMediaItemsError{}
	}

	// load torrents if needed
	if *torrents == nil && config.Transmission.Rpc != "" {
		torrentsVal, err := getTorrentsByPath(config.Transmission.Rpc)
		if err != nil {
			Log("❌ could not load torrent list", err)
		} else {
			Log("loaded", len(torrentsVal), "torrents")
			*torrents = torrentsVal
		}
	}
	if *torrents == nil {
		*torrents = make(map[string]transmissionrpc.Torrent)
	}

	tmdbAPI := TMDbAPI{ApiKey: config.TMDbApiKey, MovieGenres: config.TMDbMovieGenres, TvGenres: config.TMDbTvGenres}

	// Find torrent by lowercased file path
	torrent, ok := (*torrents)[strings.ToLower(string(path))]
	if ok {
		// load torrent info from tracker
		title, year, imdbId, err = loadTitleYearIMDbIdFromRutracker(*torrent.Comment)
		if err == nil && imdbId != "" {
			videoFiles := getVideoFiles(path)
			mediaInfo, err := loadIMDbMediaInfo(imdbId, tmdbAPI)
			if err != nil {
				return MediaFilesInfo{}, err
			}
			return MediaFilesInfo{Info: mediaInfo, Path: path, VideoFiles: videoFiles}, nil
		} else if err != nil {
			Log("could not retreive torrent data:", err)
		}
	}
	if title == "" {
		var fileName string
		if path.isDirectory() {
			fileName = path.lastPathComponent()
		} else {
			fileName = path.removingPathExtension().lastPathComponent()
		}
		// extract title and year from file name
		title, year = cleanupMovieFileName(fileName, len(videoFiles) > 1 /*multipleVideoFiles*/)
	}

	// could not extract title ?!
	if title == "" {
		return MediaFilesInfo{}, fmt.Errorf("could not determine movie name for '%s'", path.lastPathComponent())
	}

	if len(videoFiles) > 1 {
		seasonEpisodeRE := regexp.MustCompile(`(?:[Ss](?:eason)?)[\s\W]*(\d{1,2})[\s\W]*(?:[Ee](?:pisode)?)\s*(\d+)`)
		if match := seasonEpisodeRE.FindStringSubmatch(videoFiles[0].lastPathComponent()); len(match) == 3 {
			// it‘s a tv series – name matches S01E02 pattern
		} else if len(videoFiles) == 2 && computeSimilarityScore(string(videoFiles[0]), string(videoFiles[1]), false) > 90 {
			// likely it‘s a 2-part movie
			mediaInfo, score, err := findMovieMediaInfo(path, title, year, config)
			if err == nil && score > 80 {
				return MediaFilesInfo{Info: mediaInfo, Path: path, VideoFiles: videoFiles}, nil
			}
		}

		// likely it‘s TV Series
		tmdbAPI.TVShowSearch = true
		mediaInfo, score, err := findMovieByTitle(tmdbAPI, title, year)

		if err == nil && score > 80 {
			return MediaFilesInfo{Info: mediaInfo, Path: path, VideoFiles: videoFiles}, nil
		}

		// if not found and there‘s cyrillic `e` it‘s likely it may be transliterated to `ё`
		if strings.Contains(title, "е") {
			Logf("Prompting AI for corrected ё usage\n")
			correctedTitle, err := promptAiForCorrectedYoLetterUsage(title, config.OpenAiApiKey)
			if err != nil {
				Log("AI Error:", err)
			}
			Logf("Response: %s\n", correctedTitle)
			if correctedTitle != title {
				mediaInfo, _, err := findMovieByTitle(tmdbAPI, correctedTitle, year)

				if err == nil {
					return MediaFilesInfo{Info: mediaInfo, Path: path, VideoFiles: videoFiles}, nil
				}
			}
		}

		kpApi := KinopoiskAPI{ApiKey: config.KinopoiskApiKey, TvShowsOnly: true, GenresMap: config.KinopoiskGenres}
		kpMediaInfo, score, err := findMovieByTitle(kpApi, title, year)

		if err == nil && score > 70 {
			return MediaFilesInfo{Info: kpMediaInfo, Path: path, VideoFiles: videoFiles}, nil

		} else if err == nil && score > 50 && FindCommonItems(
			// if score is pretty low but the result from 2 sources matches
			filterSlice([]string{
				mediaInfo.Title,
				mediaInfo.OriginalTitle,
				mediaInfo.AlternativeTitle}, func(item string) bool { return item != "" }),
			filterSlice([]string{
				kpMediaInfo.Title,
				kpMediaInfo.OriginalTitle,
				kpMediaInfo.AlternativeTitle,
			}, func(item string) bool { return item != "" }), false /*caseSensitive*/) > 0 {

			return MediaFilesInfo{Info: mediaInfo, Path: path, VideoFiles: videoFiles}, nil

		} else {
			// find individual movies instead
			return MediaFilesInfo{}, &FolderSeemsContainingMultipleMoviesError{videoFiles: videoFiles}
		}
	}

	mediaInfo, score, err := findMovieMediaInfo(path, title, year, config)
	if score < 80 {
		return MediaFilesInfo{}, fmt.Errorf("found match '%s / %s' score is too low: %d", mediaInfo.Title, mediaInfo.OriginalTitle, score)
	}
	return MediaFilesInfo{Info: mediaInfo, Path: path, VideoFiles: videoFiles}, err
}

// TODO: if no poster try getting kinopoisk files and create local NFO
func findMovieMediaInfo(path Path, title string, year string, config Config) (MediaInfo /*score*/, int, error) {
	lang := "en-US"
	if containsCyrillicCharacters(title) {
		lang = "ru-RU"
	}

	tmdbApi := TMDbAPI{ApiKey: config.TMDbApiKey, Language: lang, TVShowSearch: false, MovieGenres: config.TMDbMovieGenres, TvGenres: config.TMDbTvGenres}
	movie, score, err := findMovieByTitle(tmdbApi, title, year)

	// if not found and there‘s cyrillic `e` it‘s likely it may be transliterated to `ё`
	if lang == "ru-RU" && (err != nil || score <= 80) && strings.Contains(title, "е") {
		Logf("Prompting AI for corrected ё usage\n")
		correctedTitle, e := promptAiForCorrectedYoLetterUsage(title, config.OpenAiApiKey)
		if e != nil {
			Log("AI Error:", e)
		}
		Logf("Response: %s\n", correctedTitle)
		if correctedTitle != title {
			movie, score, err = findMovieByTitle(tmdbApi, correctedTitle, year)
		}
	}

	if err == nil && score > 80 {
		Log("Found TMDB:", movie.Id.id, movie.Title, movie.Year)
		// load info in ru-RU
		if lang == "en-US" {
			var m MediaInfo
			if movie.IsTvShow {
				m, err = tmdbApi.LoadSeriesMediaInfo(movie.Id.id)
			} else {
				m, err = tmdbApi.LoadMovieDetails(movie.Id.id)
			}
			if err == nil {
				movie = m
			} else {
				Log("error:", err)
			}
		}
		return movie, score, nil
	} else if err != nil {
		Log("TMDB err", err)
		err = nil
	}

	tmdbApi.Language = "ru-RU"
	if lang != "ru-RU" {
		translitTitle := TransliterateToCyrillic(title)
		if translitTitle != title {
			if m, s, err := findMovieByTitle(tmdbApi, translitTitle, year); err == nil && s > score {
				movie = m
				score = s
			} else if err != nil {
				Log("TMDB translit err", err)
				err = nil
			}
			if score > 80 {
				Log("Found TMDB:", movie.Id.id, movie.Title, movie.Year)
				return movie, score, nil
			}
		}
	}

	// try searching IMDB
	imdbApi := IMDbAPI{}
	if m, s, err := findMovieByTitle(imdbApi, title, year); err == nil && s > score {
		movie = m
		score = s

		if mediaInfo, err := loadIMDbMediaInfo(m.Id.id, tmdbApi); err == nil {
			movie = mediaInfo
		}

	} else if err != nil {
		Log("IMDB err", err)
		err = nil
	}
	if score > 80 {
		Log("Found IMDB:", movie.Id.id, movie.Title, movie.Year)
		return movie, score, nil
	}

	// search Kinopoisk
	kpApi := KinopoiskAPI{ApiKey: config.KinopoiskApiKey, GenresMap: config.KinopoiskGenres}
	if m, s, err := findMovieByTitle(kpApi, title, year); err == nil && s > score {
		movie = m
		score = s
	} else if err != nil {
		Log("Kinopoisk err", err)
		err = nil
	}
	if score > 80 {
		Log("Found Kinopoisk:", movie.Id.id, movie.Title, movie.Year)
		return movie, score, nil
	}

	// prompt ChatGPT to guess a corrected name from the file name
	Logf("Prompting AI\n")
	title, year, err = promptAiForMovieNameAndYear(path.lastPathComponent(), config.OpenAiApiKey)
	if err != nil {
		return MediaInfo{}, 0, err
	}
	Logf("Response: %s (%s)\n", title, year)

	// query TMDB with title corrected by ChatGPT
	if m, s, err := findMovieByTitle(tmdbApi, title, year); err == nil && s > score {
		movie = m
		score = s
	} else if err != nil {
		Log("TMDB (corrected) err", err)
		err = nil
	}
	if score > 80 {
		Log("Found TMDB:", movie.Id.id, movie.Title, movie.Year)
		return movie, score, nil
	}

	// try searching TV series instead
	tmdbApi.TVShowSearch = true
	if m, s, err := findMovieByTitle(tmdbApi, title, year); err == nil && s > score {
		movie = m
		score = s
	} else if err != nil {
		Log("TMDB (series) err", err)
		err = nil
	}

	Log("Result:", movie.Id.id, movie.Title, movie.Year)
	if score == 0 {
		// TODO: Don‘t crash, but copy item with the present info, log failure
		panic("movie not found")
	}
	return movie, score, nil
}

// create output folder and video file links for a media item
func syncMediaItemFiles(mediaInfo MediaFilesInfo, config Config) (Path, error) {
	if mediaInfo.Info.IsTvShow {
		outputDir := findSuitableDirectoryForSymlink(mediaInfo.Path, config.Output.Series)
		if outputDir == "" {
			return Path(""), fmt.Errorf("no same-volume directory suitable for %s found in config.Output.Series", mediaInfo.Path)
		}
		return syncTvShow(mediaInfo, outputDir, config)
	} else {
		outputDir := findSuitableDirectoryForSymlink(mediaInfo.Path, config.Output.Movies)
		if outputDir == "" {
			return Path(""), fmt.Errorf("no same-volume directory suitable for %s found in config.Output.Movies", mediaInfo.Path)
		}
		return syncMovie(mediaInfo, outputDir)
	}
}

// create link for a movie file and write NFO in the Movies output dir
func syncMovie(mediaInfo MediaFilesInfo, output Path) (Path, error) {
	fileName := movieFileNameWithoutExtension(mediaInfo.VideoFiles)
	outputDir := output
	// make folder for multipart movie
	if !mediaInfo.Path.isVideoFile() {
		outputDir = output.appendingPathComponent(mediaInfo.Path.lastPathComponent())
		err := os.MkdirAll(string(outputDir), 0755)
		if err != nil {
			return "", err
		}
	}

	// download poster/background for Kinopoisk media info
	if mediaInfo.Info.PosterUrl != "" && !strings.Contains(mediaInfo.Info.PosterUrl, "image.tmdb.org") /* Kodi will download tmdb images itself */ {
		posterName := fileName + "-poster.jpg"
		posterPath := outputDir.appendingPathComponent(posterName)
		err := downloadImage(mediaInfo.Info.PosterUrl, posterPath)
		if err != nil {
			Log("Could not download poster", err)
		}
	}
	if mediaInfo.Info.BackdropUrl != "" && !strings.Contains(mediaInfo.Info.BackdropUrl, "image.tmdb.org") /* Kodi will download tmdb images itself */ {
		fanartName := fileName + "-fanart.jpg"
		fanartPath := outputDir.appendingPathComponent(fanartName)
		err := downloadImage(mediaInfo.Info.BackdropUrl, fanartPath)
		if err != nil {
			Log("Could not download fanart", err)
		}
	}

	err := writeMovieNfo(mediaInfo, outputDir)
	if err != nil {
		return "", err
	}

	for _, videoFile := range mediaInfo.VideoFiles {
		err = linkVideoFileAndRelatedItems(videoFile, outputDir, fileName, len(mediaInfo.VideoFiles) > 1)
		if err != nil {
			return "", err
		}
	}
	return output.appendingPathComponent(mediaInfo.Path.lastPathComponent()), nil
}

// create links for TV Show episodes and write NFO in the Series output dir
func syncTvShow(mediaInfo MediaFilesInfo, output Path, config Config) (Path, error) {
	if len(mediaInfo.VideoFiles) == 0 {
		mediaInfo.VideoFiles = getVideoFiles(mediaInfo.Path)
	}
	outputDir := output.appendingPathComponent(mediaInfo.Path.lastPathComponent())
	nfoPath := outputDir.appendingPathComponent("tvshow.nfo")
	if (mediaInfo.Info.Id == MediaId{}) {
		dbId, err := readTVShowNfo(nfoPath)
		if err != nil {
			return "", err
		}
		mediaInfo.Info.Id = dbId
	}

	// create TV Show directory
	if !outputDir.exists() {
		err := os.MkdirAll(string(outputDir), 0755)
		if err != nil {
			return "", err
		}
	}
	// create TV Show NFO file
	if !nfoPath.exists() {
		err := writeTVShowNfo(mediaInfo.Info, nfoPath)
		if err != nil {
			return "", err
		}
	}

	// download poster/background for Kinopoisk media info
	if mediaInfo.Info.PosterUrl != "" {
		posterPath := outputDir.appendingPathComponent("poster.jpg")
		if !posterPath.exists() {
			err := downloadImage(mediaInfo.Info.PosterUrl, posterPath)
			if err != nil {
				Log("Could not download poster", err)
			}
		}
	}
	if mediaInfo.Info.BackdropUrl != "" {
		fanartPath := outputDir.appendingPathComponent("fanart.jpg")
		if !fanartPath.exists() {
			err := downloadImage(mediaInfo.Info.BackdropUrl, fanartPath)
			if err != nil {
				Log("Could not download fanart", err)
			}
		}
	}

	// list already existing episode files
	existingFiles := getVideoFiles(outputDir)
	// Log("existing videos from", outputDir, ":", existingFiles)

	var episodes []TMDbEpisode
	var episodeMap map[int]map[int]TMDbEpisode = nil
	var err error
	// modified := false
	// create links for episodes not existing in target dir
	for _, path := range mediaInfo.VideoFiles {
		if existingIdx := indexOfEpisode(existingFiles, path.lastPathComponent()); existingIdx != -1 {
			// episode already exists; skip
			continue
		}

		s, e := getSeasonEpisodeFromPath(path, mediaInfo.VideoFiles)

		if mediaInfo.Info.Id == (MediaId{}) {
			episodeMap = make(map[int]map[int]TMDbEpisode)
		} else {
			episodeMap, episodes, err = getEpisodesMap(episodeMap, episodes, mediaInfo.Info.Id, config)
		}
		if err != nil {
			Log(err)
			episodeMap = make(map[int]map[int]TMDbEpisode)
		}
		if e == 0 {
			name, _ := cleanupMovieFileName(path.lastPathComponent(), true /*multipleVideoFiles*/)
			bestRank := -1

			for _, episode := range episodes {
				rank := computeSimilarityScore(episode.Name, name, false)
				if rank > bestRank {
					e = episode.EpisodeNumber
					s = episode.SeasonNumber
					bestRank = rank
				}
			}
		}
		_ /*episode*/, ok := episodeMap[s][e]
		if !ok && len(episodeMap) == 0 {
			_ /*episode*/ = TMDbEpisode{SeasonNumber: s, EpisodeNumber: e, ID: -1, Name: ""}
		} else if !ok {
			// TODO: if file found for an episode but no episode in the series - should throw an error (and probably reconsider the series choice)
			Log("⚠️", s, e, path, "episode not found!")
		}

		targetFileName := path.removingPathExtension().lastPathComponent()
		seasonEpisode := fmt.Sprintf("S%02dE%02d", s, e)
		if !strings.Contains(strings.ToUpper(targetFileName), seasonEpisode) {
			// prepend S01E02 if not already present in the file name
			targetFileName = seasonEpisode + " " + targetFileName
		}

		// Log(episode.SeasonNumber, episode.EpisodeNumber, episode.ID, episode.Name, path, "→", targetFileName)
		linkVideoFileAndRelatedItems(path, outputDir, targetFileName, false)
		// modified = true
	}

	return outputDir, err
}

func indexOfEpisode(existingFiles []Path, fileName string) int {
	fileNameLowercase := strings.ToLower(fileName)
	for idx, path := range existingFiles {
		name := strings.ToLower(path.lastPathComponent())
		// Log("inspecting", name, "against", fileNameLowercase)
		if name == fileNameLowercase {
			return idx
		}
		// drop "S01E01 " prefix and compare again
		if name[7:] == fileNameLowercase {
			return idx
		}
	}
	// Log(fileNameLowercase, "not found")
	return -1
}

func getEpisodesMap(existing map[int]map[int]TMDbEpisode, existingEpisodes []TMDbEpisode, id MediaId, config Config) (map[int]map[int]TMDbEpisode, []TMDbEpisode, error) {
	if existing != nil {
		return existing, existingEpisodes, nil
	}

	api := TMDbAPI{ApiKey: config.TMDbApiKey, MovieGenres: config.TMDbMovieGenres, TvGenres: config.TMDbTvGenres}
	episodes, err := api.getSeriesEpisodes(id)
	if err != nil {
		return nil, nil, err
	}

	episodeMap := make(map[int]map[int]TMDbEpisode)

	// Iterate over each episode and populate the map
	for _, episode := range episodes {
		// Check if the season exists in the map, if not, create a new map for the season
		if _, ok := episodeMap[episode.SeasonNumber]; !ok {
			episodeMap[episode.SeasonNumber] = make(map[int]TMDbEpisode)
		}

		// Add the episode to the map
		episodeMap[episode.SeasonNumber][episode.EpisodeNumber] = episode
		// Log(episode.SeasonNumber, episode.EpisodeNumber, episode.Name)
	}
	Logf("loaded %d episodes\n", len(episodes))
	return episodeMap, episodes, nil
}
