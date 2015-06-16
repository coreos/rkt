package discovery

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"
)

const (
	defaultDialTimeout = 5 * time.Second
)

var (
	// Client is the default http.Client used for discovery requests.
	Client *http.Client

	// httpGet is the internal object used by discovery to retrieve URLs; it is
	// defined here so it can be overridden for testing
	httpGet httpGetter
)

// httpGetter is an interface used to wrap http.Client for real requests and
// allow easy mocking in local tests.
type httpGetter interface {
	Get(url string) (resp *http.Response, err error)
}

func init() {
	t := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		Dial: func(n, a string) (net.Conn, error) {
			return net.DialTimeout(n, a, defaultDialTimeout)
		},
	}
	Client = &http.Client{
		Transport: t,
	}
	httpGet = Client
}

func httpsOrHTTP(name string, insecure bool) (urlStr string, body io.ReadCloser, err error) {
	fetch := func(scheme string) (urlStr string, res *http.Response, err error) {
		u, err := url.Parse(scheme + "://" + name)
		if err != nil {
			return "", nil, err
		}
		u.RawQuery = "ac-discovery=1"
		urlStr = u.String()
		res, err = httpGet.Get(urlStr)
		return
	}
	closeBody := func(res *http.Response) {
		if res != nil {
			res.Body.Close()
		}
	}
	urlStr, res, err := fetch("https")
	if err != nil || res.StatusCode != http.StatusOK {
		if insecure {
			closeBody(res)
			urlStr, res, err = fetch("http")
		}
	}

	if res != nil && res.StatusCode != http.StatusOK {
		err = fmt.Errorf("expected a 200 OK got %d", res.StatusCode)
	}

	if err != nil {
		closeBody(res)
		return "", nil, err
	}
	return urlStr, res.Body, nil
}
