package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
)

// MultimodalConfig configuração completa do sistema multimodal
type MultimodalConfig struct {
	// Master switch
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Feature flags
	Features MultimodalFeatures `json:"features" yaml:"features"`

	// Limits
	Limits MultimodalLimits `json:"limits" yaml:"limits"`

	// Quality settings
	Quality MultimodalQuality `json:"quality" yaml:"quality"`

	// Memory pipeline
	Memory MultimodalMemory `json:"memory" yaml:"memory"`

	// Retrieval
	Retrieval MultimodalRetrieval `json:"retrieval" yaml:"retrieval"`

	// Storage
	Storage MultimodalStorage `json:"storage" yaml:"storage"`
}

// MultimodalFeatures feature flags individuais
type MultimodalFeatures struct {
	ImageUpload     bool `json:"image_upload" yaml:"image_upload"`
	VideoUpload     bool `json:"video_upload" yaml:"video_upload"`
	VideoStreaming  bool `json:"video_streaming" yaml:"video_streaming"`
	VisualMemory    bool `json:"visual_memory" yaml:"visual_memory"`
	HybridRetrieval bool `json:"hybrid_retrieval" yaml:"hybrid_retrieval"`
}

// MultimodalLimits limites de tamanho e taxa
type MultimodalLimits struct {
	MaxImageSizeMB    int `json:"max_image_size_mb" yaml:"max_image_size_mb"`
	MaxVideoSizeMB    int `json:"max_video_size_mb" yaml:"max_video_size_mb"`
	VideoFrameRateFPS int `json:"video_frame_rate_fps" yaml:"video_frame_rate_fps"`
	MaxMemoryBufferMB int `json:"max_memory_buffer_mb" yaml:"max_memory_buffer_mb"`
}

// MultimodalQuality configurações de qualidade
type MultimodalQuality struct {
	ImageCompressionQuality int     `json:"image_compression_quality" yaml:"image_compression_quality"` // 1-100
	VideoCompressionQuality int     `json:"video_compression_quality" yaml:"video_compression_quality"` // 1-100
	KrylovCompressionRatio  float64 `json:"krylov_compression_ratio" yaml:"krylov_compression_ratio"`   // ex: 48 (3072/64)
}

// MultimodalMemory configuração do pipeline de memória
type MultimodalMemory struct {
	EnableEmbedding       bool `json:"enable_embedding" yaml:"enable_embedding"`
	EnableKrylovCompress  bool `json:"enable_krylov_compress" yaml:"enable_krylov_compress"`
	EnableStorage         bool `json:"enable_storage" yaml:"enable_storage"`
	FlushIntervalSeconds  int  `json:"flush_interval_seconds" yaml:"flush_interval_seconds"`
	BatchSize             int  `json:"batch_size" yaml:"batch_size"`
	KrylovWindowSize      int  `json:"krylov_window_size" yaml:"krylov_window_size"`
	ConsolidateIntervalMin int  `json:"consolidate_interval_min" yaml:"consolidate_interval_min"`
}

// MultimodalRetrieval configuração de retrieval híbrido
type MultimodalRetrieval struct {
	EnableVisualSearch bool    `json:"enable_visual_search" yaml:"enable_visual_search"`
	TextWeight         float64 `json:"text_weight" yaml:"text_weight"`
	VisualWeight       float64 `json:"visual_weight" yaml:"visual_weight"`
	TopKText           int     `json:"top_k_text" yaml:"top_k_text"`
	TopKVisual         int     `json:"top_k_visual" yaml:"top_k_visual"`
	MinScoreText       float32 `json:"min_score_text" yaml:"min_score_text"`
	MinScoreVisual     float32 `json:"min_score_visual" yaml:"min_score_visual"`
}

// MultimodalStorage configuração de storage
type MultimodalStorage struct {
	QdrantCollectionName string `json:"qdrant_collection_name" yaml:"qdrant_collection_name"`
	EnablePostgresBackup bool   `json:"enable_postgres_backup" yaml:"enable_postgres_backup"`
	EnableQdrantSync     bool   `json:"enable_qdrant_sync" yaml:"enable_qdrant_sync"`
}

// DefaultMultimodalConfig retorna configuração padrão (tudo desabilitado)
func DefaultMultimodalConfig() *MultimodalConfig {
	return &MultimodalConfig{
		Enabled: false,

		Features: MultimodalFeatures{
			ImageUpload:     false,
			VideoUpload:     false,
			VideoStreaming:  false,
			VisualMemory:    false,
			HybridRetrieval: false,
		},

		Limits: MultimodalLimits{
			MaxImageSizeMB:    7,
			MaxVideoSizeMB:    30,
			VideoFrameRateFPS: 1,
			MaxMemoryBufferMB: 100,
		},

		Quality: MultimodalQuality{
			ImageCompressionQuality: 85,
			VideoCompressionQuality: 75,
			KrylovCompressionRatio:  48.0, // 3072D / 64D
		},

		Memory: MultimodalMemory{
			EnableEmbedding:        true,
			EnableKrylovCompress:   true,
			EnableStorage:          true,
			FlushIntervalSeconds:   30,
			BatchSize:              10,
			KrylovWindowSize:       500,
			ConsolidateIntervalMin: 30,
		},

		Retrieval: MultimodalRetrieval{
			EnableVisualSearch: true,
			TextWeight:         0.7,
			VisualWeight:       0.3,
			TopKText:           5,
			TopKVisual:         3,
			MinScoreText:       0.6,
			MinScoreVisual:     0.5,
		},

		Storage: MultimodalStorage{
			QdrantCollectionName: "visual_memories_64d",
			EnablePostgresBackup: true,
			EnableQdrantSync:     true,
		},
	}
}

