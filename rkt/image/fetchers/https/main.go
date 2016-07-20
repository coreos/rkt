package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/coreos/pkg/progressutil"
	"github.com/coreos/rkt/rkt/image/fetchers"
)

// statusAcceptedError is an error returned when resumableSession
// receives a 202 HTTP status. It is mostly used for deferring
// signature downloads.
type statusAcceptedError struct{}

func (statusAcceptedError) Error() string { return "download deferred" }

func errAndExit(format string, a ...interface{}) {
	stderr(format, a...)
	os.Exit(1)
}

func stderr(format string, a ...interface{}) {
	out := fmt.Sprintf(format, a...)
	fmt.Fprintln(os.Stderr, strings.TrimSuffix(out, "\n"))
}

func main() {
	config, err := fetchers.ConfigFromStdin()
	if err != nil {
		errAndExit("error reading config: %v", err)
	}

	if config.Version != 1 {
		errAndExit("unsupported plugin schema version")
	}

	err = fetch(config)
	if err != nil {
		errAndExit("error fetching http(s) image: %v", err)
	}
}

func fetch(config *fetchers.Config) error {
	if config.InsecureOpts.SkipTLSCheck {
		stderr("warning: TLS verification has been disabled")
	}
	if config.InsecureOpts.SkipImageCheck {
		stderr("warning: image signature verification has been disabled")
	}
	if config.InsecureOpts.AllowHTTP {
		stderr("warning: image allowed to be fetched without encryption")
	}
	if !config.InsecureOpts.AllowHTTP && config.Scheme == "http" {
		return fmt.Errorf("error: http URLs not allowed")
	}
	retry := false
	if !config.InsecureOpts.SkipImageCheck {
		var err error
		retry, err = fetchSignature(config)
		if err != nil {
			return err
		}
	}
	res, err := fetchImage(config)
	if err != nil {
		return err
	}
	if !retry {
		return fetchers.ResultToStdout(res)
	}
	retry, err = fetchSignature(config)
	if err != nil {
		return err
	}
	if retry {
		return fmt.Errorf("error downloading the signature file: server asked to defer the download again")
	}
	return fetchers.ResultToStdout(res)
}

func fetchImage(config *fetchers.Config) (*fetchers.Result, error) {
	aciFile, err := os.Create(config.OutputACIPath)
	if err != nil {
		return nil, err
	}
	defer aciFile.Close()
	return fetchURL(config, "ACI", config.Scheme+"://"+config.Name, aciFile)
}

func fetchSignature(config *fetchers.Config) (bool, error) {
	ascFile, err := os.Create(config.OutputASCPath)
	if err != nil {
		return false, err
	}
	defer ascFile.Close()
	_, err = fetchURL(config, "Signature", config.Scheme+"://"+config.Name+".asc", ascFile)
	if err == nil {
		return false, nil
	}
	if _, ok := err.(*statusAcceptedError); ok {
		stderr("server requested deferring the signature download")
		return true, nil
	}
	return false, fmt.Errorf("error downloading the signature file: %v", err)
}

func fetchURL(config *fetchers.Config, name, u string, dest io.Writer) (*fetchers.Result, error) {
	stderr("fetching from URL: %s", u)
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: config.InsecureOpts.SkipTLSCheck},
	}
	check := func(req *http.Request, via []*http.Request) error {
		if req.URL.Scheme == "http" && !config.InsecureOpts.AllowHTTP {
			return fmt.Errorf("error: received redirect to an http URL")
		}
		return nil
	}
	client := &http.Client{
		Transport:     tr,
		CheckRedirect: check,
	}
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	if headers, ok := config.Headers[req.URL.Host]; ok {
		req.Header = http.Header(headers)
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	pluginResult := &fetchers.Result{}

	pluginResult.ETag = res.Header.Get("ETag")
	pluginResult.MaxAge = getMaxAge(res.Header.Get("Cache-Control"))
	switch res.StatusCode {
	case http.StatusOK, http.StatusPartialContent:
		pluginResult.UseCached = false
	case http.StatusNotModified:
		pluginResult.UseCached = true
		return pluginResult, nil
	case http.StatusAccepted:
		// If the server returns Status Accepted (HTTP 202), we should retry
		// downloading the signature later.
		return nil, &statusAcceptedError{}
	//case http.StatusRequestedRangeNotSatisfiable:
	//	return s.handleRangeNotSatisfiable()
	default:
		return nil, fmt.Errorf("bad HTTP status code: %d", res.StatusCode)
	}
	var size int64 = 0
	sizeStr := res.Header.Get("content-length")
	if sizeStr != "" {
		size, _ = strconv.ParseInt(sizeStr, 10, 64)
	}
	cpp := progressutil.NewCopyProgressPrinter()
	err = cpp.AddCopy(res.Body, "Downloading "+name, size, dest)
	if err != nil {
		return nil, err
	}
	err = cpp.PrintAndWait(os.Stderr, time.Second, nil)
	if err != nil {
		return nil, err
	}
	return pluginResult, nil
}

func getMaxAge(headerValue string) int {
	if headerValue == "" {
		return 0
	}

	maxAge := 0
	parts := strings.Split(headerValue, ",")

maxAgeLoop:
	for i := 0; i < len(parts); i++ {
		parts[i] = strings.TrimSpace(parts[i])
		attr, val := parts[i], ""
		if j := strings.Index(attr, "="); j >= 0 {
			attr, val = attr[:j], attr[j+1:]
		}
		lowerAttr := strings.ToLower(attr)

		switch lowerAttr {
		case "no-store", "no-cache":
			maxAge = 0
			break maxAgeLoop
		case "max-age":
			secs, err := strconv.Atoi(val)
			if err != nil || secs != 0 && val[0] == '0' {
				// TODO(krnowak): Set maxAge to zero.
				break
			}
			if secs <= 0 {
				maxAge = 0
			} else {
				maxAge = secs
			}
		}
	}
	return maxAge
}
