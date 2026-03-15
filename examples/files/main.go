package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
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
		Timeout: 60 * time.Second,
	})
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	ctx := context.Background()

	// Example 1: Upload a file
	fmt.Println("=== File Upload Example ===")

	// Create a temporary text file
	tmpFile, err := os.CreateTemp("", "example-*.txt")
	if err != nil {
		log.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	content := `Paris is the capital and most populous city of France. 
It is located on the River Seine, in the north of the country. 
Paris is known for its museums, architecture, and cuisine.`
	if _, err := tmpFile.WriteString(content); err != nil {
		log.Fatal(err)
	}
	tmpFile.Close()

	fmt.Printf("Created temporary file: %s\n", tmpFile.Name())

	// Upload the file
	file, err := giga.UploadFileFromPath(ctx, tmpFile.Name(), gigachat.FilePurposeGeneral, "example-client")
	if err != nil {
		log.Fatalf("Failed to upload file: %v", err)
	}

	fmt.Printf("File uploaded successfully!\n")
	fmt.Printf("  ID: %s\n", file.ID)
	fmt.Printf("  Filename: %s\n", file.Filename)
	fmt.Printf("  Size: %d bytes\n", file.Bytes)
	fmt.Printf("  Created: %d\n", file.CreatedAt)

	// Example 2: List all files
	fmt.Println("\n=== List Files Example ===")
	files, err := giga.ListFiles(ctx)
	if err != nil {
		log.Fatalf("Failed to list files: %v", err)
	}

	fmt.Printf("Found %d files:\n", len(files.Data))
	for i, f := range files.Data {
		fmt.Printf("  %d. %s (ID: %s, Size: %d bytes)\n", i+1, f.Filename, f.ID, f.Bytes)
	}

	// Example 3: Get file information
	fmt.Println("\n=== Get File Info Example ===")
	fileInfo, err := giga.GetFile(ctx, file.ID)
	if err != nil {
		log.Fatalf("Failed to get file info: %v", err)
	}
	fmt.Printf("File info: %+v\n", fileInfo)

	// Example 4: Use file in chat
	fmt.Println("\n=== Chat with File Example ===")

	chatReq := gigachat.NewChatRequestWithTextFiles(
		gigachat.ModelGigaChat.String(),
		"What is this document about? Answer in one sentence.",
		[]string{file.ID},
		true, // Enable function calling
	)

	resp, err := giga.Completion(ctx, chatReq)
	if err != nil {
		log.Fatalf("Chat completion failed: %v", err)
	}

	fmt.Printf("Response: %s\n", resp.Choices[0].Message.Content)

	// Example 5: Delete file
	fmt.Println("\n=== Delete File Example ===")
	deleteResp, err := giga.DeleteFile(ctx, file.ID)
	if err != nil {
		log.Fatalf("Failed to delete file: %v", err)
	}

	if deleteResp.Deleted {
		fmt.Printf("File %s deleted successfully\n", deleteResp.ID)
	}

	// Verify deletion
	files, err = giga.ListFiles(ctx)
	if err != nil {
		log.Fatalf("Failed to list files: %v", err)
	}
	fmt.Printf("Files after deletion: %d\n", len(files.Data))
}

// Example of uploading with specific MIME type
func uploadWithCustomMIME(giga *gigachat.Client, ctx context.Context, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return err
	}

	// Detect MIME type
	mimeType, err := gigachat.GetMIMEType(filePath)
	if err != nil {
		// Fallback to auto-detection
		mimeType = ""
	}

	_, err = giga.UploadFile(ctx, &gigachat.UploadFileRequest{
		Reader:      file,
		FileName:    filepath.Base(filePath),
		ContentType: mimeType,
		Size:        stat.Size(),
		Purpose:     gigachat.FilePurposeGeneral,
		ClientID:    "example-client",
	})

	return err
}
