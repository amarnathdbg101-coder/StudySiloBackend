package reuse

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
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