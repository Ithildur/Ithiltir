package notify

import (
	"net/http"
	"time"
)

const defaultHTTPTimeout = 10 * time.Second

var defaultHTTPClient = &http.Client{Timeout: defaultHTTPTimeout}
