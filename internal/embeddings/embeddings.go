package embeddings

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strings"
)

// Provider generates embedding vectors from text.
type Provider interface {
	Embed(text string) ([]float32, error)
	Dimensions() int
}

// Active returns true if a real provider is configured.
func Active() bool {
	_, ok := get().(*noopProvider)
	return !ok
}

var cached Provider

// Get returns the configured provider (or a noop fallback).
func Get() Provider {
	if cached != nil {
		return cached
	}
	cached = get()
	return cached
}

func get() Provider {
	explicit := strings.ToLower(os.Getenv("TOT_EMBED_PROVIDER"))
	model := os.Getenv("TOT_EMBED_MODEL")

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
	return toFloat32(out.Embeddings[0]), nil
}

// --- Noop ---

type noopProvider struct{}

func (p *noopProvider) Dimensions() int              { return 0 }
func (p *noopProvider) Embed(string) ([]float32, error) { return nil, nil }

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
