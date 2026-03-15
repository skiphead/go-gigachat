// Package gigachat provides a Go client SDK for the GigaChat API.
// It supports chat completions, models management, file uploads, and streaming.
package gigachat

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/skiphead/go-gigachat/entity"
	"github.com/skiphead/salutespeech/client"
	"github.com/skiphead/salutespeech/types"
	"golang.org/x/sync/singleflight"
)

// RequestOptions contains options for API requests.
type RequestOptions struct {
	ClientID  string
	RequestID string
	SessionID string
}

// RequestOption is a function that sets request options.
type RequestOption func(*RequestOptions)

// WithClientID sets the X-Client-ID header.
func WithClientID(clientID string) RequestOption {
	return func(o *RequestOptions) {
		o.ClientID = clientID
	}
}

// WithRequestID sets the X-Request-ID header.
// If not provided, a random UUID will be generated.
func WithRequestID(requestID string) RequestOption {
	return func(o *RequestOptions) {
		o.RequestID = requestID
	}
}

// WithSessionID sets the X-Session-ID header.
// If not provided, a random UUID will be generated.
func WithSessionID(sessionID string) RequestOption {
	return func(o *RequestOptions) {
		o.SessionID = sessionID
	}
}

type modelsCache struct {
	models    *entity.ModelsResponse
	timestamp time.Time
}

// Client is the GigaChat API client.
type Client struct {
	httpClient        *http.Client
	baseURLModels     string
	baseURLCompletion string
	baseURLFiles      string
	tokenMgr          *client.TokenManager
	logger            types.Logger
	cache             modelsCache
	cacheMu           sync.RWMutex
	cacheTTL          time.Duration
	singleflight      singleflight.Group
}

// Config configures the GigaChat client.
type Config struct {
	// BaseURL for all APIs (default: https://gigachat.devices.sberbank.ru/api/v1).
	BaseURL string
	// BaseURLModels overrides the entity API URL.
	BaseURLModels string
	// BaseURLCompletion overrides the chat completions API URL.
	BaseURLCompletion string
	// BaseURLFiles overrides the files API URL.
	BaseURLFiles string
	// AllowInsecure disables TLS verification (not recommended for production).
	AllowInsecure bool
	// Timeout for HTTP requests (default: 30s).
	Timeout time.Duration
	// Logger for debug output (optional).
	Logger types.Logger
	// ModelsCacheTTL sets how long to cache entity list (0 disables caching, default: 5m).
	ModelsCacheTTL time.Duration
}

// NewClient creates a new GigaChat client.
func NewClient(tokenMgr *client.TokenManager, cfg Config) (*Client, error) {
	if tokenMgr == nil {
		return nil, ErrTokenManagerRequired
	}

	logger := cfg.Logger
	if logger == nil {
		logger = types.NoopLogger{}
	}

	// Set default base URL if not provided
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = types.DefaultBaseURL
	}

	// Configure API endpoints
	urlModels := cfg.BaseURLModels
	if urlModels == "" {
		urlModels = baseURL + "/models"
	}

	urlCompletion := cfg.BaseURLCompletion
	if urlCompletion == "" {
		urlCompletion = baseURL + "/chat/completions"
	}

	urlFiles := cfg.BaseURLFiles
	if urlFiles == "" {
		urlFiles = baseURL + "/files"
	}

	// Set default timeout
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	// Set default cache TTL
	cacheTTL := cfg.ModelsCacheTTL
	if cacheTTL == 0 {
		cacheTTL = 5 * time.Minute
	}

	// Configure HTTP transport
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: cfg.AllowInsecure,
		},
	}

	if cfg.AllowInsecure {
		logger.Warn("TLS verification disabled - this is not secure for production")
	}

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}

	return &Client{
		httpClient:        httpClient,
		baseURLModels:     urlModels,
		baseURLCompletion: urlCompletion,
		baseURLFiles:      urlFiles,
		tokenMgr:          tokenMgr,
		logger:            logger,
		cacheTTL:          cacheTTL,
	}, nil
}