// LoadMultimodalConfigFromEnv carrega configuração de variáveis de ambiente
func LoadMultimodalConfigFromEnv() *MultimodalConfig {
	config := DefaultMultimodalConfig()

	// Master switch
	if val := os.Getenv("EVA_MULTIMODAL_ENABLED"); val != "" {
		config.Enabled = parseBool(val)
	}

	// Features
	if val := os.Getenv("EVA_MULTIMODAL_FEATURES_IMAGE_UPLOAD"); val != "" {
		config.Features.ImageUpload = parseBool(val)
	}
	if val := os.Getenv("EVA_MULTIMODAL_FEATURES_VIDEO_UPLOAD"); val != "" {
		config.Features.VideoUpload = parseBool(val)
	}
	if val := os.Getenv("EVA_MULTIMODAL_FEATURES_VIDEO_STREAMING"); val != "" {
		config.Features.VideoStreaming = parseBool(val)
	}
	if val := os.Getenv("EVA_MULTIMODAL_FEATURES_VISUAL_MEMORY"); val != "" {
		config.Features.VisualMemory = parseBool(val)
	}
	if val := os.Getenv("EVA_MULTIMODAL_FEATURES_HYBRID_RETRIEVAL"); val != "" {
		config.Features.HybridRetrieval = parseBool(val)
	}

	// Limits
	if val := os.Getenv("EVA_MULTIMODAL_LIMITS_MAX_IMAGE_SIZE_MB"); val != "" {
		config.Limits.MaxImageSizeMB = parseInt(val)
	}
	if val := os.Getenv("EVA_MULTIMODAL_LIMITS_MAX_VIDEO_SIZE_MB"); val != "" {
		config.Limits.MaxVideoSizeMB = parseInt(val)
	}
	if val := os.Getenv("EVA_MULTIMODAL_LIMITS_VIDEO_FRAME_RATE_FPS"); val != "" {
		config.Limits.VideoFrameRateFPS = parseInt(val)
	}

	// Quality
	if val := os.Getenv("EVA_MULTIMODAL_QUALITY_IMAGE_COMPRESSION"); val != "" {
		config.Quality.ImageCompressionQuality = parseInt(val)
	}

	// Memory
	if val := os.Getenv("EVA_MULTIMODAL_MEMORY_FLUSH_INTERVAL_SEC"); val != "" {
		config.Memory.FlushIntervalSeconds = parseInt(val)
	}
	if val := os.Getenv("EVA_MULTIMODAL_MEMORY_BATCH_SIZE"); val != "" {
		config.Memory.BatchSize = parseInt(val)
	}

	// Retrieval
	if val := os.Getenv("EVA_MULTIMODAL_RETRIEVAL_TEXT_WEIGHT"); val != "" {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			config.Retrieval.TextWeight = f
		}
	}
	if val := os.Getenv("EVA_MULTIMODAL_RETRIEVAL_VISUAL_WEIGHT"); val != "" {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			config.Retrieval.VisualWeight = f
		}
	}

	// Storage
	if val := os.Getenv("EVA_MULTIMODAL_STORAGE_QDRANT_COLLECTION"); val != "" {
		config.Storage.QdrantCollectionName = val
	}

	log.Printf("🎨 [MULTIMODAL_CONFIG] Loaded from env: enabled=%v, features=%+v",
		config.Enabled, config.Features)

	return config
}

// IsFeatureEnabled verifica se uma feature está habilitada (respeita master switch)
func (c *MultimodalConfig) IsFeatureEnabled(feature string) bool {
	if !c.Enabled {
		return false // Master switch off
	}

	switch feature {
	case "image_upload":
		return c.Features.ImageUpload
	case "video_upload":
		return c.Features.VideoUpload
	case "video_streaming":
		return c.Features.VideoStreaming
	case "visual_memory":
		return c.Features.VisualMemory
	case "hybrid_retrieval":
		return c.Features.HybridRetrieval
	default:
		return false
	}
}

// Validate valida configuração
func (c *MultimodalConfig) Validate() error {
	if c.Limits.MaxImageSizeMB < 1 || c.Limits.MaxImageSizeMB > 50 {
		return fmt.Errorf("max_image_size_mb must be between 1 and 50")
	}

	if c.Limits.MaxVideoSizeMB < 1 || c.Limits.MaxVideoSizeMB > 100 {
		return fmt.Errorf("max_video_size_mb must be between 1 and 100")
	}

	if c.Quality.ImageCompressionQuality < 1 || c.Quality.ImageCompressionQuality > 100 {
		return fmt.Errorf("image_compression_quality must be between 1 and 100")
	}

	if c.Retrieval.TextWeight+c.Retrieval.VisualWeight > 1.1 { // Tolerância de 0.1
		return fmt.Errorf("text_weight + visual_weight must sum to ~1.0")
	}

	return nil
}

// Helpers
func parseBool(s string) bool {
	return s == "true" || s == "1" || s == "yes"
}

func parseInt(s string) int {
	val, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return val
}
