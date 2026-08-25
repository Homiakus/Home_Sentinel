package ai

import (
	"context"
	"errors"

	"github.com/Homiakus/Home_Sentinel/internal/repository"
)

type PolicyStore struct {
	Store         *repository.Store[PrivacyPolicy]
	ProviderLocal bool
}

func (s PolicyStore) Get(ctx context.Context, cameraID string) (PrivacyPolicy, error) {
	if s.Store == nil {
		return PrivacyPolicy{}, errors.New("AI policy store unavailable")
	}
	r, err := s.Store.Get(ctx, "ai_"+cameraID)
	if errors.Is(err, repository.ErrNotFound) {
		return PrivacyPolicy{Enabled: false, AllowDescription: false, AllowRemoteProvider: false}, nil
	}
	return r.Value, err
}
func (s PolicyStore) Put(ctx context.Context, cameraID string, p PrivacyPolicy) (PrivacyPolicy, error) {
	if s.Store == nil {
		return PrivacyPolicy{}, errors.New("AI policy store unavailable")
	}
	if err := p.Validate(s.ProviderLocal); err != nil {
		return PrivacyPolicy{}, err
	}
	r, err := s.Store.Put(ctx, "ai_"+cameraID, p)
	return r.Value, err
}