// ==================== Models API ====================

// List returns all available models with caching.
func (c *Client) List(ctx context.Context) (*entity.ModelsResponse, error) {
	// Check cache if enabled
	if c.cacheTTL > 0 {
		c.cacheMu.RLock()
		cached := c.cache.models
		timestamp := c.cache.timestamp
		c.cacheMu.RUnlock()

		if cached != nil && time.Since(timestamp) < c.cacheTTL {
			c.logger.Debug("using cached models list (%d models)", len(cached.Data))
			return cached, nil
		}
	}

	// Use singleflight to prevent cache stampede
	result, err, _ := c.singleflight.Do("list_models", func() (interface{}, error) {
		c.logger.Debug("fetching models from API")
		return c.fetchModels(ctx)
	})

	if err != nil {
		return nil, err
	}

	models := result.(*entity.ModelsResponse)

	// Update cache if enabled
	if c.cacheTTL > 0 {
		c.cacheMu.Lock()
		c.cache.models = models
		c.cache.timestamp = time.Now()
		c.cacheMu.Unlock()
	}

	return models, nil
}

// fetchModels makes the actual API call to get models.
func (c *Client) fetchModels(ctx context.Context) (*entity.ModelsResponse, error) {
	authHeader, err := c.tokenMgr.GetTokenWithHeader(ctx)
	if err != nil {
		return nil, fmt.Errorf("get token: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURLModels, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", authHeader)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("models request: %w", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			return
		}
	}(resp.Body)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("models error %d: %s", resp.StatusCode, string(body))
	}

	var modelsResp entity.ModelsResponse
	if err := json.Unmarshal(body, &modelsResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return &modelsResp, nil
}

// ClearModelsCache clears the cached models list.
func (c *Client) ClearModelsCache() {
	c.cacheMu.Lock()
	c.cache.models = nil
	c.cache.timestamp = time.Time{}
	c.cacheMu.Unlock()
	c.logger.Debug("models cache cleared")
}

// ListChatModels returns only chat models.
func (c *Client) ListChatModels(ctx context.Context) ([]entity.Model, error) {
	models, err := c.List(ctx)
	if err != nil {
		return nil, err
	}

	var result []entity.Model
	for _, model := range models.Data {
		if model.Type == entity.ModelTypeChat {
			result = append(result, model)
		}
	}
	return result, nil
}

// ListAICheckModels returns only AI check models.
func (c *Client) ListAICheckModels(ctx context.Context) ([]entity.Model, error) {
	models, err := c.List(ctx)
	if err != nil {
		return nil, err
	}

	var result []entity.Model
	for _, model := range models.Data {
		if model.Type == entity.ModelTypeAICheck {
			result = append(result, model)
		}
	}
	return result, nil
}

// ListEmbedderModels returns only embedder models.
func (c *Client) ListEmbedderModels(ctx context.Context) ([]entity.Model, error) {
	models, err := c.List(ctx)
	if err != nil {
		return nil, err
	}

	var result []entity.Model
	for _, model := range models.Data {
		if model.Type == entity.ModelTypeEmbedder {
			result = append(result, model)
		}
	}
	return result, nil
}

// ListPreviewModels returns only preview models.
func (c *Client) ListPreviewModels(ctx context.Context) ([]entity.Model, error) {
	models, err := c.List(ctx)
	if err != nil {
		return nil, err
	}

	var result []entity.Model
	for _, model := range models.Data {
		if model.IsPreview {
			result = append(result, model)
		}
	}
	return result, nil
}

// ListProductionModels returns only production (non-preview) models.
func (c *Client) ListProductionModels(ctx context.Context) ([]entity.Model, error) {
	models, err := c.List(ctx)
	if err != nil {
		return nil, err
	}

	var result []entity.Model
	for _, model := range models.Data {
		if !model.IsPreview {
			result = append(result, model)
		}
	}
	return result, nil
}

