package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/taiidani/uptime/internal/backup"
)

var excludedOpts = []string{"nomad", "consul", "vault", "containerd", "cni"}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	fmt.Println("Initializing S3 Client")
	client := loadClient(ctx)

	fmt.Println("Performing backup of /opt directories")
	optBackup := backup.NewOperation(client)
	if err := optBackup.Backup(ctx, "/opt", excludedOpts); err != nil {
		log.Fatal(err)
	}
}

func loadClient(ctx context.Context) *s3.Client {
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("us-west-2"))
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}

	return s3.NewFromConfig(cfg)
}
