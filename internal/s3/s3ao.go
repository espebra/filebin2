package s3

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/dustin/go-humanize"
)

type S3AO struct {
	client          *s3.Client
	presignClient   *s3.PresignClient
	bucket          string
	endpoint        string
	secure          bool
	expiry          time.Duration
	timeout         time.Duration // timeout for quick operations (delete, head, stat)
	transferTimeout time.Duration // timeout for data transfers (put, get, copy)
}

type BucketMetrics struct {
	Objects                       uint64
	ObjectsReadable               string
	ObjectsSize                   uint64
	ObjectsSizeReadable           string
	IncompleteObjects             uint64
	IncompleteObjectsReadable     string
	IncompleteObjectsSize         uint64
	IncompleteObjectsSizeReadable string
}

// Initialize S3AO
func Init(endpoint, bucket, region, accessKey, secretKey string, secure bool, presignExpiry, timeout, transferTimeout time.Duration) (S3AO, error) {
	var s3ao S3AO

	if endpoint == "" {
		return s3ao, fmt.Errorf("endpoint is required")
	}

	// Build the endpoint URL
	protocol := "http"
	if secure {
		protocol = "https"
	}
	endpointURL := fmt.Sprintf("%s://%s", protocol, endpoint)

	// Custom HTTP transport with higher connection limits to reduce contention
	httpClient := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	// Load AWS config with custom settings
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
		config.WithHTTPClient(httpClient),
	)
	if err != nil {
		return s3ao, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create S3 client with path-style addressing and custom endpoint
	// (required for MinIO and most S3-compatible storage)
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
		o.BaseEndpoint = aws.String(endpointURL)
	})

	s3ao.client = client
	s3ao.presignClient = s3.NewPresignClient(client)
	s3ao.bucket = bucket
	s3ao.endpoint = endpoint
	s3ao.secure = secure
	s3ao.expiry = presignExpiry
	s3ao.timeout = timeout
	s3ao.transferTimeout = transferTimeout

	slog.Info("established session to S3", "endpoint", endpoint)

	// Ensure that the bucket exists, with retry logic for startup race conditions
	retryTimeout := 30 * time.Second
	retryInterval := 2 * time.Second
	startTime := time.Now()

	for {
		_, err = s3ao.client.HeadBucket(context.Background(), &s3.HeadBucketInput{
			Bucket: aws.String(bucket),
		})
		if err == nil {
			slog.Info("found S3 bucket", "bucket", bucket)
			break
		}

		// Bucket doesn't exist or S3 is unavailable, try to create it
		t0 := time.Now()
		_, createErr := s3ao.client.CreateBucket(context.Background(), &s3.CreateBucketInput{
			Bucket: aws.String(bucket),
		})
		if createErr == nil {
			slog.Info("created S3 bucket", "bucket", bucket, "duration_seconds", time.Since(t0).Seconds())
			break
		}

		// Both HeadBucket and CreateBucket failed
		elapsed := time.Since(startTime)
		if elapsed >= retryTimeout {
			slog.Error("unable to access S3 bucket", "bucket", bucket, "elapsed_seconds", elapsed.Seconds(), "error", createErr)
			return s3ao, createErr
		}

		slog.Warn("S3 not available yet, retrying",
			"elapsed_seconds", elapsed.Seconds(),
			"retry_interval_seconds", retryInterval.Seconds(),
			"error", createErr)
		time.Sleep(retryInterval)
	}

	return s3ao, nil
}

func (s S3AO) Status() bool {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(s.bucket),
	})
	if err != nil {
		slog.Warn("S3 status check failed", "bucket", s.bucket, "error", err)
		return false
	}
	return true
}

func (s S3AO) SetTrace(trace bool) {
	// AWS SDK v2 doesn't have a simple trace toggle like Minio
	// Logging can be configured via the AWS config if needed
	if trace {
		slog.Info("S3 tracing enabled (limited in AWS SDK v2)")
	}
}

func (s S3AO) PutObject(bin string, filename string, data io.Reader, size int64) (err error) {
	// Hash the path in S3
	objectKey := s.GetObjectKey(bin, filename)

	ctx, cancel := context.WithTimeout(context.Background(), s.transferTimeout)
	defer cancel()

	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(objectKey),
		Body:          data,
		ContentLength: aws.Int64(size),
		ContentType:   aws.String("application/octet-stream"),
	})
	if err != nil {
		slog.Error("unable to put object", "key", objectKey, "error", err)
		return err
	}

	return nil
}

