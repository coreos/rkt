package schema

import (
	"encoding/json"

	"github.com/containers/standard/schema/types"
)

type ContainerRuntimeManifest struct {
	ACVersion   types.SemVer             `json:"acVersion"`
	ACKind      types.ACKind             `json:"acKind"`
	UUID        types.UUID               `json:"uuid"`
	Apps        map[types.ACLabel]App    `json:"apps"`
	Volumes     []types.Volume           `json:"volumes"`
	Isolators   map[types.ACLabel]string `json:"isolators"`
	Annotations map[types.ACLabel]string `json:"annotations"`
}

// containerRuntimeManifest is a model to facilitate extra validation during the
// unmarshalling of the ContainerRuntimeManifest
type containerRuntimeManifest ContainerRuntimeManifest

func (cm *ContainerRuntimeManifest) UnmarshalJSON(data []byte) error {
	c := containerRuntimeManifest{}
	err := json.Unmarshal(data, &c)
	if err != nil {
		return err
	}
	ncm := ContainerRuntimeManifest(c)
	if err := ncm.assertValid(); err != nil {
		return err
	}
	*cm = ncm
	return nil
}

func (cm ContainerRuntimeManifest) MarshalJSON() ([]byte, error) {
	if err := cm.assertValid(); err != nil {
		return nil, err
	}
	return json.Marshal(containerRuntimeManifest(cm))
}

// assertValid performs extra assertions on an ContainerRuntimeManifest to
// ensure that fields are set appropriately, etc. It is used exclusively when
// marshalling and unmarshalling an ContainerRuntimeManifest. Most
// field-specific validation is performed through the individual types being
// marshalled; assertValid() should only deal with higher-level validation.
func (cm *ContainerRuntimeManifest) assertValid() error {
	if cm.ACKind != "ContainerRuntimeManifest" {
		return types.ACKindError(`missing or bad ACKind (must be "ContainerRuntimeManifest")`)
	}
	return nil
}

// App describes an application referenced in a ContainerRuntimeManifest
// TODO(jonboulle): typeify
type App struct {
	ImageID     types.Hash               `json:"imageID"`
	Isolators   map[types.ACLabel]string `json:"isolators"`
	Annotations map[types.ACLabel]string `json:"annotations"`
}
