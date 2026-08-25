package cameras

import (
	"github.com/Homiakus/Home_Sentinel/internal/cameras/onvif"
	"testing"
)

func TestSelectProfiles(t *testing.T) {
	ps := []onvif.Profile{{Token: "low", Video: &onvif.VideoEncoder{Width: 640, Height: 360}}, {Token: "main", Video: &onvif.VideoEncoder{Width: 3840, Height: 2160}}, {Token: "sub", Video: &onvif.VideoEncoder{Width: 1280, Height: 720}}}
	main, detect, ok := selectProfiles(ps)
	if !ok || main.Token != "main" || detect.Token != "sub" {
		t.Fatalf("main=%s detect=%s ok=%v", main.Token, detect.Token, ok)
	}
}
