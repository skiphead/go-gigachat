Отличная идея! Вот подробный файл `QUICKSTART.md` с пошаговым руководством по началу работы с SDK:

```markdown
# Quick Start Guide for go-gigachat

This guide will help you get started with the GigaChat Go SDK in just a few minutes.

## Prerequisites

- Go 1.25.8 or higher
- GigaChat API credentials (Client ID and Client Secret)
- Basic understanding of Go and REST APIs

## Installation

First, install the package using `go get`:

```bash
go get github.com/skiphead/go-gigachat
```

## Step 1: Get Your Credentials

To use GigaChat API, you need:

1. **Client ID** - Your application identifier
2. **Client Secret** - Your secret key for authentication

You can obtain these from the [Sber Developers Portal](https://developers.sber.ru/).

## Step 2: Set Up Environment Variables

For security, store your credentials as environment variables:

```bash
export GIGACHAT_CLIENT_ID="your_client_id_here"
export GIGACHAT_CLIENT_SECRET="your_client_secret_here"
```

## Step 3: Create Your First Application

### Basic Chat Completion

Create a file `main.go`:

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"
    "time"

    "github.com/skiphead/go-gigachat"
    "github.com/skiphead/salutespeech/client"
)

func main() {
	// Get credentials from environment
	clientID := os.Getenv("GIGACHAT_CLIENT_ID")
	clientSecret := os.Getenv("GIGACHAT_CLIENT_SECRET")
	
	// Get credentials from environment
	authKey := client.GenerateBasicAuthKey(clientID, clientSecret)
	
	oauthClient, err := client.NewOAuthClient(client.Config{
		//OAuthURL: "https://ngw.devices.sberbank.ru:9443",
		AuthKey: authKey,                    // Base64-encoded client credentials
		Scope:   types.ScopeGigaChatAPIPers, // API access scope for speech recognition
		Timeout: 30 * time.Second,           // HTTP client timeout
	})
	if err != nil {
		log.Fatal(err)
	}
	
	// 1. Create token manager (handles authentication)
	tokenMgr := client.NewTokenManager(oauthClient, client.TokenManagerConfig{})


	// 2. Create GigaChat client
    giga, err := gigachat.NewClient(tokenMgr, gigachat.Config{
        Timeout: 30 * time.Second,
    })
    if err != nil {
        log.Fatalf("Failed to create client: %v", err)
    }

    // 3. Prepare chat request
    req := &gigachat.ChatRequest{
        Model: gigachat.ModelGigaChat.String(), // Use standard GigaChat model
        Messages: []gigachat.Message{
            {
                Role:    gigachat.RoleUser,
                Content: "What is the capital of France?",
            },
        },
        Temperature: ptrFloat64(0.7),
        MaxTokens:   ptrInt32(100),
    }

    fmt.Println("Sending request to GigaChat...")

    // 4. Send request and get response
    resp, err := giga.Completion(context.Background(), req)
    if err != nil {
        log.Fatalf("Completion failed: %v", err)
    }

    // 5. Print the response
    fmt.Printf("\nResponse: %s\n", resp.Choices[0].Message.Content)
    fmt.Printf("Tokens used: %d\n", resp.Usage.TotalTokens)
}

// Helper functions for pointers
func ptrFloat64(v float64) *float64 {
    return &v
}

func ptrInt32(v int32) *int32 {
    return &v
}
```

Run your application:

```bash
go run main.go
```

Expected output:
```
Sending request to GigaChat...

Response: The capital of France is Paris.
Tokens used: 42
```

## Step 4: Try More Examples

### Streaming Chat

For real-time responses, use streaming:

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"
    "time"

    "github.com/skiphead/go-gigachat"
    "github.com/skiphead/salutespeech/client"
)

