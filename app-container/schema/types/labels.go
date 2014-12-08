package types

import (
	"encoding/json"
	"errors"
)

// TODO(jonboulle): this is awkward since it's inconsistent with the way we do
// things elsewhere (i.e. using strict types instead of string types), but it's
// tricky because Labels needs to be able to catch arbitrary key-values.
// Clean this up somehow?
type Labels []Label

type labels Labels

type Label struct {
	Name  ACName `json:"name"`
	Value string `json:"val"`
}

func (l Labels) assertValid() error {
	if os, ok := l.get("os"); ok && os != "linux" {
		return errors.New(`bad os (must be "linux")`)
	}
	if arch, ok := l.get("arch"); ok && arch != "amd64" {
		return errors.New(`bad arch (must be "amd64")`)
	}

	return nil
}

func (l Labels) MarshalJSON() ([]byte, error) {
	if err := l.assertValid(); err != nil {
		return nil, err
	}
	return json.Marshal(labels(l))
}

func (l *Labels) UnmarshalJSON(data []byte) error {
	var jl labels
	if err := json.Unmarshal(data, &jl); err != nil {
		return err
	}
	nl := Labels(jl)
	if err := l.assertValid(); err != nil {
		return err
	}
	*l = nl
	return nil
}

func (l Labels) get(name string) (val string, ok bool) {
	for _, lbl := range l {
		if lbl.Name.String() == name {
			return lbl.Value, true
		}
	}
	return "", false
}
