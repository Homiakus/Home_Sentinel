package catalog

import (
	"sync"

	"github.com/Homiakus/Home_Sentinel/internal/scenario/compiler"
)

type DependencyGraph struct {
	mu sync.RWMutex

	// Forward index: scenarioID -> set of dependencies
	scenarioCapabilities map[string]map[string]struct{}
	scenarioEntities     map[string]map[string]struct{}
	scenarioSubflows     map[string]map[string]struct{}
	scenarioTemplates    map[string]map[string]struct{}

	// Reverse index: dependency -> set of scenario IDs
	capabilityScenarios map[string]map[string]struct{}
	entityScenarios     map[string]map[string]struct{}
	subflowParents      map[string]map[string]struct{}
}

func NewDependencyGraph() *DependencyGraph {
	return &DependencyGraph{
		scenarioCapabilities: make(map[string]map[string]struct{}),
		scenarioEntities:     make(map[string]map[string]struct{}),
		scenarioSubflows:     make(map[string]map[string]struct{}),
		scenarioTemplates:    make(map[string]map[string]struct{}),
		capabilityScenarios:  make(map[string]map[string]struct{}),
		entityScenarios:      make(map[string]map[string]struct{}),
		subflowParents:       make(map[string]map[string]struct{}),
	}
}

// UpdateScenarioDependencies indexes or updates the active dependencies for a scenario from its compiled manifest.
func (d *DependencyGraph) UpdateScenarioDependencies(scenarioID string, manifest *compiler.Manifest, templateID string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Clear old forward & reverse indices for this scenario
	d.clearScenario(scenarioID)

	if manifest == nil {
		return
	}

	caps := make(map[string]struct{})
	for capID := range manifest.CapabilityVersions {
		caps[capID] = struct{}{}
		if d.capabilityScenarios[capID] == nil {
			d.capabilityScenarios[capID] = make(map[string]struct{})
		}
		d.capabilityScenarios[capID][scenarioID] = struct{}{}
	}
	d.scenarioCapabilities[scenarioID] = caps

	ents := make(map[string]struct{})
	for _, ent := range manifest.ReferencedEntities {
		entKey := ent.Kind + ":" + ent.ID
		ents[entKey] = struct{}{}
		if d.entityScenarios[entKey] == nil {
			d.entityScenarios[entKey] = make(map[string]struct{})
		}
		d.entityScenarios[entKey][scenarioID] = struct{}{}
	}
	d.scenarioEntities[scenarioID] = ents

	// Subflows from ADGO / System graph
	subs := make(map[string]struct{})
	if manifest.ADGOPlan != nil {
		for _, node := range manifest.ADGOPlan.Nodes {
			if node.Type == "Subflow" {
				if subID, ok := node.Properties["scenarioId"].(string); ok && subID != "" {
					subs[subID] = struct{}{}
					if d.subflowParents[subID] == nil {
						d.subflowParents[subID] = make(map[string]struct{})
					}
					d.subflowParents[subID][scenarioID] = struct{}{}
				}
			}
		}
	}
	d.scenarioSubflows[scenarioID] = subs

	if templateID != "" {
		if d.scenarioTemplates[scenarioID] == nil {
			d.scenarioTemplates[scenarioID] = make(map[string]struct{})
		}
		d.scenarioTemplates[scenarioID][templateID] = struct{}{}
	}
}

func (d *DependencyGraph) RemoveScenario(scenarioID string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.clearScenario(scenarioID)
}

func (d *DependencyGraph) clearScenario(scenarioID string) {
	if caps, ok := d.scenarioCapabilities[scenarioID]; ok {
		for capID := range caps {
			if scens, exists := d.capabilityScenarios[capID]; exists {
				delete(scens, scenarioID)
				if len(scens) == 0 {
					delete(d.capabilityScenarios, capID)
				}
			}
		}
		delete(d.scenarioCapabilities, scenarioID)
	}

	if ents, ok := d.scenarioEntities[scenarioID]; ok {
		for entKey := range ents {
			if scens, exists := d.entityScenarios[entKey]; exists {
				delete(scens, scenarioID)
				if len(scens) == 0 {
					delete(d.entityScenarios, entKey)
				}
			}
		}
		delete(d.scenarioEntities, scenarioID)
	}

	if subs, ok := d.scenarioSubflows[scenarioID]; ok {
		for subID := range subs {
			if parents, exists := d.subflowParents[subID]; exists {
				delete(parents, scenarioID)
				if len(parents) == 0 {
					delete(d.subflowParents, subID)
				}
			}
		}
		delete(d.scenarioSubflows, scenarioID)
	}

	delete(d.scenarioTemplates, scenarioID)
}

func (d *DependencyGraph) GetScenariosUsingCapability(capID string) []string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	scens := d.capabilityScenarios[capID]
	out := make([]string, 0, len(scens))
	for s := range scens {
		out = append(out, s)
	}
	return out
}

func (d *DependencyGraph) GetScenariosUsingEntity(kind, id string) []string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	entKey := kind + ":" + id
	scens := d.entityScenarios[entKey]
	out := make([]string, 0, len(scens))
	for s := range scens {
		out = append(out, s)
	}
	return out
}

func (d *DependencyGraph) GetParentScenariosForSubflow(subflowScenarioID string) []string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	parents := d.subflowParents[subflowScenarioID]
	out := make([]string, 0, len(parents))
	for p := range parents {
		out = append(out, p)
	}
	return out
}
