package hardware

import "time"

type Profile struct {
	CollectedAt time.Time       `json:"collected_at"`
	OS          OSInfo          `json:"os"`
	CPU         CPUInfo         `json:"cpu"`
	Memory      MemoryInfo      `json:"memory"`
	Video       VideoInfo       `json:"video"`
	Storage     []MountInfo     `json:"storage"`
	Network     []InterfaceInfo `json:"network"`
}
type OSInfo struct {
	ID                string `json:"id"`
	Version           string `json:"version"`
	PrettyName        string `json:"pretty_name"`
	Kernel            string `json:"kernel"`
	Arch              string `json:"arch"`
	Container         bool   `json:"container"`
	CgroupMemoryLimit uint64 `json:"cgroup_memory_limit_bytes,omitempty"`
}
type CPUInfo struct {
	Model        string          `json:"model"`
	LogicalCores int             `json:"logical_cores"`
	Flags        map[string]bool `json:"flags"`
}
type MemoryInfo struct {
	Total     uint64 `json:"total_bytes"`
	Available uint64 `json:"available_bytes"`
}
type VideoInfo struct {
	DRIDevices []string    `json:"dri_devices,omitempty"`
	VAAPI      Capability  `json:"vaapi"`
	NVIDIA     []NVIDIAGPU `json:"nvidia,omitempty"`
}
type Capability struct {
	Available bool     `json:"available"`
	Reason    string   `json:"reason,omitempty"`
	Details   []string `json:"details,omitempty"`
}
type NVIDIAGPU struct {
	Name    string `json:"name"`
	VRAMMiB int    `json:"vram_mib"`
	Driver  string `json:"driver"`
}
type MountInfo struct {
	Device     string `json:"device"`
	MountPoint string `json:"mount_point"`
	Filesystem string `json:"filesystem"`
	Total      uint64 `json:"total_bytes"`
	Free       uint64 `json:"free_bytes"`
	Rotational *bool  `json:"rotational,omitempty"`
}
type InterfaceInfo struct {
	Name          string   `json:"name"`
	Addresses     []string `json:"addresses"`
	Up            bool     `json:"up"`
	Loopback      bool     `json:"loopback"`
	ContainerLike bool     `json:"container_like"`
}
