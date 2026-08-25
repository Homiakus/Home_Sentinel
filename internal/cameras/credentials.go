package cameras

import (
	"errors"
	"net/url"

	"github.com/Homiakus/Home_Sentinel/internal/secrets"
)

type CredentialResolver interface {
	Resolve(secrets.Ref) ([]byte, error)
}

func ResolvedURL(ep Endpoint, resolver CredentialResolver) (string, error) {
	u, err := url.Parse(ep.URL)
	if err != nil {
		return "", err
	}
	if u.User != nil {
		return "", errors.New("endpoint URL already contains credentials")
	}
	if ep.Username == "" {
		return u.String(), nil
	}
	if ep.PasswordRef == "" {
		return "", errors.New("password reference required")
	}
	if resolver == nil {
		return "", errors.New("credential resolver unavailable")
	}
	password, err := resolver.Resolve(ep.PasswordRef)
	if err != nil {
		return "", err
	}
	u.User = url.UserPassword(ep.Username, string(password))
	return u.String(), nil
}
func RotatePasswordRef(c *Camera, ref secrets.Ref) error {
	if c == nil {
		return errors.New("nil camera")
	}
	changed := false
	for i := range c.Streams {
		if c.Streams[i].Endpoint.Username != "" {
			c.Streams[i].Endpoint.PasswordRef = ref
			changed = true
		}
	}
	if !changed {
		return errors.New("camera has no credentialed streams")
	}
	return nil
}
