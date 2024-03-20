package moviedb

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

type service struct {
	path   string
	client Client
}

func (srv *service) do(method, path string, opts ...RequestOption) (*http.Response, error) {
	path, err := url.JoinPath(srv.path, path)
	if err != nil {
		return nil, err
	}

	return srv.client.Do(method, path, opts...)
}

type Client interface {
	Do(method, path string, opts ...RequestOption) (*http.Response, error)

	TVSeriesService
	ConfigurationService
	TVSeasonsService
	TVEpisodesService
	SearchService
}

type client struct {
	httpClient        *http.Client
	baseURL           *url.URL
	globalRequestOpts []RequestOption

	TVSeriesService
	ConfigurationService
	TVSeasonsService
	TVEpisodesService
	SearchService
}

type ClientOption = func(*client)
type RequestOption = func(*http.Request) *http.Request

func ClientOptionWithHTTPClient(c *http.Client) ClientOption {
	return func(cl *client) {
		cl.httpClient = c
	}
}

func ClientOptionGlobalRequestOption(opt RequestOption) ClientOption {
	return func(cl *client) {
		cl.globalRequestOpts = append(cl.globalRequestOpts, opt)
	}
}

func RequestOptionWithContext(ctx context.Context) RequestOption {
	return func(r *http.Request) *http.Request {
		return r.WithContext(ctx)
	}
}

func RequestOptionWithBody(body io.Reader) RequestOption {
	return func(r *http.Request) *http.Request {
		if rc, ok := body.(io.ReadCloser); ok {
			r.Body = rc
		} else {
			r.Body = io.NopCloser(body)
		}

		return r
	}
}

func RequestOptionWithQueryParams(kvpairs ...string) RequestOption {
	if len(kvpairs)%2 != 0 {
		panic(errors.New("moviedb: kvpairs must have a length that is a multiple of 2"))
	}

	return func(r *http.Request) *http.Request {
		q := r.URL.Query()
		for i := 0; i < len(kvpairs); i += 2 {
			key := kvpairs[i]
			value := kvpairs[i+1]

			q.Set(key, value)
		}

		r.URL.RawQuery = q.Encode()
		return r
	}
}

func NewClient(baseURL string, opts ...ClientOption) (Client, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}

	c := &client{baseURL: u, httpClient: http.DefaultClient}
	for _, opt := range opts {
		opt(c)
	}

	c.TVSeriesService = NewTVSeries(c)
	c.ConfigurationService = NewConfigurationService(c)
	c.TVSeasonsService = NewTVSeasonsService(c)
	c.SearchService = NewSearchService(c)
	c.TVEpisodesService = NewTVEpisodesService(c)

	return c, nil
}

func (client *client) Do(method, path string, opts ...RequestOption) (*http.Response, error) {
	u := client.baseURL.JoinPath(path)
	req, err := http.NewRequest(method, u.String(), nil)
	if err != nil {
		return nil, err
	}

	for _, opt := range client.globalRequestOpts {
		req = opt(req)
	}
	for _, opt := range opts {
		req = opt(req)
	}

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		fmt.Println(resp.StatusCode, u)
		return nil, &HTTPError{resp}
	}

	return resp, err
}
