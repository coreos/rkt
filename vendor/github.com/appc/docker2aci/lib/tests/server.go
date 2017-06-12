package test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"strings"
	"testing"
)

func RunDockerRegistry(t *testing.T, imgPath, imgName, imgRef, manifestMediaType string) *httptest.Server {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Logf("path requested: %s", r.URL.Path)
		if r.URL.Path == "/v2/" {
			w.Header().Add("Docker-Distribution-API-Version", "registry/2.0")
			w.WriteHeader(http.StatusOK)
			return
		}
		if strings.Contains(r.URL.Path, "manifests") {
			GetManifest(t, w, r, imgPath, imgName, imgRef, manifestMediaType)
			return
		}
		if strings.Contains(r.URL.Path, "blobs") {
			GetBlob(t, w, r, imgPath, imgName, imgRef)
			return
		}
		t.Errorf("invalid path: %s", r.URL.Path)
	})
	server := httptest.NewServer(handler)
	return server
}

func GetManifest(t *testing.T, w http.ResponseWriter, r *http.Request, imgPath, imgName, imgRef, manifestMediaType string) {
	parsedImgName, parsedRef, err := parseURL("manifests", r.URL.Path)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		t.Errorf("get manifest: error parsing path: %v", err)
		return
	}
	if parsedImgName != imgName {
		w.WriteHeader(http.StatusNotFound)
		t.Errorf("get manifest: invalid image name requested: %q", parsedImgName)
		return
	}
	if parsedRef != imgRef {
		w.WriteHeader(http.StatusNotFound)
		t.Errorf("get manifest: invalid image ref requested: %q", parsedImgName)
		return
	}
	manFile, err := os.Open(path.Join(imgPath, "manifest.json"))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		t.Errorf("get manifest: couldn't open manifest: %v", err)
		return
	}
	defer manFile.Close()
	w.Header().Add("content-type", manifestMediaType)
	_, err = io.Copy(w, manFile)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		t.Errorf("get manifest: couldn't copy manifest: %v", err)
		return
	}
}

func GetBlob(t *testing.T, w http.ResponseWriter, r *http.Request, imgPath, imgName, imgRef string) {
	parsedImgName, digest, err := parseURL("blobs", r.URL.Path)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		t.Errorf("get blob: %v", err)
		return
	}
	digest = strings.TrimPrefix(digest, "sha256:")
	if parsedImgName != imgName {
		w.WriteHeader(http.StatusNotFound)
		t.Errorf("get blob: invalid image name requested: %s", parsedImgName)
		return
	}
	blobFile, err := os.Open(path.Join(imgPath, digest))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		t.Errorf("get blob: couldn't open manifest: %v", err)
		return
	}
	defer blobFile.Close()
	_, err = io.Copy(w, blobFile)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		t.Errorf("get blob: couldn't copy manifest: %v", err)
		return
	}
}

func parseURL(resource, input string) (string, string, error) {
	tokens := strings.Split(input, "/")
	tokLen := len(tokens)
	if tokLen < 5 {
		return "", "", fmt.Errorf("invalid number of tokens in path: %d", len(tokens))
	}
	if tokens[0] != "" {
		return "", "", fmt.Errorf("path parse error: tok0 = %s", tokens[0])
	}
	if tokens[1] != "v2" {
		return "", "", fmt.Errorf("path parse error: tok1 = %s", tokens[1])
	}
	if tokens[tokLen-2] != resource {
		return "", "", fmt.Errorf("path parse error: tok-2 = %s", tokens[tokLen-2])
	}
	return path.Join(tokens[2 : tokLen-2]...), tokens[tokLen-1], nil
}
