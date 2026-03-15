package entity

import (
	"encoding/json"
	"strings"
)

// ModelType represents the type of GigaChat model.
type ModelType string

const (
	// ModelTypeChat represents entity for chat completions.
	ModelTypeChat ModelType = "chat"
	// ModelTypeAICheck represents entity for AI content checking.
	ModelTypeAICheck ModelType = "aicheck"
	// ModelTypeEmbedder represents entity for text embeddings.
	ModelTypeEmbedder ModelType = "embedder"
)

// Model represents a GigaChat model.
type Model struct {
	// ID is the unique model identifier.
	ID string `json:"id"`
	// Object is the type of object ("model").
	Object string `json:"object"`
	// OwnedBy indicates the owner of the model.
	OwnedBy string `json:"owned_by"`
	// Type specifies the model type.
	Type ModelType `json:"type"`
	// IsPreview indicates if this is a preview version.
	IsPreview bool `json:"-"`
}

// UnmarshalJSON implements json.Unmarshaler and detects preview entity.
func (m *Model) UnmarshalJSON(data []byte) error {
	type Alias Model
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(m),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	m.IsPreview = strings.HasSuffix(m.ID, "-preview")
	return nil
}

// BaseName returns the model name without the preview suffix.
func (m *Model) BaseName() string {
	if m.IsPreview {
		return strings.TrimSuffix(m.ID, "-preview")
	}
	return m.ID
}

// ModelsResponse represents the API response for listing entity.
type ModelsResponse struct {
	// Data contains the list of entity.
	Data []Model `json:"data"`
	// Object is the type of object ("list").
	Object string `json:"object"`
}
