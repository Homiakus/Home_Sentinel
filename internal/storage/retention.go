package storage

import "time"

type RetentionEstimate struct {
	BitrateBitsPerSecond     int64   `json:"bitrate_bits_per_second"`
	BytesPerDay              uint64  `json:"bytes_per_day"`
	RequestedDays            int     `json:"requested_days"`
	RequiredBytes            uint64  `json:"required_bytes"`
	EstimatedDaysAtFreeSpace float64 `json:"estimated_days_at_free_space"`
}

func EstimateRetention(bitrateBitsPerSecond int64, days int, freeBytes uint64) RetentionEstimate {
	if bitrateBitsPerSecond < 0 {
		bitrateBitsPerSecond = 0
	}
	if days < 0 {
		days = 0
	}
	perDay := uint64(float64(bitrateBitsPerSecond) * time.Hour.Seconds() * 24 / 8)
	required := perDay * uint64(days)
	availableDays := 0.0
	if perDay > 0 {
		availableDays = float64(freeBytes) / float64(perDay)
	}
	return RetentionEstimate{BitrateBitsPerSecond: bitrateBitsPerSecond, BytesPerDay: perDay, RequestedDays: days, RequiredBytes: required, EstimatedDaysAtFreeSpace: availableDays}
}
