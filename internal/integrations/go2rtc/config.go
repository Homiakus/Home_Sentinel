package go2rtc

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"github.com/Homiakus/Home_Sentinel/internal/cameras"
	"github.com/Homiakus/Home_Sentinel/internal/secrets"
)

type SecretResolver interface {
	Resolve(secrets.Ref) ([]byte, error)
}

type Generated struct {
	Streams   map[string][]string `json:"streams"`
	SecretEnv map[string]string   `json:"-"`
}

var envSanitizer = regexp.MustCompile(`[^A-Z0-9_]`)

func CanonicalStreamName(cameraID string, role cameras.StreamRole) string {
	base := strings.ToLower(strings.TrimSpace(cameraID))
	base = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_', r == '-':
			return r
		default:
			return '_'
		}
	}, base)
	if role == cameras.RoleDetect {
		return base + "_sub"
	}
	return base
}

func Generate(cam cameras.Camera, resolver SecretResolver) (Generated, error) {
	if err := cam.Validate(); err != nil {
		return Generated{}, err
	}
	g := Generated{Streams: map[string][]string{}, SecretEnv: map[string]string{}}
	for _, st := range cam.Streams {
		if st.Endpoint.URL == "" {
			continue
		}
		source, env, err := sourceURL(cam.ID, st, resolver)
		if err != nil {
			return Generated{}, fmt.Errorf("stream %s: %w", st.ID, err)
		}
		name := CanonicalStreamName(cam.ID, st.Role)
		// Prevent go2rtc from taking the audio output backchannel for normal NVR streams.
		if cam.Capabilities.Talk && strings.HasPrefix(strings.ToLower(source), "rtsp://") && !strings.Contains(source, "#backchannel=") {
			source += "#backchannel=0"
		}
		g.Streams[name] = []string{source}
		for k, v := range env {
			g.SecretEnv[k] = v
		}
	}
	if len(g.Streams) == 0 {
		return Generated{}, errors.New("camera has no stream source usable by go2rtc")
	}
	return g, nil
}

func sourceURL(cameraID string, st cameras.Stream, resolver SecretResolver) (string, map[string]string, error) {
	u, err := url.Parse(st.Endpoint.URL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", nil, errors.New("invalid source URL")
	}
	if u.User != nil {
		return "", nil, errors.New("source URL must not contain embedded credentials")
	}
	env := map[string]string{}
	if st.Endpoint.Username == "" && st.Endpoint.PasswordRef == "" {
		return u.String(), env, nil
	}
	user := url.User(st.Endpoint.Username).String()
	auth := user
	if st.Endpoint.PasswordRef != "" {
		if resolver == nil {
			return "", nil, errors.New("secret resolver unavailable")
		}
		raw, err := resolver.Resolve(st.Endpoint.PasswordRef)
		if err != nil {
			return "", nil, fmt.Errorf("resolve password: %w", err)
		}
		envName := secretEnvName(cameraID, st.Role)
		env[envName] = encodePassword(string(raw))
		auth += ":{" + envName + "}"
	}
	if auth != "" {
		// Rebuild explicitly so the FRIGATE_* placeholder is not percent-escaped by net/url.
		suffix := u.EscapedPath()
		if suffix == "" {
			suffix = "/"
		}
		if u.RawQuery != "" {
			suffix += "?" + u.RawQuery
		}
		if u.Fragment != "" {
			suffix += "#" + u.Fragment
		}
		return u.Scheme + "://" + auth + "@" + u.Host + suffix, env, nil
	}
	return u.String(), env, nil
}

func secretEnvName(cameraID string, role cameras.StreamRole) string {
	s := strings.ToUpper(cameraID + "_" + string(role) + "_PASSWORD")
	s = envSanitizer.ReplaceAllString(s, "_")
	return "FRIGATE_SENTINEL_" + s
}
func encodePassword(raw string) string {
	encoded := url.UserPassword("", raw).String()
	return strings.TrimPrefix(encoded, ":")
}

func Merge(all []Generated) Generated {
	out := Generated{Streams: map[string][]string{}, SecretEnv: map[string]string{}}
	for _, g := range all {
		for k, v := range g.Streams {
			out.Streams[k] = append([]string(nil), v...)
		}
		for k, v := range g.SecretEnv {
			out.SecretEnv[k] = v
		}
	}
	return out
}

func Names(g Generated) []string {
	out := make([]string, 0, len(g.Streams))
	for k := range g.Streams {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
