package moviedb

import (
	"encoding/json"
	"net/http"
)

type Configuration struct {
	ChangeKeys []string `json:"change_keys"`
	Images     struct {
		BaseURL       string   `json:"base_url"`
		SecureBaseURL string   `json:"secure_base_url"`
		BackdropSizes []string `json:"backdrop_sizes"`
		LogoSizes     []string `json:"logo_sizes"`
		PosterSizes   []string `json:"poster_sizes"`
		ProfileSizes  []string `json:"profile_sizes"`
		StillSizes    []string `json:"still_sizes"`
	} `json:"images"`
}

type ConfigurationService interface {
	GetConfigurationDetails(dst *Configuration, opts ...RequestOption) (*http.Response, error)
}

type configurationService struct {
	service
}

func NewConfigurationService(c Client) ConfigurationService {
	return &configurationService{service{path: "configuration", client: c}}
}

func (cs *configurationService) GetConfigurationDetails(dst *Configuration, opts ...RequestOption) (*http.Response, error) {
	resp, err := cs.do(http.MethodGet, "", opts...)
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
