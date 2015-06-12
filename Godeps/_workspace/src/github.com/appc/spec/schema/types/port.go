package types

import (
	"encoding/json"
	"errors"
)

type Port struct {
	Name            ACName `json:"name"`
	Protocol        string `json:"protocol"`
	Port            uint   `json:"port"`
	Count           uint   `json:"count"`
	SocketActivated bool   `json:"socketActivated"`
}

type ExposedPort struct {
	Name     ACName `json:"name"`
	HostPort uint   `json:"hostPort"`
}

type port Port

func (p *Port) UnmarshalJSON(data []byte) error {
	var pp port
	if err := json.Unmarshal(data, &pp); err != nil {
		return err
	}
	np := Port(pp)
	if err := np.assertValid(); err != nil {
		return err
	}
	if np.Count == 0 {
		np.Count = 1
	}
	*p = np
	return nil
}

func (p Port) MarshalJSON() ([]byte, error) {
	if err := p.assertValid(); err != nil {
		return nil, err
	}
	return json.Marshal(port(p))
}

func (p Port) assertValid() error {
	// Although there are no guarantees, most (if not all)
	// transport protocols use 16 bit ports
	if p.Port > 65535 || p.Port < 1 {
		return errors.New("port must be in 1-65535 range")
	}
	if p.Port+p.Count > 65536 {
		return errors.New("end of port range must be in 1-65535 range")
	}
	return nil
}
