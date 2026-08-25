package onvif

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	HTTP      *http.Client
	DeviceURL string
	Username  string
	Password  string
}

func New(deviceURL, username, password string) *Client {
	return &Client{HTTP: &http.Client{Timeout: 8 * time.Second}, DeviceURL: deviceURL, Username: username, Password: password}
}

type DeviceInformation struct {
	Manufacturer    string `json:"manufacturer"`
	Model           string `json:"model"`
	FirmwareVersion string `json:"firmware"`
	SerialNumber    string `json:"serial_number"`
	HardwareID      string `json:"hardware_id"`
}
type Capabilities struct {
	MediaXAddr string `json:"media_xaddr"`
	PTZXAddr   string `json:"ptz_xaddr,omitempty"`
}
type Profile struct {
	Token string        `json:"token"`
	Name  string        `json:"name"`
	Video *VideoEncoder `json:"video,omitempty"`
	Audio *AudioEncoder `json:"audio,omitempty"`
}
type VideoEncoder struct {
	Encoding string  `json:"encoding"`
	Width    int     `json:"width"`
	Height   int     `json:"height"`
	FPS      float64 `json:"fps"`
}
type AudioEncoder struct {
	Encoding   string `json:"encoding"`
	SampleRate int    `json:"sample_rate,omitempty"`
	Bitrate    int    `json:"bitrate,omitempty"`
}

