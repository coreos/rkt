package store

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/url"
	"strings"
)

// copy the default of git which is a two byte prefix. We will likely want to
// add re-sharding later.
func blockTransform(s string) []string {
	// TODO(philips): use spec/types.Hash after export typ field
	parts := strings.SplitN(s, "-", 2)
	pathSlice := make([]string, 2)
	pathSlice[0] = parts[0]
	pathSlice[1] = parts[1][0:2]
	return pathSlice
}

func sha256sum(s string) string {
	h := sha256.New()
	io.WriteString(h, s)
	return fmt.Sprintf("sha256-%x", h.Sum(nil))
}

func parseAlways(s string) *url.URL {
	u, _ := url.Parse(s)
	return u
}
