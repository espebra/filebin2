package s3

import (
	//"errors"
	"fmt"
	"io"
	"time"
	//"github.com/dustin/go-humanize"
	"github.com/minio/minio-go/v6"
)

type S3AO struct {
	client *minio.Client
	bucket string
}

// Initialize S3AO
func Init(endpoint, bucket, region, accessKey, secretKey string) (S3AO, error) {
	var s3ao S3AO
	ssl := false

	// Set up client for S3AO
	minioClient, err := minio.New(endpoint, accessKey, secretKey, ssl)
	if err != nil {
		return s3ao, err
	}
	s3ao.client = minioClient
	s3ao.bucket = bucket

	fmt.Printf("Connected to S3AO at %s\n", endpoint)

	// Ensure that the bucket exists
	found, err := s3ao.client.BucketExists(bucket)
	if err != nil {
		fmt.Printf("Unable to check if S3AO bucket exists: %s\n", err.Error())
		return s3ao, err
	}
	if found {
		fmt.Printf("Found S3AO bucket: %s\n", bucket)
	} else {
		t0 := time.Now()
		if err := s3ao.client.MakeBucket(bucket, region); err != nil {
			fmt.Printf("%s\n", err.Error())
		}
		fmt.Printf("Created S3AO bucket: %s in %.3fs\n", bucket, time.Since(t0).Seconds())
	}
	return s3ao, nil
}

func (s S3AO) PutObject(name string, data io.Reader, size int64) error {
	t0 := time.Now()
	n, err := s.client.PutObject(s.bucket, name, data, size, minio.PutObjectOptions{ContentType: "application/octet-stream"})
	if err != nil {
		fmt.Printf("Unable to put object: %s\n", err.Error())
	}
	fmt.Printf("Uploaded object: %s (%d bytes) in %.3fs\n", name, n, time.Since(t0).Seconds())
	return nil
}

func (s S3AO) RemoveObject(name string) error {
	t0 := time.Now()
	err := s.client.RemoveObject(s.bucket, name)
	if err != nil {
		fmt.Printf("Unable to remove object: %s\n", err.Error())
	}
	fmt.Printf("Removed object: %s in %.3fs\n", name, time.Since(t0).Seconds())
	return nil
}

func (s S3AO) listObjects() (objects []string, err error) {
	// Create a done channel to control 'ListObjects' go routine.
	doneCh := make(chan struct{})

	// Indicate to our routine to exit cleanly upon return.
	defer close(doneCh)

	isRecursive := true
	objectCh := s.client.ListObjectsV2(s.bucket, "", isRecursive, doneCh)
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
	objects, err := s.listObjects()
	if err != nil {
		fmt.Printf("Unable to list objects: %s\n", err.Error())
	}

	// ReoveObject on all objects
	for _, object := range objects {
		if err := s.RemoveObject(object); err != nil {
			return err
		}
	}

	// RemoveBucket
	if err := s.client.RemoveBucket(s.bucket); err != nil {
		return err
	}

	fmt.Printf("Removed bucket in %.3fs\n", time.Since(t0).Seconds())
	return nil
}

func (s S3AO) GetObject(name string) (io.Reader, error) {
	object, err := s.client.GetObject(s.bucket, name, minio.GetObjectOptions{})
	if err != nil {
		return object, err
	}
	return object, err
}