func (s S3AO) RemoveObject(bin string, filename string) error {
	key := s.GetObjectKey(bin, filename)
	err := s.RemoveKey(key)
	return err
}

func (s S3AO) RemoveKey(key string) error {
	t0 := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		slog.Error("unable to remove object", "key", key, "error", err)
		return err
	}
	slog.Debug("removed object", "key", key, "duration_seconds", time.Since(t0).Seconds())
	return nil
}

func (s S3AO) ListObjects() (objects []string, err error) {
	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.Background())
		if err != nil {
			return objects, err
		}
		for _, obj := range page.Contents {
			objects = append(objects, *obj.Key)
		}
	}

	return objects, nil
}

func (s S3AO) RemoveBucket() error {
	t0 := time.Now()
	objects, err := s.ListObjects()
	if err != nil {
		slog.Error("unable to list objects", "error", err)
	}

	// RemoveObject on all objects
	for _, object := range objects {
		if err := s.RemoveKey(object); err != nil {
			return err
		}
	}

	// RemoveBucket
	_, err = s.client.DeleteBucket(context.Background(), &s3.DeleteBucketInput{
		Bucket: aws.String(s.bucket),
	})
	if err != nil {
		return err
	}

	slog.Info("removed bucket", "bucket", s.bucket, "duration_seconds", time.Since(t0).Seconds())
	return nil
}

func (s S3AO) GetObject(contentSHA256 string, start int64, end int64) (io.ReadCloser, error) {
	t0 := time.Now()

	// Use content SHA256 as the object key for content-addressable storage
	objectKey := contentSHA256

	input := &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(objectKey),
	}

	if end > 0 {
		// AWS SDK uses inclusive range, format: "bytes=start-end"
		input.Range = aws.String(fmt.Sprintf("bytes=%d-%d", start, end))
	}

	// Don't use context timeout here - we return the body for the caller to read.
	// A timeout context would cancel the read mid-stream when defer cancel() fires.
	// The caller manages the body's lifetime by calling Close() when done.
	result, err := s.client.GetObject(context.Background(), input)
	if err != nil {
		return nil, err
	}

	slog.Debug("fetched object", "key", objectKey, "duration_seconds", time.Since(t0).Seconds())
	return result.Body, nil
}

// PresignedGetObject generates a presigned URL for downloading an object.
// If clientIP is provided, the URL will require the X-Forwarded-For header to be set
// to that value when making the request (the header is included in the signature).
// This only works with objects that are not encrypted.
func (s S3AO) PresignedGetObject(contentSHA256 string, filename string, mime string, clientIP string) (presignedURL *url.URL, err error) {
	// Use content SHA256 as the object key for content-addressable storage
	objectKey := contentSHA256

	var contentDisposition string
	switch {
	case strings.HasPrefix(mime, "text/html"), strings.HasPrefix(mime, "application/pdf"):
		// Tell browser to handle this as an attachment. For text/html, this
		// is a small barrier to reduce phishing.
		contentDisposition = fmt.Sprintf("attachment; filename=\"%s\"", filename)
	default:
		// Browser to decide how to handle the rest of the content-types
		contentDisposition = fmt.Sprintf("inline; filename=\"%s\"", filename)
	}

	cacheControl := fmt.Sprintf("max-age=%.0f", s.expiry.Seconds())

	request, err := s.presignClient.PresignGetObject(context.Background(), &s3.GetObjectInput{
		Bucket:                     aws.String(s.bucket),
		Key:                        aws.String(objectKey),
		ResponseContentType:        aws.String(mime),
		ResponseContentDisposition: aws.String(contentDisposition),
		ResponseCacheControl:       aws.String(cacheControl),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = s.expiry
		if clientIP != "" {
			opts.ClientOptions = append(opts.ClientOptions, func(o *s3.Options) {
				o.APIOptions = append(o.APIOptions, func(stack *middleware.Stack) error {
					return stack.Build.Add(middleware.BuildMiddlewareFunc("AddXForwardedForHeader", func(
						ctx context.Context, in middleware.BuildInput, next middleware.BuildHandler,
					) (middleware.BuildOutput, middleware.Metadata, error) {
						if req, ok := in.Request.(*smithyhttp.Request); ok {
							req.Header.Set("X-Forwarded-For", clientIP)
						}
						return next.HandleBuild(ctx, in)
					}), middleware.Before)
				})
			})
		}
	})
	if err != nil {
		return nil, err
	}

	presignedURL, err = url.Parse(request.URL)
	if err != nil {
		return nil, err
	}

	return presignedURL, nil
}

