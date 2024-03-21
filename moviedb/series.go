package moviedb

import (
	"encoding/json"
	"net/http"
	"slices"
	"strconv"
)

type SearchSeriesDetails struct {
	ID               uint64   `json:"id"`
	Adult            bool     `json:"adult"`
	BackdropPath     string   `json:"backdrop_path"`
	GenreIDs         []uint64 `json:"genre_ids"`
	OriginCountry    []string `json:"origin_country"`
	OriginalLanguage string   `json:"original_language"`
	OriginalName     string   `json:"original_name"`
	Overview         string   `json:"overview"`
	Popularity       float64  `json:"popularity"`
	PosterPath       string   `json:"poster_path"`
	FirstAirDate     string   `json:"first_air_date"`
	Name             string   `json:"name"`
	VoteAverage      float64  `json:"vote_average"`
	VoteCount        int      `json:"vote_count"`
}

type PartialEpisodeDetails struct {
	ID             uint64  `json:"id"`
	Name           string  `json:"name"`
	Overview       string  `json:"overview"`
	VoteAverage    float64 `json:"vote_average"`
	VoteCount      int     `json:"vote_count"`
	AirDate        string  `json:"air_date"`
	EpisodeNumber  int     `json:"episode_number"`
	EpisodeType    string  `json:"episode_type"`
	ProductionCode string  `json:"production_code"`
	Runtime        int     `json:"runtime"`
	SeasonNumber   int     `json:"season_number"`
	ShowID         int     `json:"show_id"`
	StillPath      string  `json:"still_path"`
}

type SeriesDetails struct {
	ID           uint64 `json:"id"`
	Adult        bool   `json:"adult"`
	BackdropPath string `json:"backdrop_path"`
	CreatedBy    []struct {
		ID          uint64 `json:"id"`
		CreditID    string `json:"credit_id"`
		Name        string `json:"name"`
		Gender      int    `json:"gender"`
		ProfilePath string `json:"profile_path"`
	} `json:"created_by"`
	EpisodeRunTime []int  `json:"episode_run_time"`
	FirstAirDate   string `json:"first_air_date"`
	Genres         []struct {
		ID   uint64 `json:"id"`
		Name string `json:"name"`
	} `json:"genres"`
	Homepage         string                 `json:"homepage"`
	InProduction     bool                   `json:"in_production"`
	Languages        []string               `json:"languages"`
	LastAirDate      string                 `json:"last_air_date"`
	LastEpisodeToAir *PartialEpisodeDetails `json:"last_episode_to_air"`
	NextEpisodeToAir *PartialEpisodeDetails `json:"next_episode_to_air"`
	Name             string                 `json:"name"`
	Networks         []struct {
		ID            uint64 `json:"id"`
		LogoPath      string `json:"logo_path"`
		Name          string `json:"name"`
		OriginCountry string `json:"origin_country"`
	} `json:"networks"`
	NumberOfEpisodes    int      `json:"number_of_episodes"`
	NumberOfSeasons     int      `json:"number_of_seasons"`
	OriginCountry       []string `json:"origin_country"`
	OriginalLanguage    string   `json:"original_language"`
	OriginalName        string   `json:"original_name"`
	Overview            string   `json:"overview"`
	Popularity          float64  `json:"popularity"`
	PosterPath          string   `json:"poster_path"`
	ProductionCompanies []struct {
		ID            uint64 `json:"id"`
		LogoPath      string `json:"logo_path"`
		Name          string `json:"name"`
		OriginCountry string `json:"origin_country"`
	} `json:"production_companies"`
	ProductionCountries []struct {
		Iso31661 string `json:"iso_3166_1"`
		Name     string `json:"name"`
	} `json:"production_countries"`
	Seasons []struct {
		ID           uint64  `json:"id"`
		AirDate      string  `json:"air_date"`
		EpisodeCount int     `json:"episode_count"`
		Name         string  `json:"name"`
		Overview     string  `json:"overview"`
		PosterPath   string  `json:"poster_path"`
		SeasonNumber int     `json:"season_number"`
		VoteAverage  float64 `json:"vote_average"`
	} `json:"seasons"`
	SpokenLanguages []struct {
		EnglishName string `json:"english_name"`
		Iso6391     string `json:"iso_639_1"`
		Name        string `json:"name"`
	} `json:"spoken_languages"`
	Status      string  `json:"status"`
	Tagline     string  `json:"tagline"`
	Type        string  `json:"type"`
	VoteAverage float64 `json:"vote_average"`
	VoteCount   int     `json:"vote_count"`
}

type TVSeriesService interface {
	GetTVSeriesDetails(id uint64, dst *SeriesDetails, opts ...RequestOption) (*http.Response, error)
}

type tvSeriesService struct {
	service
}

func NewTVSeries(c Client) TVSeriesService {
	return &tvSeriesService{service{path: "tv", client: c}}
}

func (tvs *tvSeriesService) GetTVSeriesDetails(id uint64, dst *SeriesDetails, opts ...RequestOption) (*http.Response, error) {
	resp, err := tvs.do(http.MethodGet, strconv.FormatUint(id, 10), opts...)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(dst)
	if err != nil {
		return nil, err
	}

	return resp, err
}

func (ss *searchService) SearchTVSeriesDetails(name string, dst *SearchResults[*SearchSeriesDetails], opts ...RequestOption) (*http.Response, error) {
	opts = append(slices.Clip(opts), RequestOptionWithQueryParams(
		"query", name,
		"language", "en-US",
		"include_adult", "false",
	))

	resp, err := ss.do(http.MethodGet, "tv", opts...)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(dst)
	if err != nil {
		return nil, err
	}

	return resp, err
}
