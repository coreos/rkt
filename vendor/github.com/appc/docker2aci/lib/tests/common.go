package test

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/ioutil"
	"os"
	"path"

	"github.com/appc/docker2aci/lib/common"
	"github.com/appc/docker2aci/lib/internal/typesV2"
)

type Layer map[*tar.Header][]byte

type Docker22Image struct {
	RepoTags []string
	Layers   []Layer
	Config   typesV2.ImageConfig
}

func GenerateDocker22(destPath string, img Docker22Image) error {
	layerHashes, err := GenLayers(destPath, img.Layers)
	if err != nil {
		return err
	}
	configHash, err := GenDocker22Config(destPath, img.Config, layerHashes)
	if err != nil {
		return err
	}
	err = GenDocker22Manifest(destPath, configHash, layerHashes)
	if err != nil {
		return err
	}
	return nil
}

func GenLayers(destPath string, layers []Layer) ([]string, error) {
	var layerHashes []string
	for _, l := range layers {
		layerBuffer := &bytes.Buffer{}
		tw := tar.NewWriter(layerBuffer)
		for hdr, contents := range l {
			hdr.Size = int64(len(contents))
			err := tw.WriteHeader(hdr)
			if err != nil {
				tw.Close()
				return nil, err
			}
			_, err = tw.Write(contents)
			if err != nil {
				tw.Close()
				return nil, err
			}
		}
		tw.Close()
		layerTarBlob := layerBuffer.Bytes()
		h := sha256.New()
		h.Write(layerTarBlob)
		hashStr := hex.EncodeToString(h.Sum(nil))
		layerHashes = append(layerHashes, hashStr)
		err := ioutil.WriteFile(path.Join(destPath, hashStr), layerTarBlob, 0644)
		if err != nil {
			return nil, err
		}
	}
	return layerHashes, nil
}

func GenDocker22Config(destPath string, conf typesV2.ImageConfig, layerHashes []string) (string, error) {
	conf.RootFS = &typesV2.ImageConfigRootFS{}
	conf.RootFS.Type = "layers"
	for _, h := range layerHashes {
		conf.RootFS.DiffIDs = append(conf.RootFS.DiffIDs, "sha256:"+h)
	}
	confblob, err := json.Marshal(conf)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	h.Write(confblob)
	hashStr := hex.EncodeToString(h.Sum(nil))
	err = ioutil.WriteFile(path.Join(destPath, hashStr), confblob, 0644)
	if err != nil {
		return "", err
	}
	return hashStr, nil
}

func GenDocker22Manifest(destPath, configHash string, layerHashes []string) error {
	getDigestSize := func(digest string) (int64, error) {
		fi, err := os.Stat(path.Join(destPath, digest))
		if err != nil {
			return 0, err
		}
		return fi.Size(), nil
	}

	configSize, err := getDigestSize(configHash)
	if err != nil {
		return err
	}

	manifest := &typesV2.ImageManifest{
		SchemaVersion: 2,
		MediaType:     common.MediaTypeDockerV22Manifest,
		Config: &typesV2.ImageManifestDigest{
			MediaType: common.MediaTypeDockerV22Config,
			Size:      int(configSize),
			Digest:    "sha256:" + configHash,
		},
	}
	for _, h := range layerHashes {
		layerSize, err := getDigestSize(h)
		if err != nil {
			return err
		}
		manifest.Layers = append(manifest.Layers,
			&typesV2.ImageManifestDigest{
				MediaType: common.MediaTypeDockerV22RootFS,
				Size:      int(layerSize),
				Digest:    "sha256:" + h,
			})
	}

	manblob, err := json.Marshal(manifest)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(path.Join(destPath, "manifest.json"), manblob, 0644)
	if err != nil {
		return err
	}
	return nil
}
