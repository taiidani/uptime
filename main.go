package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/taiidani/uptime/internal/backup"
)

var (
	excludedOpts arrayFlag
	folder       string
)

func main() {
	flags()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	fmt.Println("Initializing S3 Client")
	client, err := loadClient(ctx)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Performing backup of %s subdirectories\n", folder)
	optBackup := backup.NewOperation(client)
	if err := optBackup.Backup(ctx, folder, excludedOpts); err != nil {
		log.Fatal(err)
	}
}

type arrayFlag []string

func (i *arrayFlag) Set(value string) error {
	*i = append(*i, value)
	return nil
}

func (i *arrayFlag) String() string {
	return strings.Join(*i, ",")
}

func flags() {
	flag.StringVar(&folder, "folder", "", "the folder to be backed up")
	flag.Var(&excludedOpts, "exclude", "folder name to exclude from backup")
	flag.Parse()

	if folder == "" {
		log.Fatal("-folder flag is required")
	} else if !path.IsAbs(folder) {
		log.Fatal("-folder must be an absolute path")
	}
}

func loadClient(ctx context.Context) (*s3.Client, error) {
	endpoint := os.Getenv("AWS_ENDPOINT")
	region := os.Getenv("AWS_REGION")

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithBaseEndpoint(endpoint),
	)
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}

	client := s3.NewFromConfig(cfg)

	// Perform a sanity check on the credentials
	// Make a few attempts before giving up
	for attempts := 0; attempts < 5; attempts++ {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("operation canceled")
		default:
			_, err := client.ListBuckets(ctx, &s3.ListBucketsInput{})
			if err != nil {
				log.Printf("Could not perform ListBuckets operation: %s", err)
				log.Print("Will retry in 3 seconds")
				time.Sleep(time.Second * 3)
				continue
			}

			return client, nil
		}
	}

	return nil, fmt.Errorf("unable to validate AWS credentials")
}
