package conformance

import "testing"

func TestManifest(t *testing.T) {
	manifest, err := Load("../../config/events.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate(manifest); err != nil {
		t.Fatal(err)
	}
}
