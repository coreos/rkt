package aci

import (
	"bytes"
	"encoding/hex"
	"io"
	"log"
	"net/http"
	"os/exec"
)

type FileType string

const (
	TypeGzip    = FileType("gz")
	TypeBzip2   = FileType("bz2")
	TypeXz      = FileType("xz")
	TypeTar     = FileType("tar")
	TypeText    = FileType("text")
	TypeUnknown = FileType("unknown")

	readLen = 512 // max bytes to sniff

	hexHdrGzip  = "1f8b"
	hexHdrBzip2 = "425a68"
	hexHdrXz    = "fd377a585a00"
	hexSigTar   = "7573746172"

	tarOffset = 257

	textMime = "text/plain; charset=utf-8"
)

var (
	hdrGzip  []byte
	hdrBzip2 []byte
	hdrXz    []byte
	sigTar   []byte
	tarEnd   int
)

func mustDecodeHex(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}

func init() {
	hdrGzip = mustDecodeHex(hexHdrGzip)
	hdrBzip2 = mustDecodeHex(hexHdrBzip2)
	hdrXz = mustDecodeHex(hexHdrXz)
	sigTar = mustDecodeHex(hexSigTar)
	tarEnd = tarOffset + len(sigTar)
}

// DetectFileType attempts to detect the type of file that the given reader
// represents by comparing it against known file signatures (magic numbers)
func DetectFileType(r io.Reader) (FileType, error) {
	var b bytes.Buffer
	n, err := io.CopyN(&b, r, readLen)
	if err != nil && err != io.EOF {
		return TypeUnknown, err
	}
	bs := b.Bytes()
	switch {
	case bytes.HasPrefix(bs, hdrGzip):
		return TypeGzip, nil
	case bytes.HasPrefix(bs, hdrBzip2):
		return TypeBzip2, nil
	case bytes.HasPrefix(bs, hdrXz):
		return TypeXz, nil
	case n > int64(tarEnd) && bytes.Equal(bs[tarOffset:tarEnd], sigTar):
		return TypeTar, nil
	case http.DetectContentType(bs) == textMime:
		return TypeText, nil
	default:
		return TypeUnknown, nil
	}
}

// XzReader shells out to a command line xz executable (if
// available) to decompress the given io.Reader using the xz
// compression format
func XzReader(r io.Reader) io.ReadCloser {
	rpipe, wpipe := io.Pipe()
	ex, err := exec.LookPath("xz")
	if err != nil {
		log.Fatalf("couldn't find xz executable: %v", err)
	}
	cmd := exec.Command(ex, "--decompress", "--stdout")
	cmd.Stdin = r
	cmd.Stdout = wpipe

	go func() {
		err := cmd.Run()
		wpipe.CloseWithError(err)
	}()

	return rpipe
}
