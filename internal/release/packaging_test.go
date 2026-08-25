package release

import (
	"os"
	"strings"
	"testing"
)

func TestRejectsDangerousCompose(t *testing.T) {
	f := ValidateCompose(strings.NewReader("services:\n x:\n  image: x:latest\n  volumes:\n   - /var/run/docker.sock:/var/run/docker.sock\n  ports:\n   - 1883:1883\n"))
	if len(f) < 3 {
		t.Fatalf("expected findings %+v", f)
	}
}
func TestAcceptsRequiredPinnedEnvSkeleton(t *testing.T) {
	f := ValidateCompose(strings.NewReader("services:\n x:\n  image: ${X_IMAGE:?set X_IMAGE}\n  ports:\n   - 127.0.0.1:8080:8080\n"))
	if len(f) != 0 {
		t.Fatalf("unexpected %+v", f)
	}
}

func TestProductionComposePassesValidator(t *testing.T) {
	f, err := os.Open("../../deploy/compose/compose.prod.yml")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if findings := ValidateCompose(f); len(findings) != 0 {
		t.Fatalf("production compose findings: %+v", findings)
	}
}

func TestReleaseDockerfileRequiresImmutableInputs(t *testing.T) {
	f, err := os.Open("../../deploy/compose/Dockerfile.release")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if findings := ValidateReleaseDockerfile(f); len(findings) != 0 {
		t.Fatalf("release Dockerfile findings: %+v", findings)
	}
}
