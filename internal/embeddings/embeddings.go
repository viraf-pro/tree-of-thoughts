package embeddings

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/knights-analytics/hugot"
	"github.com/knights-analytics/hugot/pipelines"
)

// Provider generates embedding vectors from text.
type Provider interface {
	Embed(text string) ([]float32, error)
	Dimensions() int
}

// Active returns true if a real provider is configured.
func Active() bool {
	_, ok := Get().(*noopProvider)
	return !ok
}

var (
	cached     Provider
	cachedOnce sync.Once
)

// Get returns the configured provider (or a noop fallback).
// Safe for concurrent use.
func Get() Provider {
	cachedOnce.Do(func() {
		cached = get()
	})
	return cached
}

// SetProvider overrides the cached provider. For testing only.
func SetProvider(p Provider) {
	cachedOnce.Do(func() {}) // ensure sync.Once is spent
	cached = p
}

func get() Provider {
	explicit := strings.ToLower(os.Getenv("TOT_EMBED_PROVIDER"))
	model := os.Getenv("TOT_EMBED_MODEL")

	// Explicit API providers take priority
	if explicit == "openai" || (explicit == "" && os.Getenv("OPENAI_API_KEY") != "") {
		if model == "" {
			model = "text-embedding-3-small"
		}
		return &openaiProvider{key: os.Getenv("OPENAI_API_KEY"), model: model}
	}
	if explicit == "voyage" || (explicit == "" && os.Getenv("VOYAGE_API_KEY") != "") {
		if model == "" {
			model = "voyage-3-lite"
		}
		return &voyageProvider{key: os.Getenv("VOYAGE_API_KEY"), model: model}
	}
	if explicit == "ollama" || (explicit == "" && os.Getenv("OLLAMA_BASE_URL") != "") {
		base := os.Getenv("OLLAMA_BASE_URL")
		if base == "" {
			base = "http://localhost:11434"
		}
		if model == "" {
			model = "mxbai-embed-large"
		}
		return &ollamaProvider{base: base, model: model}
	}

	// No API key set — try local on-device embeddings as default.
	// This gives semantic search out of the box without any configuration.
	if explicit == "local" || explicit == "" {
		p, err := newLocalProvider(model)
		if err != nil {
			if explicit == "local" {
				// User explicitly asked for local — fail loud
				log.Printf("[embeddings] local provider failed to initialize: %v", err)
			}
			// Else silently fall through to noop
		} else {
			return p
		}
	}

	return &noopProvider{}
}

// --- OpenAI ---

type openaiProvider struct {
	key, model string
}

func (p *openaiProvider) Dimensions() int {
	if strings.Contains(p.model, "3-small") {
		return 1536
	}
	return 3072
}

func (p *openaiProvider) Embed(text string) ([]float32, error) {
	body, _ := json.Marshal(map[string]any{"input": text, "model": p.model})
	req, _ := http.NewRequest("POST", "https://api.openai.com/v1/embeddings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.key)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openai %d: %s", resp.StatusCode, b)
	}
	var out struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&out)
	if len(out.Data) == 0 {
		return nil, fmt.Errorf("openai returned empty embedding data")
	}
	return toFloat32(out.Data[0].Embedding), nil
}

// --- Voyage ---

type voyageProvider struct {
	key, model string
}

func (p *voyageProvider) Dimensions() int { return 512 }

func (p *voyageProvider) Embed(text string) ([]float32, error) {
	body, _ := json.Marshal(map[string]any{"input": []string{text}, "model": p.model})
	req, _ := http.NewRequest("POST", "https://api.voyageai.com/v1/embeddings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.key)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("voyage %d: %s", resp.StatusCode, b)
	}
	var out struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&out)
	if len(out.Data) == 0 {
		return nil, fmt.Errorf("voyage returned empty embedding data")
	}
	return toFloat32(out.Data[0].Embedding), nil
}

// --- Ollama ---

type ollamaProvider struct {
	base, model string
}

func (p *ollamaProvider) Dimensions() int { return 1024 }

func (p *ollamaProvider) Embed(text string) ([]float32, error) {
	body, _ := json.Marshal(map[string]any{"model": p.model, "input": text})
	resp, err := http.Post(p.base+"/api/embed", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama %d: %s", resp.StatusCode, b)
	}
	var out struct {
		Embeddings [][]float64 `json:"embeddings"`
	}
	json.NewDecoder(resp.Body).Decode(&out)
	if len(out.Embeddings) == 0 {
		return nil, fmt.Errorf("ollama returned empty embedding data")
	}
	return toFloat32(out.Embeddings[0]), nil
}

