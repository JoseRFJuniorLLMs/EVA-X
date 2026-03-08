// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package nietzsche

import (
	"context"
	"fmt"

	"eva/internal/brainstem/logger"
	"eva/internal/util"
)

// WiederkehrAdapter wraps NietzscheDB's Wiederkehr daemon agent system for EVA.
//
// Wiederkehr daemons are autonomous, energy-bounded agents that patrol the graph
// on a schedule: they scan nodes matching an ON pattern, evaluate a WHEN condition,
// and execute a THEN action (DELETE, SET, or DIFFUSE). All operations are NQL-based,
// routed through client.Query().
//
// NQL reference:
//
//	CREATE DAEMON <name> ON (n:Label) WHEN <cond> THEN <action> EVERY INTERVAL("1h") [ENERGY 0.8]
//	DROP DAEMON <name>
//	SHOW DAEMONS
type WiederkehrAdapter struct {
	client *Client
}

// NewWiederkehrAdapter creates an adapter for Wiederkehr daemon operations.
func NewWiederkehrAdapter(client *Client) *WiederkehrAdapter {
	return &WiederkehrAdapter{client: client}
}

// ── Daemon CRUD ──────────────────────────────────────────────────────────────

// DaemonInfo represents a daemon definition returned by SHOW DAEMONS.
type DaemonInfo struct {
	Name         string  // unique daemon name
	Energy       float64 // current energy level (0.0–1.0); reaped at min_energy
	IntervalSecs float64 // tick interval in seconds
	LastRun      float64 // Unix timestamp of the last execution
}

// DaemonAction specifies what a daemon does when its condition fires.
type DaemonAction string

const (
	// DaemonActionDelete removes the matched node.
	DaemonActionDelete DaemonAction = "DELETE"
	// DaemonActionSet updates fields on the matched node.
	DaemonActionSet DaemonAction = "SET"
	// DaemonActionDiffuse triggers a diffusion walk from the matched node.
	DaemonActionDiffuse DaemonAction = "DIFFUSE"
)

// CreateDaemonOpts configures a new Wiederkehr daemon.
type CreateDaemonOpts struct {
	// Name is the unique daemon identifier (e.g. "guardian", "reaper").
	Name string
	// Label is the optional node type filter (e.g. "Memory", "Episodic").
	// If empty, the daemon scans all nodes.
	Label string
	// Alias is the variable name used in WHEN/THEN (e.g. "n").
	Alias string
	// WhenCondition is the NQL condition expression (e.g. "n.energy > 0.8").
	WhenCondition string
	// ThenClause is the full THEN clause (e.g. "DELETE n", "SET n.tagged = true",
	// "DIFFUSE FROM n WITH t=[0.1, 1.0] MAX_HOPS 5").
	ThenClause string
	// Interval is the scheduling interval (e.g. "1h", "30m", "7d").
	Interval string
	// Energy is the initial energy budget (0.0–1.0). 0 defaults to 1.0 on the server.
	Energy float64
}

