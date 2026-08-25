package secrets

import (
	"encoding/json"
	"errors"
	"strings"
)

// Ref is an opaque reference to a secret held by a provider. It deliberately
// contains no secret value and is safe to persist in desired-state records.
type Ref string

func ParseRef(v string) (Ref, error) {
	if !strings.HasPrefix(v, "secret://") || len(v) <= len("secret://") {
		return "", errors.New("invalid secret reference")
	}
	return Ref(v), nil
}

func (r Ref) String() string { return string(r) }

// MarshalJSON is allowed because Ref contains only a locator, never secret bytes.
func (r Ref) MarshalJSON() ([]byte, error) { return json.Marshal(string(r)) }

type Provider interface {
	Resolve(ref Ref) ([]byte, error)
}
