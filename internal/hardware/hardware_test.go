package hardware

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeRunner map[string]string

func (f fakeRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	key := name + " " + strings.Join(args, " ")
	v, ok := f[key]
	if !ok {
		return nil, errors.New("not found")
	}
	return []byte(v), nil
}
func TestVideoProbe(t *testing.T) {
	r := fakeRunner{"nvidia-smi --query-gpu=name,memory.total,driver_version --format=csv,noheader,nounits": "RTX Test, 8192, 999.1\n"}
	v := ProbeVideo(context.Background(), r)
	if len(v.NVIDIA) != 1 || v.NVIDIA[0].VRAMMiB != 8192 {
		t.Fatalf("video=%+v", v)
	}
}
func TestRecommend(t *testing.T) {
	p := Profile{Memory: MemoryInfo{Total: 32 << 30}, Video: VideoInfo{NVIDIA: []NVIDIAGPU{{Name: "GPU", VRAMMiB: 12288}}}}
	r := Recommend(p)
	if r.AIProfile != "HIGH" || r.VideoDecoder != "nvidia" || r.Detector != "nvidia" {
		t.Fatalf("recommend=%+v", r)
	}
}
func TestDiscoveryCIDRs(t *testing.T) {
	got := DiscoveryCIDRs([]InterfaceInfo{{Name: "eth0", Up: true, Addresses: []string{"192.168.30.15/24"}}, {Name: "docker0", Up: true, ContainerLike: true, Addresses: []string{"172.17.0.1/16"}}, {Name: "lo", Up: true, Loopback: true, Addresses: []string{"127.0.0.1/8"}}})
	if len(got) != 1 || got[0] != "192.168.30.0/24" {
		t.Fatalf("cidrs=%v", got)
	}
}
func TestSMARTJSON(t *testing.T) {
	r := fakeRunner{"smartctl -j -H -A /dev/sda": `{"smart_status":{"passed":true},"temperature":{"current":31}}`}
	s := ProbeSMART(context.Background(), r, "/dev/sda")
	if !s.Available || s.Passed == nil || !*s.Passed || s.TemperatureC == nil || *s.TemperatureC != 31 {
		t.Fatalf("smart=%+v", s)
	}
}
func TestHostProbesDoNotPanic(t *testing.T) {
	if ProbeCPU().LogicalCores < 1 {
		t.Fatal("no cores")
	}
	_ = ProbeOS()
	_ = ProbeMemory()
	_ = ProbeStorage()
	_ = ProbeNetwork()
}
