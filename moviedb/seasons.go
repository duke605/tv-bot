package moviedb

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type EpisodeDetails struct {
	AirDate        string  `json:"air_date"`
	EpisodeNumber  int     `json:"episode_number"`
	EpisodeType    string  `json:"episode_type"`
	ID             uint64  `json:"id"`
	Name           string  `json:"name"`
	Overview       string  `json:"overview"`
	ProductionCode string  `json:"production_code"`
	Runtime        int     `json:"runtime"`
	SeasonNumber   int     `json:"season_number"`
	ShowID         int     `json:"show_id"`
	StillPath      string  `json:"still_path"`
	VoteAverage    float64 `json:"vote_average"`
	VoteCount      int     `json:"vote_count"`
	Crew           []struct {
		Job                string  `json:"job"`
		Department         string  `json:"department"`
		CreditID           string  `json:"credit_id"`
		Adult              bool    `json:"adult"`
		Gender             int     `json:"gender"`
		ID                 uint64  `json:"id"`
		KnownForDepartment string  `json:"known_for_department"`
		Name               string  `json:"name"`
		OriginalName       string  `json:"original_name"`
		Popularity         float64 `json:"popularity"`
		ProfilePath        string  `json:"profile_path"`
	} `json:"crew"`
	GuestStars []struct {
		Character          string  `json:"character"`
		CreditID           string  `json:"credit_id"`
		Order              int     `json:"order"`
		Adult              bool    `json:"adult"`
		Gender             int     `json:"gender"`
		ID                 uint64  `json:"id"`
		KnownForDepartment string  `json:"known_for_department"`
		Name               string  `json:"name"`
		OriginalName       string  `json:"original_name"`
		Popularity         float64 `json:"popularity"`
		ProfilePath        string  `json:"profile_path"`
	} `json:"guest_stars"`
}

type SeasonDetails struct {
	ID           uint64           `json:"id"`
	AirDate      string           `json:"air_date"`
	Episodes     []EpisodeDetails `json:"episodes"`
	Name         string           `json:"name"`
	Overview     string           `json:"overview"`
	PosterPath   string           `json:"poster_path"`
	SeasonNumber int              `json:"season_number"`
	VoteAverage  float64          `json:"vote_average"`
}

type TVSeasonsService interface {
	GetTVSeasonDetails(id uint64, season int, dst *SeasonDetails, opts ...RequestOption) (*http.Response, error)
}

type tvSeasonsService struct {
	service
}

func NewTVSeasonsService(c Client) TVSeasonsService {
	return &tvSeasonsService{service{"tv", c}}
}

func (tss *tvSeasonsService) GetTVSeasonDetails(id uint64, season int, dst *SeasonDetails, opts ...RequestOption) (*http.Response, error) {
	path := fmt.Sprintf("%d/season/%d", id, season)
	resp, err := tss.do(http.MethodGet, path, opts...)
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
