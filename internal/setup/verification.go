package setup

type VerificationStatus string

const (
	VerifyPass VerificationStatus = "PASS"
	VerifyFail VerificationStatus = "FAIL"
	VerifySkip VerificationStatus = "SKIP"
)

type VerificationCheck struct {
	ID       string             `json:"id"`
	Name     string             `json:"name"`
	Status   VerificationStatus `json:"status"`
	Detail   string             `json:"detail,omitempty"`
	Duration int64              `json:"duration_ms,omitempty"`
}

type VerificationReport struct {
	OK     bool                `json:"ok"`
	Checks []VerificationCheck `json:"checks"`
}

func NewVerification(checks ...VerificationCheck) VerificationReport {
	r := VerificationReport{OK: true, Checks: checks}
	for _, c := range checks {
		if c.Status == VerifyFail {
			r.OK = false
		}
	}
	return r
}
