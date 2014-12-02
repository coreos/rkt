package cas

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/coreos/rocket/Godeps/_workspace/src/github.com/mitchellh/ioprogress"
	"github.com/coreos/rocket/app-container/schema/types"
)

func NewRemote(name string, mirrors []string) *Remote {
	// TODO: don't assume the name is a mirror?
	r := &Remote{}
	r.Name = name
	r.Mirrors = []string{name}
	return r
}

type Remote struct {
	Name    string
	Mirrors []string
	ETag    string
	Blob    string
}

func (r Remote) Marshal() []byte {
	m, _ := json.Marshal(r)
	return m
}

func (r *Remote) Unmarshal(data []byte) {
	if err := json.Unmarshal(data, r); err != nil {
		panic(err)
	}
}

func (r Remote) Hash() string {
	return types.NewHashSHA256([]byte(r.Name)).String()
}

func (r Remote) Type() int64 {
	return remoteType
}

// TODO: add locking
func (r Remote) Download(ds Store) (*Remote, error) {
	res, err := http.Get(r.Name)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	prefix := "Downloading aci"
	fmtBytesSize := 18
	barSize := int64(80 - len(prefix) - fmtBytesSize)
	bar := ioprogress.DrawTextFormatBar(barSize)
	fmtfunc := func(progress, total int64) string {
		return fmt.Sprintf(
			"%s: %s %s",
			prefix,
			bar(progress, total),
			ioprogress.DrawTextFormatBytes(progress, total),
		)
	}

	reader := &ioprogress.Reader{
		Reader:       res.Body,
		Size:         res.ContentLength,
		DrawFunc:     ioprogress.DrawTerminalf(os.Stdout, fmtfunc),
		DrawInterval: time.Second,
	}

	// TODO(jonboulle): handle http more robustly (redirects?)
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad HTTP status code: %d", res.StatusCode)
	}

	key, err := ds.WriteACI(r.Hash(), reader)
	if err != nil {
		return nil, err
	}

	r.Blob = key
	ds.WriteIndex(&r)

	return &r, nil
}
