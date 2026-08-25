package onvif

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestONVIFDeviceProfilesAndURI(t *testing.T) {
	var base string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := make([]byte, 8192)
		n, _ := r.Body.Read(raw)
		body := string(raw[:n])
		w.Header().Set("Content-Type", "application/soap+xml")
		switch {
		case strings.Contains(body, "GetDeviceInformation"):
			fmt.Fprint(w, `<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"><s:Body><GetDeviceInformationResponse><Manufacturer>Acme</Manufacturer><Model>Cam1</Model><FirmwareVersion>1.2</FirmwareVersion><SerialNumber>S1</SerialNumber><HardwareId>H1</HardwareId></GetDeviceInformationResponse></s:Body></s:Envelope>`)
		case strings.Contains(body, "GetCapabilities"):
			fmt.Fprintf(w, `<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"><s:Body><GetCapabilitiesResponse><Capabilities><Media><XAddr>%s/media</XAddr></Media><PTZ><XAddr>%s/ptz</XAddr></PTZ></Capabilities></GetCapabilitiesResponse></s:Body></s:Envelope>`, base, base)
		case strings.Contains(body, "GetProfiles"):
			fmt.Fprint(w, `<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"><s:Body><GetProfilesResponse><Profiles token="main"><Name>Main</Name><VideoEncoderConfiguration><Encoding>H264</Encoding><Resolution><Width>1920</Width><Height>1080</Height></Resolution><RateControl><FrameRateLimit>20</FrameRateLimit></RateControl></VideoEncoderConfiguration><AudioEncoderConfiguration><Encoding>AAC</Encoding><Bitrate>64</Bitrate><SampleRate>48</SampleRate></AudioEncoderConfiguration></Profiles></GetProfilesResponse></s:Body></s:Envelope>`)
		case strings.Contains(body, "GetStreamUri"):
			fmt.Fprint(w, `<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"><s:Body><GetStreamUriResponse><MediaUri><Uri>rtsp://admin:secret@192.168.1.20/live</Uri></MediaUri></GetStreamUriResponse></s:Body></s:Envelope>`)
		case strings.Contains(body, "GetNodes"):
			fmt.Fprint(w, `<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"><s:Body><GetNodesResponse><PTZNode><SupportedPTZSpaces><ContinuousPanTiltVelocitySpace><URI>urn:pan</URI></ContinuousPanTiltVelocitySpace><AbsoluteZoomPositionSpace><URI>urn:zoom</URI></AbsoluteZoomPositionSpace></SupportedPTZSpaces></PTZNode></GetNodesResponse></s:Body></s:Envelope>`)
		default:
			http.Error(w, "unknown", 400)
		}
	}))
	defer srv.Close()
	base = srv.URL
	c := New(base+"/device", "user", "password")
	info, err := c.GetDeviceInformation(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if info.Manufacturer != "Acme" || info.Model != "Cam1" {
		t.Fatalf("info=%+v", info)
	}
	caps, err := c.GetCapabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	ps, err := c.GetProfiles(context.Background(), caps.MediaXAddr)
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 1 || ps[0].Video.Width != 1920 || ps[0].Audio.Encoding != "AAC" {
		t.Fatalf("profiles=%+v", ps)
	}
	uri, err := c.GetStreamURI(context.Background(), caps.MediaXAddr, "main")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(uri, "secret") || uri != "rtsp://192.168.1.20/live" {
		t.Fatalf("uri=%q", uri)
	}
	ptz, err := c.GetPTZInfo(context.Background(), base+"/ptz")
	if err != nil || !ptz.Supported || !ptz.ContinuousPanTilt || !ptz.AbsoluteZoom {
		t.Fatalf("ptz=%+v err=%v", ptz, err)
	}
}
func TestWSSEDoesNotExposePlainPassword(t *testing.T) {
	h, err := wsSecurity("admin", "super-secret")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(h, "super-secret") {
		t.Fatal("password leaked in WSSE header")
	}
	if !strings.Contains(h, "PasswordDigest") {
		t.Fatal("digest type missing")
	}
}
