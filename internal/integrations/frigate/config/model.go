package config

type Managed struct {
	MQTT      *MQTT             `json:"mqtt,omitempty"`
	FFmpeg    *GlobalFFmpeg     `json:"ffmpeg,omitempty"`
	Go2RTC    Go2RTC            `json:"go2rtc"`
	Record    *Record           `json:"record,omitempty"`
	Snapshots *Snapshots        `json:"snapshots,omitempty"`
	Cameras   map[string]Camera `json:"cameras"`
}
type MQTT struct {
	Enabled     *bool  `json:"enabled,omitempty"`
	Host        string `json:"host,omitempty"`
	User        string `json:"user,omitempty"`
	Password    string `json:"password,omitempty"`
	TopicPrefix string `json:"topic_prefix,omitempty"`
}
type GlobalFFmpeg struct {
	HWAccelArgs string `json:"hwaccel_args,omitempty"`
}
type Go2RTC struct {
	Streams map[string][]string `json:"streams"`
	WebRTC  *WebRTC             `json:"webrtc,omitempty"`
}
type WebRTC struct {
	Candidates []string `json:"candidates,omitempty"`
}
type Camera struct {
	Enabled   *bool           `json:"enabled,omitempty"`
	FFmpeg    CameraFFmpeg    `json:"ffmpeg"`
	Detect    Detect          `json:"detect"`
	Record    *Record         `json:"record,omitempty"`
	Snapshots *Snapshots      `json:"snapshots,omitempty"`
	Objects   *Objects        `json:"objects,omitempty"`
	Zones     map[string]Zone `json:"zones,omitempty"`
}
type CameraFFmpeg struct {
	Inputs     []Input     `json:"inputs"`
	OutputArgs *OutputArgs `json:"output_args,omitempty"`
}
type Input struct {
	Path      string   `json:"path"`
	InputArgs string   `json:"input_args,omitempty"`
	Roles     []string `json:"roles"`
}
type OutputArgs struct {
	Record string `json:"record,omitempty"`
}
type Detect struct {
	Enabled *bool   `json:"enabled,omitempty"`
	Width   int     `json:"width,omitempty"`
	Height  int     `json:"height,omitempty"`
	FPS     float64 `json:"fps,omitempty"`
}
type Record struct {
	Enabled    *bool        `json:"enabled,omitempty"`
	Continuous *RetainDays  `json:"continuous,omitempty"`
	Motion     *RetainDays  `json:"motion,omitempty"`
	Alerts     *RetainBlock `json:"alerts,omitempty"`
	Detections *RetainBlock `json:"detections,omitempty"`
}
type RetainDays struct {
	Days float64 `json:"days"`
}
type RetainBlock struct {
	Retain Retain `json:"retain"`
}
type Retain struct {
	Days float64 `json:"days"`
	Mode string  `json:"mode,omitempty"`
}
type Snapshots struct {
	Enabled *bool           `json:"enabled,omitempty"`
	Retain  *SnapshotRetain `json:"retain,omitempty"`
}
type SnapshotRetain struct {
	Default float64 `json:"default"`
}
type Objects struct {
	Track []string `json:"track,omitempty"`
}
type Zone struct {
	Coordinates string   `json:"coordinates"`
	Objects     []string `json:"objects,omitempty"`
}

type Ownership struct {
	CameraNames        []string
	StreamNames        []string
	ManageMQTT         bool
	ManageGlobalFFmpeg bool
	ManageRecord       bool
	ManageSnapshots    bool
	ManageGo2RTCWebRTC bool
}

func Bool(v bool) *bool { return &v }
