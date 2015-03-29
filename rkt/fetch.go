package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/coreos/rocket/cas"
	"github.com/coreos/rocket/pkg/keystore"

	"github.com/coreos/rocket/Godeps/_workspace/src/github.com/appc/docker2aci/lib"
	"github.com/coreos/rocket/Godeps/_workspace/src/github.com/appc/spec/aci"
	"github.com/coreos/rocket/Godeps/_workspace/src/github.com/appc/spec/discovery"
	"github.com/coreos/rocket/Godeps/_workspace/src/github.com/appc/spec/schema/types"
	"github.com/coreos/rocket/Godeps/_workspace/src/github.com/mitchellh/ioprogress"
	"github.com/coreos/rocket/Godeps/_workspace/src/golang.org/x/crypto/openpgp"
)

const (
	defaultOS   = runtime.GOOS
	defaultArch = runtime.GOARCH

	defaultPathPerm os.FileMode = 0777
)

var (
	cmdFetch = &Command{
		Name:    "fetch",
		Summary: "Fetch image(s) and store them in the local cache",
		Usage:   "IMAGE_URL...",
		Run:     runFetch,
	}
)

func init() {
	commands = append(commands, cmdFetch)
}

func runFetch(args []string) (exit int) {
	if len(args) < 1 {
		stderr("fetch: Must provide at least one image")
		return 1
	}

	ds, err := cas.NewStore(globalFlags.Dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fetch: cannot open store: %v\n", err)
		return 1
	}
	ks := getKeystore()

	for _, img := range args {
		hash, err := fetchImage(img, ds, ks, true)
		if err != nil {
			stderr("%v", err)
			return 1
		}
		shortHash := types.ShortHash(hash)
		fmt.Println(shortHash)
	}

	return
}

// fetchImage will take an image as either a URL or a name string and import it
// into the store if found.  If discover is true meta-discovery is enabled.
func fetchImage(img string, ds *cas.Store, ks *keystore.Keystore, discover bool) (string, error) {
	u, err := url.Parse(img)
	if err == nil && discover && u.Scheme == "" {
		if app := newDiscoveryApp(img); app != nil {
			stdout("rkt: searching for app image %s", img)
			ep, attempts, err := discovery.DiscoverEndpoints(*app, true)

			if globalFlags.Debug {
				for _, a := range attempts {
					stderr("meta tag 'ac-discovery' not found on %s: %v", a.Prefix, a.Error)
				}
			}

			if err != nil {
				return "", err
			}

			if len(ep.ACIEndpoints) == 0 {
				return "", fmt.Errorf("no endpoints discovered")
			}

			latest := false
			// No specified version label, mark it as latest
			if _, ok := app.Labels["version"]; !ok {
				latest = true
			}
			return fetchImageFromEndpoints(ep, ds, ks, latest)
		}
	}
	if err != nil {
		return "", fmt.Errorf("not a valid URL (%s)", img)
	}
	switch u.Scheme {
	case "http", "https", "docker":
	default:
		return "", fmt.Errorf("rkt only supports http, https or docker URLs (%s)", img)
	}
	return fetchImageFromURL(u.String(), u.Scheme, ds, ks, false)
}

func fetchImageFromEndpoints(ep *discovery.Endpoints, ds *cas.Store, ks *keystore.Keystore, latest bool) (string, error) {
	return downloadImage(ep.ACIEndpoints[0].ACI, ep.ACIEndpoints[0].ASC, "", ds, ks, latest)
}

func fetchImageFromURL(imgurl string, scheme string, ds *cas.Store, ks *keystore.Keystore, latest bool) (string, error) {
	return downloadImage(imgurl, ascURLFromImgURL(imgurl), scheme, ds, ks, latest)
}

func downloadImage(aciURL string, ascURL string, scheme string, ds *cas.Store, ks *keystore.Keystore, latest bool) (string, error) {
	stdout("rkt: fetching image from %s", aciURL)
	if globalFlags.InsecureSkipVerify {
		stdout("rkt: warning: signature verification has been disabled")
	} else if scheme == "docker" {
		return "", fmt.Errorf("signature verification for docker images is not supported (try --insecure-skip-verify)")
	}
	rem, ok, err := ds.GetRemote(aciURL)
	if err != nil {
		return "", err
	}
	if !ok {
		entity, aciFile, err := download(aciURL, ascURL, ds, ks)
		if err != nil {
			return "", err
		}
		defer os.Remove(aciFile.Name())

		if entity != nil && !globalFlags.InsecureSkipVerify {
			fmt.Println("rkt: signature verified: ")
			for _, v := range entity.Identities {
				stdout("  %s", v.Name)
			}
		}
		key, err := ds.WriteACI(aciFile, latest)
		if err != nil {
			return "", err
		}
		rem = cas.NewRemote(aciURL, ascURL)
		rem.BlobKey = key
		err = ds.WriteRemote(rem)
		if err != nil {
			return "", err
		}

	}
	return rem.BlobKey, nil
}

