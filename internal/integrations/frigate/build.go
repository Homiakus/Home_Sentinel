package frigate

import (
	"context"
	"errors"
	"sort"

	"github.com/Homiakus/Home_Sentinel/internal/cameras"
	"github.com/Homiakus/Home_Sentinel/internal/hardware"
	fgconfig "github.com/Homiakus/Home_Sentinel/internal/integrations/frigate/config"
	"github.com/Homiakus/Home_Sentinel/internal/integrations/go2rtc"
)

type BuildResult struct {
	Managed   fgconfig.Managed
	Ownership fgconfig.Ownership
	SecretEnv map[string]string `json:"-"`
}

func BuildManaged(_ context.Context, cams []cameras.Camera, hw hardware.Recommendation, resolver go2rtc.SecretResolver, policy func(cameras.Camera) CameraPolicy) (BuildResult, error) {
	out := BuildResult{Managed: fgconfig.Managed{Go2RTC: fgconfig.Go2RTC{Streams: map[string][]string{}}, Cameras: map[string]fgconfig.Camera{}}, SecretEnv: map[string]string{}}
	out.Managed.FFmpeg = HardwareFFmpeg(hw)
	out.Ownership.ManageGlobalFFmpeg = out.Managed.FFmpeg != nil
	sort.Slice(cams, func(i, j int) bool { return cams[i].ID < cams[j].ID })
	for _, cam := range cams {
		m, err := MapCamera(cam, hw, resolver)
		if err != nil {
			return BuildResult{}, err
		}
		p := DefaultCameraPolicy()
		if policy != nil {
			p = policy(cam)
		}
		if err := ApplyPolicy(&m.Config, p); err != nil {
			return BuildResult{}, err
		}
		if _, exists := out.Managed.Cameras[m.Name]; exists {
			return BuildResult{}, errors.New("duplicate canonical Frigate camera name")
		}
		out.Managed.Cameras[m.Name] = m.Config
		out.Ownership.CameraNames = append(out.Ownership.CameraNames, m.Name)
		for n, s := range m.Go2RTC.Streams {
			out.Managed.Go2RTC.Streams[n] = s
			out.Ownership.StreamNames = append(out.Ownership.StreamNames, n)
		}
		for k, v := range m.Go2RTC.SecretEnv {
			out.SecretEnv[k] = v
		}
	}
	sort.Strings(out.Ownership.CameraNames)
	sort.Strings(out.Ownership.StreamNames)
	return out, nil
}
