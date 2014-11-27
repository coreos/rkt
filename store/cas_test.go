package store

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestObjectStore(t *testing.T) {
	ds := NewStore()
	for _, valueStr := range []string{
		"I am a manually placed object",
	} {
		ds.stores[objectType].Write(sha256sum(valueStr), []byte(valueStr))
	}

	ds.Dump(false)
}

func TestDownloading(t *testing.T) {
	body := "I am a UStar tarball"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, body)
	}))
	defer ts.Close()

	tests := []struct {
		r    Remote
		body string
		hit  bool
	}{
		{Remote{ts.URL, []string{}, "12", "425656b85674825423ae7285e7837b60bc53401b"}, body, false},
		{Remote{ts.URL, []string{}, "12", "425656b85674825423ae7285e7837b60bc53401b"}, body, true},
	}

	ds := NewStore()

	for _, tt := range tests {
		_, err := ds.stores[remoteType].Read(tt.r.Hash())
		if tt.hit == false && err == nil {
			panic("expected miss got a hit")
		}
		if tt.hit == true && err != nil {
			panic("expected a hit got a miss")
		}
		ds.stores[remoteType].Write(tt.r.Hash(), tt.r.Marshal())
		_, err = tt.r.Download(*ds)
		if err != nil {
			panic(err)
		}
	}

	ds.Dump(false)
}
