package backup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Operation struct {
	bucket       string
	bucketPrefix string
	client       *s3.Client
}

const (
	defaultBucketName   = "archive.ryannixon.com"
	defaultBucketPrefix = "app-backups"
)

func NewOperation(client *s3.Client) *Operation {
	host, err := os.Hostname()
	if err != nil {
		log.Fatalf("Could not determine system hostname: %s", err)
	}

	return &Operation{
		bucket:       defaultBucketName,
		bucketPrefix: path.Join(defaultBucketPrefix, host),
		client:       client,
	}
}

func (o *Operation) Backup(ctx context.Context, baseDir string, excludes []string) error {
	fmt.Println("Scanning", baseDir)
	dirs, err := os.ReadDir(baseDir)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", baseDir, err)
	}

	for _, dir := range dirs {
		if !dir.IsDir() || o.isExcluded(dir.Name(), excludes) {
			continue
		}

		absPath := filepath.Join(baseDir, dir.Name())
		fmt.Println("Backing up", absPath)

		// Create the archive file
		fmt.Println("Creating archive...")
		f, err := os.CreateTemp("", "uptime")
		if err != nil {
			log.Printf("Could not create temporary archive file: %s", err)
		}
		defer os.RemoveAll(f.Name())

		if err := o.archiveDir(absPath, f); err != nil {
			log.Printf("Could not archive %s: %s", absPath, err)
		}
		f.Close()

		// Now upload the file to S3
		f, _ = os.Open(f.Name())
		uploadPath := path.Join(o.bucketPrefix, dir.Name())
		fmt.Printf("Uploading archive to %s://%s...\n", o.bucket, uploadPath)
		_, err = o.client.PutObject(ctx, &s3.PutObjectInput{
			Bucket: aws.String(o.bucket),
			Key:    aws.String(uploadPath),
			Body:   f,
		})
		if err != nil {
			log.Printf("Could not upload archive to S3: %s", err)
		}

		fmt.Println("Upload complete!")
	}

	return nil
}

func (o *Operation) isExcluded(candidate string, excludedNames []string) bool {
	for _, excluded := range excludedNames {
		if excluded == candidate {
			return true
		}
	}

	return false
}

func (o *Operation) archiveDir(dir string, dest io.WriteCloser) error {
	defer dest.Close()

	gzipWriter := gzip.NewWriter(dest)
	defer gzipWriter.Close()

	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("failed to read directory entry %s: %w", path, err)
		} else if d.IsDir() {
			return nil
		}

		fmt.Println("Compressing", path)
		if err := o.addFileToTarWriter(path, tarWriter); err != nil {
			return fmt.Errorf("could not add file '%s', to tarball, got error: %w", path, err)
		}

		return nil
	})
}

// https://gist.github.com/maximilien/328c9ac19ab0a158a8df
func (o *Operation) addFileToTarWriter(filePath string, tarWriter *tar.Writer) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("could not open file '%s', error: %w", filePath, err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("could not get stat for file '%s', error: %w", filePath, err)
	}

	header := &tar.Header{
		Name:    filePath,
		Size:    stat.Size(),
		Mode:    int64(stat.Mode()),
		ModTime: stat.ModTime(),
	}

	err = tarWriter.WriteHeader(header)
	if err != nil {
		return fmt.Errorf("could not write header for file '%s', error: %w", filePath, err)
	}

	_, err = io.Copy(tarWriter, file)
	if err != nil {
		return fmt.Errorf("could not copy the file '%s' data to the tarball, error: %w", filePath, err)
	}

	return nil
}
