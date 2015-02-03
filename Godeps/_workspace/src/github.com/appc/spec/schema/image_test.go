package schema

import "testing"

func TestEmptyApp(t *testing.T) {
	imj := `
		{
		    "acKind": "ImageManifest",
		    "acVersion": "0.2.0",
		    "name": "example.com/test"
		}
		`

	var im ImageManifest

	err := im.UnmarshalJSON([]byte(imj))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Marshal and Unmarshal to verify that no "app": {} is generated on
	// Marshal and converted to empty struct on Unmarshal
	buf, err := im.MarshalJSON()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	err = im.UnmarshalJSON(buf)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
