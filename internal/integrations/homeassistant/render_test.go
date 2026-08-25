package homeassistant

import (
	"strings"
	"testing"
)

func TestGeneratedAssetsDoNotTouchStorage(t *testing.T) {
	b, err := RenderDashboard([]string{"camera.front_door"})
	if err != nil {
		t.Fatal(err)
	}
	all := string(b) + string(RenderPackage())
	if strings.Contains(all, ".storage") {
		t.Fatal("generated asset references .storage")
	}
	if !strings.Contains(all, "camera.front_door") {
		t.Fatal("camera missing")
	}
}
