package soliddns

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/plan"
	"sigs.k8s.io/external-dns/provider"
)

// Provider implements the external-dns provider interface for EfficientIP SolidDNS
type Provider struct {
	provider.BaseProvider
	client       EfficientIPClient
	domainFilter endpoint.DomainFilter
	context      context.Context
	config       *EfficientIPConfig
}

// Records fetches all DNS records from configured zones
func (p *Provider) Records(ctx context.Context) ([]*endpoint.Endpoint, error) {
	log.Debugf("Fetching DNS records from EfficientIP SolidDNS")

	zones, err := p.Zones()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch zones: %w", err)
	}

	var endpoints []*endpoint.Endpoint
	for _, zone := range zones {
		log.Debugf("Fetching DNS records from Zone %s", zone.Name)

		records, err := p.client.RecordList(*zone)
		if err != nil {
			return nil, fmt.Errorf("failed to get records for zone %s: %w", zone.Name, err)
		}
		endpoints = append(endpoints, records...)
	}

	log.Debugf("Fetched %d records from EfficientIP SolidDNS", len(endpoints))
	return endpoints, nil
}

func (p *Provider) ApplyChanges(ctx context.Context, changes *plan.Changes) error {
	log.Info("Applying DNS changes to EfficientIP SolidDNS")

	if changes == nil {
		log.Debug("No changes to apply")
		return nil
	}

	// Process deletion first
	if err := p.processDeletions(changes.Delete); err != nil {
		return err
	}
	// Process updateOld (deletions for updates)
	if err := p.processDeletions(changes.UpdateOld); err != nil {
		return err
	}
	// Process creates (including updateNew)
	if err := p.processCreations(changes.Create); err != nil {
		return err
	}

	if err := p.processCreations(changes.UpdateNew); err != nil {
		return err
	}
	log.Info("Successfully applied all DNS changes to EfficientIP SolidDNS")
	return nil
}

// processDeletions handles deletion of endpoints
func (p *Provider) processDeletions(endpoints []*endpoint.Endpoint) error {
	for _, ep := range endpoints {
		if err := p.DeleteChanges(p.context, ep); err != nil {
			return fmt.Errorf("failed to delete endpoint %s: %w", ep.DNSName, err)
		}
	}
	return nil
}

// processCreations handles creation of endpoints
func (p *Provider) processCreations(endpoints []*endpoint.Endpoint) error {
	for _, ep := range endpoints {
		if err := p.CreateChanges(p.context, ep); err != nil {
			return fmt.Errorf("failed to create endpoint %s: %w", ep.DNSName, err)
		}
	}
	return nil
}

// AdjustEndpoints modifies endpoint before they are processed
func (p *Provider) AdjustEndpoints(endpoints []*endpoint.Endpoint) ([]*endpoint.Endpoint, error) {
	if len(endpoints) == 0 {
		return endpoints, nil
	}

	adjusted := make([]*endpoint.Endpoint, 0, len(endpoints))

	for _, ep := range endpoints {
		// Set default ttl if not configured
		if !ep.RecordTTL.IsConfigured() {
			ep.RecordTTL = endpoint.TTL(p.config.DefaultTTL)
		}

		adjusted = append(adjusted, ep)

		// skip PTR handling if disabled
		if !p.config.CreatePTR {
			continue
		}

		// Add PTR tracking for A records
		if ep.RecordType == endpoint.RecordTypeA {
			p.addPTRRecordTracking(ep)
		}
	}
	return adjusted, nil
}

// addPTRRecordTracking adds provider-specific metadata for PTR record tracking
func (p *Provider) addPTRRecordTracking(ep *endpoint.Endpoint) {
	found := false
	for i := range ep.ProviderSpecific {
		if ep.ProviderSpecific[i].Name == providerSpecificEfficientipPtrRecord {
			ep.ProviderSpecific[i].Value = "true"
			found = true
			break
		}
	}

	if !found {
		ep.WithProviderSpecific(providerSpecificEfficientipPtrRecord, "true")
	}
}

// Zones returns all DNS zones matching the domain filter
func (p *Provider) Zones() ([]*ZoneAuth, error) {
	zones, err := p.client.ZonesList(p.config)
	if err != nil {
		return nil, fmt.Errorf("failed to list zones: %w", err)
	}

	var filtered []*ZoneAuth
	for _, zone := range zones {
		if !p.domainFilter.Match(zone.Name) {
			log.Debugf("Ignoring zones '%s' (doesn't match domain filter)", zone.Name)
			continue
		}
		filtered = append(filtered, zone)
	}
	log.Debugf("Found %d matching zones", len(filtered))
	return filtered, nil
}

// DeleteChanges handles deletion of DNS records
func (p *Provider) DeleteChanges(_ context.Context, ep *endpoint.Endpoint) error {
	if p.config.DryRun {
		for _, target := range ep.Targets {
			log.Infof("[DryRun] Would delete %s record '%s' -> '%s'",
				ep.RecordType,
				ep.DNSName,
				target,
			)
		}
		return nil
	}

	if err := p.client.RecordDelete(ep); err != nil {
		return fmt.Errorf("failed to delete record: %w", err)
	}

	for _, target := range ep.Targets {
		log.Infof("Deleted %s record '%s' -> '%s'",
			ep.RecordType,
			ep.DNSName,
			target,
		)
	}

	return nil
}

// CreateChanges handles creation of DNS records
func (p *Provider) CreateChanges(_ context.Context, ep *endpoint.Endpoint) error {
	if p.config.DryRun {
		for _, target := range ep.Targets {
			log.Infof("[DryRun] Would create %s record '%s' -> '%s' (TTL: %d)",
				ep.RecordType,
				ep.DNSName,
				target,
				ep.RecordTTL,
			)
		}
		return nil
	}

	if err := p.client.RecordAdd(ep); err != nil {
		return fmt.Errorf("failed to create record: %w", err)
	}

	for _, target := range ep.Targets {
		log.Infof("Created %s record '%s' -> '%s' (TTL: %d)",
			ep.RecordType,
			ep.DNSName,
			target,
			ep.RecordTTL,
		)
	}

	return nil
}
