package engloop

import "testing"

func TestGenerateEdgeSuitePairwiseWithConstraintAndRequired(t *testing.T) {
	model := EdgeModel{
		Name:     "admission-crash-space",
		Strength: 2,
		Factors: []EdgeFactor{
			{Name: "ownership", Values: []string{"free", "held"}},
			{Name: "crash", Values: []string{"none", "after-provider"}},
			{Name: "recovery", Values: []string{"reconcile", "restart"}},
		},
		Forbidden: []EdgeAssignment{
			{"ownership": "free", "crash": "after-provider"},
		},
		Required: []EdgeAssignment{
			{"ownership": "held", "crash": "after-provider", "recovery": "reconcile"},
		},
	}

	suite, err := GenerateEdgeSuite(model)
	if err != nil {
		t.Fatalf("GenerateEdgeSuite() error=%v", err)
	}
	if len(suite.Vectors) == 0 {
		t.Fatal("expected vectors")
	}
	if !containsAssignment(suite.Vectors, model.Required[0]) {
		t.Fatalf("required vector not represented: %#v", suite.Vectors)
	}
	for _, vector := range suite.Vectors {
		if vector["ownership"] == "free" && vector["crash"] == "after-provider" {
			t.Fatalf("forbidden vector emitted: %#v", vector)
		}
	}
	assertAllTuplesCovered(t, model, suite)
}

func TestGenerateEdgeSuiteRejectsCandidateExplosion(t *testing.T) {
	model := EdgeModel{
		Name:          "too-large",
		Strength:      2,
		MaxCandidates: 3,
		Factors: []EdgeFactor{
			{Name: "a", Values: []string{"0", "1"}},
			{Name: "b", Values: []string{"0", "1"}},
		},
	}
	if _, err := GenerateEdgeSuite(model); err == nil {
		t.Fatal("expected max_candidates error")
	}
}

func TestEdgeModelRejectsUnknownConstraintValue(t *testing.T) {
	model := EdgeModel{
		Name:     "invalid",
		Strength: 1,
		Factors:  []EdgeFactor{{Name: "time", Values: []string{"before", "after"}}},
		Forbidden: []EdgeAssignment{
			{"time": "never"},
		},
	}
	if err := model.Validate(); err == nil {
		t.Fatal("expected validation error")
	}
}

func assertAllTuplesCovered(t *testing.T, model EdgeModel, suite EdgeSuite) {
	t.Helper()
	candidates := allValidAssignmentsForTest(model)
	combos := chooseIndexes(len(model.Factors), model.Strength)
	want := map[string]struct{}{}
	for _, row := range candidates {
		for _, key := range tupleKeys(row, model.Factors, combos) {
			want[key] = struct{}{}
		}
	}
	for _, row := range suite.Vectors {
		for _, key := range tupleKeys(row, model.Factors, combos) {
			delete(want, key)
		}
	}
	if len(want) != 0 {
		t.Fatalf("suite left %d tuples uncovered: %v", len(want), want)
	}
}

func allValidAssignmentsForTest(model EdgeModel) []EdgeAssignment {
	out := []EdgeAssignment{}
	row := EdgeAssignment{}
	var visit func(int)
	visit = func(pos int) {
		if pos == len(model.Factors) {
			if !forbidden(row, model.Forbidden) {
				out = append(out, cloneAssignment(row))
			}
			return
		}
		factor := model.Factors[pos]
		for _, value := range factor.Values {
			row[factor.Name] = value
			visit(pos + 1)
		}
		delete(row, factor.Name)
	}
	visit(0)
	return out
}

func containsAssignment(rows []EdgeAssignment, partial EdgeAssignment) bool {
	for _, row := range rows {
		if matchesPartial(row, partial) {
			return true
		}
	}
	return false
}
