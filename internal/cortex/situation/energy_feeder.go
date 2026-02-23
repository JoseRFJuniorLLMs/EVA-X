package situation

import (
	"context"
	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"
)

// EnergyFeeder translates clinical situational analysis into graph energy.
// This fulfills the "Simbiose AGI" feedback loop by feeding reflexive nodes.
type EnergyFeeder struct {
	nietzsche *nietzscheInfra.Client
}

// NewEnergyFeeder creates a new EnergyFeeder.
func NewEnergyFeeder(nietzsche *nietzscheInfra.Client) *EnergyFeeder {
	return &EnergyFeeder{
		nietzsche: nietzsche,
	}
}

// FeedReflexes injects energy into Action nodes based on clinical stressors.
func (ef *EnergyFeeder) FeedReflexes(ctx context.Context, patientID string, sit Situation, collection string) error {
	// Default increment
	energyIncrement := 0.05

	// Clinical stressor multipliers
	if contains(sit.Stressors, "crise") {
		energyIncrement = 0.4 // High sensitivity for emergency reflexes
	} else if contains(sit.Stressors, "hospital") || contains(sit.Stressors, "luto") {
		energyIncrement = 0.2 // Moderate sensitivity
	}

	// NQL to propagate energy specifically to clinical Action nodes.
	// We use the 'action' property presence as an indicator.
	nql := "MATCH (a) WHERE a.action IS NOT NULL SET a.energy = a.energy + $inc RETURN a"

	params := map[string]interface{}{
		"inc": energyIncrement,
	}

	_, err := ef.nietzsche.Query(ctx, nql, params, collection)
	return err
}
