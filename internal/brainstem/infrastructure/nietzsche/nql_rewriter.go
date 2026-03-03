// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later
//
// NQL Rewriter — transparently rewrites custom node type labels in NQL queries
// to use the 4 built-in types (Semantic, Episodic, Concept, DreamSnapshot) with
// a node_label content filter.
//
// NietzscheDB NQL only supports 4 built-in types in MATCH.
// Custom types like "Person", "Significante", "Zettel" etc. are stored as
// Semantic nodes with a `node_label` content field.
//
// This rewriter transforms:
//
//	MATCH (p:Person) WHERE p.name = "foo"
//	→ MATCH (p:Semantic) WHERE p.node_label = "Person" AND p.name = "foo"
//
//	MATCH (s:Significante) RETURN s
//	→ MATCH (s:Semantic) WHERE s.node_label = "Significante" RETURN s
//
//	MATCH (p:Person {id: $id})
//	→ MATCH (p:Semantic {node_label: "Person", id: $id})
package nietzsche

import (
	"fmt"
	"regexp"
	"strings"
)

// BuiltinTypes are the only valid NQL MATCH node types.
var BuiltinTypes = map[string]bool{
	"Semantic":      true,
	"Episodic":      true,
	"Concept":       true,
	"DreamSnapshot": true,
}

// matchLabelRe matches patterns like (varName:TypeLabel) in NQL MATCH clauses.
var matchLabelRe = regexp.MustCompile(`\((\w+):(\w+)\)`)

// matchLabelWithPropsRe matches (varName:TypeLabel {key: val}) patterns.
var matchLabelWithPropsRe = regexp.MustCompile(`\((\w+):(\w+)\s+\{([^}]*)\}\)`)

// rewriteEntry tracks a single variable→customType mapping for WHERE injection.
type rewriteEntry struct {
	varName  string
	typeName string
}

// RewriteNQL rewrites custom node type labels in NQL queries to use
// Semantic + node_label filter. Built-in types are left unchanged.
//
// Three-phase rewrite:
//  1. (var:Custom {props}) → (var:Semantic {node_label: "Custom", props})
//  2. (var:Custom) → (var:Semantic) + collect for WHERE injection
//  3. Inject WHERE var.node_label = "Custom" for all phase-2 rewrites
func RewriteNQL(nql string) string {
	if !strings.Contains(nql, "MATCH") {
		return nql
	}

	// Track phase-2 rewrites that need WHERE injection
	var pendingWhere []rewriteEntry

	// Phase 1: Handle (var:Type {props}) patterns first — inline node_label into props
	nql = matchLabelWithPropsRe.ReplaceAllStringFunc(nql, func(match string) string {
		groups := matchLabelWithPropsRe.FindStringSubmatch(match)
		if len(groups) < 4 {
			return match
		}
		varName := groups[1]
		typeName := groups[2]
		props := groups[3]

		if BuiltinTypes[typeName] {
			return match // built-in type, leave as-is
		}

		// Rewrite: (p:Person {id: $id}) → (p:Semantic {node_label: "Person", id: $id})
		_ = varName // used only for the rebuild
		return "(" + varName + ":Semantic {node_label: \"" + typeName + "\", " + props + "})"
	})

	// Phase 2: Handle simple (var:Type) patterns (no inline props)
	// Must run AFTER phase 1 to avoid double-rewriting
	nql = matchLabelRe.ReplaceAllStringFunc(nql, func(match string) string {
		groups := matchLabelRe.FindStringSubmatch(match)
		if len(groups) < 3 {
			return match
		}
		varName := groups[1]
		typeName := groups[2]

		if BuiltinTypes[typeName] {
			return match // built-in type
		}

		// This won't match phase-1 results because those have "{" which
		// this simpler regex doesn't capture
		if strings.Contains(match, "{") {
			return match
		}

		// Track for WHERE injection
		pendingWhere = append(pendingWhere, rewriteEntry{varName: varName, typeName: typeName})

		// Rewrite: (p:Person) → (p:Semantic)
		return "(" + varName + ":Semantic)"
	})

	// Phase 3: Inject node_label WHERE clauses for simple rewrites
	if len(pendingWhere) == 0 {
		return nql
	}
	return injectNodeLabelWhere(nql, pendingWhere)
}

// injectNodeLabelWhere injects node_label filters into the WHERE clause
// for variables that were rewritten from custom types to Semantic.
func injectNodeLabelWhere(nql string, entries []rewriteEntry) string {
	// Build the conjunction: var1.node_label = "Type1" AND var2.node_label = "Type2"
	var conditions []string
	for _, e := range entries {
		conditions = append(conditions, fmt.Sprintf(`%s.node_label = "%s"`, e.varName, e.typeName))
	}
	injection := strings.Join(conditions, " AND ")

	// Case 1: Query already has WHERE — prepend our conditions with AND
	whereIdx := strings.Index(nql, " WHERE ")
	if whereIdx >= 0 {
		// Insert right after "WHERE "
		insertPos := whereIdx + len(" WHERE ")
		nql = nql[:insertPos] + injection + " AND " + nql[insertPos:]
		return nql
	}

	// Case 2: No WHERE — inject between MATCH clause and next keyword
	// Find the end of the MATCH pattern (after last ")" in MATCH clause)
	// Keywords that can follow MATCH: WHERE, RETURN, SET, DELETE, CREATE, WITH, ORDER, LIMIT
	nextKeywords := []string{" RETURN ", " SET ", " DELETE ", " CREATE ", " WITH ", " ORDER ", " LIMIT "}
	insertPos := -1
	for _, kw := range nextKeywords {
		idx := strings.Index(nql, kw)
		if idx >= 0 && (insertPos < 0 || idx < insertPos) {
			insertPos = idx
		}
	}

	if insertPos >= 0 {
		// Insert WHERE before the next keyword
		nql = nql[:insertPos] + " WHERE " + injection + nql[insertPos:]
	} else {
		// No keyword found — append WHERE at the end
		nql = nql + " WHERE " + injection
	}

	return nql
}

// NormalizeNodeType checks if nodeType is a built-in NietzscheDB type.
// If not, returns "Semantic" (all custom types are stored as Semantic).
func NormalizeNodeType(nodeType string) (normalized string, isCustom bool) {
	if nodeType == "" {
		return "Semantic", false
	}
	if BuiltinTypes[nodeType] {
		return nodeType, false
	}
	return "Semantic", true
}

// NormalizeContent ensures that if nodeType is custom, node_label is injected
// into the content map. This is essential for MergeNode and InsertNode.
func NormalizeContent(nodeType string, content map[string]interface{}) (normalizedType string, normalizedContent map[string]interface{}) {
	normalized, isCustom := NormalizeNodeType(nodeType)
	if !isCustom {
		return normalized, content
	}

	// Clone content to avoid mutating the caller's map
	out := make(map[string]interface{}, len(content)+1)
	for k, v := range content {
		out[k] = v
	}
	out["node_label"] = nodeType
	return normalized, out
}
