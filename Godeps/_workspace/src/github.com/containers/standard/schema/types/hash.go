package types

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Hash encodes a hash specified in a string of the form:
//    "<type>:<value>"
// for example
//    "sha1-06c733b1838136838e6d2d3e8fa5aea4c7905e92"
// Valid types are currently "md5" and "sha1"
type Hash struct {
	typ string
	Val string
}

func NewHash(s string) (*Hash, error) {
	elems := strings.Split(s, "-")
	if len(elems) != 2 {
		return nil, errors.New("badly formatted hash string")
	}
	nh := Hash{
		typ: elems[0],
		Val: elems[1],
	}
	if err := nh.assertValid(); err != nil {
		return nil, err
	}
	return &nh, nil
}

func (h Hash) String() string {
	return fmt.Sprintf("%s-%s", h.typ, h.Val)
}

func (h Hash) assertValid() error {
	switch h.typ {
	case "sha1", "md5":
	case "":
		return fmt.Errorf("unexpected empty hash type")
	default:
		return fmt.Errorf("unrecognized hash type: %v", h.typ)
	}
	if h.Val == "" {
		return fmt.Errorf("unexpected empty hash value")
	}
	return nil
}

func (h *Hash) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	nh, err := NewHash(s)
	if err != nil {
		return err
	}
	*h = *nh
	return nil
}

func (h Hash) MarshalJSON() ([]byte, error) {
	if err := h.assertValid(); err != nil {
		return nil, err
	}
	return json.Marshal(h.String())
}
