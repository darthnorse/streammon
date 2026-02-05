package httputil

import (
	"io"
	"net/http"
	"time"
)

const DefaultTimeout = 10 * time.Second
const ExtendedTimeout = 15 * time.Second

func NewClient() *http.Client {
	return &http.Client{Timeout: DefaultTimeout}
}

func NewClientWithTimeout(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout}
}

// DrainBody ensures the connection can be reused for keep-alive.
func DrainBody(resp *http.Response) {
	if resp != nil && resp.Body != nil {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}
