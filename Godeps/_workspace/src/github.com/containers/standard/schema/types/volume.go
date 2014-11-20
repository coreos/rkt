package types

import (
	"encoding/json"
	"errors"
)

// Volume encapsulates a volume which should be mounted into the filesystem
// of all apps in a ContainerRuntimeManifest
type Volume struct {
	Kind     string    `json:"kind"`
	Fulfills []ACLabel `json:"fulfills"`

	// currently used only by "host"
	// TODO(jonboulle): factor out?
	Source   string `json:"source,omitempty"`
	ReadOnly bool   `json:"readOnly,omitempty"`
}

type volume Volume

func (v Volume) assertValid() error {
	switch v.Kind {
	case "empty":
		return nil
	case "host":
		if v.Source == "" {
			return errors.New("source for host volume cannot be empty")
		}
		return nil
	default:
		return errors.New(`unrecognized volume type: should be one of "empty", "host"`)
	}
}

func (v *Volume) UnmarshalJSON(data []byte) error {
	var vv volume
	if err := json.Unmarshal(data, &vv); err != nil {
		return err
	}
	nv := Volume(vv)
	if err := nv.assertValid(); err != nil {
		return err
	}
	*v = nv
	return nil
}

func (v Volume) MarshalJSON() ([]byte, error) {
	if err := v.assertValid(); err != nil {
		return nil, err
	}
	return json.Marshal(volume(v))
}
