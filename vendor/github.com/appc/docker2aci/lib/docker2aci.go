// Copyright 2015 The appc Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package docker2aci implements a simple library for converting docker images to
// App Container Images (ACIs).
package docker2aci

import (
	"archive/tar"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/appc/docker2aci/lib/common"
	"github.com/appc/docker2aci/lib/internal"
	"github.com/appc/docker2aci/lib/internal/backend/file"
	"github.com/appc/docker2aci/lib/internal/backend/repository"
	"github.com/appc/docker2aci/lib/internal/docker"
	"github.com/appc/docker2aci/lib/internal/tarball"
	"github.com/appc/docker2aci/lib/internal/types"
	"github.com/appc/docker2aci/lib/internal/util"
	"github.com/appc/docker2aci/pkg/log"
	"github.com/appc/spec/pkg/acirenderer"
	"github.com/appc/spec/schema"
	appctypes "github.com/appc/spec/schema/types"
	gzip "github.com/klauspost/pgzip"
)

// CommonConfig represents the shared configuration options for converting
// Docker images.
type CommonConfig struct {
	Squash      bool               // squash the layers in one file
	OutputDir   string             // where to put the resulting ACI
	TmpDir      string             // directory to use for temporary files
	Compression common.Compression // which compression to use for the resulting file(s)
}

// RemoteConfig represents the remote repository specific configuration for
// converting Docker images.
type RemoteConfig struct {
	CommonConfig
	Username string                // username to use if the image to convert needs authentication
	Password string                // password to use if the image to convert needs authentication
	Insecure common.InsecureConfig // Insecure options
}

// FileConfig represents the saved file specific configuration for converting
// Docker images.
type FileConfig struct {
	CommonConfig
	DockerURL string // select an image if there are several images/tags in the file, Syntax: "{docker registry URL}/{image name}:{tag}"
}

// ConvertRemoteRepo generates ACI images from docker registry URLs.  It takes
// as input a dockerURL of the form:
//
//     {registry URL}/{repository}:{reference[tag|digest]}
//
// It then gets all the layers of the requested image and converts each of
// them to ACI.
// It returns the list of generated ACI paths.
func ConvertRemoteRepo(dockerURL string, config RemoteConfig) ([]string, error) {
	repositoryBackend := repository.NewRepositoryBackend(config.Username, config.Password, config.Insecure)
	return convertReal(repositoryBackend, dockerURL, config.Squash, config.OutputDir, config.TmpDir, config.Compression)
}

// ConvertSavedFile generates ACI images from a file generated with "docker
// save".  If there are several images/tags in the file, a particular image can
// be chosen via FileConfig.DockerURL.
//
// It returns the list of generated ACI paths.
func ConvertSavedFile(dockerSavedFile string, config FileConfig) ([]string, error) {
	f, err := os.Open(dockerSavedFile)
	if err != nil {
		return nil, fmt.Errorf("error opening file: %v", err)
	}
	defer f.Close()

	fileBackend := file.NewFileBackend(f)
	return convertReal(fileBackend, config.DockerURL, config.Squash, config.OutputDir, config.TmpDir, config.Compression)
}

// GetIndexName returns the docker index server from a docker URL.
func GetIndexName(dockerURL string) string {
	index, _ := docker.SplitReposName(dockerURL)
	return index
}

// GetDockercfgAuth reads a ~/.dockercfg file and returns the username and password
// of the given docker index server.
func GetDockercfgAuth(indexServer string) (string, string, error) {
	return docker.GetAuthInfo(indexServer)
}

func convertReal(backend internal.Docker2ACIBackend, dockerURL string, squash bool, outputDir string, tmpDir string, compression common.Compression) ([]string, error) {
	log.Debug("Getting image info...")
	ancestry, parsedDockerURL, err := backend.GetImageInfo(dockerURL)
	if err != nil {
		return nil, err
	}

	layersOutputDir := outputDir
	if squash {
		layersOutputDir, err = ioutil.TempDir(tmpDir, "docker2aci-")
		if err != nil {
			return nil, fmt.Errorf("error creating dir: %v", err)
		}
		defer os.RemoveAll(layersOutputDir)
	}

	conversionStore := newConversionStore()

	// only compress individual layers if we're not squashing
	layerCompression := compression
	if squash {
		layerCompression = common.NoCompression
	}

	aciLayerPaths, aciManifests, err := backend.BuildACI(ancestry, parsedDockerURL, layersOutputDir, tmpDir, layerCompression)
	if err != nil {
		return nil, err
	}

	var images acirenderer.Images
	for i, aciLayerPath := range aciLayerPaths {
		key, err := conversionStore.WriteACI(aciLayerPath)
		if err != nil {
			return nil, fmt.Errorf("error inserting in the conversion store: %v", err)
		}

		images = append(images, acirenderer.Image{Im: aciManifests[i], Key: key, Level: uint16(len(aciLayerPaths) - 1 - i)})
	}

	// acirenderer expects images in order from upper to base layer
	images = util.ReverseImages(images)
	if squash {
		squashedImagePath, err := squashLayers(images, conversionStore, *parsedDockerURL, outputDir, compression)
		if err != nil {
			return nil, fmt.Errorf("error squashing image: %v", err)
		}
		aciLayerPaths = []string{squashedImagePath}
	}

	return aciLayerPaths, nil
}