// CreateDaemon registers a new Wiederkehr daemon agent via NQL.
//
// Example:
//
//	adapter.CreateDaemon(ctx, "eva_core", CreateDaemonOpts{
//	    Name:          "energy_guard",
//	    Label:         "Memory",
//	    Alias:         "n",
//	    WhenCondition: "n.energy > 0.8",
//	    ThenClause:    "DIFFUSE FROM n WITH t=[0.1, 1.0] MAX_HOPS 5",
//	    Interval:      "1h",
//	    Energy:        0.8,
//	})
func (w *WiederkehrAdapter) CreateDaemon(ctx context.Context, collection string, opts CreateDaemonOpts) error {
	log := logger.Nietzsche()

	if opts.Name == "" {
		return fmt.Errorf("wiederkehr: daemon name is required")
	}
	if opts.Alias == "" {
		opts.Alias = "n"
	}
	if opts.WhenCondition == "" {
		return fmt.Errorf("wiederkehr: WHEN condition is required")
	}
	if opts.ThenClause == "" {
		return fmt.Errorf("wiederkehr: THEN clause is required")
	}
	if opts.Interval == "" {
		return fmt.Errorf("wiederkehr: EVERY interval is required")
	}

	// Build ON pattern: (n:Label) or (n) if no label
	onPattern := fmt.Sprintf("(%s)", opts.Alias)
	if opts.Label != "" {
		onPattern = fmt.Sprintf("(%s:%s)", opts.Alias, opts.Label)
	}

	// Build NQL: CREATE DAEMON <name> ON (n:Label) WHEN <cond> THEN <action> EVERY INTERVAL("1h") [ENERGY 0.8]
	nql := fmt.Sprintf(`CREATE DAEMON %s ON %s WHEN %s THEN %s EVERY INTERVAL("%s")`,
		opts.Name, onPattern, opts.WhenCondition, opts.ThenClause, opts.Interval)

	if opts.Energy > 0 {
		nql += fmt.Sprintf(" ENERGY %.2f", opts.Energy)
	}

	result, err := w.client.Query(ctx, nql, nil, collection)
	if err != nil {
		log.Error().Err(err).
			Str("collection", collection).
			Str("daemon", opts.Name).
			Msg("[Wiederkehr] CREATE DAEMON failed")
		return fmt.Errorf("wiederkehr create daemon %s: %w", opts.Name, err)
	}

	if result.Error != "" {
		return fmt.Errorf("wiederkehr create daemon %s: %s", opts.Name, result.Error)
	}

	log.Info().
		Str("collection", collection).
		Str("daemon", opts.Name).
		Str("interval", opts.Interval).
		Float64("energy", opts.Energy).
		Msg("[Wiederkehr] daemon created")
	return nil
}

// DropDaemon removes a Wiederkehr daemon agent by name.
func (w *WiederkehrAdapter) DropDaemon(ctx context.Context, collection string, name string) error {
	log := logger.Nietzsche()

	if name == "" {
		return fmt.Errorf("wiederkehr: daemon name is required")
	}

	nql := fmt.Sprintf("DROP DAEMON %s", name)
	result, err := w.client.Query(ctx, nql, nil, collection)
	if err != nil {
		log.Error().Err(err).
			Str("collection", collection).
			Str("daemon", name).
			Msg("[Wiederkehr] DROP DAEMON failed")
		return fmt.Errorf("wiederkehr drop daemon %s: %w", name, err)
	}

	if result.Error != "" {
		return fmt.Errorf("wiederkehr drop daemon %s: %s", name, result.Error)
	}

	log.Info().
		Str("collection", collection).
		Str("daemon", name).
		Msg("[Wiederkehr] daemon dropped")
	return nil
}

// ListDaemons returns all registered Wiederkehr daemons for a collection.
func (w *WiederkehrAdapter) ListDaemons(ctx context.Context, collection string) ([]DaemonInfo, error) {
	log := logger.Nietzsche()

	result, err := w.client.Query(ctx, "SHOW DAEMONS", nil, collection)
	if err != nil {
		log.Error().Err(err).
			Str("collection", collection).
			Msg("[Wiederkehr] SHOW DAEMONS failed")
		return nil, fmt.Errorf("wiederkehr list daemons: %w", err)
	}

	if result.Error != "" {
		return nil, fmt.Errorf("wiederkehr list daemons: %s", result.Error)
	}

	daemons := make([]DaemonInfo, 0, len(result.ScalarRows))
	for _, row := range result.ScalarRows {
		d := DaemonInfo{}
		if v, ok := row["name"]; ok {
			if s, ok := v.(string); ok {
				d.Name = s
			}
		}
		if v, ok := row["energy"]; ok {
			d.Energy = util.ToFloat64(v)
		}
		if v, ok := row["interval_secs"]; ok {
			d.IntervalSecs = util.ToFloat64(v)
		}
		if v, ok := row["last_run"]; ok {
			d.LastRun = util.ToFloat64(v)
		}
		daemons = append(daemons, d)
	}

	log.Debug().
		Str("collection", collection).
		Int("count", len(daemons)).
		Msg("[Wiederkehr] daemons listed")
	return daemons, nil
}

