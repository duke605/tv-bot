package moviedb

import "net/http"

type SearchResults[T any] struct {
	Page         int `json:"page"`
	TotalPages   int `json:"total_pages"`
	TotalResults int `json:"total_results"`
	Results      []T `json:"results"`
}

type SearchService interface {
	SearchTVSeriesDetails(name string, dst *SearchResults[*SearchSeriesDetails], opts ...RequestOption) (*http.Response, error)
}

type searchService struct {
	service
}

func NewSearchService(c Client) SearchService {
	return &searchService{service{path: "search", client: c}}
}