// GetModel returns a model by ID.
func (c *Client) GetModel(ctx context.Context, modelID string) (*entity.Model, error) {
	models, err := c.List(ctx)
	if err != nil {
		return nil, err
	}

	for _, model := range models.Data {
		if model.ID == modelID {
			return &model, nil
		}
	}
	return nil, fmt.Errorf("%w: %s", ErrModelNotFound, modelID)
}

// ==================== Chat API ====================

// Completion sends a chat completion request.
func (c *Client) Completion(ctx context.Context, req *ChatRequest, opts ...RequestOption) (*ChatResponse, error) {
	if err := validateChatRequest(req); err != nil {
		return nil, err
	}

	authHeader, err := c.tokenMgr.GetTokenWithHeader(ctx)
	if err != nil {
		return nil, fmt.Errorf("get token: %w", err)
	}

	options := applyRequestOptions(opts...)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURLCompletion, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	setRequestHeaders(httpReq, authHeader, options)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("chat request: %w", err)
	}
	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			return
		}
	}(resp.Body)

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("chat error %d: %s", resp.StatusCode, string(respBody))
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return &chatResp, nil
}

// CompletionStream sends a streaming chat completion request.
// Returns two channels: one for chunks and one for errors.
func (c *Client) CompletionStream(ctx context.Context, req *ChatRequest, opts ...RequestOption) (<-chan StreamChunk, <-chan error) {
	chunkChan := make(chan StreamChunk)
	errChan := make(chan error, 1)

	if err := validateChatRequest(req); err != nil {
		errChan <- err
		close(chunkChan)
		close(errChan)
		return chunkChan, errChan
	}

	// Ensure streaming is enabled
	req.Stream = true

	go func() {
		defer close(chunkChan)
		defer close(errChan)

		authHeader, err := c.tokenMgr.GetTokenWithHeader(ctx)
		if err != nil {
			errChan <- fmt.Errorf("get token: %w", err)
			return
		}

		options := applyRequestOptions(opts...)

		body, err := json.Marshal(req)
		if err != nil {
			errChan <- fmt.Errorf("marshal request: %w", err)
			return
		}

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURLCompletion, bytes.NewReader(body))
		if err != nil {
			errChan <- fmt.Errorf("create request: %w", err)
			return
		}

		setStreamHeaders(httpReq, authHeader, options)

		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			errChan <- fmt.Errorf("stream request: %w", err)
			return
		}
		defer func(Body io.ReadCloser) {
			err = Body.Close()
			if err != nil {
				errChan <- fmt.Errorf("close response body: %w", err)
			}
		}(resp.Body)

		if resp.StatusCode != http.StatusOK {
			body, _ = io.ReadAll(resp.Body)
			errChan <- fmt.Errorf("stream error %d: %s", resp.StatusCode, string(body))
			return
		}

		scanner := bufio.NewScanner(resp.Body)
		scanner.Split(splitSSE)

		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				return
			}

			var chunk StreamChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				c.logger.Error("failed to parse chunk: %v", err)
				continue
			}

			select {
			case chunkChan <- chunk:
			case <-ctx.Done():
				errChan <- ctx.Err()
				return
			}
		}

		if err := scanner.Err(); err != nil {
			if ctx.Err() != nil {
				errChan <- ctx.Err()
			} else {
				errChan <- fmt.Errorf("scan stream: %w", err)
			}
		}
	}()

	return chunkChan, errChan
}

// validateChatRequest validates a chat request.
func validateChatRequest(req *ChatRequest) error {
	if req == nil {
		return fmt.Errorf("%w: request is nil", ErrBadRequest)
	}
	if req.Model == "" {
		return fmt.Errorf("%w: model is required", ErrBadRequest)
	}
	if len(req.Messages) == 0 {
		return fmt.Errorf("%w: at least one message is required", ErrBadRequest)
	}
	// Validate system message position
	for i, msg := range req.Messages {
		if msg.Role == RoleSystem && i != 0 {
			return fmt.Errorf("%w: system message must be the first message", ErrBadRequest)
		}
	}
	return nil
}

