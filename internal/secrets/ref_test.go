package secrets

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRefContainsNoSecretValue(t *testing.T) {
	r, err := ParseRef("secret://env/CAMERA_FRONT_PASSWORD")
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "hunter2") {
		t.Fatal("unexpected secret material")
	}
}
