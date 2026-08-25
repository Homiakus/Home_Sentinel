package capability

import (
	"fmt"
	"strconv"
	"strings"
)

type SemVer struct {
	Major int
	Minor int
	Patch int
}

func ParseSemVer(value string) (SemVer, error) {
	value = strings.TrimSpace(value)
	parts := strings.Split(value, ".")
	if len(parts) != 3 {
		return SemVer{}, fmt.Errorf("capability: version %q must use major.minor.patch", value)
	}
	values := [3]int{}
	for i, part := range parts {
		if part == "" {
			return SemVer{}, fmt.Errorf("capability: invalid version %q", value)
		}
		for _, ch := range part {
			if ch < '0' || ch > '9' {
				return SemVer{}, fmt.Errorf("capability: invalid version %q", value)
			}
		}
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return SemVer{}, fmt.Errorf("capability: invalid version %q", value)
		}
		values[i] = n
	}
	return SemVer{Major: values[0], Minor: values[1], Patch: values[2]}, nil
}

func (v SemVer) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

func compareVersion(left, right SemVer) int {
	if left.Major != right.Major {
		if left.Major < right.Major {
			return -1
		}
		return 1
	}
	if left.Minor != right.Minor {
		if left.Minor < right.Minor {
			return -1
		}
		return 1
	}
	if left.Patch < right.Patch {
		return -1
	}
	if left.Patch > right.Patch {
		return 1
	}
	return 0
}

// IsBackwardCompatible implements a conservative pre-v1 rule: v1+ is
// compatible within the same major; v0 requires the same minor line.
func IsBackwardCompatible(required, candidate string) bool {
	req, err := ParseSemVer(required)
	if err != nil {
		return false
	}
	cand, err := ParseSemVer(candidate)
	if err != nil {
		return false
	}
	if cand.Major != req.Major || compareVersion(cand, req) < 0 {
		return false
	}
	if req.Major == 0 && cand.Minor != req.Minor {
		return false
	}
	return true
}
