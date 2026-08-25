package capability

import (
	"context"

	"github.com/Homiakus/Home_Sentinel/internal/scenario/model"
)

type ScenarioUse struct {
	ScenarioID model.ID      `json:"scenarioId"`
	Version    model.Version `json:"version"`
}

type DependencyResolver interface {
	UsesCapability(context.Context, Key) ([]ScenarioUse, error)
	UsesEntity(context.Context, string) ([]ScenarioUse, error)
}

type NoDependencies struct{}

func (NoDependencies) UsesCapability(context.Context, Key) ([]ScenarioUse, error) { return nil, nil }
func (NoDependencies) UsesEntity(context.Context, string) ([]ScenarioUse, error)  { return nil, nil }