func (s S3AO) GetBucketMetrics() (metrics BucketMetrics) {
	// List incomplete multipart uploads
	var size int64
	var numObjects uint64

	paginator := s3.NewListMultipartUploadsPaginator(s.client, &s3.ListMultipartUploadsInput{
		Bucket: aws.String(s.bucket),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.Background())
		if err != nil {
			slog.Error("failed to get bucket metrics", "error", err)
			return metrics
		}

		for _, upload := range page.Uploads {
			// List parts to get size info
			partsOutput, err := s.client.ListParts(context.Background(), &s3.ListPartsInput{
				Bucket:   aws.String(s.bucket),
				Key:      upload.Key,
				UploadId: upload.UploadId,
			})
			if err != nil {
				continue
			}
			for _, part := range partsOutput.Parts {
				if part.Size != nil {
					size += *part.Size
				}
			}
			numObjects++
		}
	}

	metrics.IncompleteObjects = numObjects
	metrics.IncompleteObjectsReadable = humanize.Comma(int64(numObjects))
	metrics.IncompleteObjectsSize = uint64(size)
	metrics.IncompleteObjectsSizeReadable = humanize.Bytes(metrics.IncompleteObjectsSize)
	return metrics
}

func (s S3AO) GetObjectKey(bin string, filename string) (key string) {
	b := sha256.New()
	b.Write([]byte(bin))
	f := sha256.New()
	f.Write([]byte(filename))
	key = path.Join(fmt.Sprintf("%x", b.Sum(nil)), fmt.Sprintf("%x", f.Sum(nil)))
	return key
}

// PutObjectByHash uploads an object using content-addressable storage (SHA256 as key)
func (s S3AO) PutObjectByHash(contentSHA256 string, data io.Reader, size int64) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.transferTimeout)
	defer cancel()

	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(contentSHA256),
		Body:          data,
		ContentLength: aws.Int64(size),
		ContentType:   aws.String("application/octet-stream"),
	})
	if err != nil {
		slog.Error("unable to put object", "sha256", contentSHA256, "error", err)
		return err
	}

	return nil
}

// RemoveObjectByHash removes an object using content-addressable storage (SHA256 as key)
func (s S3AO) RemoveObjectByHash(contentSHA256 string) error {
	t0 := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(contentSHA256),
	})
	if err != nil {
		slog.Error("unable to remove object", "sha256", contentSHA256, "error", err)
		return err
	}
	slog.Debug("removed object", "sha256", contentSHA256, "duration_seconds", time.Since(t0).Seconds())
	return nil
}

// StatObject checks if an object exists and returns its metadata
func (s S3AO) StatObject(key string) (*s3.HeadObjectOutput, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	return s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
}

// CopyObject copies an object from one key to another within the same bucket
func (s S3AO) CopyObject(srcKey, dstKey string) error {
	ctx, cancel := context.WithTimeout(context.Background(), s.transferTimeout)
	defer cancel()

	_, err := s.client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(s.bucket),
		CopySource: aws.String(fmt.Sprintf("%s/%s", s.bucket, srcKey)),
		Key:        aws.String(dstKey),
	})
	return err
}

// GetClient returns the underlying S3 client (for advanced operations)
func (s S3AO) GetClient() *s3.Client {
	return s.client
}

// GetBucket returns the bucket name
func (s S3AO) GetBucket() string {
	return s.bucket
}

// GetObjectURL returns the full S3 URL for a content SHA256
func (s S3AO) GetObjectURL(contentSHA256 string) string {
	protocol := "http"
	if s.secure {
		protocol = "https"
	}
	return fmt.Sprintf("%s://%s/%s/%s", protocol, s.endpoint, s.bucket, contentSHA256)
}

// ListObjectsWithPrefix lists objects with a given prefix
func (s S3AO) ListObjectsWithPrefix(prefix string) ([]types.Object, error) {
	var objects []types.Object

	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(prefix),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.Background())
		if err != nil {
			return nil, err
		}
		objects = append(objects, page.Contents...)
	}

	return objects, nil
}
