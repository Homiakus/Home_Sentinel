package ai

import (
	"context"
	"errors"
	"sync"
	"time"
)

type Job struct {
	Request  AnalysisRequest
	Policy   PrivacyPolicy
	Enqueued time.Time
	Result   chan JobResult
}

type JobResult struct {
	Analysis AnalysisResult
	Err      error
}

type Service struct {
	Provider Provider
	Local    bool
	Profile  RuntimeProfile
	queue    chan Job
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

func NewService(parent context.Context, provider Provider, local bool, profile RuntimeProfile, queueSize int) (*Service, error) {
	if provider == nil {
		return nil, errors.New("AI provider required")
	}
	if profile.MaxParallel < 1 {
		return nil, errors.New("AI runtime profile disables inference")
	}
	if queueSize <= 0 {
		queueSize = 32
	}
	ctx, cancel := context.WithCancel(parent)
	s := &Service{Provider: provider, Local: local, Profile: profile, queue: make(chan Job, queueSize), cancel: cancel}
	for i := 0; i < profile.MaxParallel; i++ {
		s.wg.Add(1)
		go s.worker(ctx)
	}
	return s, nil
}
func (s *Service) Submit(ctx context.Context, req AnalysisRequest, policy PrivacyPolicy) (<-chan JobResult, error) {
	if err := policy.Validate(s.Local); err != nil {
		return nil, err
	}
	selected, err := SelectRepresentativeFrames(req.Frames, s.Profile.MaxFrames)
	if err != nil {
		return nil, err
	}
	req.Frames = selected
	ch := make(chan JobResult, 1)
	job := Job{Request: req, Policy: policy, Enqueued: time.Now(), Result: ch}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case s.queue <- job:
		return ch, nil
	default:
		return nil, errors.New("AI queue full")
	}
}
func (s *Service) worker(ctx context.Context) {
	defer s.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-s.queue:
			start := time.Now()
			result, err := s.Provider.Analyze(ctx, job.Request)
			result.QueueDuration = start.Sub(job.Enqueued)
			job.Result <- JobResult{Analysis: result, Err: err}
			close(job.Result)
		}
	}
}
func (s *Service) Close() {
	if s == nil || s.cancel == nil {
		return
	}
	s.cancel()
	s.wg.Wait()
}
func (s *Service) QueueDepth() int {
	if s == nil {
		return 0
	}
	return len(s.queue)
}
