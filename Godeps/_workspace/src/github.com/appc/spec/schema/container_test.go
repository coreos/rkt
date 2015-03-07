package schema

import "testing"

func TestContainerRuntimeManifestMerge(t *testing.T) {
	cmj := `{}`
	cm := &ContainerRuntimeManifest{}

	if cm.UnmarshalJSON([]byte(cmj)) == nil {
		t.Fatal("Manifest JSON without acKind and acVersion unmarshalled successfully")
	}

	cm = BlankContainerRuntimeManifest()

	err := cm.UnmarshalJSON([]byte(cmj))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
