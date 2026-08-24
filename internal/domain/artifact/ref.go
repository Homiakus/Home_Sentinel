package artifact

import (
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

var (
	ErrInvalidURI    = errors.New("artifact: invalid URI")
	ErrInvalidDigest = errors.New("artifact: invalid SHA-256 digest")
	ErrInvalidSize   = errors.New("artifact: invalid size")
)

// Ref is the only artifact representation allowed in durable workflow facts.
// Raw media bytes belong to the media/artifact store, not the control plane.
type Ref struct {
	URI       string            `json:"uri"`
	Digest    string            `json:"digest"`
	Size      int64             `json:"size"`
	MediaType string            `json:"mediaType,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

func (r Ref) Validate() error {
	if strings.TrimSpace(r.URI) == "" {
		return ErrInvalidURI
	}
	parsed, err := url.Parse(r.URI)
	if err != nil || parsed.Scheme == "" {
		return fmt.Errorf("%w: %q", ErrInvalidURI, r.URI)
	}
	const prefix = "sha256:"
	if !strings.HasPrefix(r.Digest, prefix) {
		return fmt.Errorf("%w: expected sha256 prefix", ErrInvalidDigest)
	}
	raw := strings.TrimPrefix(r.Digest, prefix)
	decoded, err := hex.DecodeString(raw)
	if err != nil || len(decoded) != 32 {
		return fmt.Errorf("%w: %q", ErrInvalidDigest, r.Digest)
	}
	if r.Size < 0 {
		return ErrInvalidSize
	}
	return nil
}
