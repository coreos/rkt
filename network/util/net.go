package util

type Net struct {
	Filename string
	Name     string `json:"name,omitempty"`
	Type     string `json:"type,omitempty"`
	IPAlloc  struct {
		Type   string `json:"type,omitempty"`
		Subnet string `json:"subnet,omitempty"`
	}
}
