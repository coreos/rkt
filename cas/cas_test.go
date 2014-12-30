package cas

import (
	"bytes"
	"encoding/base64"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/appc/spec/schema/types"
)

const tstprefix = "cas-test"

func TestBlobStore(t *testing.T) {
	dir, err := ioutil.TempDir("", tstprefix)
	if err != nil {
		t.Fatalf("error creating tempdir: %v", err)
	}
	defer os.RemoveAll(dir)
	ds := NewStore(dir)
	for _, valueStr := range []string{
		"I am a manually placed object",
	} {
		ds.stores[blobType].Write(types.NewHashSHA512([]byte(valueStr)).String(), []byte(valueStr))
	}

	ds.Dump(false)
}

func TestDownloading(t *testing.T) {
	dir, err := ioutil.TempDir("", tstprefix)
	if err != nil {
		t.Fatalf("error creating tempdir: %v", err)
	}
	defer os.RemoveAll(dir)
	// TODO(philips): construct a real tarball using go, this is a base64 tarball with an empty file
	body, _ := base64.StdEncoding.DecodeString("H4sIAIWbdlQAA+3PPQrCQBiE4ZU0NnoDcTstv8T9OYaNF7AwGFAIJlqn9wba5Cp6EG9gbWtiII1oF0R4n2bYZVhm81WWq46JiDNG1+mdfaVEzblhrQ4jM7M2tE5ESxhZb5SWrofV9lm+3FVT0nWySdLsY6+qxfGXd5qf6Db/xPjYV8X5sFDB/dIbVBfX8jHfDn3ZNgofnG6jiZr+bCMAAAAAAAAAAAAAAAAA4N0T/slETwAoAAA=")
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(body)
	}))
	defer ts.Close()

	tests := []struct {
		r    Remote
		body []byte
		hit  bool
	}{
		{Remote{ts.URL, []string{}, "12", "96609004016e9625763c7153b74120c309c8cb1bd794345bf6fa2e60ac001cd7"}, body, false},
		{Remote{ts.URL, []string{}, "12", "96609004016e9625763c7153b74120c309c8cb1bd794345bf6fa2e60ac001cd7"}, body, true},
	}

	ds := NewStore(dir)

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

func TestResolveKey(t *testing.T) {
	dir, err := ioutil.TempDir("", tstprefix)
	if err != nil {
		t.Fatalf("error creating tempdir: %v", err)
	}
	defer os.RemoveAll(dir)
	ds := NewStore(dir)

	// Set up store (use key == data for simplicity)
	data := []*bytes.Buffer{
		bytes.NewBufferString("sha512-1234567890"),
		bytes.NewBufferString("sha512-abcdefghi"),
		bytes.NewBufferString("sha512-abcjklmno"),
		bytes.NewBufferString("sha512-abcpqwert"),
	}
	for _, d := range data {
		if err := ds.WriteStream(d.String(), d); err != nil {
			t.Fatalf("error writing to store: %v", err)
		}
	}

	// Full key already - should return short version of the full key
	fkl := "sha512-67147019a5b56f5e2ee01e989a8aa4787f56b8445960be2d8678391cf111009bc0780f31001fd181a2b61507547aee4caa44cda4b8bdb238d0e4ba830069ed2c"
	fks := "sha512-67147019a5b56f5e2ee01e989a8aa4787f56b8445960be2d8678391cf111009b"
	for _, k := range []string{fkl, fks} {
		key, err := ds.ResolveKey(k)
		if key != fks {
			t.Errorf("expected ResolveKey to return unaltered short key, but got %q", key)
		}
		if err != nil {
			t.Errorf("expected err=nil, got %v", err)
		}
	}

	// Unambiguous prefix match
	k, err := ds.ResolveKey("sha512-123")
	if k != "sha512-1234567890" {
		t.Errorf("expected %q, got %q", "sha512-1234567890", k)
	}
	if err != nil {
		t.Errorf("expected err=nil, got %v", err)
	}

	// Ambiguous prefix match
	k, err = ds.ResolveKey("sha512-abc")
	if k != "" {
		t.Errorf("expected %q, got %q", "", k)
	}
	if err == nil {
		t.Errorf("expected non-nil error!")
	}
}
