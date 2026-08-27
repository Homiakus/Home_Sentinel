package engloop

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

type MutationStatus string

const (
	MutationKilled     MutationStatus = "KILLED"
	MutationLived      MutationStatus = "LIVED"
	MutationNotCovered MutationStatus = "NOT COVERED"
	MutationTimedOut   MutationStatus = "TIMED OUT"
	MutationNotViable  MutationStatus = "NOT VIABLE"
	MutationSkipped    MutationStatus = "SKIPPED"
)

type MutationFinding struct {
	File     string         `json:"file"`
	Line     int            `json:"line"`
	Column   int            `json:"column"`
	Type     string         `json:"type"`
	Status   MutationStatus `json:"status"`
	Critical bool           `json:"critical"`
}

type MutationReport struct {
	ToolEfficacy     float64           `json:"tool_efficacy"`
	MutantsTotal     int               `json:"mutants_total"`
	CriticalBlockers []MutationFinding `json:"critical_blockers"`
	NonCriticalLived []MutationFinding `json:"noncritical_lived"`
}

func (r MutationReport) HasCriticalBlockers() bool { return len(r.CriticalBlockers) > 0 }

func EvaluateGremlins(r io.Reader) (MutationReport, error) {
	var raw struct {
		TestEfficacy float64 `json:"test_efficacy"`
		MutantsTotal int     `json:"mutants_total"`
		Files        []struct {
			FileName  string `json:"file_name"`
			Mutations []struct {
				Line   int            `json:"line"`
				Column int            `json:"column"`
				Type   string         `json:"type"`
				Status MutationStatus `json:"status"`
			} `json:"mutations"`
		} `json:"files"`
	}
	dec := json.NewDecoder(r)
	if err := dec.Decode(&raw); err != nil {
		return MutationReport{}, fmt.Errorf("decode Gremlins report: %w", err)
	}
	report := MutationReport{ToolEfficacy: raw.TestEfficacy, MutantsTotal: raw.MutantsTotal}
	for _, file := range raw.Files {
		path := cleanPath(file.FileName)
		critical := isCriticalSurface(path)
		for _, mutation := range file.Mutations {
			finding := MutationFinding{
				File:     path,
				Line:     mutation.Line,
				Column:   mutation.Column,
				Type:     mutation.Type,
				Status:   mutation.Status,
				Critical: critical,
			}
			switch mutation.Status {
			case MutationLived:
				if critical {
					report.CriticalBlockers = append(report.CriticalBlockers, finding)
				} else {
					report.NonCriticalLived = append(report.NonCriticalLived, finding)
				}
			case MutationNotCovered, MutationTimedOut:
				if critical {
					report.CriticalBlockers = append(report.CriticalBlockers, finding)
				}
			case MutationKilled, MutationNotViable, MutationSkipped:
				// No blocking action. NOT VIABLE and SKIPPED remain visible in the raw artifact.
			default:
				if critical && strings.TrimSpace(string(mutation.Status)) != "" {
					report.CriticalBlockers = append(report.CriticalBlockers, finding)
				}
			}
		}
	}
	return report, nil
}
