package ai

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
	"time"
)

func jpegFrame(v uint8) []byte {
	img := image.NewGray(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			img.SetGray(x, y, color.Gray{Y: v})
		}
	}
	var b bytes.Buffer
	_ = jpeg.Encode(&b, img, nil)
	return b.Bytes()
}
func TestRepresentativeFramesDeduplicateNearIdentical(t *testing.T) {
	now := time.Now()
	a := jpegFrame(10)
	b := jpegFrame(10)
	c := jpegFrame(240)
	out, err := SelectRepresentativeFrames([]Frame{{JPEG: a, Score: 1, Timestamp: now}, {JPEG: b, Score: .9, Timestamp: now.Add(time.Second)}, {JPEG: c, Score: .8, Timestamp: now.Add(2 * time.Second)}}, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("frames=%d", len(out))
	}
}
func TestAnalysisValidation(t *testing.T) {
	good := AnalysisResult{Summary: "person at door", Activity: "approach", Risk: RiskLow, Confidence: .8}
	if err := good.Validate(); err != nil {
		t.Fatal(err)
	}
	bad := good
	bad.Confidence = 1.2
	if bad.Validate() == nil {
		t.Fatal("invalid confidence accepted")
	}
}
