// Package api - Associations Routes
// API endpoints para visualização de associações (Edge Zones)
// Fase C do plano de implementação
package api

import (
	"net/http"
	"strconv"

	"eva-mind/internal/hippocampus/memory"

	"github.com/gin-gonic/gin"
)

// AssociationsHandler handler para rotas de associações
type AssociationsHandler struct {
	edgeClassifier *memory.EdgeClassifier
}

// NewAssociationsHandler cria um novo handler
func NewAssociationsHandler(edgeClassifier *memory.EdgeClassifier) *AssociationsHandler {
	return &AssociationsHandler{
		edgeClassifier: edgeClassifier,
	}
}

// RegisterRoutes registra as rotas de associações
func (h *AssociationsHandler) RegisterRoutes(router *gin.Engine) {
	associations := router.Group("/api/v1/associations")
	{
		associations.GET("/consolidated/:patient_id", h.GetConsolidated)
		associations.GET("/emerging/:patient_id", h.GetEmerging)
		associations.GET("/weak/:patient_id", h.GetWeak)
		associations.GET("/statistics/:patient_id", h.GetStatistics)
		associations.POST("/prune/:patient_id", h.PruneWeak)
	}
}

// GetConsolidated retorna associações consolidadas
// GET /api/v1/associations/consolidated/:patient_id
func (h *AssociationsHandler) GetConsolidated(c *gin.Context) {
	patientID, err := strconv.ParseInt(c.Param("patient_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid patient_id"})
		return
	}

	edges, err := h.edgeClassifier.GetConsolidatedEdges(c.Request.Context(), patientID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"patient_id":        patientID,
		"zone":              "consolidated",
		"description":       "Strong, well-established associations (weight > 0.7)",
		"action":            "Automatically preloaded in Gemini context",
		"count":             len(edges),
		"associations":      edges,
	})
}

// GetEmerging retorna associações emergentes
// GET /api/v1/associations/emerging/:patient_id
func (h *AssociationsHandler) GetEmerging(c *gin.Context) {
	patientID, err := strconv.ParseInt(c.Param("patient_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid patient_id"})
		return
	}

	edges, err := h.edgeClassifier.GetEmergingEdges(c.Request.Context(), patientID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"patient_id":        patientID,
		"zone":              "emerging",
		"description":       "Associations being formed (0.3 < weight < 0.7)",
		"action":            "Review with caregiver for confirmation",
		"count":             len(edges),
		"associations":      edges,
		"suggestion":        "Review these patterns with the caregiver to confirm or reject",
	})
}

// GetWeak retorna associações fracas (candidatas a pruning)
// GET /api/v1/associations/weak/:patient_id
func (h *AssociationsHandler) GetWeak(c *gin.Context) {
	patientID, err := strconv.ParseInt(c.Param("patient_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid patient_id"})
		return
	}

	edges, err := h.edgeClassifier.GetWeakEdges(c.Request.Context(), patientID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"patient_id":        patientID,
		"zone":              "weak",
		"description":       "Weak or decaying associations (weight < 0.3)",
		"action":            "Candidate for periodic pruning",
		"count":             len(edges),
		"associations":      edges,
		"warning":           "These associations may be pruned automatically if not reactivated",
	})
}

// GetStatistics retorna estatísticas das zonas
// GET /api/v1/associations/statistics/:patient_id
func (h *AssociationsHandler) GetStatistics(c *gin.Context) {
	patientID, err := strconv.ParseInt(c.Param("patient_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid patient_id"})
		return
	}

	stats, err := h.edgeClassifier.GetZoneStatistics(c.Request.Context(), patientID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	totalEdges := stats.ConsolidatedCount + stats.EmergingCount + stats.WeakCount

	c.JSON(http.StatusOK, gin.H{
		"patient_id": patientID,
		"zones": gin.H{
			"consolidated": gin.H{
				"count":      stats.ConsolidatedCount,
				"percentage": calculatePercentage(stats.ConsolidatedCount, totalEdges),
				"threshold":  "> 0.7",
			},
			"emerging": gin.H{
				"count":      stats.EmergingCount,
				"percentage": calculatePercentage(stats.EmergingCount, totalEdges),
				"threshold":  "0.3 - 0.7",
			},
			"weak": gin.H{
				"count":      stats.WeakCount,
				"percentage": calculatePercentage(stats.WeakCount, totalEdges),
				"threshold":  "< 0.3",
			},
		},
		"total_edges": totalEdges,
		"avg_weight":  stats.AvgWeight,
		"max_weight":  stats.MaxWeight,
		"min_weight":  stats.MinWeight,
		"timestamp":   stats.Timestamp,
	})
}

// PruneWeak executa pruning de associações fracas
// POST /api/v1/associations/prune/:patient_id
func (h *AssociationsHandler) PruneWeak(c *gin.Context) {
	patientID, err := strconv.ParseInt(c.Param("patient_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid patient_id"})
		return
	}

	result, err := h.edgeClassifier.PruneWeakEdges(c.Request.Context(), patientID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":  err.Error(),
			"result": result,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"patient_id":   patientID,
		"action":       "pruning_completed",
		"edges_pruned": result.EdgesPruned,
		"threshold":    result.Threshold,
		"pruning_age":  result.PruningAge,
		"timestamp":    result.Timestamp,
		"message":      "Weak associations pruned successfully",
	})
}

// Helper functions

func calculatePercentage(count, total int) float64 {
	if total == 0 {
		return 0.0
	}
	return float64(count) / float64(total) * 100.0
}
