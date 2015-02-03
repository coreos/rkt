package schema

import (
	"encoding/json"
	"errors"

	"github.com/coreos/rocket/Godeps/_workspace/src/github.com/appc/spec/schema/types"
)

type ContainerRuntimeManifest struct {
	ACVersion   types.SemVer      `json:"acVersion"`
	ACKind      types.ACKind      `json:"acKind"`
	UUID        types.UUID        `json:"uuid"`
	Apps        AppList           `json:"apps"`
	Volumes     []types.Volume    `json:"volumes"`
	Isolators   []types.Isolator  `json:"isolators"`
	Annotations types.Annotations `json:"annotations"`
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

type AppList []RuntimeApp

// Get retrieves an app by the specified name from the AppList; if there is
// no such app, nil is returned. The returned *RuntimeApp MUST be considered
// read-only.
func (al AppList) Get(name types.ACName) *RuntimeApp {
	for _, a := range al {
		if name.Equals(a.Name) {
			aa := a
			return &aa
		}
	}
	return nil
}

// Mount describes the mapping between a volume and an apps
// MountPoint that will be fulfilled at runtime.
type Mount struct {
	Volume     types.ACName `json:"volume"`
	MountPoint types.ACName `json:"mountPoint"`
}

func (r Mount) assertValid() error {
	if r.Volume.Empty() {
		return errors.New("volume must be set")
	}
	if r.MountPoint.Empty() {
		return errors.New("mountPoint must be set")
	}
	return nil
}

// RuntimeApp describes an application referenced in a ContainerRuntimeManifest
type RuntimeApp struct {
	Name        types.ACName      `json:"name"`
	ImageID     types.Hash        `json:"imageID"`
	Mounts      []Mount           `json:"mounts"`
	Isolators   []types.Isolator  `json:"isolators"`
	Annotations types.Annotations `json:"annotations"`
}
