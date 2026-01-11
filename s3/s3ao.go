package s3

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type S3AO struct {
	client   *minio.Client
	bucket   string
	endpoint string
	secure   bool
	expiry   time.Duration
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
func Init(endpoint, bucket, region, accessKey, secretKey string, secure bool, presignExpiry time.Duration) (S3AO, error) {
	var s3ao S3AO

	// Set up client for S3AO
	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: secure,
		Region: region,
	})
	if err != nil {
		return s3ao, err
	}
	minioClient.SetAppInfo("filebin", "2.0.1")

	s3ao.client = minioClient
	s3ao.bucket = bucket
	s3ao.endpoint = endpoint
	s3ao.secure = secure
	s3ao.expiry = presignExpiry

	fmt.Printf("Established session to S3AO at %s\n", endpoint)

	// Ensure that the bucket exists
	found, err := s3ao.client.BucketExists(context.Background(), bucket)
	if err != nil {
		fmt.Printf("Unable to check if S3AO bucket exists: %s\n", err.Error())
		return s3ao, err
	}
	if found {
		fmt.Printf("Found S3AO bucket: %s\n", bucket)
	} else {
		t0 := time.Now()
		if err := s3ao.client.MakeBucket(context.Background(), bucket, minio.MakeBucketOptions{Region: region}); err != nil {
			fmt.Printf("%s\n", err.Error())
		}
		fmt.Printf("Created S3AO bucket: %s in %.3fs\n", bucket, time.Since(t0).Seconds())
	}
	return s3ao, nil
}

func (s S3AO) Status() bool {
	found, err := s.client.BucketExists(context.Background(), s.bucket)
	if err != nil {
		fmt.Printf("Error from S3 when checking if bucket %s exists: %s\n", s.bucket, err.Error())
		return false
	}
	if found == false {
		fmt.Printf("S3 bucket %s does not exist\n", s.bucket)
		return false
	}
	return true
}

func (s S3AO) SetTrace(trace bool) {
	if trace {
		s.client.TraceOn(nil)
	} else {
		s.client.TraceOff()
	}
}

func (s S3AO) PutObject(bin string, filename string, data io.Reader, size int64) (err error) {
	t0 := time.Now()

	// Hash the path in S3
	objectKey := s.GetObjectKey(bin, filename)

	var objectSize uint64
	var content io.Reader

	// Do not encrypt the content during upload. This allows for presigned downloads.
	content = data
	objectSize = uint64(size)

	_, err = s.client.PutObject(context.Background(), s.bucket, objectKey, content, int64(objectSize), minio.PutObjectOptions{ContentType: "application/octet-stream"})
	if err != nil {
		fmt.Printf("Unable to put object: %s\n", err.Error())
		return err
	}

	fmt.Printf("Stored object: %s (%d bytes) in %.3fs\n", objectKey, objectSize, time.Since(t0).Seconds())
	return nil
}

func (s S3AO) RemoveObject(bin string, filename string) error {
	key := s.GetObjectKey(bin, filename)
	err := s.RemoveKey(key)
	return err
}

func (s S3AO) RemoveKey(key string) error {
	t0 := time.Now()

	opts := minio.RemoveObjectOptions{
		// The following is used in the Minio SDK documentation,
		// but it seems not all S3 server side implementations
		// support this. One example is DigitalOcean Spaces.
		//GovernanceBypass: true,
	}

	err := s.client.RemoveObject(context.Background(), s.bucket, key, opts)
	if err != nil {
		fmt.Printf("Unable to remove object: %s\n", err.Error())
		return err
	}
	fmt.Printf("Removed object: %s in %.3fs\n", key, time.Since(t0).Seconds())
	return nil
}

