// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package nietzsche

import (
	"testing"
)

func TestRewriteNQL_BuiltinTypesUnchanged(t *testing.T) {
	tests := []string{
		`MATCH (n:Semantic) WHERE n.id = "abc" RETURN n`,
		`MATCH (n:Episodic) RETURN n`,
		`MATCH (n:Concept) WHERE n.name = "foo" RETURN n`,
		`MATCH (n:DreamSnapshot) RETURN n`,
	}
	for _, nql := range tests {
		result := RewriteNQL(nql)
		if result != nql {
			t.Errorf("built-in type should not be rewritten:\n  input:  %s\n  output: %s", nql, result)
		}
	}
}

func TestRewriteNQL_CustomTypeWithWhere(t *testing.T) {
	input := `MATCH (p:Person) WHERE p.name = "foo" RETURN p`
	expected := `MATCH (p:Semantic) WHERE p.node_label = "Person" AND p.name = "foo" RETURN p`
	result := RewriteNQL(input)
	if result != expected {
		t.Errorf("custom type with WHERE:\n  input:    %s\n  expected: %s\n  got:      %s", input, expected, result)
	}
}

func TestRewriteNQL_CustomTypeWithoutWhere(t *testing.T) {
	input := `MATCH (s:Significante) RETURN s`
	expected := `MATCH (s:Semantic) WHERE s.node_label = "Significante" RETURN s`
	result := RewriteNQL(input)
	if result != expected {
		t.Errorf("custom type without WHERE:\n  input:    %s\n  expected: %s\n  got:      %s", input, expected, result)
	}
}

func TestRewriteNQL_CustomTypeWithInlineProps(t *testing.T) {
	input := `MATCH (p:Person {id: $id}) RETURN p`
	expected := `MATCH (p:Semantic {node_label: "Person", id: $id}) RETURN p`
	result := RewriteNQL(input)
	if result != expected {
		t.Errorf("custom type with inline props:\n  input:    %s\n  expected: %s\n  got:      %s", input, expected, result)
	}
}

func TestRewriteNQL_MultipleCustomTypes(t *testing.T) {
	input := `MATCH (p:Person)-[r:KNOWS]-(q:Clinic) RETURN p, q`
	result := RewriteNQL(input)
	// Should rewrite both Person and Clinic
	if result == input {
		t.Errorf("multiple custom types should be rewritten, but got same input")
	}
	// Both should have node_label conditions
	for _, expect := range []string{`p.node_label = "Person"`, `q.node_label = "Clinic"`} {
		if !contains(result, expect) {
			t.Errorf("expected %q in result:\n  %s", expect, result)
		}
	}
	// Both should have :Semantic
	if !contains(result, "(p:Semantic)") {
		t.Errorf("expected (p:Semantic) in result:\n  %s", result)
	}
	if !contains(result, "(q:Semantic)") {
		t.Errorf("expected (q:Semantic) in result:\n  %s", result)
	}
}

func TestRewriteNQL_MixedBuiltinAndCustom(t *testing.T) {
	input := `MATCH (p:Person)-[r:HAS]-(e:Episodic) RETURN p, e`
	result := RewriteNQL(input)
	// Person → Semantic, Episodic unchanged
	if !contains(result, "(p:Semantic)") {
		t.Errorf("expected (p:Semantic) in result:\n  %s", result)
	}
	if !contains(result, "(e:Episodic)") {
		t.Errorf("expected (e:Episodic) unchanged in result:\n  %s", result)
	}
	if !contains(result, `p.node_label = "Person"`) {
		t.Errorf("expected node_label filter for Person in result:\n  %s", result)
	}
}

func TestRewriteNQL_NoMatch(t *testing.T) {
	input := `RETURN 42`
	result := RewriteNQL(input)
	if result != input {
		t.Errorf("no MATCH clause should pass through unchanged:\n  input:  %s\n  output: %s", input, result)
	}
}

func TestNormalizeNodeType(t *testing.T) {
	tests := []struct {
		input      string
		normalized string
		isCustom   bool
	}{
		{"Semantic", "Semantic", false},
		{"Episodic", "Episodic", false},
		{"Concept", "Concept", false},
		{"DreamSnapshot", "DreamSnapshot", false},
		{"Person", "Semantic", true},
		{"Clinic", "Semantic", true},
		{"Zettel", "Semantic", true},
		{"", "Semantic", false},
	}
	for _, tt := range tests {
		norm, custom := NormalizeNodeType(tt.input)
		if norm != tt.normalized || custom != tt.isCustom {
			t.Errorf("NormalizeNodeType(%q) = (%q, %v), want (%q, %v)",
				tt.input, norm, custom, tt.normalized, tt.isCustom)
		}
	}
}

func TestNormalizeContent(t *testing.T) {
	content := map[string]interface{}{"name": "John"}
	normalized, newContent := NormalizeContent("Person", content)

	if normalized != "Semantic" {
		t.Errorf("expected Semantic, got %s", normalized)
	}
	if newContent["node_label"] != "Person" {
		t.Errorf("expected node_label=Person, got %v", newContent["node_label"])
	}
	if newContent["name"] != "John" {
		t.Errorf("expected name=John, got %v", newContent["name"])
	}
	// Original map should be unchanged
	if _, ok := content["node_label"]; ok {
		t.Errorf("original content should not be mutated")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
