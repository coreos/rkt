package cas

import (
	"compress/bzip2"
	"compress/gzip"
	"errors"
	"io"
	"net/url"
	"strings"

	"github.com/appc/spec/aci"
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

func parseAlways(s string) *url.URL {
	u, _ := url.Parse(s)
	return u
}

func decompress(rs io.Reader, typ aci.FileType) (io.Reader, error) {
	var (
		dr  io.Reader
		err error
	)
	switch typ {
	case aci.TypeGzip:
		dr, err = gzip.NewReader(rs)
		if err != nil {
			return nil, err
		}
	case aci.TypeBzip2:
		dr = bzip2.NewReader(rs)
	case aci.TypeXz:
		dr = aci.XzReader(rs)
	case aci.TypeTar:
		dr = rs
	case aci.TypeUnknown:
		return nil, errors.New("error: unknown image filetype")
	default:
		return nil, errors.New("no type returned from DetectFileType?")
	}
	return dr, nil
}
