package internal

import (
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/digitalocean/godo"
)

const (
	// ipCheckURL is the public service used to discover the host's external IP.
	ipCheckURL = "https://api.ipify.org"

	// checkInterval is how often the public IP is compared against the DNS record.
	checkInterval = 60 * time.Second
)

// DynDNSConfig holds the configuration for the dynamic DNS updater.
type DynDNSConfig struct {
	// Token is the DigitalOcean personal access token.
	Token string
	// Domain is the DigitalOcean-managed domain (e.g. "example.com").
	Domain string
	// UpdateRecords is the list of A-record names to keep in sync with the
	// host's public IP (e.g. ["home", "vpn", "@"]).
	UpdateRecords []string
}

// DynDNSOperation watches for public IP changes and keeps DigitalOcean DNS
// A records in sync with the host's current external address.
type DynDNSOperation struct {
	cfg    DynDNSConfig
	client *godo.Client
}

// NewDynDNSOperation constructs a DynDNSOperation from environment variables:
//
//	DO_TOKEN          DigitalOcean personal access token (required)
//	DO_DOMAIN         Domain managed in DigitalOcean, e.g. "example.com" (required)
//	DO_UPDATE_RECORDS Comma-separated A-record names to keep in sync, e.g. "home,vpn,@" (required)
func NewDynDNSOperation() (*DynDNSOperation, error) {
	token := os.Getenv("DO_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("DO_TOKEN environment variable is required")
	}

	domain := os.Getenv("DO_DOMAIN")
	if domain == "" {
		return nil, fmt.Errorf("DO_DOMAIN environment variable is required")
	}

	updateRecordsEnv := os.Getenv("DO_UPDATE_RECORDS")
	if updateRecordsEnv == "" {
		return nil, fmt.Errorf("DO_UPDATE_RECORDS environment variable is required")
	}

	updateRecords := strings.Split(updateRecordsEnv, ",")
	for i, r := range updateRecords {
		updateRecords[i] = strings.TrimSpace(r)
	}

	return &DynDNSOperation{
		cfg: DynDNSConfig{
			Token:         token,
			Domain:        domain,
			UpdateRecords: updateRecords,
		},
		client: godo.NewFromToken(token),
	}, nil
}

// Run starts the DNS monitoring loop. It performs an immediate check on
// startup and then repeats every 60 seconds until ctx is cancelled.
func (d *DynDNSOperation) Run(ctx context.Context) error {
	log.Printf("dyndns: monitoring %s (checking %d record(s) every %s)",
		d.cfg.Domain, len(d.cfg.UpdateRecords), checkInterval)

	// Perform an immediate check before entering the ticker loop so the
	// operator gets fast feedback on startup.
	if err := d.checkAndUpdate(ctx); err != nil {
		log.Printf("dyndns: initial check failed: %v", err)
	}

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("dyndns: shutting down")
			return nil
		case <-ticker.C:
			if err := d.checkAndUpdate(ctx); err != nil {
				log.Printf("dyndns: check failed: %v", err)
			}
		}
	}
}

// checkAndUpdate fetches the current public IP and compares it against each
// configured A record, updating any that are missing or out of date.
func (d *DynDNSOperation) checkAndUpdate(ctx context.Context) error {
	publicIP, err := d.getPublicIP(ctx)
	if err != nil {
		return fmt.Errorf("get public IP: %w", err)
	}
	log := slog.With("ip", publicIP, "domain", d.cfg.Domain)

	var erred bool
	for _, name := range d.cfg.UpdateRecords {
		log := log.With("record", name)
		rec, err := d.findARecord(ctx, name)
		if err != nil || rec == nil {
			erred = true
			log.Warn("dyndns: no A record found – skipping it", "error", err)
			continue
		}

		if rec.Data == publicIP {
			log.Info("dyndns: value is current – no update needed")
			continue
		}

		if err := d.updateARecord(ctx, *rec, publicIP); err != nil {
			erred = true
			log.Error("dyndns: failed to update A record", "error", err)
		} else {
			log.Info("dyndns: updated successfully")
		}
	}

	if erred {
		return fmt.Errorf("some records failed to update")
	}
	return nil
}

// getPublicIP returns the host's current public IPv4 address by querying
// api.ipify.org.
func (d *DynDNSOperation) getPublicIP(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ipCheckURL, nil)
	if err != nil {
		return "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status %d from %s", resp.StatusCode, ipCheckURL)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(body)), nil
}
