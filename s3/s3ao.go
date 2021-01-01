package s3

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"golang.org/x/crypto/hkdf"
	"io"
	"path"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/minio/sio"
)

type S3AO struct {
	client        *minio.Client
	bucket        string
	encryptionKey string
}

type BucketInfo struct {
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
func Init(endpoint, bucket, region, accessKey, secretKey, encryptionKey string, secure bool) (S3AO, error) {
	var s3ao S3AO

	// Set up client for S3AO
	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: secure,
	})
	if err != nil {
		return s3ao, err
	}
	minioClient.SetAppInfo("filebin", "2.0.0")

	s3ao.client = minioClient
	s3ao.bucket = bucket
	s3ao.encryptionKey = encryptionKey

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
		fmt.Printf("Error from S3 when checking if bucket %s exists: %s\n", s.bucket, err.Error)
		return false
	}
	if found == false {
		fmt.Printf("S3 bucket %s does not exist\n")
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

func (s S3AO) GenerateNonce() []byte {
	var nonce []byte
	if _, err := io.ReadFull(rand.Reader, nonce[:]); err != nil {
		fmt.Printf("Failed to read random data: %v", err) // add error handling
	}
	return nonce
}

func (s S3AO) PutObject(bin string, filename string, data io.Reader, size int64) ([]byte, error) {
	//t0 := time.Now()

	// Hash the path in S3
	objectKey := s.GetObjectKey(bin, filename)

	// the master key used to derive encryption keys
	// this key must be keep secret
	//masterkey, err := hex.DecodeString(s.encryptionKey) // use your own key here
	//if err != nil {
	//	fmt.Printf("Cannot decode hex key: %v", err) // add error handling
	//	return err
	//}
	// TODO: Should this be hex?
	masterkey := []byte(s.encryptionKey)

	// generate a random nonce to derive an encryption key from the master key
	// this nonce must be saved to be able to decrypt the data again - it is not
	// required to keep it secret
	nonce := s.GenerateNonce()

	// derive an encryption key from the master key and the nonce
	var key [32]byte
	kdf := hkdf.New(sha256.New, masterkey, nonce[:], nil)
	if _, err := io.ReadFull(kdf, key[:]); err != nil {
		fmt.Printf("Failed to derive encryption key: %v", err) // add error handling
		return nonce, err
	}

	encrypted, err := sio.EncryptReader(data, sio.Config{Key: key[:]})
	if err != nil {
		fmt.Printf("Failed to encrypted reader: %v", err) // add error handling
		return nonce, err
	}

	encryptedSize, err := sio.EncryptedSize(uint64(size))
	if err != nil {
		fmt.Printf("Failed to compute size of encrypted object: %v", err) // add error handling
		return nonce, err
	}

	_, err = s.client.PutObject(context.Background(), s.bucket, objectKey, encrypted, int64(encryptedSize), minio.PutObjectOptions{ContentType: "application/octet-stream"})
	if err != nil {
		fmt.Printf("Unable to put object: %s\n", err.Error())
		return nonce, err
	}
	//s3size := info.Size
	//fmt.Printf("Stored object: %s (%d bytes) in %.3fs\n", objectKey, s3size, time.Since(t0).Seconds())
	return nonce, nil
}

func (s S3AO) RemoveObject(bin string, filename string) error {
	key := s.GetObjectKey(bin, filename)
	err := s.RemoveKey(key)
	return err
}

func (s S3AO) RemoveKey(key string) error {
	t0 := time.Now()

	opts := minio.RemoveObjectOptions{
		GovernanceBypass: true,
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

func (s S3AO) GetObject(bin string, filename string, nonce []byte, start int64, end int64) (io.Reader, error) {
	t0 := time.Now()

	// Hash the path in S3
	b := sha256.New()
	b.Write([]byte(bin))
	f := sha256.New()
	f.Write([]byte(filename))
	objectKey := path.Join(fmt.Sprintf("%x", b.Sum(nil)), fmt.Sprintf("%x", f.Sum(nil)))
	var object io.Reader

	// the master key used to derive encryption keys
	//masterkey, err := hex.DecodeString(s.encryptionKey) // use your own key here
	//if err != nil {
	//	fmt.Printf("Cannot decode hex key: %v", err) // add error handling
	//	return object, err
	//}
	// TODO: Should this be hex?
	masterkey := []byte(s.encryptionKey)

	// derive the encryption key from the master key and the nonce
	var key [32]byte
	kdf := hkdf.New(sha256.New, masterkey, nonce[:], nil)
	if _, err := io.ReadFull(kdf, key[:]); err != nil {
		fmt.Printf("Failed to derive encryption key: %v", err) // add error handling
		return object, err
	}

	opts := minio.GetObjectOptions{}

	if end > 0 {
		opts.SetRange(start, end)
	}

	object, err := s.client.GetObject(context.Background(), s.bucket, objectKey, opts)
	if err != nil {
		return object, err
	}

	decryptedObject, err := sio.DecryptReader(object, sio.Config{Key: key[:]})
	if err != nil {
		if _, ok := err.(sio.Error); ok {
			fmt.Printf("Malformed encrypted data: %v", err) // add error handling - here we know that the data is malformed/not authentic.
			return object, err
		}
		fmt.Printf("Failed to decrypt data: %v", err) // add error handling
		return object, err
	}
	fmt.Printf("Fetched object: %s in %.3fs\n", objectKey, time.Since(t0).Seconds())
	return decryptedObject, err
}

func (s S3AO) GetBucketInfo() (info BucketInfo) {
	opts := minio.ListObjectsOptions{
		Prefix:    "",
		Recursive: true,
	}

	objectCh := s.client.ListObjects(context.Background(), s.bucket, opts)
	var size int64
	var numObjects uint64
	for object := range objectCh {
		if object.Err != nil {
			fmt.Println(object.Err)
			return info
		}
		size = size + object.Size
		numObjects = numObjects + 1
	}

	info.Objects = numObjects
	info.ObjectsReadable = humanize.Comma(int64(numObjects))
	info.ObjectsSize = uint64(size)
	info.ObjectsSizeReadable = humanize.Bytes(info.ObjectsSize)

	multiPartObjectCh := s.client.ListIncompleteUploads(context.Background(), s.bucket, "", true)
	size = 0
	numObjects = 0
	for multiPartObject := range multiPartObjectCh {
		if multiPartObject.Err != nil {
			fmt.Println(multiPartObject.Err)
			return info
		}
		size = size + multiPartObject.Size
		numObjects = numObjects + 1
	}
	info.IncompleteObjects = numObjects
	info.IncompleteObjectsReadable = humanize.Comma(int64(numObjects))
	info.IncompleteObjectsSize = uint64(size)
	info.IncompleteObjectsSizeReadable = humanize.Bytes(info.IncompleteObjectsSize)
	return info
}

func (s S3AO) GetObjectKey(bin string, filename string) (key string) {
	b := sha256.New()
	b.Write([]byte(bin))
	f := sha256.New()
	f.Write([]byte(filename))
	key = path.Join(fmt.Sprintf("%x", b.Sum(nil)), fmt.Sprintf("%x", f.Sum(nil)))
	return key
}
