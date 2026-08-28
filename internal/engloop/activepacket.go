package engloop

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const ActiveWorkPacketPath = "docs/engineering/ACTIVE_WORK_PACKET.json"

var commitSHA40 = regexp.MustCompile(`^[0-9a-fA-F]{40}$`)

type ActiveWorkPacket struct {
	Packet       string `json:"packet"`
	MutationBase string `json:"mutation_base"`
	MaxCommits   int    `json:"max_commits"`
}

// LoadActiveWorkPacket validates the packet-range descriptor and the referenced
// Work Packet. Git ancestry/existence is deliberately verified by the workflow,
// because this package does not execute git commands.
func LoadActiveWorkPacket(root string) (ActiveWorkPacket, WorkPacket, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return ActiveWorkPacket{}, WorkPacket{}, fmt.Errorf("resolve root: %w", err)
	}
	path := filepath.Join(abs, filepath.FromSlash(ActiveWorkPacketPath))
	file, err := os.Open(path)
	if err != nil {
		return ActiveWorkPacket{}, WorkPacket{}, fmt.Errorf("open active work packet: %w", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(io.LimitReader(file, 1<<20))
	decoder.DisallowUnknownFields()
	var active ActiveWorkPacket
	if err := decoder.Decode(&active); err != nil {
		return ActiveWorkPacket{}, WorkPacket{}, fmt.Errorf("decode active work packet: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return ActiveWorkPacket{}, WorkPacket{}, fmt.Errorf("decode active work packet: multiple JSON values")
		}
		return ActiveWorkPacket{}, WorkPacket{}, fmt.Errorf("decode active work packet trailing data: %w", err)
	}
	if err := active.Validate(); err != nil {
		return ActiveWorkPacket{}, WorkPacket{}, err
	}

	packetRel := filepath.ToSlash(filepath.Clean(active.Packet))
	packetPath := filepath.Join(abs, filepath.FromSlash(packetRel))
	workPacketRoot := filepath.Join(abs, "docs", "engineering", "work-packets")
	relToRoot, err := filepath.Rel(workPacketRoot, packetPath)
	if err != nil || relToRoot == "." || strings.HasPrefix(relToRoot, ".."+string(filepath.Separator)) || relToRoot == ".." || filepath.IsAbs(relToRoot) {
		return ActiveWorkPacket{}, WorkPacket{}, fmt.Errorf("active packet path must resolve inside docs/engineering/work-packets")
	}
	packetFile, err := os.Open(packetPath)
	if err != nil {
		return ActiveWorkPacket{}, WorkPacket{}, fmt.Errorf("open referenced work packet %q: %w", active.Packet, err)
	}
	defer packetFile.Close()
	packet, err := DecodeWorkPacket(io.LimitReader(packetFile, 1<<20))
	if err != nil {
		return ActiveWorkPacket{}, WorkPacket{}, fmt.Errorf("validate referenced work packet %q: %w", active.Packet, err)
	}
	return active, packet, nil
}

func (a ActiveWorkPacket) Validate() error {
	packet := filepath.ToSlash(filepath.Clean(strings.TrimSpace(a.Packet)))
	if !strings.HasPrefix(packet, "docs/engineering/work-packets/") || !strings.HasSuffix(strings.ToLower(packet), ".json") {
		return fmt.Errorf("active packet must reference a JSON file under docs/engineering/work-packets")
	}
	if !commitSHA40.MatchString(strings.TrimSpace(a.MutationBase)) {
		return fmt.Errorf("mutation_base must be a 40-character commit SHA")
	}
	if a.MaxCommits < 1 || a.MaxCommits > 256 {
		return fmt.Errorf("max_commits must be between 1 and 256")
	}
	return nil
}
