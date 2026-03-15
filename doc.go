/*
Package salutespeech provides a comprehensive Go client for SaluteSpeech API by Sber.

Basic usage:

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

Features:
  - Automatic token management with caching
  - Configurable logging
  - Context support with timeouts
  - Retry with exponential backoff
  - Type-safe parameters
*/

package gigachat
