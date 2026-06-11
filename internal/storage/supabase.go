package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client uploads and manages files in Supabase Storage.
type Client struct {
	baseURL        string
	serviceRoleKey string
	bucket         string
	httpClient     *http.Client
}

// NewClient creates a Supabase Storage client.
func NewClient(baseURL, serviceRoleKey, bucket string) *Client {
	return &Client{
		baseURL:        strings.TrimRight(baseURL, "/"),
		serviceRoleKey: serviceRoleKey,
		bucket:         bucket,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// Upload stores a file at the given object path.
func (c *Client) Upload(ctx context.Context, path string, reader io.Reader, contentType string, size int64) error {
	url := fmt.Sprintf("%s/storage/v1/object/%s/%s", c.baseURL, c.bucket, path)

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, reader)
	if err != nil {
		return fmt.Errorf("build upload request: %w", err)
	}

	request.Header.Set("Authorization", "Bearer "+c.serviceRoleKey)
	request.Header.Set("apikey", c.serviceRoleKey)
	request.Header.Set("Content-Type", contentType)
	request.ContentLength = size

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("upload file: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("upload file: storage returned status %d", response.StatusCode)
	}

	return nil
}

// Delete removes a file at the given object path.
func (c *Client) Delete(ctx context.Context, path string) error {
	url := fmt.Sprintf("%s/storage/v1/object/%s/%s", c.baseURL, c.bucket, path)

	request, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("build delete request: %w", err)
	}

	request.Header.Set("Authorization", "Bearer "+c.serviceRoleKey)
	request.Header.Set("apikey", c.serviceRoleKey)

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("delete file: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode >= http.StatusBadRequest && response.StatusCode != http.StatusNotFound {
		return fmt.Errorf("delete file: storage returned status %d", response.StatusCode)
	}

	return nil
}

// SignedURL creates a temporary download URL for a private object.
func (c *Client) SignedURL(ctx context.Context, path string, expiresIn time.Duration) (string, error) {
	url := fmt.Sprintf("%s/storage/v1/object/sign/%s/%s", c.baseURL, c.bucket, path)

	body := strings.NewReader(fmt.Sprintf(`{"expiresIn":%d}`, int(expiresIn.Seconds())))
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return "", fmt.Errorf("build sign request: %w", err)
	}

	request.Header.Set("Authorization", "Bearer "+c.serviceRoleKey)
	request.Header.Set("apikey", c.serviceRoleKey)
	request.Header.Set("Content-Type", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("sign url: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode >= http.StatusBadRequest {
		return "", fmt.Errorf("sign url: storage returned status %d", response.StatusCode)
	}

	var parsed struct {
		SignedURL string `json:"signedURL"`
	}
	if err := json.NewDecoder(response.Body).Decode(&parsed); err != nil {
		return "", fmt.Errorf("decode sign response: %w", err)
	}

	if parsed.SignedURL == "" {
		return "", fmt.Errorf("sign url: empty signed URL")
	}

	if strings.HasPrefix(parsed.SignedURL, "http") {
		return parsed.SignedURL, nil
	}

	return c.baseURL + "/storage/v1" + parsed.SignedURL, nil
}
