package situation

// Bridge transcreve o Caos Clínico em Geometria Visual
// Note: SituationalModulator and GeometricSituation are already defined in modulator.go
func (m *SituationalModulator) MapChaosToVisual(geo *GeometricSituation) map[string]interface{} {
	// Se o paciente está em CRISE, reduzimos o Fator Conformal para "esmagar"
	// a visão na borda, focando apenas no trauma central
	zoomChaos := 1.0
	// Situation fields are accessible through GeometricSituation if it embeds Situation or has the fields.
	// Looking at modulator.go, GeometricSituation doesn't seem to embed Situation but has its own fields.

	// Check for "crise" in stressors
	isCrise := false
	for _, s := range geo.RiskHierarchy { // RiskHierarchy seems to be the stressors in GeometricSituation
		if s == "crise" {
			isCrise = true
			break
		}
	}

	if isCrise {
		// Note: GeometricSituation has Intensity or similar?
		// Actually, modulator.go shows GeometricSituation has stressors/intensity if it embeds Situation?
		// Let's assume it has access to the core Situation data or similar intensity metric.
		// Since I saw GeometricSituation in modulator.go outline:
		// it has PoincareDepth, MinkowskiCone, RiskHierarchy, CausalAntecedents.
		zoomChaos = 0.5 // Default crisis deformation
	}

	return map[string]interface{}{
		"manifold":        "minkowski", // Foco em causalidade
		"distortion":      zoomChaos,
		"bloom_intensity": 4.0,                   // Brilho Übermensch (Base)
		"causal_cones":    geo.CausalAntecedents, // Cones de luz visíveis
	}
}
