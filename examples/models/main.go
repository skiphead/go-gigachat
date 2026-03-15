package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/skiphead/go-gigachat"
	"github.com/skiphead/salutespeech/client"
	"github.com/skiphead/salutespeech/types"
)

func main() {
	// Generate Basic Authentication credentials from client ID and secret
	// These credentials are used to obtain OAuth tokens from the SaluteSpeech API
	authKey := client.GenerateBasicAuthKey("client_id", "client_secret")

	// Create OAuth client for token management
	// The OAuth client handles the authentication flow and token retrieval
	oauthClient, err := client.NewOAuthClient(client.Config{
		AuthKey: authKey,                    // Base64-encoded client credentials
		Scope:   types.ScopeGigaChatAPIPers, // API access scope for speech recognition
		Timeout: 30 * time.Second,           // HTTP client timeout
	})
	if err != nil {
		log.Fatal(err)
	}
	// Create token manager for automatic token refresh
	// The token manager handles token caching, refresh, and provides valid tokens for API requests
	tokenMgr := client.NewTokenManager(oauthClient, client.TokenManagerConfig{})

	// Create GigaChat client with caching
	giga, err := gigachat.NewClient(tokenMgr, gigachat.Config{
		Timeout:        30 * time.Second,
		ModelsCacheTTL: 10 * time.Minute, // Cache entity for 10 minutes
	})
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	ctx := context.Background()

	// Example 1: List all entity
	fmt.Println("=== All Models ===")
	allModels, err := giga.List(ctx)
	if err != nil {
		log.Fatalf("Failed to list entity: %v", err)
	}

	fmt.Printf("Total entity: %d\n", len(allModels.Data))
	for i, model := range allModels.Data {
		preview := ""
		if model.IsPreview {
			preview = " (preview)"
		}
		fmt.Printf("  %d. %s [%s]%s\n", i+1, model.ID, model.Type, preview)
	}

	// Example 2: Filter by type
	fmt.Println("\n=== Chat Models ===")
	chatModels, err := giga.ListChatModels(ctx)
	if err != nil {
		log.Fatalf("Failed to list chat entity: %v", err)
	}
	fmt.Printf("Chat entity (%d):\n", len(chatModels))
	for _, model := range chatModels {
		fmt.Printf("  - %s\n", model.ID)
	}

	fmt.Println("\n=== AI Check Models ===")
	checkModels, err := giga.ListAICheckModels(ctx)
	if err != nil {
		log.Fatalf("Failed to list AI check entity: %v", err)
	}
	fmt.Printf("AI check entity (%d):\n", len(checkModels))
	for _, model := range checkModels {
		fmt.Printf("  - %s\n", model.ID)
	}

	fmt.Println("\n=== Embedder Models ===")
	embedderModels, err := giga.ListEmbedderModels(ctx)
	if err != nil {
		log.Fatalf("Failed to list embedder entity: %v", err)
	}
	fmt.Printf("Embedder entity (%d):\n", len(embedderModels))
	for _, model := range embedderModels {
		fmt.Printf("  - %s\n", model.ID)
	}

	// Example 3: Preview vs Production
	fmt.Println("\n=== Preview Models ===")
	previewModels, err := giga.ListPreviewModels(ctx)
	if err != nil {
		log.Fatalf("Failed to list preview entity: %v", err)
	}
	fmt.Printf("Preview entity (%d):\n", len(previewModels))
	for _, model := range previewModels {
		fmt.Printf("  - %s (base: %s)\n", model.ID, model.BaseName())
	}

	fmt.Println("\n=== Production Models ===")
	prodModels, err := giga.ListProductionModels(ctx)
	if err != nil {
		log.Fatalf("Failed to list production entity: %v", err)
	}
	fmt.Printf("Production entity (%d):\n", len(prodModels))
	for _, model := range prodModels {
		fmt.Printf("  - %s\n", model.ID)
	}

	// Example 4: Get specific model
	fmt.Println("\n=== Get Specific Model ===")
	modelID := "GigaChat"
	model, err := giga.GetModel(ctx, modelID)
	if err != nil {
		log.Printf("Model %s not found: %v", modelID, err)
	} else {
		fmt.Printf("Model: %s\n", model.ID)
		fmt.Printf("  Type: %s\n", model.Type)
		fmt.Printf("  Owned by: %s\n", model.OwnedBy)
		fmt.Printf("  Is preview: %v\n", model.IsPreview)
		fmt.Printf("  Base name: %s\n", model.BaseName())
	}

	// Example 5: Using model constants
	fmt.Println("\n=== Predefined Models ===")
	fmt.Printf("Standard entity:\n")
	fmt.Printf("  GigaChat: %s\n", gigachat.ModelGigaChat)
	fmt.Printf("  GigaChat-Pro: %s\n", gigachat.ModelGigaChatPro)
	fmt.Printf("  GigaChat-Plus: %s\n", gigachat.ModelGigaChatPlus)
	fmt.Printf("  GigaChat-2-Max: %s\n", gigachat.ModelGigaChat2Max)

	fmt.Printf("\nPreview entity:\n")
	fmt.Printf("  GigaChat preview: %s\n", gigachat.ModelGigaChatPreview)
	fmt.Printf("  GigaChat-Pro preview: %s\n", gigachat.ModelGigaChatProPreview)

	// Example 6: Create custom model with preview
	fmt.Println("\n=== Custom Model ===")
	customModel := gigachat.NewModel("Custom-Model", true)
	fmt.Printf("Custom preview model: %s\n", customModel)

	// Create chat request with preview model
	chatReq := gigachat.NewChatRequest(customModel, []gigachat.Message{
		{Role: gigachat.RoleUser, Content: "Hello"},
	})
	fmt.Printf("Chat request with custom model: %+v\n", chatReq)

	// Example 7: Clear cache
	fmt.Println("\n=== Cache Management ===")
	fmt.Println("Clearing entity cache...")
	giga.ClearModelsCache()
	fmt.Println("Cache cleared")

	// This will fetch fresh data from API
	fmt.Println("Fetching fresh entity list...")
	freshModels, err := giga.List(ctx)
	if err != nil {
		log.Fatalf("Failed to list entity: %v", err)
	}
	fmt.Printf("Fetched %d entity\n", len(freshModels.Data))
}