// squashLayers receives a list of ACI layer file names ordered from base image
// to application image and squashes them into one ACI
func squashLayers(images []acirenderer.Image, aciRegistry acirenderer.ACIRegistry, parsedDockerURL types.ParsedDockerURL, outputDir string, compression common.Compression) (path string, err error) {
	log.Debug("Squashing layers...")
	log.Debug("Rendering ACI...")
	renderedACI, err := acirenderer.GetRenderedACIFromList(images, aciRegistry)
	if err != nil {
		return "", fmt.Errorf("error rendering squashed image: %v", err)
	}
	manifests, err := getManifests(renderedACI, aciRegistry)
	if err != nil {
		return "", fmt.Errorf("error getting manifests: %v", err)
	}

	squashedFilename := getSquashedFilename(parsedDockerURL)
	squashedImagePath := filepath.Join(outputDir, squashedFilename)

	squashedTempFile, err := ioutil.TempFile(outputDir, "docker2aci-squashedFile-")
	if err != nil {
		return "", err
	}
	defer func() {
		if err == nil {
			err = squashedTempFile.Close()
		} else {
			// remove temp file on error
			// we ignore its error to not mask the real error
			os.Remove(squashedTempFile.Name())
		}
	}()

	log.Debug("Writing squashed ACI...")
	if err := writeSquashedImage(squashedTempFile, renderedACI, aciRegistry, manifests, compression); err != nil {
		return "", fmt.Errorf("error writing squashed image: %v", err)
	}

	log.Debug("Validating squashed ACI...")
	if err := internal.ValidateACI(squashedTempFile.Name()); err != nil {
		return "", fmt.Errorf("error validating image: %v", err)
	}

	if err := os.Rename(squashedTempFile.Name(), squashedImagePath); err != nil {
		return "", err
	}

	log.Debug("ACI squashed!")
	return squashedImagePath, nil
}

func getSquashedFilename(parsedDockerURL types.ParsedDockerURL) string {
	squashedFilename := strings.Replace(parsedDockerURL.ImageName, "/", "-", -1)
	if parsedDockerURL.Tag != "" {
		squashedFilename += "-" + parsedDockerURL.Tag
	}
	squashedFilename += ".aci"

	return squashedFilename
}

func getManifests(renderedACI acirenderer.RenderedACI, aciRegistry acirenderer.ACIRegistry) ([]schema.ImageManifest, error) {
	var manifests []schema.ImageManifest

	for _, aci := range renderedACI {
		im, err := aciRegistry.GetImageManifest(aci.Key)
		if err != nil {
			return nil, err
		}
		manifests = append(manifests, *im)
	}

	return manifests, nil
}