func (s S3AO) ListObjects() (objects []string, err error) {
	opts := minio.ListObjectsOptions{
		Prefix:    "",
		Recursive: true,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	objectCh := s.client.ListObjects(ctx, s.bucket, opts)
	for object := range objectCh {
		if object.Err != nil {
			return objects, object.Err
		}
		objects = append(objects, object.Key)
	}
	return objects, nil
}

func (s S3AO) RemoveBucket() error {
	t0 := time.Now()
	objects, err := s.ListObjects()
	if err != nil {
		fmt.Printf("Unable to list objects: %s\n", err.Error())
	}

	// ReoveObject on all objects
	for _, object := range objects {
		if err := s.RemoveKey(object); err != nil {
			return err
		}
	}

	// RemoveBucket
	if err := s.client.RemoveBucket(context.Background(), s.bucket); err != nil {
		return err
	}

	fmt.Printf("Removed bucket in %.3fs\n", time.Since(t0).Seconds())
	return nil
}

func (s S3AO) GetObject(contentSHA256 string, start int64, end int64) (*minio.Object, error) {
	t0 := time.Now()

	// Use content SHA256 as the object key for content-addressable storage
	objectKey := contentSHA256
	opts := minio.GetObjectOptions{}

	if end > 0 {
		opts.SetRange(start, end)
	}

	object, err := s.client.GetObject(context.Background(), s.bucket, objectKey, opts)
	if err != nil {
		return object, err
	}

	fmt.Printf("Fetched object: %s in %.3fs\n", objectKey, time.Since(t0).Seconds())
	return object, err
}

// This only works with objects that are not encrypted
func (s S3AO) PresignedGetObject(contentSHA256 string, filename string, mime string) (presignedURL *url.URL, err error) {
	// Use content SHA256 as the object key for content-addressable storage
	objectKey := contentSHA256

	reqParams := make(url.Values)
	reqParams.Set("response-content-type", mime)

	switch {
	case strings.HasPrefix(mime, "text/html"), strings.HasPrefix(mime, "application/pdf"):
		// Tell browser to handle this as an attachment. For text/html, this
		// is a small barrier to reduce phishing.
		reqParams.Set("response-content-disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	default:
		// Browser to decide how to handle the rest of the content-types
		reqParams.Set("response-content-disposition", fmt.Sprintf("inline; filename=\"%s\"", filename))
	}

	reqParams.Set("response-cache-control", fmt.Sprintf("max-age=%.0f", s.expiry.Seconds()))

	presignedURL, err = s.client.PresignedGetObject(context.Background(), s.bucket, objectKey, s.expiry, reqParams)
	if err != nil {
		return presignedURL, err
	}

	return presignedURL, nil
}

func (s S3AO) GetBucketMetrics() (metrics BucketMetrics) {
	//opts := minio.ListObjectsOptions{
	//	Prefix:    "",
	//	Recursive: true,
	//}

	//objectCh := s.client.ListObjects(context.Background(), s.bucket, opts)
	var size int64
	var numObjects uint64
	//for object := range objectCh {
	//	if object.Err != nil {
	//		fmt.Println(object.Err)
	//		return metrics
	//	}
	//	size = size + object.Size
	//	numObjects = numObjects + 1
	//}

	//metrics.Objects = numObjects
	//metrics.ObjectsReadable = humanize.Comma(int64(numObjects))
	//metrics.ObjectsSize = uint64(size)
	//metrics.ObjectsSizeReadable = humanize.Bytes(metrics.ObjectsSize)

	multiPartObjectCh := s.client.ListIncompleteUploads(context.Background(), s.bucket, "", true)
	for multiPartObject := range multiPartObjectCh {
		if multiPartObject.Err != nil {
			fmt.Println(multiPartObject.Err)
			return metrics
		}
		size = size + multiPartObject.Size
		numObjects = numObjects + 1
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
	t0 := time.Now()

	var objectSize uint64
	var content io.Reader

	// Do not encrypt the content during upload. This allows for presigned downloads.
	content = data
	objectSize = uint64(size)

	_, err = s.client.PutObject(context.Background(), s.bucket, contentSHA256, content, int64(objectSize), minio.PutObjectOptions{ContentType: "application/octet-stream"})
	if err != nil {
		fmt.Printf("Unable to put object: %s\n", err.Error())
		return err
	}

	fmt.Printf("Stored object: %s (%d bytes) in %.3fs\n", contentSHA256, objectSize, time.Since(t0).Seconds())
	return nil
}

// RemoveObjectByHash removes an object using content-addressable storage (SHA256 as key)
func (s S3AO) RemoveObjectByHash(contentSHA256 string) error {
	t0 := time.Now()

	opts := minio.RemoveObjectOptions{}

	err := s.client.RemoveObject(context.Background(), s.bucket, contentSHA256, opts)
	if err != nil {
		fmt.Printf("Unable to remove object: %s\n", err.Error())
		return err
	}
	fmt.Printf("Removed object: %s in %.3fs\n", contentSHA256, time.Since(t0).Seconds())
	return nil
}

// GetClient returns the underlying minio client (for migration and advanced operations)
func (s S3AO) GetClient() *minio.Client {
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
