package gigachat

import "errors"

// Common errors returned by the client.
var (
	ErrBadRequest           = errors.New("bad request")
	ErrTokenManagerRequired = errors.New("token manager is required")
	ErrModelNotFound        = errors.New("model not found")
	ErrFileRequired         = errors.New("file is required")
	ErrFileNameRequired     = errors.New("file name is required")
	ErrFileSizeRequired     = errors.New("file size must be positive")
)
