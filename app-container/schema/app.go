package schema

import (
	"encoding/json"
	"errors"

	"github.com/coreos/rocket/app-container/schema/types"
)

const (
	ACIExtension = ".aci"
)

type ImageManifest struct {
	ACKind        types.ACKind       `json:"acKind"`
	ACVersion     types.SemVer       `json:"acVersion"`
	Name          types.ACName       `json:"name"`
	Labels        types.Labels       `json:"labels"`
	App           types.App          `json:"app"`
	Annotations   types.Annotations  `json:"annotations"`
	Dependencies  types.Dependencies `json:"dependencies"`
	PathWhitelist []string           `json:"pathWhitelist"`
}

// appManifest is a model to facilitate extra validation during the
// unmarshalling of the ImageManifest
type appManifest ImageManifest

func (am *ImageManifest) UnmarshalJSON(data []byte) error {
	a := appManifest{}
	err := json.Unmarshal(data, &a)
	if err != nil {
		return err
	}
	nam := ImageManifest(a)
	if err := nam.assertValid(); err != nil {
		return err
	}
	*am = nam
	return nil
}

func (am ImageManifest) MarshalJSON() ([]byte, error) {
	if err := am.assertValid(); err != nil {
		return nil, err
	}
	return json.Marshal(appManifest(am))
}

// assertValid performs extra assertions on an ImageManifest to ensure that
// fields are set appropriately, etc. It is used exclusively when marshalling
// and unmarshalling an ImageManifest. Most field-specific validation is
// performed through the individual types being marshalled; assertValid()
// should only deal with higher-level validation.
func (am *ImageManifest) assertValid() error {
	if am.ACKind != "ImageManifest" {
		return types.ACKindError(`missing or bad ACKind (must be "ImageManifest")`)
	}
	if am.ACVersion.Empty() {
		return errors.New(`acVersion must be set`)
	}
	if am.Name.Empty() {
		return errors.New(`name must be set`)
	}
	return nil
}