func main() {
	authKey := client.GenerateBasicAuthKey("client_id", "client_secret")
	
	oauthClient, err := client.NewOAuthClient(client.Config{
		AuthKey: authKey,                   
		Scope:   types.ScopeGigaChatAPIPers,
		Timeout: 30 * time.Second,
	})
	if err != nil {
		log.Fatal(err)
	}
	
	tokenMgr := client.NewTokenManager(oauthClient, client.TokenManagerConfig{})

	giga, _ := gigachat.NewClient(tokenMgr, gigachat.Config{})

    req := &gigachat.ChatRequest{
        Model: gigachat.ModelGigaChat.String(),
        Messages: []gigachat.Message{
            {
                Role:    gigachat.RoleUser,
                Content: "Tell me a short story about a robot.",
            },
        },
    }

    ctx := context.Background()
    chunkChan, errChan := giga.CompletionStream(ctx, req)

    fmt.Println("Streaming response:")
    for {
        select {
        case chunk, ok := <-chunkChan:
            if !ok {
                return
            }
            if chunk.Choices[0].Delta.Content != nil {
                fmt.Print(*chunk.Choices[0].Delta.Content)
            }
        case err := <-errChan:
            if err != nil {
                log.Fatal(err)
            }
            return
        }
    }
}
```

### Working with Models

List available models:

```go
// Get all entity
models, err := giga.List(context.Background())
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Found %d entity:\n", len(models.Data))
for _, model := range models.Data {
    fmt.Printf("- %s (type: %s, preview: %v)\n", 
        model.ID, model.Type, model.IsPreview)
}

// Get only chat entity
chatModels, err := giga.ListChatModels(context.Background())
```

### File Upload and Analysis

Upload a file and use it in chat:

```go
// Upload a file
file, err := giga.UploadFileFromPath(
    context.Background(),
    "document.pdf",
    gigachat.FilePurposeGeneral,
    "my-app",
)
if err != nil {
    log.Fatal(err)
}

// Use file in chat
chatReq := gigachat.NewChatRequestWithTextFiles(
    gigachat.ModelGigaChat.String(),
    "Summarize this document",
    []string{file.ID},
    true, // Enable function calling
)

resp, err := giga.Completion(context.Background(), chatReq)
```

## Configuration Options

### Client Configuration

```go
client, err := gigachat.NewClient(tokenMgr, gigachat.Config{
    // Custom base URL (optional)
    BaseURL: "https://custom.url/api/v1",
    
    // Timeout for HTTP requests
    Timeout: 60 * time.Second,
    
    // Disable TLS verification (only for testing!)
    AllowInsecure: false,
    
    // Cache entity for 10 minutes
    ModelsCacheTTL: 10 * time.Minute,
    
    // Custom logger
    Logger: &customLogger{},
})
```

### Request Options

```go
resp, err := client.Completion(
    ctx,
    req,
    gigachat.WithClientID("my-app"),
    gigachat.WithRequestID("custom-request-id"),
    gigachat.WithSessionID("user-session-123"),
)
```

## Error Handling

Always handle errors properly:

```go
resp, err := client.Completion(ctx, req)
if err != nil {
    switch {
    case errors.Is(err, gigachat.ErrBadRequest):
        fmt.Println("Invalid request:", err)
    case errors.Is(err, gigachat.ErrModelNotFound):
        fmt.Println("Model not found:", err)
    default:
        fmt.Println("Unknown error:", err)
    }
    return
}
```

## Available Models

The SDK provides predefined model constants:

```go
// Production entity
gigachat.ModelGigaChat      // "GigaChat"
gigachat.ModelGigaChatPro   // "GigaChat-Pro"
gigachat.ModelGigaChatPlus  // "GigaChat-Plus"
gigachat.ModelGigaChat2Max  // "GigaChat-2-Max"

// Preview entity (with -preview suffix)
gigachat.ModelGigaChatPreview
gigachat.ModelGigaChatProPreview
```

## Next Steps

1. **Explore the [examples](./examples)** directory for more use cases
2. **Read the full [documentation](https://pkg.go.dev/github.com/skiphead/go-gigachat)** on pkg.go.dev
3. **Check the [API Reference](https://developers.sber.ru/docs/ru/gigachat/api/reference)** for GigaChat API details
4. **Join the community** for questions and contributions

## Troubleshooting

### Common Issues

1. **Authentication Failed**
   ```
   Error: get token: invalid client credentials
   ```
   Solution: Verify your CLIENT_ID and CLIENT_SECRET are correct.

2. **Timeout Error**
   ```
   Error: context deadline exceeded
   ```
   Solution: Increase timeout in client configuration.

3. **Model Not Found**
   ```
   Error: model not found: GigaChat
   ```
   Solution: Check available models with `client.List(ctx)`.

### Getting Help

- Open an issue on [GitHub](https://github.com/skiphead/go-gigachat/issues)
- Check the [GigaChat API documentation](https://developers.sber.ru/docs/ru/gigachat/api/overview)
- Contact Sber developer support

---

Happy coding! 🚀
```
