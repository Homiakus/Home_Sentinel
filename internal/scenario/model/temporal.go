package model

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "time/tzdata"
)

type TimeOfDay struct {
	Hour   int `json:"hour"`
	Minute int `json:"minute"`
	Second int `json:"second,omitempty"`
}

func ParseTimeOfDay(value string) (TimeOfDay, error) {
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) < 2 || len(parts) > 3 {
		return TimeOfDay{}, fmt.Errorf("scenario: time of day must be HH:MM or HH:MM:SS")
	}
	parsed := [3]int{}
	for i, part := range parts {
		if len(part) != 2 {
			return TimeOfDay{}, fmt.Errorf("scenario: time of day component %q must have two digits", part)
		}
		n, err := strconv.Atoi(part)
		if err != nil {
			return TimeOfDay{}, fmt.Errorf("scenario: invalid time of day %q", value)
		}
		parsed[i] = n
	}
	result := TimeOfDay{Hour: parsed[0], Minute: parsed[1], Second: parsed[2]}
	if err := result.Validate(); err != nil {
		return TimeOfDay{}, err
	}
	return result, nil
}

func (t TimeOfDay) Validate() error {
	if t.Hour < 0 || t.Hour > 23 || t.Minute < 0 || t.Minute > 59 || t.Second < 0 || t.Second > 59 {
		return fmt.Errorf("scenario: invalid time of day %02d:%02d:%02d", t.Hour, t.Minute, t.Second)
	}
	return nil
}

func (t TimeOfDay) String() string {
	if t.Second == 0 {
		return fmt.Sprintf("%02d:%02d", t.Hour, t.Minute)
	}
	return fmt.Sprintf("%02d:%02d:%02d", t.Hour, t.Minute, t.Second)
}

func (t TimeOfDay) seconds() int { return t.Hour*3600 + t.Minute*60 + t.Second }

func ResolveWallClock(date time.Time, value TimeOfDay, timezone string, policy DSTPolicy) (time.Time, bool, error) {
	if err := value.Validate(); err != nil {
		return time.Time{}, false, err
	}
	if !validDSTPolicy(policy) {
		return time.Time{}, false, fmt.Errorf("scenario: invalid DST policy %q", policy)
	}
	loc, err := time.LoadLocation(strings.TrimSpace(timezone))
	if err != nil {
		return time.Time{}, false, fmt.Errorf("scenario: invalid timezone %q: %w", timezone, err)
	}
	year, month, day := date.Date()
	offsets := map[int]struct{}{}
	for delta := -2; delta <= 2; delta++ {
		probe := time.Date(year, month, day+delta, 12, 0, 0, 0, loc)
		_, offset := probe.Zone()
		offsets[offset] = struct{}{}
	}
	wallUTC := time.Date(year, month, day, value.Hour, value.Minute, value.Second, 0, time.UTC)
	matches := make([]time.Time, 0, 2)
	seen := map[int64]struct{}{}
	for offset := range offsets {
		candidate := wallUTC.Add(-time.Duration(offset) * time.Second)
		local := candidate.In(loc)
		y, m, d := local.Date()
		if y != year || m != month || d != day || local.Hour() != value.Hour || local.Minute() != value.Minute || local.Second() != value.Second {
			continue
		}
		if _, ok := seen[candidate.UnixNano()]; ok {
			continue
		}
		seen[candidate.UnixNano()] = struct{}{}
		matches = append(matches, candidate.UTC())
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].Before(matches[j]) })
	if len(matches) == 0 {
		if policy == DSTSkipInvalid {
			return time.Time{}, false, nil
		}
		return time.Time{}, false, fmt.Errorf("scenario: local wall time %s does not exist in %s", value.String(), timezone)
	}
	if policy == DSTWallClockLast {
		return matches[len(matches)-1], true, nil
	}
	return matches[0], true, nil
}

func TimeWindowContains(start, end TimeOfDay, at TimeOfDay) bool {
	s, e, x := start.seconds(), end.seconds(), at.seconds()
	if s == e {
		return true
	}
	if s < e {
		return x >= s && x < e
	}
	return x >= s || x < e
}

type TemporalKind string

const (
	TemporalFor            TemporalKind = "for"
	TemporalWithin         TemporalKind = "within"
	TemporalAfter          TemporalKind = "after"
	TemporalBefore         TemporalKind = "before"
	TemporalDebounce       TemporalKind = "debounce"
	TemporalThrottle       TemporalKind = "throttle"
	TemporalCooldown       TemporalKind = "cooldown"
	TemporalOncePer        TemporalKind = "once_per"
	TemporalRepeatWithin   TemporalKind = "repeat_within"
	TemporalUntil          TemporalKind = "until"
	TemporalTimeout        TemporalKind = "timeout"
	TemporalScheduleWindow TemporalKind = "schedule_window"
)