func (c *Client) GetDeviceInformation(ctx context.Context) (DeviceInformation, error) {
	body := `<tds:GetDeviceInformation xmlns:tds="http://www.onvif.org/ver10/device/wsdl"/>`
	raw, err := c.call(ctx, c.DeviceURL, "http://www.onvif.org/ver10/device/wsdl/GetDeviceInformation", body)
	if err != nil {
		return DeviceInformation{}, err
	}
	var env struct {
		Body struct {
			Response struct {
				Manufacturer    string `xml:"Manufacturer"`
				Model           string `xml:"Model"`
				FirmwareVersion string `xml:"FirmwareVersion"`
				SerialNumber    string `xml:"SerialNumber"`
				HardwareID      string `xml:"HardwareId"`
			} `xml:"GetDeviceInformationResponse"`
		} `xml:"Body"`
	}
	if err := xml.Unmarshal(raw, &env); err != nil {
		return DeviceInformation{}, err
	}
	r := env.Body.Response
	return DeviceInformation{Manufacturer: r.Manufacturer, Model: r.Model, FirmwareVersion: r.FirmwareVersion, SerialNumber: r.SerialNumber, HardwareID: r.HardwareID}, nil
}
func (c *Client) GetCapabilities(ctx context.Context) (Capabilities, error) {
	body := `<tds:GetCapabilities xmlns:tds="http://www.onvif.org/ver10/device/wsdl"><tds:Category>All</tds:Category></tds:GetCapabilities>`
	raw, err := c.call(ctx, c.DeviceURL, "http://www.onvif.org/ver10/device/wsdl/GetCapabilities", body)
	if err != nil {
		return Capabilities{}, err
	}
	var env struct {
		Body struct {
			Response struct {
				Caps struct {
					Media struct {
						XAddr string `xml:"XAddr"`
					} `xml:"Media"`
					PTZ struct {
						XAddr string `xml:"XAddr"`
					} `xml:"PTZ"`
				} `xml:"Capabilities"`
			} `xml:"GetCapabilitiesResponse"`
		} `xml:"Body"`
	}
	if err := xml.Unmarshal(raw, &env); err != nil {
		return Capabilities{}, err
	}
	caps := Capabilities{MediaXAddr: strings.TrimSpace(env.Body.Response.Caps.Media.XAddr), PTZXAddr: strings.TrimSpace(env.Body.Response.Caps.PTZ.XAddr)}
	if caps.MediaXAddr == "" {
		return Capabilities{}, errors.New("ONVIF media XAddr missing")
	}
	if err := validateHTTPURL(caps.MediaXAddr); err != nil {
		return Capabilities{}, fmt.Errorf("invalid ONVIF media XAddr: %w", err)
	}
	if caps.PTZXAddr != "" {
		if err := validateHTTPURL(caps.PTZXAddr); err != nil {
			caps.PTZXAddr = ""
		}
	}
	return caps, nil
}
func (c *Client) GetProfiles(ctx context.Context, mediaURL string) ([]Profile, error) {
	body := `<trt:GetProfiles xmlns:trt="http://www.onvif.org/ver10/media/wsdl"/>`
	raw, err := c.call(ctx, mediaURL, "http://www.onvif.org/ver10/media/wsdl/GetProfiles", body)
	if err != nil {
		return nil, err
	}
	var env struct {
		Body struct {
			Response struct {
				Profiles []struct {
					Token string `xml:"token,attr"`
					Name  string `xml:"Name"`
					Video *struct {
						Encoding   string `xml:"Encoding"`
						Resolution struct {
							Width  int `xml:"Width"`
							Height int `xml:"Height"`
						} `xml:"Resolution"`
						Rate struct {
							FPS float64 `xml:"FrameRateLimit"`
						} `xml:"RateControl"`
					} `xml:"VideoEncoderConfiguration"`
					Audio *struct {
						Encoding   string `xml:"Encoding"`
						Bitrate    int    `xml:"Bitrate"`
						SampleRate int    `xml:"SampleRate"`
					} `xml:"AudioEncoderConfiguration"`
				} `xml:"Profiles"`
			} `xml:"GetProfilesResponse"`
		} `xml:"Body"`
	}
	if err := xml.Unmarshal(raw, &env); err != nil {
		return nil, err
	}
	out := make([]Profile, 0, len(env.Body.Response.Profiles))
	for _, p := range env.Body.Response.Profiles {
		x := Profile{Token: p.Token, Name: p.Name}
		if p.Video != nil {
			x.Video = &VideoEncoder{Encoding: p.Video.Encoding, Width: p.Video.Resolution.Width, Height: p.Video.Resolution.Height, FPS: p.Video.Rate.FPS}
		}
		if p.Audio != nil {
			x.Audio = &AudioEncoder{Encoding: p.Audio.Encoding, Bitrate: p.Audio.Bitrate, SampleRate: p.Audio.SampleRate}
		}
		if x.Token != "" {
			out = append(out, x)
		}
	}
	return out, nil
}
func (c *Client) GetStreamURI(ctx context.Context, mediaURL, profileToken string) (string, error) {
	body := fmt.Sprintf(`<trt:GetStreamUri xmlns:trt="http://www.onvif.org/ver10/media/wsdl" xmlns:tt="http://www.onvif.org/ver10/schema"><trt:StreamSetup><tt:Stream>RTP-Unicast</tt:Stream><tt:Transport><tt:Protocol>RTSP</tt:Protocol></tt:Transport></trt:StreamSetup><trt:ProfileToken>%s</trt:ProfileToken></trt:GetStreamUri>`, xmlEscape(profileToken))
	raw, err := c.call(ctx, mediaURL, "http://www.onvif.org/ver10/media/wsdl/GetStreamUri", body)
	if err != nil {
		return "", err
	}
	var env struct {
		Body struct {
			Response struct {
				MediaURI struct {
					URI string `xml:"Uri"`
				} `xml:"MediaUri"`
			} `xml:"GetStreamUriResponse"`
		} `xml:"Body"`
	}
	if err := xml.Unmarshal(raw, &env); err != nil {
		return "", err
	}
	uri := strings.TrimSpace(env.Body.Response.MediaURI.URI)
	u, err := url.Parse(uri)
	if err != nil || u.Scheme != "rtsp" {
		return "", errors.New("ONVIF returned invalid RTSP URI")
	}
	u.User = nil
	return u.String(), nil
}
func (c *Client) GetSnapshotURI(ctx context.Context, mediaURL, profileToken string) (string, error) {
	body := fmt.Sprintf(`<trt:GetSnapshotUri xmlns:trt="http://www.onvif.org/ver10/media/wsdl"><trt:ProfileToken>%s</trt:ProfileToken></trt:GetSnapshotUri>`, xmlEscape(profileToken))
	raw, err := c.call(ctx, mediaURL, "http://www.onvif.org/ver10/media/wsdl/GetSnapshotUri", body)
	if err != nil {
		return "", err
	}
	var env struct {
		Body struct {
			Response struct {
				MediaURI struct {
					URI string `xml:"Uri"`
				} `xml:"MediaUri"`
			} `xml:"GetSnapshotUriResponse"`
		} `xml:"Body"`
	}
	if err := xml.Unmarshal(raw, &env); err != nil {
		return "", err
	}
	uri := strings.TrimSpace(env.Body.Response.MediaURI.URI)
	if err := validateHTTPURL(uri); err != nil {
		return "", err
	}
	u, _ := url.Parse(uri)
	u.User = nil
	return u.String(), nil
}

