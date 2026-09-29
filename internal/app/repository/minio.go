package repository

import (
	"context"
	"fmt"
	"mime/multipart"
	"path/filepath"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const (
	minioEndpoint  = "localhost:9000"
	minioAccessKey = "root"
	minioSecretKey = "rootpassword"
	minioBucket    = "brakes"
)

var minioClient *minio.Client

func getMinioClient() (*minio.Client, error) {
	if minioClient != nil {
		return minioClient, nil
	}

	client, err := minio.New(minioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(minioAccessKey, minioSecretKey, ""),
		Secure: false,
	})
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	exists, err := client.BucketExists(ctx, minioBucket)
	if err != nil {
		return nil, err
	}
	if !exists {
		if err := client.MakeBucket(ctx, minioBucket, minio.MakeBucketOptions{}); err != nil {
			return nil, err
		}
	}

	minioClient = client
	return minioClient, nil
}

// UploadFile — загружает файл в бакет brakes и возвращает его публичный URL
func UploadFile(file multipart.File, header *multipart.FileHeader, prefix string) (string, error) {
	client, err := getMinioClient()
	if err != nil {
		return "", err
	}

	fileName := fmt.Sprintf("%s_%d%s", prefix, time.Now().Unix(), filepath.Ext(header.Filename))

	_, err = client.PutObject(context.Background(), minioBucket, fileName, file, header.Size, minio.PutObjectOptions{
		ContentType: header.Header.Get("Content-Type"),
	})
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("http://%s/%s/%s", minioEndpoint, minioBucket, fileName), nil
}
