package adapters

import (
	"encoding/json"
	"testing"
)

func TestFrigateReviewNormalized(t *testing.T) {
	e, err := FrigateReview([]byte(`{"type":"new","after":{"id":"171-x","camera":"front","severity":"alert","start_time":1700000000.5,"data":{"objects":["person"]}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if e.Type != "frigate.review.updated" {
		t.Fatal(e.Type)
	}
	var p FrigateReviewPayload
	if err := json.Unmarshal(e.Payload, &p); err != nil || p.EventID != "171-x" || p.CameraID != "front" {
		t.Fatalf("p=%+v err=%v", p, err)
	}
}
func TestIntercomRejectsWrongVersion(t *testing.T) {
	if _, err := IntercomButton("sentinel/intercom/front/event/button", []byte(`{"schema_version":2}`)); err == nil {
		t.Fatal("expected error")
	}
}
