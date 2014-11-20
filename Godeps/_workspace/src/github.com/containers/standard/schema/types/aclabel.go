package types

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	valchars = `abcdefghijklmnopqrstuvwxyz0123456789.-/`
)

// ACLabel (an App-Container Label) is a format used by keys in different
// formats of the App Container Standard. An ACLabel is restricted to
// characters accepted by the DNS RFC[1] and "/". ACLabels are
// case-insensitive for comparison purposes, but case-preserving.
//
// [1] http://tools.ietf.org/html/rfc1123#page-13
type ACLabel string

func (l ACLabel) String() string {
	return string(l)
}

// Equals checks whether a given ACLabel is equal to this one.
func (l ACLabel) Equals(o ACLabel) bool {
	return strings.ToLower(string(l)) == strings.ToLower(string(o))
}

// NewACLabel generates a new ACLabel from a string. If the given string is
// not a valid ACLabel, nil and an error are returned.
func NewACLabel(s string) (*ACLabel, error) {
	for _, c := range s {
		if !strings.ContainsRune(valchars, c) {
			msg := fmt.Sprintf("invalid char in ACLabel: %c", c)
			return nil, ACLabelError(msg)
		}
	}
	return (*ACLabel)(&s), nil
}

// UnmarshalJSON implements the json.Unmarshaler interface
func (l *ACLabel) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	nl, err := NewACLabel(s)
	if err != nil {
		return err
	}
	*l = *nl
	return nil
}

// MarshalJSON implements the json.Marshaler interface
func (l *ACLabel) MarshalJSON() ([]byte, error) {
	return json.Marshal(l.String())
}
