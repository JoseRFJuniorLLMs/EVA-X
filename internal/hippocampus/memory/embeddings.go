package memory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	// Google AI Studio (API Key AIzaSy...) - gemini-embedding-001: 3072 dims
	geminiEmbeddingEndpoint = "https://generativelanguage.googleapis.com/v1beta/models/gemini-embedding-001:embedContent"

	// Vertex AI (API Key AQ...) - gemini-embedding-001: 3072 dims (QUALIDADE MÁXIMA)
	vertexEmbeddingEndpoint = "https://aiplatform.googleapis.com/v1/publishers/google/models/gemini-embedding-001:predict"

	// Retry config
	maxRetries       = 5
	initialBackoff   = 5 * time.Second
	maxBackoff       = 1 * time.Minute
	backoffMultipler = 1.5
)

// AuthMode define o tipo de autenticação
type AuthMode int

const (
	AuthModeGoogleAI AuthMode = iota // Google AI Studio (AIzaSy...) - 3072 dims
	AuthModeVertexAI                 // Vertex AI (AQ...) - 3072 dims (gemini-embedding-001)
)

// EmbeddingService gera embeddings usando Gemini/Vertex API
type EmbeddingService struct {
	APIKey          string
	AuthMode        AuthMode
	ExpectedDim     int
	HTTPClient      *http.Client
}

// NewEmbeddingService cria um novo serviço de embeddings
// Detecta automaticamente se é Google AI Studio ou Vertex AI
func NewEmbeddingService(credential string) *EmbeddingService {
	svc := &EmbeddingService{
		APIKey: credential,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	// Detecta tipo de credencial
	if strings.HasPrefix(credential, "AIza") {
		svc.AuthMode = AuthModeGoogleAI
		svc.ExpectedDim = 3072
		log.Println("🔑 [EMBEDDING] Usando Google AI Studio (3072 dims)")
	} else if strings.HasPrefix(credential, "AQ.") {
		svc.AuthMode = AuthModeVertexAI
		svc.ExpectedDim = 3072
		log.Println("🔐 [EMBEDDING] Usando Vertex AI gemini-embedding-001 (3072 dims)")
	} else {
		// Assume Vertex AI para outras chaves
		svc.AuthMode = AuthModeVertexAI
		svc.ExpectedDim = 3072
		log.Println("🔐 [EMBEDDING] Usando Vertex AI (3072 dims) - formato desconhecido")
	}

	return svc
}

// NewEmbeddingServiceFromEnv cria serviço a partir de variáveis de ambiente
// Prioriza VERTEX_API_KEY sobre GOOGLE_API_KEY
func NewEmbeddingServiceFromEnv() *EmbeddingService {
	// Primeiro tenta Vertex AI API Key
	if apiKey := os.Getenv("VERTEX_API_KEY"); apiKey != "" {
		return NewEmbeddingService(apiKey)
	}
	// Fallback para Google AI Studio
	if apiKey := os.Getenv("GOOGLE_API_KEY"); apiKey != "" {
		return NewEmbeddingService(apiKey)
	}
	log.Println("⚠️ [EMBEDDING] Nenhuma credencial encontrada!")
	return &EmbeddingService{
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		ExpectedDim: 768,
	}
}

// ========== Request/Response para Google AI Studio ==========

type googleAIRequest struct {
	Content struct {
		Parts []struct {
			Text string `json:"text"`
		} `json:"parts"`
	} `json:"content"`
	OutputDimensionality int `json:"outputDimensionality,omitempty"`
}

type googleAIResponse struct {
	Embedding struct {
		Values []float32 `json:"values"`
	} `json:"embedding"`
}

// ========== Request/Response para Vertex AI ==========

type vertexAIRequest struct {
	Instances []struct {
		Content string `json:"content"`
	} `json:"instances"`
	Parameters struct {
		OutputDimensionality int `json:"outputDimensionality"`
	} `json:"parameters"`
}

type vertexAIResponse struct {
	Predictions []struct {
		Embeddings struct {
			Values []float32 `json:"values"`
		} `json:"embeddings"`
	} `json:"predictions"`
}

// GenerateEmbedding gera um vetor de embedding para o texto fornecido
func (e *EmbeddingService) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	// Truncar texto se muito longo
	if len(text) > 8000 {
		text = text[:8000]
	}

	var url string
	var jsonData []byte
	var err error

	if e.AuthMode == AuthModeGoogleAI {
		// Google AI Studio format
		reqBody := googleAIRequest{}
		reqBody.Content.Parts = []struct {
			Text string `json:"text"`
		}{{Text: text}}
		reqBody.OutputDimensionality = 3072

		jsonData, err = json.Marshal(reqBody)
		if err != nil {
			return nil, fmt.Errorf("erro ao serializar request: %w", err)
		}
		url = fmt.Sprintf("%s?key=%s", geminiEmbeddingEndpoint, e.APIKey)
	} else {
		// Vertex AI format com gemini-embedding-001 (3072 dims)
		reqBody := vertexAIRequest{
			Instances: []struct {
				Content string `json:"content"`
			}{{Content: text}},
		}
		reqBody.Parameters.OutputDimensionality = 3072

		jsonData, err = json.Marshal(reqBody)
		if err != nil {
			return nil, fmt.Errorf("erro ao serializar request: %w", err)
		}
		url = fmt.Sprintf("%s?key=%s", vertexEmbeddingEndpoint, e.APIKey)
	}

	// Retry loop com backoff exponencial
	var lastErr error
	backoff := initialBackoff

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			log.Printf("⏳ [EMBEDDING] Retry %d/%d, aguardando %v...", attempt, maxRetries, backoff)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
			backoff = time.Duration(float64(backoff) * backoffMultipler)
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}

		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
		if err != nil {
			return nil, fmt.Errorf("erro ao criar request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := e.HTTPClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("erro na requisição HTTP: %w", err)
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		// Rate limit - retry
		if resp.StatusCode == 429 {
			lastErr = fmt.Errorf("rate limit (429)")
			continue
		}

		// Outros erros - não retry
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("API retornou status %d: %s", resp.StatusCode, string(body))
		}

		// Parse response baseado no modo
		var values []float32
		if e.AuthMode == AuthModeGoogleAI {
			var result googleAIResponse
			if err := json.Unmarshal(body, &result); err != nil {
				return nil, fmt.Errorf("erro ao decodificar resposta: %w", err)
			}
			values = result.Embedding.Values
		} else {
			var result vertexAIResponse
			if err := json.Unmarshal(body, &result); err != nil {
				return nil, fmt.Errorf("erro ao decodificar resposta: %w", err)
			}
			if len(result.Predictions) == 0 {
				return nil, fmt.Errorf("nenhuma prediction retornada")
			}
			values = result.Predictions[0].Embeddings.Values
		}

		if len(values) == 0 {
			return nil, fmt.Errorf("embedding vazio retornado pela API")
		}

		return values, nil
	}

	return nil, fmt.Errorf("falhou após %d tentativas: %w", maxRetries, lastErr)
}

