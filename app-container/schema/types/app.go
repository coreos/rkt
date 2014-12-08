package types

import (
	"encoding/json"
	"errors"
)

type App struct {
	Exec          []string          `json:"exec"`
	EventHandlers []EventHandler    `json:"eventHandlers"`
	User          string            `json:"user"`
	Group         string            `json:"group"`
	Environment   map[string]string `json:"environment"`
	MountPoints   []MountPoint      `json:"mountPoints"`
	Ports         []Port            `json:"ports"`
	Isolators     []Isolator        `json:"isolators"`
}

// app is a model to facilitate extra validation during the
// unmarshalling of the App
type app App

func (a *App) UnmarshalJSON(data []byte) error {
	ja := app{}
	err := json.Unmarshal(data, &ja)
	if err != nil {
		return err
	}
	na := App(ja)
	if err := na.assertValid(); err != nil {
		return err
	}
	if na.Environment == nil {
		na.Environment = make(map[string]string)
	}
	*a = na
	return nil
}

func (a App) MarshalJSON() ([]byte, error) {
	if err := a.assertValid(); err != nil {
		return nil, err
	}
	return json.Marshal(app(a))
}

func (a *App) assertValid() error {
	if len(a.Exec) < 1 {
		return errors.New(`Exec cannot be empty`)
	}
	return nil
}
