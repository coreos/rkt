package types

import (
	"encoding/json"
)

type CAF struct {
	ACVersion SemVer    `json:"acVersion"`
	ACKind    ACKind    `json:"acKind"`
	Files     map[string]File `json:"files"`
}

type caf CAF

func (c CAF) MarshalJSON() ([]byte, error) {
	return json.Marshal(caf(c))
}

func (c *CAF) UnmarshalJSON(data []byte) error {
	var caf CAF
	if err := json.Unmarshal(data, &caf); err != nil {
		return err
	}
	*c = caf
	return nil
}
