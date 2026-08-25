package release

import (
	"bytes"
	"testing"
	"time"
)

func TestQualificationBlocksMandatoryPending(t *testing.T) {
	r := QualificationReport{Version: "1.0.0", GeneratedAt: time.Now(), Checks: []QualificationCheck{
		{ID: "CI", Name: "CI", Mandatory: true, Status: StatusPass, Evidence: []string{"run:1"}},
		{ID: "HIL", Name: "Hardware", Mandatory: true, Status: StatusPending},
	}}
	ok, blockers := r.Releasable()
	if ok || len(blockers) != 1 || blockers[0] != "HIL=PENDING" {
		t.Fatalf("unexpected %v %v", ok, blockers)
	}
}

func TestQualificationRequiresEvidenceForPass(t *testing.T) {
	r := QualificationReport{Version: "1.0.0", Checks: []QualificationCheck{{ID: "CI", Name: "CI", Mandatory: true, Status: StatusPass}}}
	if r.Validate() == nil {
		t.Fatal("expected evidence validation")
	}
}

func TestQualificationMarkdown(t *testing.T) {
	r := QualificationReport{Version: "1.0.0", GeneratedAt: time.Unix(0, 0).UTC(), Checks: []QualificationCheck{{ID: "CI", Name: "CI", Mandatory: true, Status: StatusPass, Evidence: []string{"run:1"}}}}
	var b bytes.Buffer
	if err := WriteQualificationMarkdown(&b, r); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(b.Bytes(), []byte("**State:** READY")) {
		t.Fatalf("%s", b.String())
	}
}