// --- Noop ---

type noopProvider struct{}

func (p *noopProvider) Dimensions() int                   { return 0 }
func (p *noopProvider) Embed(string) ([]float32, error)   { return nil, nil }

// --- Local on-device embeddings via Hugot (pure Go ONNX backend) ---

const (
	defaultLocalModel = "sentence-transformers/all-MiniLM-L6-v2"
	localDimensions   = 384 // all-MiniLM-L6-v2 output dimension
)

type LocalProvider struct {
	session  *hugot.Session
	pipeline *pipelines.FeatureExtractionPipeline
	mu       sync.Mutex // Hugot pipelines are not goroutine-safe
	dims     int
}

// newLocalProvider initializes the Hugot session with pure Go backend.
// On first run it downloads the model from HuggingFace (~22MB) and caches it
// in ~/.tot-mcp/models/. Subsequent starts load from cache in <1 second.
func newLocalProvider(model string) (*LocalProvider, error) {
	if model == "" {
		model = defaultLocalModel
	}

	// Cache directory for downloaded models
	cacheDir := os.Getenv("TOT_MODEL_CACHE")
	if cacheDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("cannot determine home directory: %w", err)
		}
		cacheDir = filepath.Join(home, ".tot-mcp", "models")
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, fmt.Errorf("cannot create model cache dir %s: %w", cacheDir, err)
	}

	// Download model from HuggingFace if not already cached
	modelPath, err := hugot.DownloadModel(model, cacheDir, hugot.NewDownloadOptions())
	if err != nil {
		return nil, fmt.Errorf("hugot model download failed: %w", err)
	}

	// Initialize Hugot with pure Go backend (zero CGO)
	session, err := hugot.NewGoSession()
	if err != nil {
		return nil, fmt.Errorf("hugot session init failed: %w", err)
	}

	// Load the embedding model
	pipeline, err := hugot.NewPipeline(session, hugot.FeatureExtractionConfig{
		ModelPath:    modelPath,
		Name:         "tot-embedding",
		OnnxFilename: "model.onnx",
		Options:      []hugot.FeatureExtractionOption{pipelines.WithOutputName("last_hidden_state")},
	})
	if err != nil {
		session.Destroy()
		return nil, fmt.Errorf("hugot pipeline init for %s failed: %w", model, err)
	}

	log.Printf("[embeddings] local provider ready: %s (%d-dim, pure Go backend)", model, localDimensions)

	return &LocalProvider{
		session:  session,
		pipeline: pipeline,
		dims:     localDimensions,
	}, nil
}

func (p *LocalProvider) Dimensions() int { return p.dims }

func (p *LocalProvider) Embed(text string) ([]float32, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	result, err := p.pipeline.RunPipeline([]string{text})
	if err != nil {
		return nil, fmt.Errorf("local embed failed: %w", err)
	}
	if len(result.Embeddings) == 0 || len(result.Embeddings[0]) == 0 {
		return nil, fmt.Errorf("local embed returned empty result")
	}

	// Hugot v0.5+ returns sentence-level embeddings (already mean-pooled).
	// result.Embeddings is [][]float32 — one vector per input text.
	embedding := result.Embeddings[0]

	// L2 normalize for cosine similarity
	var norm float64
	for _, v := range embedding {
		norm += float64(v) * float64(v)
	}
	norm = math.Sqrt(norm)
	if norm > 0 {
		for d := range embedding {
			embedding[d] = float32(float64(embedding[d]) / norm)
		}
	}

	return embedding, nil
}

// Destroy cleans up the Hugot session. Call on graceful shutdown.
func (p *LocalProvider) Destroy() {
	if p.session != nil {
		p.session.Destroy()
	}
}

// --- Pure-Go cosine similarity (no sqlite-vector needed) ---

// CosineSimilarity computes cosine similarity between two vectors.
func CosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	denom := math.Sqrt(normA) * math.Sqrt(normB)
	if denom == 0 {
		return 0
	}
	return dot / denom
}

func toFloat32(in []float64) []float32 {
	out := make([]float32, len(in))
	for i, v := range in {
		out[i] = float32(v)
	}
	return out
}
