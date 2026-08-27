package engloop

import (
	"fmt"
	"sort"
	"strings"
)

const defaultMaxEdgeCandidates = 100000

type EdgeFactor struct {
	Name   string   `json:"name"`
	Values []string `json:"values"`
}

type EdgeAssignment map[string]string

type EdgeModel struct {
	Name          string           `json:"name"`
	Strength      int              `json:"strength"`
	MaxCandidates int              `json:"max_candidates,omitempty"`
	Factors       []EdgeFactor     `json:"factors"`
	Forbidden     []EdgeAssignment `json:"forbidden,omitempty"`
	Required      []EdgeAssignment `json:"required,omitempty"`
}

type EdgeSuite struct {
	Name           string           `json:"name"`
	Strength       int              `json:"strength"`
	Factors        []string         `json:"factors"`
	Vectors        []EdgeAssignment `json:"vectors"`
	CandidateCount int              `json:"candidate_count"`
	CoveredTuples  int              `json:"covered_tuples"`
}

func (m EdgeModel) Validate() error {
	if strings.TrimSpace(m.Name) == "" {
		return fmt.Errorf("edge model name is required")
	}
	if len(m.Factors) == 0 {
		return fmt.Errorf("at least one factor is required")
	}
	if m.Strength < 1 || m.Strength > len(m.Factors) {
		return fmt.Errorf("strength must be in [1,%d]", len(m.Factors))
	}
	if m.MaxCandidates < 0 {
		return fmt.Errorf("max_candidates cannot be negative")
	}

	factorValues := make(map[string]map[string]struct{}, len(m.Factors))
	for _, factor := range m.Factors {
		name := strings.TrimSpace(factor.Name)
		if name == "" {
			return fmt.Errorf("factor name is required")
		}
		if _, exists := factorValues[name]; exists {
			return fmt.Errorf("duplicate factor %q", name)
		}
		if len(factor.Values) == 0 {
			return fmt.Errorf("factor %q has no values", name)
		}
		values := make(map[string]struct{}, len(factor.Values))
		for _, value := range factor.Values {
			if strings.TrimSpace(value) == "" {
				return fmt.Errorf("factor %q contains an empty value", name)
			}
			if _, exists := values[value]; exists {
				return fmt.Errorf("factor %q contains duplicate value %q", name, value)
			}
			values[value] = struct{}{}
		}
		factorValues[name] = values
	}

	for i, assignment := range append(append([]EdgeAssignment(nil), m.Forbidden...), m.Required...) {
		if len(assignment) == 0 {
			return fmt.Errorf("constraint/required assignment %d is empty", i)
		}
		for factor, value := range assignment {
			values, ok := factorValues[factor]
			if !ok {
				return fmt.Errorf("assignment references unknown factor %q", factor)
			}
			if _, ok := values[value]; !ok {
				return fmt.Errorf("assignment uses unknown value %q for factor %q", value, factor)
			}
		}
	}
	return nil
}

