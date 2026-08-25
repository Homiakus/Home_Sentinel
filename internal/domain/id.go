package domain

import (
	"crypto/rand"
	"encoding/base32"
	"errors"
	"strings"
	"unicode"
)

type ID string

var crockford = base32.NewEncoding("0123456789ABCDEFGHJKMNPQRSTVWXYZ").WithPadding(base32.NoPadding)

func NewID(prefix string) (ID, error) {
	if prefix == "" {
		return "", errors.New("invalid id prefix")
	}
	for _, r := range prefix {
		if !(unicode.IsLower(r) && unicode.IsLetter(r)) && !unicode.IsDigit(r) {
			return "", errors.New("invalid id prefix")
		}
	}
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return ID(prefix + "_" + strings.ToLower(crockford.EncodeToString(buf))), nil
}

func (id ID) String() string { return string(id) }

func (id ID) ValidFor(prefix string) bool {
	s := string(id)
	if !strings.HasPrefix(s, prefix+"_") {
		return false
	}
	body := strings.TrimPrefix(s, prefix+"_")
	if len(body) != 26 {
		return false
	}
	for _, r := range strings.ToUpper(body) {
		if !strings.ContainsRune("0123456789ABCDEFGHJKMNPQRSTVWXYZ", r) {
			return false
		}
	}
	return true
}
