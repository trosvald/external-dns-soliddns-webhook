package soliddns

import (
	"context"
	"fmt"
	"strconv"

	eip "github.com/efficientip-labs/solidserver-go-client/sdsclient"
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/external-dns/endpoint"
)

// EfficientIPAPI provides methods to interact with the EfficientIP SolidDNS API.
// It implements the EfficientIPClient interface for DNS operations.
type EfficientIPAPI struct {
	client  *eip.APIClient  // Underlying EfficientIP API client
	context context.Context // Context for API requests
	dnsName string          // DNS smart name to operate on
	dnsView string          // DNS view name (optional)
}

// EfficientIPClient defines the interface for interacting with EfficientIP SolidDNS.
// This interface allows for easier testing and alternative implementations.
type EfficientIPClient interface {
	// ZonesList retrieves all DNS zones matching the given configuration
	ZonesList(config *EfficientIPConfig) ([]*ZoneAuth, error)

	// RecordAdd creates new DNS records based on the provided endpoint
	RecordAdd(rr *endpoint.Endpoint) error

	// RecordDelete removes DNS records specified by the endpoint
	RecordDelete(rr *endpoint.Endpoint) error

	// RecordList retrieves all DNS records for a specific zone
	RecordList(Zone ZoneAuth) (endpoints []*endpoint.Endpoint, _ error)
}

// NewEfficientIPAPI creates a new instance of the EfficientIP API client.
// Parameters:
//   - ctx: Context for API requests (should include authentication)
//   - config: EfficientIP API configuration
//   - eipConfig: Provider-specific configuration
//
// Returns:
//   - Initialized EfficientIPAPI instance
func NewEfficientIPAPI(ctx context.Context, config *eip.Configuration, eipConfig *EfficientIPConfig) EfficientIPAPI {
	return EfficientIPAPI{
		client:  eip.NewAPIClient(config),
		context: ctx,
		dnsName: eipConfig.DnsSmart,
		dnsView: eipConfig.DnsView,
	}
}

// ZonesList retrieves all DNS zones matching the configuration.
// It constructs a query based on the DNS smart name and optional view,
// then converts the API response to our internal ZoneAuth format.
// Parameters:
//   - config: Configuration containing DNS smart name and view
//
// Returns:
//   - Slice of ZoneAuth pointers representing matching zones
//   - Error if API request fails or response indicates failure
func (e *EfficientIPAPI) ZonesList(config *EfficientIPConfig) ([]*ZoneAuth, error) {
	whereClause := buildZoneWhereClause(config)
	log.Debugf("Listing Zones with filter: %s", whereClause)

	zones, resp, err := e.client.DnsAPI.DnsZoneList(e.context).Where(whereClause).Execute()

	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API returned status: %d", resp.StatusCode)
	}

	if !zones.HasSuccess() || !zones.GetSuccess() {
		return nil, fmt.Errorf("API response indicated failure")
	}

	return convertZoneData(zones.GetData()), nil
}

