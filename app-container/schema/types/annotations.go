package types

import "encoding/json"

// TODO(jonboulle): this is awkward since it's inconsistent with the way we do
// things elsewhere (i.e. using strict types instead of string types), but it's
// tricky because Annotations needs to be able to catch arbitrary key-values.
// Clean this up somehow?
type Annotations map[ACName]string

type annotations Annotations

func (a Annotations) assertValid() error {
	if c, ok := a["created"]; ok {
		if _, err := NewDate(c); err != nil {
			return err
		}
	}
	if h, ok := a["homepage"]; ok {
		if _, err := NewURL(h); err != nil {
			return err
		}
	}
	if d, ok := a["documentation"]; ok {
		if _, err := NewURL(d); err != nil {
			return err
		}
	}

	return nil
}

func (a Annotations) MarshalJSON() ([]byte, error) {
	if err := a.assertValid(); err != nil {
		return nil, err
	}
	return json.Marshal(annotations(a))
}

func (a *Annotations) UnmarshalJSON(data []byte) error {
	var ja annotations
	if err := json.Unmarshal(data, &ja); err != nil {
		return err
	}
	na := Annotations(ja)
	if err := a.assertValid(); err != nil {
		return err
	}
	*a = na
	return nil
}
