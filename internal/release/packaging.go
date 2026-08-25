package release

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strings"
)

type Finding struct{ Code, Message string }

var forbiddenHostPort = regexp.MustCompile(`(?m)^\s*-\s*["']?[^\n]*:(1883|1984|11434|5000)(?:/tcp|/udp)?["']?\s*$`)

func ValidateCompose(r io.Reader) []Finding {
	b := new(strings.Builder)
	_, _ = io.Copy(b, r)
	s := b.String()
	low := strings.ToLower(s)
	var out []Finding
	if strings.Contains(low, ":latest") || strings.Contains(low, "@master") {
		out = append(out, Finding{"FLOATING_REFERENCE", "production compose contains floating latest/master reference"})
	}
	if strings.Contains(low, "/var/run/docker.sock") {
		out = append(out, Finding{"DOCKER_SOCKET", "runtime must not mount Docker socket"})
	}
	for _, m := range forbiddenHostPort.FindAllStringSubmatch(s, -1) {
		out = append(out, Finding{"INTERNAL_PORT_PUBLISHED", fmt.Sprintf("internal service port %s appears host-published", m[1])})
	}
	sc := bufio.NewScanner(strings.NewReader(s))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "image:") && strings.Contains(line, "${") && !strings.Contains(line, ":?") {
			out = append(out, Finding{"UNPINNED_IMAGE_ENV", "image env must be required with :? so deployment cannot silently float"})
		}
	}
	return out
}

func ValidateReleaseDockerfile(r io.Reader) []Finding {
	b := new(strings.Builder)
	_, _ = io.Copy(b, r)
	s := b.String()
	low := strings.ToLower(s)
	var out []Finding
	if strings.Contains(low, "from golang:") || strings.Contains(low, "from debian:") || strings.Contains(low, "from ubuntu:") {
		out = append(out, Finding{"FLOATING_BASE_IMAGE", "release Dockerfile must receive immutable base refs as build arguments"})
	}
	if !strings.Contains(s, "FROM ${GO_BUILD_IMAGE}") || !strings.Contains(s, "FROM ${RUNTIME_IMAGE}") {
		out = append(out, Finding{"RELEASE_BASE_ARGS", "release Dockerfile must use GO_BUILD_IMAGE and RUNTIME_IMAGE build arguments"})
	}
	if strings.Contains(s, "ARG GO_BUILD_IMAGE=") || strings.Contains(s, "ARG RUNTIME_IMAGE=") {
		out = append(out, Finding{"DEFAULT_RELEASE_BASE", "release base image arguments must not have floating defaults"})
	}
	return out
}