// applyRequestOptions applies request options and returns the result.
func applyRequestOptions(opts ...RequestOption) *RequestOptions {
	options := &RequestOptions{}
	for _, opt := range opts {
		opt(options)
	}
	return options
}

// setRequestHeaders sets headers for a regular request.
func setRequestHeaders(req *http.Request, authHeader string, opts *RequestOptions) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", authHeader)

	if opts.ClientID != "" {
		req.Header.Set("X-Client-ID", opts.ClientID)
	}
	if opts.RequestID != "" {
		req.Header.Set("X-Request-ID", opts.RequestID)
	} else {
		req.Header.Set("X-Request-ID", uuid.New().String())
	}
	if opts.SessionID != "" {
		req.Header.Set("X-Session-ID", opts.SessionID)
	} else {
		req.Header.Set("X-Session-ID", uuid.New().String())
	}
}

// setStreamHeaders sets headers for a streaming request.
func setStreamHeaders(req *http.Request, authHeader string, opts *RequestOptions) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", authHeader)

	if opts.ClientID != "" {
		req.Header.Set("X-Client-ID", opts.ClientID)
	}
	if opts.RequestID != "" {
		req.Header.Set("X-Request-ID", opts.RequestID)
	} else {
		req.Header.Set("X-Request-ID", uuid.New().String())
	}
	if opts.SessionID != "" {
		req.Header.Set("X-Session-ID", opts.SessionID)
	} else {
		req.Header.Set("X-Session-ID", uuid.New().String())
	}
}

// splitSSE is a split function for scanning Server-Sent Events.
func splitSSE(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}

	if i := bytes.Index(data, []byte("\n\n")); i >= 0 {
		return i + 2, data[:i], nil
	}

	if atEOF {
		return len(data), data, nil
	}

	return 0, nil, nil
}

// ==================== Files API ====================

// UploadFile uploads a file to GigaChat storage using streaming.
func (c *Client) UploadFile(ctx context.Context, req *UploadFileRequest) (*File, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	purpose := req.Purpose
	if purpose == "" {
		purpose = FilePurposeGeneral
	}
	if purpose != FilePurposeGeneral {
		return nil, fmt.Errorf("purpose must be 'general', got: %s", purpose)
	}

	// Detect content type if not provided
	contentType := req.ContentType
	reader := req.Reader
	if contentType == "" {
		detected, peekedReader, err := detectContentType(reader)
		if err != nil {
			return nil, fmt.Errorf("detect content type: %w", err)
		}
		contentType = detected
		reader = peekedReader
	}

	// Validate file size
	if err := validateFileSize(req.Size, contentType); err != nil {
		return nil, err
	}

	authHeader, err := c.tokenMgr.GetTokenWithHeader(ctx)
	if err != nil {
		return nil, fmt.Errorf("get token: %w", err)
	}

	// Create pipe for streaming
	pr, pw := io.Pipe()
	multipartWriter := multipart.NewWriter(pw)

	errChan := make(chan error, 1)

	// Write multipart form in goroutine
	go func() {
		defer func(pw *io.PipeWriter) {
			err = pw.Close()
			if err != nil {
				return
			}
		}(pw)
		defer func(multipartWriter *multipart.Writer) {
			err = multipartWriter.Close()
			if err != nil {
				return
			}
		}(multipartWriter)

		// Add file part
		h := make(textproto.MIMEHeader)
		h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, req.FileName))
		h.Set("Content-Type", contentType)

		part, err := multipartWriter.CreatePart(h)
		if err != nil {
			errChan <- fmt.Errorf("create form file: %w", err)
			return
		}

		if _, err := io.Copy(part, reader); err != nil {
			errChan <- fmt.Errorf("write file data: %w", err)
			return
		}

		// Add purpose field
		if err := multipartWriter.WriteField("purpose", string(purpose)); err != nil {
			errChan <- fmt.Errorf("write purpose field: %w", err)
			return
		}
	}()

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURLFiles, pr)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", multipartWriter.FormDataContentType())
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Authorization", authHeader)

	if req.ClientID != "" {
		httpReq.Header.Set("X-Client-ID", req.ClientID)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("upload request: %w", err)
	}
	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			return
		}
	}(resp.Body)

	// Check for errors from goroutine
	select {
	case err = <-errChan:
		if err != nil {
			return nil, err
		}
	default:
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upload error %d: %s", resp.StatusCode, string(respBody))
	}

	var file File
	if err := json.Unmarshal(respBody, &file); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if file.ID == "" {
		return nil, fmt.Errorf("empty file id in response")
	}

	return &file, nil
}

