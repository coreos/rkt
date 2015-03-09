package types

import (
	"encoding/json"
	"errors"
)

var (
	isolatorMap map[string]IsolatorValueConstructor
)

func init() {
	isolatorMap = make(map[string]IsolatorValueConstructor)
}

type IsolatorValueConstructor func() IsolatorValue

func AddIsolatorValueConstructor(i IsolatorValueConstructor) {
	t := i()
	n := t.Name()
	isolatorMap[n] = i
}

type Isolators []Isolator

// GetByName returns the last isolator in the list by the given name.
func (is *Isolators) GetByName(name string) *Isolator {
	var i Isolator
	for j := len(*is) - 1; j >= 0; j-- {
		i = []Isolator(*is)[j]
		if i.Name == name {
			return &i
		}
	}
	return nil
}

type IsolatorValue interface {
	Name() string
	UnmarshalJSON(b []byte) error
	AssertValid() error
}
type Isolator struct {
	Name     string          `json:"name"`
	ValueRaw json.RawMessage `json:"value"`
	value    IsolatorValue
}
type isolator Isolator

func (i *Isolator) Value() IsolatorValue {
	return i.value
}

func (i *Isolator) UnmarshalJSON(b []byte) error {
	var ii isolator
	err := json.Unmarshal(b, &ii)
	if err != nil {
		return err
	}

	var dst IsolatorValue
	con, ok := isolatorMap[ii.Name]
	if !ok {
		return errors.New("unrecognized isolator " + ii.Name)
	}
	dst = con()
	err = dst.UnmarshalJSON(ii.ValueRaw)
	if err != nil {
		return err
	}

	i.value = dst
	i.Name = ii.Name

	return nil
}

func (i *Isolator) assertValid() error {
	if i.value == nil {
		return errors.New("nil Value")
	}
	return i.value.AssertValid()
}