// ── Daemon Lifecycle Helpers ─────────────────────────────────────────────────

// CreateEnergyGuard creates a daemon that runs DIFFUSE on high-energy nodes
// to NietzscheDBtribute activation across the graph. This prevents energy hotspots
// that can destabilize the Poincare geometry.
func (w *WiederkehrAdapter) CreateEnergyGuard(ctx context.Context, collection string, threshold float64, interval string) error {
	return w.CreateDaemon(ctx, collection, CreateDaemonOpts{
		Name:          fmt.Sprintf("energy_guard_%s", collection),
		Label:         "",
		Alias:         "n",
		WhenCondition: fmt.Sprintf("n.energy > %.2f", threshold),
		ThenClause:    "DIFFUSE FROM n WITH t=[0.1, 1.0] MAX_HOPS 5",
		Interval:      interval,
		Energy:        0.9,
	})
}

// CreateDecayReaper creates a daemon that deletes nodes whose energy has
// fallen below a threshold — implementing natural forgetting.
func (w *WiederkehrAdapter) CreateDecayReaper(ctx context.Context, collection string, minEnergy float64, interval string) error {
	return w.CreateDaemon(ctx, collection, CreateDaemonOpts{
		Name:          fmt.Sprintf("decay_reaper_%s", collection),
		Label:         "",
		Alias:         "n",
		WhenCondition: fmt.Sprintf("n.energy < %.4f", minEnergy),
		ThenClause:    "DELETE n",
		Interval:      interval,
		Energy:        0.8,
	})
}

// CreateStaleNodeTagger creates a daemon that tags nodes that have not been
// accessed recently by setting a "stale" metadata flag.
func (w *WiederkehrAdapter) CreateStaleNodeTagger(ctx context.Context, collection string, maxAgeSecs float64, interval string) error {
	return w.CreateDaemon(ctx, collection, CreateDaemonOpts{
		Name:          fmt.Sprintf("stale_tagger_%s", collection),
		Label:         "",
		Alias:         "n",
		WhenCondition: fmt.Sprintf("NOW() - n.created_at > %.0f", maxAgeSecs),
		ThenClause:    "SET n.stale = true",
		Interval:      interval,
		Energy:        0.7,
	})
}

// ── Temporal Query Helpers (via Zaratustra echo snapshots) ───────────────────
//
// NietzscheDB's Zaratustra engine (Phase 2: Eternal Recurrence) creates temporal
// echo snapshots for high-energy nodes during evolution cycles. These can be
// queried via NQL to compare node states over time.

// QueryNodeHistory queries for temporal echo snapshots of a specific node.
// Echo snapshots are created by the Zaratustra Eternal Recurrence phase and
// stored as DreamSnapshot-type nodes linked to the original.
func (w *WiederkehrAdapter) QueryNodeHistory(ctx context.Context, collection, nodeID string) ([]map[string]interface{}, error) {
	log := logger.Nietzsche()

	// Query for DreamSnapshot nodes connected to the target node
	nql := `MATCH (n)-[e]->(snap) WHERE n.id = $node_id AND snap.node_type = "DreamSnapshot" RETURN snap`
	params := map[string]interface{}{
		"node_id": nodeID,
	}

	result, err := w.client.Query(ctx, nql, params, collection)
	if err != nil {
		log.Error().Err(err).
			Str("collection", collection).
			Str("node_id", nodeID).
			Msg("[Wiederkehr] query node history failed")
		return nil, fmt.Errorf("wiederkehr query node history: %w", err)
	}

	if result.Error != "" {
		return nil, fmt.Errorf("wiederkehr query node history: %s", result.Error)
	}

	snapshots := make([]map[string]interface{}, 0, len(result.Nodes))
	for _, node := range result.Nodes {
		snapshots = append(snapshots, NodeResultToMap(node))
	}

	log.Debug().
		Str("collection", collection).
		Str("node_id", nodeID).
		Int("snapshots", len(snapshots)).
		Msg("[Wiederkehr] node history retrieved")
	return snapshots, nil
}

