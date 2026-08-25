package redact

import (
	"net/url"
	"regexp"
	"strings"
)

var sensitiveKV = regexp.MustCompile(`(?i)(authorization|token|password|passwd|secret|api[_-]?key)(\s*[:=]\s*)([^\s,;]+)`)

// String removes common secret-bearing URL/userinfo and key/value forms before
// values cross logging, audit, or user-visible error boundaries. It is a
// defense-in-depth helper; callers should still avoid constructing errors that
// contain secrets in the first place.
func String(v string) string {
	if v == "" {
		return ""
	}
	out := sensitiveKV.ReplaceAllString(v, `$1$2[REDACTED]`)
	fields := strings.Fields(out)
	for i, f := range fields {
		if strings.Contains(f, "://") {
			fields[i] = urlString(f)
		}
	}
	if len(fields) > 0 {
		out = strings.Join(fields, " ")
	}
	return out
}

func urlString(raw string) string {
	trimRight := strings.TrimRight(raw, ",;.)]")
	suffix := strings.TrimPrefix(raw, trimRight)
	u, err := url.Parse(trimRight)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return raw
	}
	if u.User != nil {
		u.User = url.User("[REDACTED]")
	}
	q := u.Query()
	for k := range q {
		lk := strings.ToLower(k)
		if strings.Contains(lk, "token") || strings.Contains(lk, "pass") || strings.Contains(lk, "secret") || strings.Contains(lk, "key") {
			q.Set(k, "[REDACTED]")
		}
	}
	u.RawQuery = q.Encode()
	return u.String() + suffix
}
