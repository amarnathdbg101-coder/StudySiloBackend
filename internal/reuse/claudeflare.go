package reuse

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"path/filepath"
	"goProjectStructure/internal/utils"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

const (
	MaxImageSizeBytes = 8 * 1024 * 1024 // 8MB limit
)

var (
	ErrImageTooLarge    = errors.New("file size exceeds maximum allowed 8MB")
	ErrInvalidImageType = errors.New("invalid file format; only JPEG, PNG, WebP, and PDF are allowed")
	allowedContentTypes = map[string]bool{
		"image/jpeg":      true,
		"image/jpg":       true,
		"image/png":       true,
		"image/webp":      true,
		"application/pdf": true,
	}

	r2Once         sync.Once
	cachedR2Client *s3.S3
)

// NewR2Client creates a singleton S3-compatible client for Cloudflare R2 / AWS S3
func NewR2Client() *s3.S3 {
	r2Once.Do(func() {
		cfg := utils.MustLoad()
		sess := session.Must(session.NewSession(&aws.Config{
			Region:   aws.String("auto"),
			Endpoint: aws.String(cfg.Endpoint),
			Credentials: credentials.NewStaticCredentials(
				cfg.AccessKey,
				cfg.SecretKey,
				"",
			),
			S3ForcePathStyle: aws.Bool(true),
		}))
		cachedR2Client = s3.New(sess)
	})
	return cachedR2Client
}

// UploadToR2 uploads a file to the configured S3 / Cloudflare R2 bucket and returns the public URL
func UploadToR2(file multipart.File, header *multipart.FileHeader, folder string) (string, error) {
	if header.Size > MaxImageSizeBytes {
		return "", ErrImageTooLarge
	}

	buf := bytes.NewBuffer(nil)
	if _, err := io.Copy(buf, file); err != nil {
		return "", fmt.Errorf("failed to read file bytes: %w", err)
	}

	contentType := header.Header.Get("Content-Type")
	if !allowedContentTypes[contentType] {
		ext := strings.ToLower(filepath.Ext(header.Filename))
		switch ext {
		case ".jpg", ".jpeg":
			contentType = "image/jpeg"
		case ".png":
			contentType = "image/png"
		case ".webp":
			contentType = "image/webp"
		case ".pdf":
			contentType = "application/pdf"
		default:
			return "", ErrInvalidImageType
		}
	}

	cfg := utils.MustLoad()
	client := NewR2Client()

	ext := filepath.Ext(header.Filename)
	fileName := fmt.Sprintf("%s/%d_%s%s", folder, time.Now().UnixNano(), Slugify(strings.TrimSuffix(header.Filename, ext)), ext)

	_, err := client.PutObject(&s3.PutObjectInput{
		Bucket:      aws.String(cfg.Bucket),
		Key:         aws.String(fileName),
		Body:        bytes.NewReader(buf.Bytes()),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload file to R2: %w", err)
	}

	publicURL, err := url.JoinPath(cfg.PublicURL, fileName)
	if err != nil {
		return fmt.Sprintf("%s/%s", strings.TrimRight(cfg.PublicURL, "/"), fileName), nil
	}

	return publicURL, nil
}

// UploadBytesToR2 uploads raw byte data (such as decoded base64 images or voice notes) directly to Cloudflare R2
func UploadBytesToR2(data []byte, contentType, folder string) (string, error) {
	if len(data) == 0 {
		return "", errors.New("cannot upload empty data")
	}
	if int64(len(data)) > MaxImageSizeBytes {
		return "", ErrImageTooLarge
	}

	cfg := utils.MustLoad()
	if strings.TrimSpace(cfg.Bucket) == "" || strings.TrimSpace(cfg.Endpoint) == "" {
		return "", errors.New("Cloudflare R2 is not configured in environment (R2_BUCKET or R2_ENDPOINT missing)")
	}

	contentType = strings.TrimSpace(contentType)
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}
	if idx := strings.Index(contentType, ";"); idx != -1 {
		contentType = strings.TrimSpace(contentType[:idx])
	}

	var ext string
	switch strings.ToLower(contentType) {
	case "image/jpeg", "image/jpg":
		ext = ".jpg"
		contentType = "image/jpeg"
	case "image/png":
		ext = ".png"
		contentType = "image/png"
	case "image/webp":
		ext = ".webp"
		contentType = "image/webp"
	case "application/pdf":
		ext = ".pdf"
		contentType = "application/pdf"
	case "audio/mpeg", "audio/mp3":
		ext = ".mp3"
		contentType = "audio/mpeg"
	case "audio/wav":
		ext = ".wav"
		contentType = "audio/wav"
	case "audio/m4a", "audio/mp4", "audio/aac":
		ext = ".m4a"
		contentType = "audio/m4a"
	default:
		// Default to jpg for image attachments
		ext = ".jpg"
		contentType = "image/jpeg"
	}

	client := NewR2Client()
	cleanFolder := strings.Trim(folder, "/")
	if cleanFolder == "" {
		cleanFolder = "chat_images"
	}
	fileName := fmt.Sprintf("%s/%d%s", cleanFolder, time.Now().UnixNano(), ext)

	_, err := client.PutObject(&s3.PutObjectInput{
		Bucket:      aws.String(cfg.Bucket),
		Key:         aws.String(fileName),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload bytes to R2: %w", err)
	}

	publicURL, err := url.JoinPath(cfg.PublicURL, fileName)
	if err != nil {
		return fmt.Sprintf("%s/%s", strings.TrimRight(cfg.PublicURL, "/"), fileName), nil
	}

	return publicURL, nil
}