// CompareNodeStates compares two nodes (or a node with its echo snapshot) by
// retrieving both and returning a diff of their energy, depth, and content.
func (w *WiederkehrAdapter) CompareNodeStates(ctx context.Context, collection, nodeIDA, nodeIDB string) (*NodeStateDiff, error) {
	log := logger.Nietzsche()

	nodeA, err := w.client.GetNode(ctx, nodeIDA, collection)
	if err != nil {
		return nil, fmt.Errorf("wiederkehr compare: get node A (%s): %w", nodeIDA, err)
	}
	if !nodeA.Found {
		return nil, fmt.Errorf("wiederkehr compare: node A (%s) not found", nodeIDA)
	}

	nodeB, err := w.client.GetNode(ctx, nodeIDB, collection)
	if err != nil {
		return nil, fmt.Errorf("wiederkehr compare: get node B (%s): %w", nodeIDB, err)
	}
	if !nodeB.Found {
		return nil, fmt.Errorf("wiederkehr compare: node B (%s) not found", nodeIDB)
	}

	diff := &NodeStateDiff{
		NodeIDA:     nodeIDA,
		NodeIDB:     nodeIDB,
		EnergyA:     nodeA.Energy,
		EnergyB:     nodeB.Energy,
		EnergyDelta: nodeB.Energy - nodeA.Energy,
		DepthA:      nodeA.Depth,
		DepthB:      nodeB.Depth,
		DepthDelta:  nodeB.Depth - nodeA.Depth,
		ContentA:    nodeA.Content,
		ContentB:    nodeB.Content,
	}

	log.Debug().
		Str("collection", collection).
		Str("node_a", nodeIDA).
		Str("node_b", nodeIDB).
		Float32("energy_delta", diff.EnergyDelta).
		Msg("[Wiederkehr] node state comparison complete")
	return diff, nil
}

// NodeStateDiff holds the difference between two node states.
type NodeStateDiff struct {
	NodeIDA     string
	NodeIDB     string
	EnergyA     float32
	EnergyB     float32
	EnergyDelta float32
	DepthA      float32
	DepthB      float32
	DepthDelta  float32
	ContentA    map[string]interface{}
	ContentB    map[string]interface{}
}

// ── Daemon Monitoring ────────────────────────────────────────────────────────

// GetDaemon retrieves a single daemon by name from the list.
// Returns nil if the daemon is not found.
func (w *WiederkehrAdapter) GetDaemon(ctx context.Context, collection, name string) (*DaemonInfo, error) {
	daemons, err := w.ListDaemons(ctx, collection)
	if err != nil {
		return nil, err
	}
	for _, d := range daemons {
		if d.Name == name {
			return &d, nil
		}
	}
	return nil, nil
}

// PruneDaemons drops all daemons whose energy has fallen below a threshold.
// Returns the names of dropped daemons.
func (w *WiederkehrAdapter) PruneDaemons(ctx context.Context, collection string, minEnergy float64) ([]string, error) {
	log := logger.Nietzsche()

	daemons, err := w.ListDaemons(ctx, collection)
	if err != nil {
		return nil, err
	}

	var pruned []string
	for _, d := range daemons {
		if d.Energy < minEnergy {
			if dropErr := w.DropDaemon(ctx, collection, d.Name); dropErr != nil {
				log.Warn().Err(dropErr).Str("daemon", d.Name).Msg("[Wiederkehr] failed to prune daemon")
				continue
			}
			pruned = append(pruned, d.Name)
		}
	}

	log.Info().
		Str("collection", collection).
		Int("pruned", len(pruned)).
		Msg("[Wiederkehr] daemon pruning complete")
	return pruned, nil
}

