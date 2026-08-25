package compiler

import (
	"fmt"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/scenario/model"
)

// AnalyzeTemporal scans triggers and flow steps to extract temporal requirements and determine if durable scheduling is needed.
func AnalyzeTemporal(s model.Scenario) ([]TemporalRequirement, Diagnostics) {
	var diags Diagnostics
	var reqs []TemporalRequirement

	for i, trig := range s.Triggers {
		trigPath := fmt.Sprintf("triggers[%d]", i)
		if trig.Kind == model.TriggerSchedule {
			reqs = append(reqs, TemporalRequirement{
				Kind:    "schedule",
				Spec:    trig.Capability.ID,
				Durable: true,
			})
		}
		for j, temp := range trig.Temporal {
			tempPath := fmt.Sprintf("%s.temporal[%d]", trigPath, j)
			switch temp.Kind {
			case model.TemporalDebounce:
				reqs = append(reqs, TemporalRequirement{
					Kind:     "debounce",
					Duration: temp.Duration,
					Durable:  temp.Duration > 5*time.Second,
				})
			case model.TemporalThrottle:
				reqs = append(reqs, TemporalRequirement{
					Kind:     "throttle",
					Duration: temp.Duration,
					Durable:  temp.Duration > 5*time.Second,
				})
			case model.TemporalRepeatWithin:
				reqs = append(reqs, TemporalRequirement{
					Kind:     "repeat_within",
					Duration: temp.Duration,
					Durable:  true,
				})
			case model.TemporalScheduleWindow:
				spec := ""
				if temp.Start != nil && temp.End != nil {
					spec = fmt.Sprintf("%s-%s", temp.Start.String(), temp.End.String())
				}
				reqs = append(reqs, TemporalRequirement{
					Kind:     "schedule_window",
					Spec:     spec,
					Timezone: temp.Timezone,
					Durable:  false,
				})
			case model.TemporalUntil:
				reqs = append(reqs, TemporalRequirement{
					Kind:    "until",
					Durable: true,
				})
			default:
				_ = tempPath
			}
		}
	}

	var scanFlow func(path string, flow model.Flow)
	scanFlow = func(path string, flow model.Flow) {
		for i, step := range flow.Steps {
			stepPath := fmt.Sprintf("%s.steps[%d]", path, i)
			if step.Wait != nil {
				reqs = append(reqs, TemporalRequirement{
					Kind:     "wait",
					Duration: step.Wait.Duration,
					Durable:  true, // All explicit workflow waits require durable continuation across restarts
				})
			}
			if step.HumanApproval != nil && step.HumanApproval.Timeout > 0 {
				reqs = append(reqs, TemporalRequirement{
					Kind:     "human_approval_timeout",
					Duration: step.HumanApproval.Timeout,
					Durable:  true,
				})
			}
			if step.If != nil {
				scanFlow(stepPath+".if.then", step.If.Then)
				if step.If.Else != nil {
					scanFlow(stepPath+".if.else", *step.If.Else)
				}
			}
			if step.Switch != nil {
				for j, c := range step.Switch.Cases {
					scanFlow(fmt.Sprintf("%s.switch.cases[%d].flow", stepPath, j), c.Flow)
				}
				if step.Switch.Default != nil {
					scanFlow(stepPath+".switch.default", *step.Switch.Default)
				}
			}
			if step.Parallel != nil {
				for j, b := range step.Parallel.Branches {
					scanFlow(fmt.Sprintf("%s.parallel.branches[%d]", stepPath, j), b)
				}
			}
		}
	}

	scanFlow("flow", s.Flow)
	return reqs, diags
}
