package go2rtc

import (
	"github.com/Homiakus/Home_Sentinel/internal/cameras"
	"github.com/Homiakus/Home_Sentinel/internal/secrets"
	"strings"
	"testing"
)

type fakeSecrets map[secrets.Ref][]byte

func (f fakeSecrets) Resolve(r secrets.Ref) ([]byte, error) { return f[r], nil }
func TestGenerateDoesNotLeakComplexPassword(t *testing.T) {
	ref, _ := secrets.ParseRef("secret://env/CAM_PASS")
	cam := cameras.Camera{ID: "cam_front-01", Name: "Front", Type: cameras.TypeRTSP, Capabilities: cameras.Capabilities{Talk: true}, Streams: []cameras.Stream{{ID: "s1", Role: cameras.RoleMain, Endpoint: cameras.Endpoint{URL: "rtsp://192.168.1.2/live?x=1", Username: "user@example", PasswordRef: ref}}}}
	g, err := Generate(cam, fakeSecrets{ref: []byte("p@ss:%word")})
	if err != nil {
		t.Fatal(err)
	}
	src := g.Streams["cam_front-01"][0]
	if strings.Contains(src, "p@ss") || strings.Contains(src, "%word") {
		t.Fatalf("secret leaked: %s", src)
	}
	if !strings.Contains(src, "{FRIGATE_SENTINEL_CAM_FRONT_01_MAIN_PASSWORD}") {
		t.Fatalf("missing placeholder: %s", src)
	}
	if got := g.SecretEnv["FRIGATE_SENTINEL_CAM_FRONT_01_MAIN_PASSWORD"]; got != "p%40ss%3A%25word" {
		t.Fatalf("encoded=%q", got)
	}
	if !strings.HasSuffix(src, "#backchannel=0") {
		t.Fatalf("talk stream should disable backchannel for NVR: %s", src)
	}
}
func TestCanonicalNamesStable(t *testing.T) {
	if CanonicalStreamName("cam_ABC", cameras.RoleDetect) != "cam_abc_sub" {
		t.Fatal("bad detect name")
	}
}
