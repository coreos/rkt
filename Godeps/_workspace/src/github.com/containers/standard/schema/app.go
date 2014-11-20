package schema

import (
	"encoding/json"
	"errors"

	"github.com/containers/standard/schema/types"
)

type AppManifest struct {
	ACVersion     types.SemVer                 `json:"acVersion"`
	ACKind        types.ACKind                 `json:"acKind"`
	Name          types.ACLabel                `json:"name"`
	OS            string                       `json:"os"`
	Arch          string                       `json:"arch"`
	Exec          []string                     `json:"exec"`
	EventHandlers []types.EventHandler         `json:"eventHandlers"`
	User          string                       `json:"user"`
	Group         string                       `json:"group"`
	Environment   map[string]string            `json:"environment"`
	MountPoints   map[types.ACLabel]MountPoint `json:"mountPoints"`
	Ports         map[types.ACLabel]Port       `json:"ports"`
	Isolators     map[types.ACLabel]string     `json:"isolators"`
	Files         map[string]types.File        `json:"files"`
	Annotations   types.Annotations            `json:"annotations"`
}

// appManifest is a model to facilitate extra validation during the
// unmarshalling of the AppManifest
type appManifest AppManifest

func (am *AppManifest) UnmarshalJSON(data []byte) error {
	a := appManifest{}
	err := json.Unmarshal(data, &a)
	if err != nil {
		return err
	}
	nam := AppManifest(a)
	if err := nam.assertValid(); err != nil {
		return err
	}
	*am = nam
	return nil
}

func (am AppManifest) MarshalJSON() ([]byte, error) {
	if err := am.assertValid(); err != nil {
		return nil, err
	}
	return json.Marshal(appManifest(am))
}

/* TODO(jonboulle): It would be really nice to have this work, but it's messy
* relying on hashes of the serialized JSON because json is unordered and can
* contain arbitrary whitespace..

// ID returns a Hash representing the ID of the AppManifest, which is equal the
// sha1sum of the marshaled manifest. If the AppManifest cannot be successfully
// marshaled, an error is returned.
func (am AppManifest) ID() (*types.Hash, error) {
	b, err := json.Marshal(am)
	if err != nil {
		return nil, err
	}
	h := sha1.Sum(b)
	return types.NewHash(fmt.Sprintf("sha1-%x", h))
}
*/

// assertValid performs extra assertions on an AppManifest to ensure that
// fields are set appropriately, etc. It is used exclusively when marshalling
// and unmarshalling an AppManifest. Most field-specific validation is
// performed through the individual types being marshalled; assertValid()
// should only deal with higher-level validation.
func (am *AppManifest) assertValid() error {
	if am.ACKind != "AppManifest" {
		return types.ACKindError(`missing or bad ACKind (must be "AppManifest")`)
	}
	if am.OS != "linux" {
		return errors.New(`missing or bad OS (must be "linux")`)
	}
	if am.Arch != "amd64" {
		return errors.New(`missing or bad Arch (must be "amd64")`)
	}
	// TODO(jonboulle): assert hashes is not empty?
	return nil
}

// TODO(jonboulle): typify these
type MountPoint struct {
	Path     string
	ReadOnly bool
}

type Port struct {
	Protocol string
	Port     uint
}
