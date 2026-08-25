package ai

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"sort"
)

type hashedFrame struct {
	frame      Frame
	hash       uint64
	mean       uint32
	perceptual bool
	exact      [32]byte
}

func SelectRepresentativeFrames(in []Frame, max int) ([]Frame, error) {
	if max <= 0 {
		return nil, nil
	}
	candidates := make([]hashedFrame, 0, len(in))
	for _, f := range in {
		if len(f.JPEG) == 0 {
			continue
		}
		h, mean, ok := averageHash(f.JPEG)
		candidates = append(candidates, hashedFrame{frame: f, hash: h, mean: mean, perceptual: ok, exact: sha256.Sum256(f.JPEG)})
	}
	if len(candidates) == 0 {
		return nil, errors.New("no decodable/non-empty AI frames")
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].frame.Score == candidates[j].frame.Score {
			return candidates[i].frame.Timestamp.Before(candidates[j].frame.Timestamp)
		}
		return candidates[i].frame.Score > candidates[j].frame.Score
	})
	out := make([]hashedFrame, 0, max)
	for _, c := range candidates {
		dup := false
		for _, chosen := range out {
			if c.exact == chosen.exact || (c.perceptual && chosen.perceptual && bitsDiff(c.hash, chosen.hash) <= 4 && absU32(c.mean, chosen.mean) <= 4096) {
				dup = true
				break
			}
		}
		if dup {
			continue
		}
		out = append(out, c)
		if len(out) >= max {
			break
		}
	}
	frames := make([]Frame, len(out))
	for i := range out {
		frames[i] = out[i].frame
	}
	return frames, nil
}

func averageHash(data []byte) (uint64, uint32, bool) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return 0, 0, false
	}
	b := img.Bounds()
	if b.Dx() < 1 || b.Dy() < 1 {
		return 0, 0, false
	}
	var vals [64]uint32
	var total uint64
	for y := 0; y < 8; y++ {
		py := b.Min.Y + (y*b.Dy()+b.Dy()/2)/8
		if py >= b.Max.Y {
			py = b.Max.Y - 1
		}
		for x := 0; x < 8; x++ {
			px := b.Min.X + (x*b.Dx()+b.Dx()/2)/8
			if px >= b.Max.X {
				px = b.Max.X - 1
			}
			r, g, bl, _ := img.At(px, py).RGBA()
			gray := uint32((299*uint64(r) + 587*uint64(g) + 114*uint64(bl)) / 1000)
			vals[y*8+x] = gray
			total += uint64(gray)
		}
	}
	avg := uint32(total / 64)
	var h uint64
	for i, v := range vals {
		if v >= avg {
			h |= 1 << uint(i)
		}
	}
	return h, avg, true
}
func absU32(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}
func bitsDiff(a, b uint64) int {
	x := a ^ b
	n := 0
	for x != 0 {
		x &= x - 1
		n++
	}
	return n
}