func writeSquashedImage(outputFile *os.File, renderedACI acirenderer.RenderedACI, aciProvider acirenderer.ACIProvider, manifests []schema.ImageManifest, compression common.Compression) error {
	var tarWriterTarget io.WriteCloser = outputFile

	switch compression {
	case common.NoCompression:
	case common.GzipCompression:
		tarWriterTarget = gzip.NewWriter(outputFile)
		defer tarWriterTarget.Close()
	default:
		return fmt.Errorf("unexpected compression enum value: %d", compression)
	}

	outputWriter := tar.NewWriter(tarWriterTarget)
	defer outputWriter.Close()

	finalManifest := mergeManifests(manifests)

	if err := internal.WriteManifest(outputWriter, finalManifest); err != nil {
		return err
	}

	if err := internal.WriteRootfsDir(outputWriter); err != nil {
		return err
	}

	type hardLinkEntry struct {
		firstLinkCleanName string
		firstLinkHeader    tar.Header
		keepOriginal       bool
		walked             bool
	}
	// map aciFileKey -> cleanTarget -> hardLinkEntry
	hardLinks := make(map[string]map[string]hardLinkEntry)

	// first pass: read all the entries and build the hardLinks map in memory
	// but don't write on disk
	for _, aciFile := range renderedACI {
		rs, err := aciProvider.ReadStream(aciFile.Key)
		if err != nil {
			return err
		}
		defer rs.Close()

		hardLinks[aciFile.Key] = map[string]hardLinkEntry{}

		squashWalker := func(t *tarball.TarFile) error {
			cleanName := filepath.Clean(t.Name())
			// the rootfs and the squashed manifest are added separately
			if cleanName == "manifest" || cleanName == "rootfs" {
				return nil
			}
			_, keep := aciFile.FileMap[cleanName]
			if keep && t.Header.Typeflag == tar.TypeLink {
				cleanTarget := filepath.Clean(t.Linkname())
				if _, ok := hardLinks[aciFile.Key][cleanTarget]; !ok {
					_, keepOriginal := aciFile.FileMap[cleanTarget]
					hardLinks[aciFile.Key][cleanTarget] = hardLinkEntry{cleanName, *t.Header, keepOriginal, false}
				}
			}
			return nil
		}

		tr := tar.NewReader(rs)
		if err := tarball.Walk(*tr, squashWalker); err != nil {
			return err
		}
	}

	// second pass: write on disk
	for _, aciFile := range renderedACI {
		rs, err := aciProvider.ReadStream(aciFile.Key)
		if err != nil {
			return err
		}
		defer rs.Close()

		squashWalker := func(t *tarball.TarFile) error {
			cleanName := filepath.Clean(t.Name())
			// the rootfs and the squashed manifest are added separately
			if cleanName == "manifest" || cleanName == "rootfs" {
				return nil
			}
			_, keep := aciFile.FileMap[cleanName]

			if link, ok := hardLinks[aciFile.Key][cleanName]; ok {
				if keep != link.keepOriginal {
					return fmt.Errorf("logic error: should we keep file %q?", cleanName)
				}
				if keep {
					if err := outputWriter.WriteHeader(t.Header); err != nil {
						return fmt.Errorf("error writing header: %v", err)
					}
					if _, err := io.Copy(outputWriter, t.TarStream); err != nil {
						return fmt.Errorf("error copying file into the tar out: %v", err)
					}
				} else {
					// The current file does not remain but there is a hard link pointing to
					// it. Write the current file but with the filename of the first hard link
					// pointing to it. That first hard link will not be written later, see
					// variable "alreadyWritten".
					link.firstLinkHeader.Size = t.Header.Size
					link.firstLinkHeader.Typeflag = t.Header.Typeflag
					link.firstLinkHeader.Linkname = ""

					if err := outputWriter.WriteHeader(&link.firstLinkHeader); err != nil {
						return fmt.Errorf("error writing header: %v", err)
					}
					if _, err := io.Copy(outputWriter, t.TarStream); err != nil {
						return fmt.Errorf("error copying file into the tar out: %v", err)
					}
				}
			} else if keep {
				alreadyWritten := false
				if t.Header.Typeflag == tar.TypeLink {
					cleanTarget := filepath.Clean(t.Linkname())
					if link, ok := hardLinks[aciFile.Key][cleanTarget]; ok {
						if !link.keepOriginal {
							if link.walked {
								t.Header.Linkname = link.firstLinkCleanName
							} else {
								alreadyWritten = true
							}
						}
						link.walked = true
						hardLinks[aciFile.Key][cleanTarget] = link
					}
				}

				if !alreadyWritten {
					if err := outputWriter.WriteHeader(t.Header); err != nil {
						return fmt.Errorf("error writing header: %v", err)
					}
					if _, err := io.Copy(outputWriter, t.TarStream); err != nil {
						return fmt.Errorf("error copying file into the tar out: %v", err)
					}
				}
			}
			return nil
		}

		tr := tar.NewReader(rs)
		if err := tarball.Walk(*tr, squashWalker); err != nil {
			return err
		}
	}

	return nil
}

func mergeManifests(manifests []schema.ImageManifest) schema.ImageManifest {
	// FIXME(iaguis) we take app layer's manifest as the final manifest for now
	manifest := manifests[0]

	manifest.Dependencies = nil

	layerIndex := -1
	for i, l := range manifest.Labels {
		if l.Name.String() == "layer" {
			layerIndex = i
		}
	}

	if layerIndex != -1 {
		manifest.Labels = append(manifest.Labels[:layerIndex], manifest.Labels[layerIndex+1:]...)
	}

	nameWithoutLayerID := appctypes.MustACIdentifier(stripLayerID(manifest.Name.String()))

	manifest.Name = *nameWithoutLayerID

	// once the image is squashed, we don't need a pathWhitelist
	manifest.PathWhitelist = nil

	return manifest
}

// striplayerID strips the layer ID from an app name:
//
// myregistry.com/organization/app-name-85738f8f9a7f1b04b5329c590ebcb9e425925c6d0984089c43a022de4f19c281
// myregistry.com/organization/app-name
func stripLayerID(layerName string) string {
	n := strings.LastIndex(layerName, "-")
	return layerName[:n]
}
