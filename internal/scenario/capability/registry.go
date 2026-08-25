package capability

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
)

var (
	ErrAlreadyRegistered          = errors.New("capability: already registered")
	ErrNotFound                   = errors.New("capability: not found")
	ErrInUse                      = errors.New("capability: resource is in use")
	ErrDependencyResolverRequired = errors.New("capability: dependency resolver is required")
	ErrEntityReference            = errors.New("capability: referenced by registered entity")
)

type Filter struct {
	Role               Role
	Kind               Kind
	Category           Category
	EntityID           string
	IncludeUnavailable bool
}

type Snapshot struct {
	Capabilities []Descriptor       `json:"capabilities"`
	Entities     []EntityDescriptor `json:"entities"`
}

type Registry struct {
	mu           sync.RWMutex
	capabilities map[string]Descriptor
	entities     map[string]EntityDescriptor
	usage        DependencyResolver
}

func NewRegistry(usage DependencyResolver) *Registry {
	return &Registry{
		capabilities: make(map[string]Descriptor),
		entities:     make(map[string]EntityDescriptor),
		usage:        usage,
	}
}

func (r *Registry) Register(source Descriptor) error {
	descriptor, err := NormalizeDescriptor(source)
	if err != nil {
		return err
	}
	key := descriptor.Key().String()
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.capabilities[key]; exists {
		return fmt.Errorf("%w: %s", ErrAlreadyRegistered, key)
	}
	r.capabilities[key] = descriptor
	return nil
}

func (r *Registry) RegisterEntity(source EntityDescriptor) error {
	entity, err := normalizeEntity(source)
	if err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.entities[entity.ID]; exists {
		return fmt.Errorf("%w: entity %s", ErrAlreadyRegistered, entity.ID)
	}
	for _, key := range entity.Capabilities {
		descriptor, exists := r.capabilities[key.String()]
		if !exists {
			return fmt.Errorf("capability: entity %q references %w %q", entity.ID, ErrNotFound, key.String())
		}
		if len(descriptor.EntityKinds) > 0 && !contains(descriptor.EntityKinds, entity.Kind) {
			return fmt.Errorf("capability: %q is not compatible with entity kind %q", key.String(), entity.Kind)
		}
	}
	r.entities[entity.ID] = entity
	return nil
}

func (r *Registry) Get(id, version string) (Descriptor, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	descriptor, ok := r.capabilities[Key{ID: id, Version: version}.String()]
	if !ok {
		return Descriptor{}, false
	}
	clone, err := cloneDescriptor(descriptor)
	if err != nil {
		return Descriptor{}, false
	}
	return clone, true
}

func (r *Registry) GetEntity(id string) (EntityDescriptor, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entity, ok := r.entities[id]
	if !ok {
		return EntityDescriptor{}, false
	}
	clone, err := normalizeEntity(entity)
	if err != nil {
		return EntityDescriptor{}, false
	}
	return clone, true
}