func GenerateEdgeSuite(model EdgeModel) (EdgeSuite, error) {
	if err := model.Validate(); err != nil {
		return EdgeSuite{}, err
	}
	maxCandidates := model.MaxCandidates
	if maxCandidates == 0 {
		maxCandidates = defaultMaxEdgeCandidates
	}

	factorNames := make([]string, len(model.Factors))
	for i, factor := range model.Factors {
		factorNames[i] = factor.Name
	}

	candidates := make([]EdgeAssignment, 0)
	current := make(EdgeAssignment, len(model.Factors))
	var build func(int) error
	build = func(pos int) error {
		if pos == len(model.Factors) {
			if forbidden(current, model.Forbidden) {
				return nil
			}
			if len(candidates) >= maxCandidates {
				return fmt.Errorf("edge candidate space exceeds max_candidates=%d", maxCandidates)
			}
			candidates = append(candidates, cloneAssignment(current))
			return nil
		}
		factor := model.Factors[pos]
		for _, value := range factor.Values {
			current[factor.Name] = value
			if err := build(pos + 1); err != nil {
				return err
			}
		}
		delete(current, factor.Name)
		return nil
	}
	if err := build(0); err != nil {
		return EdgeSuite{}, err
	}
	if len(candidates) == 0 {
		return EdgeSuite{}, fmt.Errorf("constraints eliminate every candidate")
	}

	indexCombos := chooseIndexes(len(model.Factors), model.Strength)
	rowTuples := make([][]string, len(candidates))
	uncovered := make(map[string]struct{})
	for i, row := range candidates {
		keys := tupleKeys(row, model.Factors, indexCombos)
		rowTuples[i] = keys
		for _, key := range keys {
			uncovered[key] = struct{}{}
		}
	}
	totalTuples := len(uncovered)

	selected := make(map[int]struct{})
	order := make([]int, 0)
	add := func(index int) {
		if _, ok := selected[index]; ok {
			return
		}
		selected[index] = struct{}{}
		order = append(order, index)
		for _, key := range rowTuples[index] {
			delete(uncovered, key)
		}
	}

	for _, required := range model.Required {
		best := -1
		bestScore := -1
		for i, row := range candidates {
			if !matchesPartial(row, required) {
				continue
			}
			score := countUncovered(rowTuples[i], uncovered)
			if score > bestScore {
				best = i
				bestScore = score
			}
		}
		if best < 0 {
			return EdgeSuite{}, fmt.Errorf("required assignment %v cannot be satisfied under constraints", required)
		}
		add(best)
	}

	for len(uncovered) > 0 {
		best := -1
		bestScore := 0
		for i := range candidates {
			if _, ok := selected[i]; ok {
				continue
			}
			score := countUncovered(rowTuples[i], uncovered)
			if score > bestScore {
				best = i
				bestScore = score
			}
		}
		if best < 0 {
			return EdgeSuite{}, fmt.Errorf("unable to cover %d remaining tuples", len(uncovered))
		}
		add(best)
	}

	vectors := make([]EdgeAssignment, 0, len(order))
	for _, index := range order {
		vectors = append(vectors, cloneAssignment(candidates[index]))
	}
	return EdgeSuite{
		Name:           model.Name,
		Strength:       model.Strength,
		Factors:        factorNames,
		Vectors:        vectors,
		CandidateCount: len(candidates),
		CoveredTuples:  totalTuples,
	}, nil
}

func forbidden(row EdgeAssignment, forbiddenList []EdgeAssignment) bool {
	for _, rule := range forbiddenList {
		if matchesPartial(row, rule) {
			return true
		}
	}
	return false
}

func matchesPartial(row, partial EdgeAssignment) bool {
	for factor, value := range partial {
		if row[factor] != value {
			return false
		}
	}
	return true
}

func cloneAssignment(in EdgeAssignment) EdgeAssignment {
	out := make(EdgeAssignment, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func chooseIndexes(n, k int) [][]int {
	out := make([][]int, 0)
	current := make([]int, 0, k)
	var visit func(int)
	visit = func(start int) {
		if len(current) == k {
			combo := append([]int(nil), current...)
			out = append(out, combo)
			return
		}
		remaining := k - len(current)
		for i := start; i <= n-remaining; i++ {
			current = append(current, i)
			visit(i + 1)
			current = current[:len(current)-1]
		}
	}
	visit(0)
	return out
}

func tupleKeys(row EdgeAssignment, factors []EdgeFactor, combos [][]int) []string {
	keys := make([]string, 0, len(combos))
	for _, combo := range combos {
		parts := make([]string, 0, len(combo))
		for _, index := range combo {
			factor := factors[index]
			parts = append(parts, factor.Name+"="+row[factor.Name])
		}
		keys = append(keys, strings.Join(parts, "\x1f"))
	}
	return keys
}

func countUncovered(keys []string, uncovered map[string]struct{}) int {
	score := 0
	for _, key := range keys {
		if _, ok := uncovered[key]; ok {
			score++
		}
	}
	return score
}

func SortedAssignmentKeys(a EdgeAssignment) []string {
	keys := make([]string, 0, len(a))
	for k := range a {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
