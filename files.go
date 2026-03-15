package gigachat

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// FilePurpose represents the purpose of a file upload.
type FilePurpose string

const (
	// FilePurposeGeneral for use in chat completions.
	FilePurposeGeneral FilePurpose = "general"
)

// AccessPolicy represents file access policy.
type AccessPolicy string

const (
	// AccessPolicyPrivate means only the uploader can access.
	AccessPolicyPrivate AccessPolicy = "private"
)

// Modality represents file modality type.
type Modality string

const (
	// ModalityImage for image files.
	ModalityImage Modality = "image"
	// ModalityAudio for audio files.
	ModalityAudio Modality = "audio"
	// ModalityText for text files.
	ModalityText Modality = "text"
	// ModalityModel3D for 3D model files.
	ModalityModel3D Modality = "3d_model"
)

// File represents a file in GigaChat storage.
type File struct {
	// ID is the unique file identifier.
	ID string `json:"id"`
	// Object is the type of object ("file").
	Object string `json:"object"`
	// Bytes is the file size in bytes.
	Bytes int64 `json:"bytes"`
	// CreatedAt is the upload timestamp.
	CreatedAt int64 `json:"created_at"`
	// Filename is the original file name.
	Filename string `json:"filename"`
	// Purpose of the file.
	Purpose FilePurpose `json:"purpose"`
	// AccessPolicy determines file access.
	AccessPolicy AccessPolicy `json:"access_policy"`
	// Modalities supported by the file.
	Modalities []Modality `json:"modalities,omitempty"`
}

// FilesResponse represents a list of files.
type FilesResponse struct {
	// Data contains the list of files.
	Data []File `json:"data"`
	// Object is the type of object ("list").
	Object string `json:"object"`
}

// DeleteResponse represents a file deletion response.
type DeleteResponse struct {
	// ID of the deleted file.
	ID string `json:"id"`
	// Object is the type of object ("file").
	Object string `json:"object"`
	// Deleted indicates if deletion was successful.
	Deleted bool `json:"deleted"`
}

// UploadFileRequest represents a request to upload a file.
type UploadFileRequest struct {
	// Reader provides the file data.
	Reader io.Reader
	// FileName is the name of the file.
	FileName string
	// ContentType is the MIME type (auto-detected if empty).
	ContentType string
	// Size is the file size in bytes.
	Size int64
	// Purpose of the file (defaults to "general").
	Purpose FilePurpose
	// ClientID for X-Client-ID header.
	ClientID string
}

// Validate validates the upload request.
func (r *UploadFileRequest) Validate() error {
	if r.Reader == nil {
		return ErrFileRequired
	}
	if r.FileName == "" {
		return ErrFileNameRequired
	}
	if r.Size <= 0 {
		return ErrFileSizeRequired
	}
	return nil
}

// GetMIMEType returns the MIME type for a file extension.
func GetMIMEType(filename string) (string, error) {
	ext := strings.ToLower(filepath.Ext(filename))

	mimeTypes := map[string]string{
		// Text documents
		".txt":  "text/plain",
		".doc":  "application/msword",
		".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		".pdf":  "application/pdf",
		".epub": "application/epub",
		".ppt":  "application/vnd.ms-powerpoint",
		".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
		".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",

		// Images
		".jpeg": "image/jpeg",
		".jpg":  "image/jpeg",
		".png":  "image/png",
		".tiff": "image/tiff",
		".bmp":  "image/bmp",

		// Audio
		".mp4":  "audio/mp4",
		".mp3":  "audio/mpeg",
		".m4a":  "audio/x-m4a",
		".wav":  "audio/wav",
		".weba": "audio/webm",
		".ogg":  "audio/ogg",
		".opus": "audio/opus",
	}

	if mime, ok := mimeTypes[ext]; ok {
		return mime, nil
	}
	return "", fmt.Errorf("unsupported file extension: %s", ext)
}
