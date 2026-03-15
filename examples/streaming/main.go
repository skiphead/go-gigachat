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

	// Create GigaChat client
	giga, err := gigachat.NewClient(tokenMgr, gigachat.Config{
		Timeout: 60 * time.Second, // Longer timeout for streaming
	})
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Create streaming request
	req := &gigachat.ChatRequest{
		Model: gigachat.ModelGigaChat.String(),
		Messages: []gigachat.Message{
			{
				Role:    gigachat.RoleUser,
				Content: "Tell me a short story about a robot learning to paint.",
			},
		},
		Temperature: ptrFloat64(0.8),
		MaxTokens:   ptrInt32(200),
		// Stream is automatically set to true by CompletionStream
	}

	fmt.Println("Starting streaming response (press Ctrl+C to stop):")
	fmt.Println("---")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get streaming channels
	chunkChan, errChan := giga.CompletionStream(ctx, req)

	// Process stream
	for {
		select {
		case chunk, ok := <-chunkChan:
			if !ok {
				// Channel closed normally
				fmt.Println("\n---")
				fmt.Println("Stream finished")
				return
			}

			// Print content if available
			if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != nil {
				fmt.Print(*chunk.Choices[0].Delta.Content)
			}

			// Check for completion
			if len(chunk.Choices) > 0 && chunk.Choices[0].FinishReason != nil {
				fmt.Printf("\n\nFinish reason: %s\n", *chunk.Choices[0].FinishReason)
			}

			// Print usage if available (final chunk)
			if chunk.Usage != nil {
				fmt.Printf("Token usage: %+v\n", *chunk.Usage)
			}

		case err := <-errChan:
			if err != nil {
				log.Fatalf("Stream error: %v", err)
			}
			return

		case <-ctx.Done():
			log.Fatal("Timeout")
		}
	}
}

func ptrFloat64(v float64) *float64 {
	return &v
}

func ptrInt32(v int32) *int32 {
	return &v
}
