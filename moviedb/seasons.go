package moviedb

import (
	"encoding/json"
	"fmt"
	"net/http"
)

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
