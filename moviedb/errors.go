package moviedb

import (
	"net/http"
)

type HTTPError struct {
	Response *http.Response
}

func (err *HTTPError) Error() string {
	return "moviedb: non-2xx status code"
}
