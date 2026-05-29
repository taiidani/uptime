package internal

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/digitalocean/godo"
)

// findARecord returns the first DigitalOcean A record with the given name
// under the configured domain, paginating through results as needed.
func (d *DynDNSOperation) findARecord(ctx context.Context, name string) (*godo.DomainRecord, error) {
	opts := &godo.ListOptions{PerPage: 200}

	for {
		records, resp, err := d.client.Domains.RecordsByType(ctx, d.cfg.Domain, "A", opts)
		if err != nil {
			return nil, err
		}
		slog.Info("Parsing domain records", "records", records)

		for _, r := range records {
			if r.Name == name || r.Name == fmt.Sprintf("%s.%s", name, d.cfg.Domain) {
				return &r, nil
			}
		}

		if resp.Links == nil || resp.Links.IsLastPage() {
			break
		}

		page, err := resp.Links.CurrentPage()
		if err != nil {
			return nil, err
		}
		opts.Page = page + 1
	}

	return nil, fmt.Errorf("no A record named %q found in domain %s", name, d.cfg.Domain)
}

// updateARecord updates the existing A record for name to ip. If the update fails, an error is returned.
func (d *DynDNSOperation) updateARecord(ctx context.Context, record godo.DomainRecord, ip string) error {
	_, _, err := d.client.Domains.EditRecord(ctx, d.cfg.Domain, record.ID, &godo.DomainRecordEditRequest{
		Type: "A",
		Name: record.Name,
		Data: ip,
		TTL:  record.TTL,
	})
	return err
}