// GenerateEmbeddingWithLog igual ao GenerateEmbedding mas sempre loga sucesso
func (e *EmbeddingService) GenerateEmbeddingWithLog(ctx context.Context, text string) ([]float32, error) {
	emb, err := e.GenerateEmbedding(ctx, text)
	if err != nil {
		return nil, err
	}
	preview := text
	if len(preview) > 50 {
		preview = preview[:50] + "..."
	}
	preview = strings.ReplaceAll(preview, "\n", " ")
	log.Printf("✅ [EMBEDDING] %d dims: %s", len(emb), preview)
	return emb, nil
}

// GenerateBatch gera embeddings para múltiplos textos
func (e *EmbeddingService) GenerateBatch(ctx context.Context, texts []string) ([][]float32, error) {
	embeddings := make([][]float32, len(texts))

	for i, text := range texts {
		emb, err := e.GenerateEmbedding(ctx, text)
		if err != nil {
			return nil, fmt.Errorf("erro no texto %d: %w", i, err)
		}
		embeddings[i] = emb

		// Rate limiting
		if i < len(texts)-1 {
			time.Sleep(200 * time.Millisecond) // Vertex AI tem mais quota
		}
	}

	return embeddings, nil
}

// GetExpectedDimension retorna a dimensão esperada para o modo atual
func (e *EmbeddingService) GetExpectedDimension() int {
	return e.ExpectedDim
}
