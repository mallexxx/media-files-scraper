package main

type MediaInfo struct {
	Id               MediaId
	Title            string
	OriginalTitle    string
	AlternativeTitle string
	Year             string
	Description      string
	IsTvShow         bool
	Url              string
	PosterUrl        string
	BackdropUrl      string
	Genres           []string
}

type IdType int

const (
	IMDB IdType = iota
	TMDB
	KPID
	KPHD
)

type MediaId struct {
	id     string
	idType IdType
}

func (id MediaId) getType() string {
	if id.idType == IMDB {
		return "imdb"
	} else if id.idType == TMDB {
		return "tmdb"
	} else {
		return "kinopoisk"
	}
}