// detectContentType reads the first 512 bytes to detect content type.
// Returns the detected type and a reader that includes the peeked bytes.
func detectContentType(r io.Reader) (string, io.Reader, error) {
	peekBuf := make([]byte, 512)
	n, err := r.Read(peekBuf)
	if err != nil && err != io.EOF {
		return "", nil, err
	}

	contentType := http.DetectContentType(peekBuf[:n])
	return contentType, io.MultiReader(bytes.NewReader(peekBuf[:n]), r), nil
}

// validateFileSize validates file size based on content type.
func validateFileSize(size int64, contentType string) error {
	switch {
	case strings.HasPrefix(contentType, "audio/"):
		if size > 35*1024*1024 {
			return fmt.Errorf("audio file too large: %d bytes (max 35MB)", size)
		}
	case strings.HasPrefix(contentType, "image/"):
		if size > 15*1024*1024 {
			return fmt.Errorf("image file too large: %d bytes (max 15MB)", size)
		}
	default:
		if size > 40*1024*1024 {
			return fmt.Errorf("file too large: %d bytes (max 40MB)", size)
		}
	}
	return nil
}

// UploadFileFromPath uploads a file from the local filesystem.
func (c *Client) UploadFileFromPath(ctx context.Context, filePath string, purpose FilePurpose, clientID string) (*File, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer func(file *os.File) {
		err = file.Close()
		if err != nil {
			return
		}
	}(file)

	stat, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat file: %w", err)
	}

	return c.UploadFile(ctx, &UploadFileRequest{
		Reader:   file,
		FileName: filepath.Base(filePath),
		Size:     stat.Size(),
		Purpose:  purpose,
		ClientID: clientID,
	})
}

// ListFiles returns all uploaded files.
func (c *Client) ListFiles(ctx context.Context) (*FilesResponse, error) {
	authHeader, err := c.tokenMgr.GetTokenWithHeader(ctx)
	if err != nil {
		return nil, fmt.Errorf("get token: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURLFiles, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", authHeader)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list files request: %w", err)
	}
	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			return
		}
	}(resp.Body)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list files error %d: %s", resp.StatusCode, string(body))
	}

	var filesResp FilesResponse
	if err := json.Unmarshal(body, &filesResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return &filesResp, nil
}

// GetFile returns file information by ID.
func (c *Client) GetFile(ctx context.Context, fileID string) (*File, error) {
	if fileID == "" {
		return nil, fmt.Errorf("file ID is required")
	}

	authHeader, err := c.tokenMgr.GetTokenWithHeader(ctx)
	if err != nil {
		return nil, fmt.Errorf("get token: %w", err)
	}

	url := fmt.Sprintf("%s/%s", strings.TrimSuffix(c.baseURLFiles, "/"), fileID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", authHeader)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get file request: %w", err)
	}
	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			return
		}
	}(resp.Body)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get file error %d: %s", resp.StatusCode, string(body))
	}

	var file File
	if err := json.Unmarshal(body, &file); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return &file, nil
}

