package capability

import (
	"sort"
	"strings"
)

type UIHints struct {
	Icon        string   `json:"icon,omitempty"`
	Group       string   `json:"group,omitempty"`
	Control     string   `json:"control,omitempty"`
	Placeholder string   `json:"placeholder,omitempty"`
	Unit        string   `json:"unit,omitempty"`
	Order       int      `json:"order,omitempty"`
	Advanced    bool     `json:"advanced,omitempty"`
	Options     []string `json:"options,omitempty"`
}

func normalizeUIHints(hints UIHints) UIHints {
	hints.Icon = strings.TrimSpace(hints.Icon)
	hints.Group = strings.TrimSpace(hints.Group)
	hints.Control = strings.TrimSpace(hints.Control)
	hints.Placeholder = strings.TrimSpace(hints.Placeholder)
	hints.Unit = strings.TrimSpace(hints.Unit)
	if len(hints.Options) > 0 {
		options := make([]string, 0, len(hints.Options))
		seen := map[string]struct{}{}
		for _, option := range hints.Options {
			option = strings.TrimSpace(option)
			if option == "" {
				continue
			}
			if _, exists := seen[option]; exists {
				continue
			}
			seen[option] = struct{}{}
			options = append(options, option)
		}
		sort.Strings(options)
		hints.Options = options
	}
	return hints
}