func (c *Client) call(ctx context.Context, endpoint, action, body string) ([]byte, error) {
	if err := validateHTTPURL(endpoint); err != nil {
		return nil, err
	}
	security, err := wsSecurity(c.Username, c.Password)
	if err != nil {
		return nil, err
	}
	envelope := `<?xml version="1.0" encoding="UTF-8"?><s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"><s:Header>` + security + `</s:Header><s:Body>` + body + `</s:Body></s:Envelope>`
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewBufferString(envelope))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", `application/soap+xml; charset=utf-8; action="`+action+`"`)
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("ONVIF HTTP status %d", resp.StatusCode)
	}
	if bytes.Contains(raw, []byte("<Fault")) || bytes.Contains(raw, []byte(":Fault")) {
		return nil, errors.New("ONVIF SOAP fault")
	}
	return raw, nil
}
func wsSecurity(username, password string) (string, error) {
	if username == "" {
		return "", nil
	}
	nonce := make([]byte, 20)
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	created := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	h := sha1.New()
	h.Write(nonce)
	h.Write([]byte(created))
	h.Write([]byte(password))
	digest := base64.StdEncoding.EncodeToString(h.Sum(nil))
	nonce64 := base64.StdEncoding.EncodeToString(nonce)
	return fmt.Sprintf(`<wsse:Security s:mustUnderstand="1" xmlns:wsse="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-secext-1.0.xsd" xmlns:wsu="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-utility-1.0.xsd"><wsse:UsernameToken><wsse:Username>%s</wsse:Username><wsse:Password Type="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-username-token-profile-1.0#PasswordDigest">%s</wsse:Password><wsse:Nonce EncodingType="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-soap-message-security-1.0#Base64Binary">%s</wsse:Nonce><wsu:Created>%s</wsu:Created></wsse:UsernameToken></wsse:Security>`, xmlEscape(username), digest, nonce64, created), nil
}
func validateHTTPURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("HTTP(S) URL required")
	}
	if u.Hostname() == "" {
		return errors.New("URL host required")
	}
	return nil
}
func xmlEscape(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

type PTZInfo struct {
	Supported         bool `json:"supported"`
	ContinuousPanTilt bool `json:"continuous_pan_tilt"`
	ContinuousZoom    bool `json:"continuous_zoom"`
	AbsolutePanTilt   bool `json:"absolute_pan_tilt"`
	AbsoluteZoom      bool `json:"absolute_zoom"`
}

// GetPTZInfo reads the PTZ node advertised by the camera and derives which
// movement spaces are actually present instead of assuming that an XAddr means
// every PTZ operation is supported.
func (c *Client) GetPTZInfo(ctx context.Context, ptzURL string) (PTZInfo, error) {
	if strings.TrimSpace(ptzURL) == "" {
		return PTZInfo{}, errors.New("PTZ XAddr missing")
	}
	body := `<tptz:GetNodes xmlns:tptz="http://www.onvif.org/ver20/ptz/wsdl"/>`
	raw, err := c.call(ctx, ptzURL, "http://www.onvif.org/ver20/ptz/wsdl/GetNodes", body)
	if err != nil {
		return PTZInfo{}, err
	}
	var env struct {
		Body struct {
			Response struct {
				Nodes []struct {
					Spaces struct {
						ContinuousPanTilt []struct {
							URI string `xml:"URI"`
						} `xml:"ContinuousPanTiltVelocitySpace"`
						ContinuousZoom []struct {
							URI string `xml:"URI"`
						} `xml:"ContinuousZoomVelocitySpace"`
						AbsolutePanTilt []struct {
							URI string `xml:"URI"`
						} `xml:"AbsolutePanTiltPositionSpace"`
						AbsoluteZoom []struct {
							URI string `xml:"URI"`
						} `xml:"AbsoluteZoomPositionSpace"`
					} `xml:"SupportedPTZSpaces"`
				} `xml:"PTZNode"`
			} `xml:"GetNodesResponse"`
		} `xml:"Body"`
	}
	if err := xml.Unmarshal(raw, &env); err != nil {
		return PTZInfo{}, err
	}
	info := PTZInfo{}
	for _, n := range env.Body.Response.Nodes {
		info.ContinuousPanTilt = info.ContinuousPanTilt || len(n.Spaces.ContinuousPanTilt) > 0
		info.ContinuousZoom = info.ContinuousZoom || len(n.Spaces.ContinuousZoom) > 0
		info.AbsolutePanTilt = info.AbsolutePanTilt || len(n.Spaces.AbsolutePanTilt) > 0
		info.AbsoluteZoom = info.AbsoluteZoom || len(n.Spaces.AbsoluteZoom) > 0
	}
	info.Supported = info.ContinuousPanTilt || info.ContinuousZoom || info.AbsolutePanTilt || info.AbsoluteZoom
	return info, nil
}
