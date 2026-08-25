package model

import (
	"fmt"
	"sort"
	"strings"
)

type NodeLayout struct {
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	Collapsed bool    `json:"collapsed,omitempty"`
}

type Metadata struct {
	Tags   []string              `json:"tags,omitempty"`
	Labels map[string]string     `json:"labels,omitempty"`
	Layout map[string]NodeLayout `json:"layout,omitempty"`
}

func normalizeMetadata(metadata Metadata) (Metadata, error) {
	tags := metadata.Tags[:0]
	seen := make(map[string]struct{}, len(metadata.Tags))
	for _, tag := range metadata.Tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	metadata.Tags = tags

	if len(metadata.Labels) > 0 {
		keys := make([]string, 0, len(metadata.Labels))
		for key := range metadata.Labels {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		normalized := make(map[string]string, len(metadata.Labels))
		for _, original := range keys {
			key := strings.TrimSpace(original)
			if key == "" {
				continue
			}
			if _, exists := normalized[key]; exists {
				return Metadata{}, fmt.Errorf("scenario: duplicate normalized metadata label %q", key)
			}
			normalized[key] = strings.TrimSpace(metadata.Labels[original])
		}
		metadata.Labels = normalized
	}
	return metadata, nil
}