// DeleteFile deletes a file by ID.
func (c *Client) DeleteFile(ctx context.Context, fileID string) (*DeleteResponse, error) {
	if fileID == "" {
		return nil, fmt.Errorf("file ID is required")
	}

	authHeader, err := c.tokenMgr.GetTokenWithHeader(ctx)
	if err != nil {
		return nil, fmt.Errorf("get token: %w", err)
	}

	url := fmt.Sprintf("%s/%s/delete", strings.TrimSuffix(c.baseURLFiles, "/"), fileID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", authHeader)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("delete file request: %w", err)
	}
	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			return
		}
	}(resp.Body)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("delete file error %d: %s", resp.StatusCode, string(body))
	}

	var deleteResp DeleteResponse
	if err := json.Unmarshal(body, &deleteResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if !deleteResp.Deleted {
		return nil, fmt.Errorf("file was not deleted")
	}

	return &deleteResp, nil
}

// ==================== Preview Model Helpers ====================

// ModelWithPreview represents a model with preview support.
type ModelWithPreview struct {
	BaseName  string
	IsPreview bool
}

// String returns the full model name with preview suffix if needed.
func (m ModelWithPreview) String() string {
	if m.IsPreview {
		return m.BaseName + "-preview"
	}
	return m.BaseName
}

// NewModel creates a new model with preview support.
func NewModel(baseName string, preview bool) ModelWithPreview {
	return ModelWithPreview{
		BaseName:  baseName,
		IsPreview: preview,
	}
}

// Common GigaChat models.
var (
	ModelGigaChat     = NewModel("GigaChat", false)
	ModelGigaChatPro  = NewModel("GigaChat-Pro", false)
	ModelGigaChatPlus = NewModel("GigaChat-Plus", false)
	ModelGigaChat2Max = NewModel("GigaChat-2-Max", false)

	ModelGigaChatPreview     = NewModel("GigaChat", true)
	ModelGigaChatProPreview  = NewModel("GigaChat-Pro", true)
	ModelGigaChatPlusPreview = NewModel("GigaChat-Plus", true)
	ModelGigaChat2MaxPreview = NewModel("GigaChat-2-Max", true)
)

// NewChatRequest creates a new chat request with preview model support.
func NewChatRequest(model ModelWithPreview, messages []Message) *ChatRequest {
	return &ChatRequest{
		Model:    model.String(),
		Messages: messages,
		Stream:   false,
	}
}

// ==================== Chat with Attachments ====================

// NewChatRequestWithTextFiles creates a chat request for text document analysis.
func NewChatRequestWithTextFiles(model string, prompt string, fileIDs []string, functionCallAuto bool) *ChatRequest {
	req := &ChatRequest{
		Model: model,
		Messages: []Message{
			{
				Role:        RoleUser,
				Content:     prompt,
				Attachments: fileIDs,
			},
		},
		Stream: false,
	}

	if functionCallAuto {
		auto := NewFunctionCallMode(FunctionCallAuto)
		req.FunctionCall = &auto
	}

	return req
}

// NewChatRequestWithImages creates a chat request for image analysis.
// Note: Only one image per message is allowed.
func NewChatRequestWithImages(model string, prompt string, imageIDs []string) *ChatRequest {
	if len(imageIDs) > 10 {
		imageIDs = imageIDs[:10] // Max 10 images per request
	}

	messages := make([]Message, 0, len(imageIDs))
	for _, imageID := range imageIDs {
		messages = append(messages, Message{
			Role:        RoleUser,
			Content:     prompt,
			Attachments: []string{imageID},
		})
	}

	return &ChatRequest{
		Model:    model,
		Messages: messages,
		Stream:   false,
	}
}

// NewChatRequestWithAudio creates a chat request for audio analysis.
func NewChatRequestWithAudio(model string, prompt string, audioIDs []string) *ChatRequest {
	auto := NewFunctionCallMode(FunctionCallAuto)
	return &ChatRequest{
		Model: model,
		Messages: []Message{
			{
				Role:        RoleUser,
				Content:     prompt,
				Attachments: audioIDs,
			},
		},
		FunctionCall: &auto, // Recommended for audio
	}
}