func (r *Registry) ResolveCompatible(id, required string) (Descriptor, bool) {
	if _, err := ParseSemVer(required); err != nil {
		return Descriptor{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	var best Descriptor
	var bestVersion SemVer
	found := false
	for _, descriptor := range r.capabilities {
		if descriptor.ID != id || !IsBackwardCompatible(required, descriptor.Version) {
			continue
		}
		version, _ := ParseSemVer(descriptor.Version)
		if !found || compareVersion(version, bestVersion) > 0 {
			best = descriptor
			bestVersion = version
			found = true
		}
	}
	if !found {
		return Descriptor{}, false
	}
	clone, err := cloneDescriptor(best)
	if err != nil {
		return Descriptor{}, false
	}
	return clone, true
}

func (r *Registry) List(filter Filter) ([]Descriptor, error) {
	if err := validateRole(filter.Role); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	var entityKeys map[string]struct{}
	if strings.TrimSpace(filter.EntityID) != "" {
		entity, exists := r.entities[filter.EntityID]
		if !exists {
			return nil, nil
		}
		entityKeys = make(map[string]struct{}, len(entity.Capabilities))
		for _, key := range entity.Capabilities {
			entityKeys[key.String()] = struct{}{}
		}
	}
	out := make([]Descriptor, 0, len(r.capabilities))
	for key, descriptor := range r.capabilities {
		if entityKeys != nil {
			if _, exists := entityKeys[key]; !exists {
				continue
			}
		}
		if filter.Kind != "" && descriptor.Kind != filter.Kind {
			continue
		}
		if filter.Category != "" && descriptor.Category != filter.Category {
			continue
		}
		if !visibleTo(descriptor.Visibility, filter.Role) {
			continue
		}
		if !filter.IncludeUnavailable && !descriptor.Available {
			continue
		}
		clone, err := cloneDescriptor(descriptor)
		if err != nil {
			return nil, err
		}
		out = append(out, clone)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Category != out[j].Category {
			return out[i].Category < out[j].Category
		}
		if out[i].ID != out[j].ID {
			return out[i].ID < out[j].ID
		}
		left, _ := ParseSemVer(out[i].Version)
		right, _ := ParseSemVer(out[j].Version)
		return compareVersion(left, right) < 0
	})
	return out, nil
}

func (r *Registry) ListEntities(role Role, includeUnavailable bool) ([]EntityDescriptor, error) {
	if err := validateRole(role); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]EntityDescriptor, 0, len(r.entities))
	for _, entity := range r.entities {
		if !visibleTo(entity.Visibility, role) {
			continue
		}
		if !includeUnavailable && !entity.Available {
			continue
		}
		if !r.entityHasVisibleCapabilityLocked(entity, role, includeUnavailable) {
			continue
		}
		clone, err := normalizeEntity(entity)
		if err != nil {
			return nil, err
		}
		out = append(out, clone)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (r *Registry) SetCapabilityHealth(key Key, available bool, health HealthStatus) error {
	if !validHealth(health) {
		return fmt.Errorf("capability: invalid health %q", health)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	descriptor, exists := r.capabilities[key.String()]
	if !exists {
		return ErrNotFound
	}
	descriptor.Available = available
	descriptor.Health = health
	r.capabilities[key.String()] = descriptor
	return nil
}

func (r *Registry) SetEntityHealth(id string, available bool, health HealthStatus) error {
	if !validHealth(health) {
		return fmt.Errorf("capability: invalid health %q", health)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	entity, exists := r.entities[id]
	if !exists {
		return ErrNotFound
	}
	entity.Available = available
	entity.Health = health
	r.entities[id] = entity
	return nil
}

func (r *Registry) Remove(ctx context.Context, key Key) error {
	r.mu.RLock()
	if _, exists := r.capabilities[key.String()]; !exists {
		r.mu.RUnlock()
		return ErrNotFound
	}
	if entityID, exists := r.entityReferenceLocked(key); exists {
		r.mu.RUnlock()
		return fmt.Errorf("%w: %s by entity %s", ErrEntityReference, key.String(), entityID)
	}
	usage := r.usage
	r.mu.RUnlock()
	if usage == nil {
		return ErrDependencyResolverRequired
	}
	uses, err := usage.UsesCapability(ctx, key)
	if err != nil {
		return err
	}
	if len(uses) > 0 {
		return fmt.Errorf("%w: %s", ErrInUse, key.String())
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.capabilities[key.String()]; !exists {
		return ErrNotFound
	}
	if entityID, exists := r.entityReferenceLocked(key); exists {
		return fmt.Errorf("%w: %s by entity %s", ErrEntityReference, key.String(), entityID)
	}
	delete(r.capabilities, key.String())
	return nil
}

func (r *Registry) RemoveEntity(ctx context.Context, id string) error {
	r.mu.RLock()
	_, exists := r.entities[id]
	usage := r.usage
	r.mu.RUnlock()
	if !exists {
		return ErrNotFound
	}
	if usage == nil {
		return ErrDependencyResolverRequired
	}
	uses, err := usage.UsesEntity(ctx, id)
	if err != nil {
		return err
	}
	if len(uses) > 0 {
		return fmt.Errorf("%w: entity %s", ErrInUse, id)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.entities[id]; !exists {
		return ErrNotFound
	}
	delete(r.entities, id)
	return nil
}

func (r *Registry) ScenariosUsingCapability(ctx context.Context, key Key) ([]ScenarioUse, error) {
	if r.usage == nil {
		return nil, ErrDependencyResolverRequired
	}
	return r.usage.UsesCapability(ctx, key)
}

func (r *Registry) ScenariosUsingEntity(ctx context.Context, id string) ([]ScenarioUse, error) {
	if r.usage == nil {
		return nil, ErrDependencyResolverRequired
	}
	return r.usage.UsesEntity(ctx, id)
}

func (r *Registry) Snapshot() (Snapshot, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	snapshot := Snapshot{
		Capabilities: make([]Descriptor, 0, len(r.capabilities)),
		Entities:     make([]EntityDescriptor, 0, len(r.entities)),
	}
	for _, descriptor := range r.capabilities {
		clone, err := cloneDescriptor(descriptor)
		if err != nil {
			return Snapshot{}, err
		}
		snapshot.Capabilities = append(snapshot.Capabilities, clone)
	}
	for _, entity := range r.entities {
		clone, err := normalizeEntity(entity)
		if err != nil {
			return Snapshot{}, err
		}
		snapshot.Entities = append(snapshot.Entities, clone)
	}
	sort.Slice(snapshot.Capabilities, func(i, j int) bool {
		return snapshot.Capabilities[i].Key().String() < snapshot.Capabilities[j].Key().String()
	})
	sort.Slice(snapshot.Entities, func(i, j int) bool { return snapshot.Entities[i].ID < snapshot.Entities[j].ID })
	return snapshot, nil
}

func (r *Registry) Digest() (string, error) {
	snapshot, err := r.Snapshot()
	if err != nil {
		return "", err
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func (r *Registry) entityHasVisibleCapabilityLocked(entity EntityDescriptor, role Role, includeUnavailable bool) bool {
	for _, key := range entity.Capabilities {
		descriptor, exists := r.capabilities[key.String()]
		if !exists || !visibleTo(descriptor.Visibility, role) {
			continue
		}
		if includeUnavailable || descriptor.Available {
			return true
		}
	}
	return false
}

func (r *Registry) entityReferenceLocked(key Key) (string, bool) {
	for _, entity := range r.entities {
		for _, ref := range entity.Capabilities {
			if ref == key {
				return entity.ID, true
			}
		}
	}
	return "", false
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