// download downloads and verifies the remote ACI.
// If Keystore is nil signature verification will be skipped.
// Download returns the signer, an *os.File representing the ACI, and an error if any.
// err will be nil if the ACI downloads successfully and the ACI is verified.
func download(aciURL string, ascURL string, ds *cas.Store, ks *keystore.Keystore) (*openpgp.Entity, *os.File, error) {
	var entity *openpgp.Entity
	u, err := url.Parse(aciURL)
	if err != nil {
		return nil, nil, fmt.Errorf("error parsing ACI url: %v", err)
	}
	if u.Scheme == "docker" {
		registryURL := strings.TrimPrefix(aciURL, "docker://")

		tmpDir, err := tmpDir()
		if err != nil {
			return nil, nil, fmt.Errorf("error creating temporary dir for docker to ACI conversion: %v", err)
		}

		acis, err := docker2aci.Convert(registryURL, true, tmpDir)
		if err != nil {
			return nil, nil, fmt.Errorf("error converting docker image to ACI: %v", err)
		}

		aciFile, err := os.Open(acis[0])
		if err != nil {
			return nil, nil, fmt.Errorf("error opening squashed ACI file: %v", err)
		}

		return nil, aciFile, nil
	}

	var sigTempFile *os.File
	if ks != nil {
		stdout("Downloading signature from %v\n", ascURL)
		sigTempFile, err = downloadSignatureFile(ascURL)
		if err != nil {
			return nil, nil, fmt.Errorf("error downloading the signature file: %v", err)
		}
		defer sigTempFile.Close()
		defer os.Remove(sigTempFile.Name())
	}

	acif, err := downloadACI(ds, aciURL)
	if err != nil {
		return nil, acif, fmt.Errorf("error downloading the aci image: %v", err)
	}

	if ks != nil {
		manifest, err := aci.ManifestFromImage(acif)
		if err != nil {
			return nil, acif, err
		}

		if _, err := acif.Seek(0, 0); err != nil {
			return nil, acif, err
		}
		if _, err := sigTempFile.Seek(0, 0); err != nil {
			return nil, acif, err
		}
		if entity, err = ks.CheckSignature(manifest.Name.String(), acif, sigTempFile); err != nil {
			return nil, acif, err
		}
	}

	if _, err := acif.Seek(0, 0); err != nil {
		return nil, acif, err
	}
	return entity, acif, nil
}

// downloadACI gets the aci specified at aciurl
func downloadACI(ds *cas.Store, aciurl string) (*os.File, error) {
	return downloadHTTP(aciurl, "ACI", tmpFile)
}

// downloadSignatureFile gets the signature specified at sigurl
func downloadSignatureFile(sigurl string) (*os.File, error) {
	getTemp := func() (*os.File, error) {
		return ioutil.TempFile("", "")
	}

	return downloadHTTP(sigurl, "signature", getTemp)
}

// downloadHTTP retrieves url, creating a temp file using getTempFile
// file:// http:// and https:// urls supported
func downloadHTTP(url, label string, getTempFile func() (*os.File, error)) (*os.File, error) {
	tmp, err := getTempFile()
	if err != nil {
		return nil, fmt.Errorf("error downloading %s: %v", label, err)
	}
	defer func() {
		if err != nil {
			os.Remove(tmp.Name())
			tmp.Close()
		}
	}()

	res, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	prefix := "Downloading " + label
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

	if _, err := io.Copy(tmp, reader); err != nil {
		return nil, fmt.Errorf("error copying %s: %v", label, err)
	}

	if err := tmp.Sync(); err != nil {
		return nil, fmt.Errorf("error writing %s: %v", label, err)
	}

	return tmp, nil
}

func validateURL(s string) error {
	u, err := url.Parse(s)
	if err != nil {
		return fmt.Errorf("discovery: fetched URL (%s) is invalid (%v)", s, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("rkt only supports http or https URLs (%s)", s)
	}
	return nil
}

func ascURLFromImgURL(imgurl string) string {
	s := strings.TrimSuffix(imgurl, ".aci")
	return s + ".aci.asc"
}

// newDiscoveryApp creates a discovery app if the given img is an app name and
// has a URL-like structure, for example example.com/reduce-worker.
// Or it returns nil.
func newDiscoveryApp(img string) *discovery.App {
	app, err := discovery.NewAppFromString(img)
	if err != nil {
		return nil
	}
	u, err := url.Parse(app.Name.String())
	if err != nil || u.Scheme != "" {
		return nil
	}
	if _, ok := app.Labels["arch"]; !ok {
		app.Labels["arch"] = defaultArch
	}
	if _, ok := app.Labels["os"]; !ok {
		app.Labels["os"] = defaultOS
	}
	return app
}

func tmpFile() (*os.File, error) {
	dir, err := tmpDir()
	if err != nil {
		return nil, err
	}
	return ioutil.TempFile(dir, "")
}

func tmpDir() (string, error) {
	dir := filepath.Join(globalFlags.Dir, "tmp")
	if err := os.MkdirAll(dir, defaultPathPerm); err != nil {
		return "", err
	}
	return dir, nil
}
