package ai

import "errors"

type PrivacyPolicy struct {
	Enabled             bool `json:"enabled"`
	AllowDescription    bool `json:"allow_description"`
	AllowFace           bool `json:"allow_face"`
	AllowRemoteProvider bool `json:"allow_remote_provider"`
	RetainInputFrames   bool `json:"retain_input_frames"`
}

func (p PrivacyPolicy) Validate(providerLocal bool) error {
	if !p.Enabled {
		return nil
	}
	if !providerLocal && !p.AllowRemoteProvider {
		return errors.New("remote AI provider denied by camera privacy policy")
	}
	if !p.AllowDescription {
		return errors.New("AI description disabled by camera privacy policy")
	}
	return nil
}
