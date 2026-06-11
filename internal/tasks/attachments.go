package tasks

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Attachment is a file linked to a task.
type Attachment struct {
	ID          uuid.UUID `json:"id"`
	TaskID      uuid.UUID `json:"task_id"`
	UserID      string    `json:"user_id"`
	FileName    string    `json:"file_name"`
	MimeType    string    `json:"mime_type"`
	SizeBytes   int64     `json:"size_bytes"`
	StoragePath string    `json:"storage_path"`
	DownloadURL string    `json:"download_url,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// CreateAttachmentParams is validated attachment metadata.
type CreateAttachmentParams struct {
	TaskID      string
	UserID      string
	FileName    string
	MimeType    string
	SizeBytes   int64
	StoragePath string
}

var allowedMimePrefixes = []string{
	"image/",
	"application/pdf",
	"text/plain",
}

// ValidateAttachment validates upload metadata.
func ValidateAttachment(fileName, mimeType string, sizeBytes, maxBytes int64) error {
	if strings.TrimSpace(fileName) == "" {
		return fmt.Errorf("file name is required")
	}
	if sizeBytes <= 0 {
		return fmt.Errorf("file cannot be empty")
	}
	if sizeBytes > maxBytes {
		return fmt.Errorf("file exceeds maximum size of %d bytes", maxBytes)
	}
	if !isAllowedMimeType(mimeType) {
		return fmt.Errorf("file type is not allowed")
	}
	return nil
}

func isAllowedMimeType(mimeType string) bool {
	for _, allowed := range allowedMimePrefixes {
		if strings.HasPrefix(mimeType, allowed) || mimeType == allowed {
			return true
		}
	}
	return false
}