type DSTPolicy string

const (
	DSTWallClockFirst DSTPolicy = "wall_clock_first"
	DSTWallClockLast  DSTPolicy = "wall_clock_last"
	DSTSkipInvalid    DSTPolicy = "skip_invalid"
)

type CatchUpPolicy string

const (
	CatchUpSkip       CatchUpPolicy = "skip"
	CatchUpOnce       CatchUpPolicy = "once"
	CatchUpAllBounded CatchUpPolicy = "all_bounded"
)

type TemporalSpec struct {
	Kind             TemporalKind  `json:"kind"`
	Duration         time.Duration `json:"duration,omitempty"`
	RelatedTriggerID string        `json:"relatedTriggerId,omitempty"`
	Count            int           `json:"count,omitempty"`
	Until            Expr          `json:"until,omitempty"`
	Start            *TimeOfDay    `json:"start,omitempty"`
	End              *TimeOfDay    `json:"end,omitempty"`
	Timezone         string        `json:"timezone,omitempty"`
	DST              DSTPolicy     `json:"dst,omitempty"`
	CatchUp          CatchUpPolicy `json:"catchUp,omitempty"`
	MaxCatchUp       int           `json:"maxCatchUp,omitempty"`
}

func (s TemporalSpec) Validate() error {
	switch s.Kind {
	case TemporalFor, TemporalWithin, TemporalDebounce, TemporalThrottle, TemporalCooldown, TemporalOncePer, TemporalTimeout:
		if s.Duration <= 0 {
			return fmt.Errorf("scenario: temporal %q requires positive duration", s.Kind)
		}
	case TemporalAfter, TemporalBefore:
		if s.Duration < 0 || strings.TrimSpace(s.RelatedTriggerID) == "" {
			return fmt.Errorf("scenario: temporal %q requires related trigger and non-negative duration", s.Kind)
		}
	case TemporalRepeatWithin:
		if s.Duration <= 0 || s.Count < 2 {
			return fmt.Errorf("scenario: repeat_within requires count >= 2 and positive duration")
		}
	case TemporalUntil:
		if s.Until.IsZero() {
			return fmt.Errorf("scenario: until requires expression")
		}
		if err := s.Until.Validate(); err != nil {
			return fmt.Errorf("scenario: until: %w", err)
		}
	case TemporalScheduleWindow:
		if s.Start == nil || s.End == nil || strings.TrimSpace(s.Timezone) == "" {
			return fmt.Errorf("scenario: schedule_window requires start, end and timezone")
		}
		if err := s.Start.Validate(); err != nil {
			return err
		}
		if err := s.End.Validate(); err != nil {
			return err
		}
		if _, err := time.LoadLocation(s.Timezone); err != nil {
			return fmt.Errorf("scenario: invalid timezone %q: %w", s.Timezone, err)
		}
		if !validDSTPolicy(s.DST) {
			return fmt.Errorf("scenario: invalid DST policy %q", s.DST)
		}
		if !validCatchUpPolicy(s.CatchUp) {
			return fmt.Errorf("scenario: invalid catch-up policy %q", s.CatchUp)
		}
		if s.CatchUp == CatchUpAllBounded {
			if s.MaxCatchUp <= 0 || s.MaxCatchUp > 1000 {
				return fmt.Errorf("scenario: bounded catch-up requires maxCatchUp in [1,1000]")
			}
		} else if s.MaxCatchUp != 0 {
			return fmt.Errorf("scenario: maxCatchUp is only valid with all_bounded catch-up")
		}
	default:
		return fmt.Errorf("scenario: unknown temporal kind %q", s.Kind)
	}
	return nil
}

func NormalizeTemporal(s TemporalSpec) (TemporalSpec, error) {
	s.RelatedTriggerID = strings.TrimSpace(s.RelatedTriggerID)
	s.Timezone = strings.TrimSpace(s.Timezone)
	if !s.Until.IsZero() {
		normalized, err := normalizeExpr(s.Until)
		if err != nil {
			return TemporalSpec{}, err
		}
		s.Until = normalized
	}
	if err := s.Validate(); err != nil {
		return TemporalSpec{}, err
	}
	return s, nil
}

func validDSTPolicy(policy DSTPolicy) bool {
	return policy == DSTWallClockFirst || policy == DSTWallClockLast || policy == DSTSkipInvalid
}

func validCatchUpPolicy(policy CatchUpPolicy) bool {
	return policy == CatchUpSkip || policy == CatchUpOnce || policy == CatchUpAllBounded
}
