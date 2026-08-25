//go:build sqlite_cgo

package cameras

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/cameras/rtsp"
	"github.com/Homiakus/Home_Sentinel/internal/database"
	"github.com/Homiakus/Home_Sentinel/internal/media"
	"github.com/Homiakus/Home_Sentinel/internal/repository"
	"github.com/Homiakus/Home_Sentinel/internal/security/netpolicy"
)

func TestOnboardRTSPPersistsCamera(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, database.Options{Path: filepath.Join(t.TempDir(), "cam.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := database.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	guard, _ := netpolicy.New([]string{"192.168.30.0/24"})
	svc := Service{Store: repository.NewStore[Camera](db, repository.KindCamera), Network: guard, RTSPProbe: func(_ context.Context, u string, _ time.Duration) (rtsp.Result, error) {
		return rtsp.Result{Reachable: true, Media: media.ProbeResult{Video: []media.VideoStream{{Codec: "h264", Width: 1920, Height: 1080, FPS: 20}}, Audio: []media.AudioStream{{Codec: "aac"}}, ProbeLatency: 25 * time.Millisecond}}, nil
	}}
	cam, err := svc.OnboardRTSP(ctx, RTSPOnboardRequest{Name: "Front", URL: "rtsp://192.168.30.20/live"})
	if err != nil {
		t.Fatal(err)
	}
	if cam.ID == "" || cam.Streams[0].Width != 1920 || !cam.Capabilities.Audio {
		t.Fatalf("cam=%+v", cam)
	}
	list, err := svc.List(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != cam.ID {
		t.Fatalf("list=%+v", list)
	}
}