// RecordList retrieves all DNS records for a specific zone.
// It handles different record types (A, TXT, CNAME) and converts them
// to external-dns endpoint format.
// Parameters:
//   - zone: The zone to list records for
//
// Returns:
//   - Slice of endpoints representing DNS records
//   - Error if API request fails or response indicates failure
func (e *EfficientIPAPI) RecordList(zone ZoneAuth) ([]*endpoint.Endpoint, error) {
	log.Debugf("Listing records for zone ID: %s (%s)", zone.ID, zone.Name)

	records, resp, err := e.client.DnsAPI.DnsRrList(e.context).
		Where("zone_id=" + zone.ID).
		Orderby("rr_full_name").
		Execute()

	if err != nil {
		return nil, fmt.Errorf("API request failed for zone %s: %w", zone.Name, err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API returned status %d for zone %s", resp.StatusCode, zone.Name)
	}

	if !records.HasSuccess() || !records.GetSuccess() {
		return nil, fmt.Errorf("API response indicated failure for zone %s", zone.Name)
	}

	return convertRecordsToEndpoints(records.GetData())
}

// RecordAdd creates new DNS records based on the provided endpoint.
// It handles multiple targets by creating individual records for each target.
// Parameters:
//   - ep: Endpoint containing record details (type, name, targets, TTL)
//
// Returns:
//   - Error if no targets provided or any record creation fails
func (e *EfficientIPAPI) RecordAdd(ep *endpoint.Endpoint) error {
	if len(ep.Targets) == 0 {
		return fmt.Errorf("no targets provided for record %s", ep.DNSName)
	}

	for _, target := range ep.Targets {
		if err := e.createSingleRecord(ep, target); err != nil {
			return err
		}
	}
	return nil
}

// RecordDelete removes DNS records specified by the endpoint.
// It handles multiple targets by deleting individual records for each target.
// Parameters:
//   - ep: Endpoint containing record details to delete
//
// Returns:
//   - Error if no targets provided or any record deletion fails
func (e *EfficientIPAPI) RecordDelete(ep *endpoint.Endpoint) error {
	if len(ep.Targets) == 0 {
		return fmt.Errorf("no targets provided for record %s", ep.DNSName)
	}

	for _, target := range ep.Targets {
		if err := e.deleteSingleRecord(ep, target); err != nil {
			return err
		}
	}
	return nil
}

// createSingleRecord handles creation of a single DNS record.
// This is an internal helper method called by RecordAdd for each target.
// Parameters:
//   - ep: Endpoint containing record details
//   - target: Specific target value for this record
//
// Returns:
//   - Error if API request fails or response indicates failure
func (e *EfficientIPAPI) createSingleRecord(ep *endpoint.Endpoint, target string) error {
	log.Debugf("Creating %s record: %s -> %s (TTL: %d)", ep.RecordType, ep.DNSName, target, ep.RecordTTL)

	ttl := int32(ep.RecordTTL)
	input := eip.DnsRrAddInput{
		ServerName: &e.dnsName,
		ViewName:   &e.dnsView,
		RrName:     &ep.DNSName,
		RrType:     &ep.RecordType,
		RrTtl:      &ttl,
		RrValue1:   &target,
	}

	_, resp, err := e.client.DnsAPI.DnsRrAdd(e.context).DnsRrAddInput(input).Execute()
	if err != nil {
		return fmt.Errorf("failed to create %s record %s: %w", ep.RecordType, ep.DNSName, err)
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("API returned status %s when creating record %s", resp.StatusCode, ep.DNSName)
	}
	log.Infof("Successfully created %s record: %s -> %s (TTL: %d)", ep.RecordType, ep.DNSName, target)
	return nil
}

// deleteSingleRecord handles deletion of a single DNS record.
// This is an internal helper method called by RecordDelete for each target.
// Parameters:
//   - ep: Endpoint containing record details to delete
//   - target: Specific target value for this record
//
// Returns:
//   - Error if API request fails or response indicates failure
func (e *EfficientIPAPI) deleteSingleRecord(ep *endpoint.Endpoint, target string) error {
	log.Debugf("Deleting %s record: %s -> %s", ep.RecordType, ep.DNSName, target)

	_, resp, err := e.client.DnsAPI.DnsRrDelete(e.context).
		RrName(ep.DNSName).
		RrType(ep.RecordType).
		RrValue1(target).
		Execute()
	if err != nil {
		return fmt.Errorf("failed to delete %s record %s: %w", ep.RecordType, ep.DNSName, err)
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("API returned status %d when deleting record %s", resp.StatusCode, ep.DNSName)
	}

	log.Infof("Successfully deleted %s record: %s -> %s", ep.RecordType, ep.DNSName, target)
	return nil
}

// buildZoneWhereClause constructs the filter for zone listing.
// Combines the DNS smart name with optional view name if specified.
// Parameters:
//   - config: Configuration containing DNS smart name and view
//
// Returns:
//   - SQL-like WHERE clause string for API filtering
func buildZoneWhereClause(config *EfficientIPConfig) string {
	where := fmt.Sprintf("server_name='%s'", config.DnsSmart)
	if config.DnsView != "" {
		where += fmt.Sprintf(" AND view = '%s'", config.DnsView)
	}
	return where
}

// convertZoneData transforms API zone data to our internal ZoneAuth format.
// Parameters:
//   - zones: Slice of API zone data objects
//
// Returns:
//   - Slice of ZoneAuth pointers representing the same zones
func convertZoneData(zones []eip.DataInnerDnsZoneData) []*ZoneAuth {
	result := make([]*ZoneAuth, 0, len(zones))
	for _, zone := range zones {
		result = append(result, NewZoneAuth(zone))
	}
	return result
}

// convertRecordsToEndpoints transforms API records to external-dns endpoints.
// Handles different record types (A, TXT, CNAME) and combines A records with multiple targets.
// Parameters:
//   - records: Slice of API record data objects
//
// Returns:
//   - Slice of endpoint objects
//   - Error if any record processing fails (though currently always returns nil error)
func convertRecordsToEndpoints(records []eip.DataInnerDnsRrData) ([]*endpoint.Endpoint, error) {
	var endpoints []*endpoint.Endpoint
	hostRecords := make(map[string]*endpoint.Endpoint)

	for _, rr := range records {
		ttl, err := strconv.Atoi(rr.GetRrTtl())
		if err != nil {
			log.Warnf("Invalid TTL for '%s' for record %s, using default", rr.GetRrTtl(), rr.GetRrFullName())
			ttl = 300 // Default ttl if parsing failed
		}

		switch rr.GetRrType() {
		case "A":
			handleARecord(rr, ttl, hostRecords)
		case "TXT", "CNAME":
			endpoints = append(endpoints, createStandardEndpoint(rr, ttl))
		default:
			log.Debugf("Skipping unsupported record type %s for %s", rr.GetRrType(), rr.GetRrFullName())
		}
	}
	// Add all A records to the final endpoints
	for _, record := range hostRecords {
		endpoints = append(endpoints, record)
	}

	return endpoints, nil
}

// handleARecord processes A records with potential multiple targets.
// Groups A records by name and combines their targets.
// Parameters:
//   - rr: API record data object
//   - ttl: TTL value for the record
//   - hostRecords: Map to store and group A records by name
func handleARecord(rr eip.DataInnerDnsRrData, ttl int, hostRecords map[string]*endpoint.Endpoint) {
	key := rr.GetRrFullName() + ":A"
	if existing, found := hostRecords[key]; found {
		existing.Targets = append(existing.Targets, rr.GetRrAllValue())
	} else {
		hostRecords[key] = endpoint.NewEndpointWithTTL(
			rr.GetRrFullName(),
			endpoint.RecordTypeA,
			endpoint.TTL(ttl),
			rr.GetRrAllValue(),
		)
	}
}

// createStandardEndpoint creates an endpoint for standard record types (TXT, CNAME).
// Parameters:
//   - rr: API record data object
//   - ttl: TTL value for the record
//
// Returns:
//   - New endpoint object representing the record
func createStandardEndpoint(rr eip.DataInnerDnsRrData, ttl int) *endpoint.Endpoint {
	return endpoint.NewEndpointWithTTL(
		rr.GetRrFullName(),
		rr.GetRrType(),
		endpoint.TTL(ttl),
		rr.GetRrAllValue(),
	)
}
