package soliddns

import eip "github.com/efficientip-labs/solidserver-go-client/sdsclient"

const (
	providerSpecificEfficientipPtrRecord = "efficientip-ptr-record-exists"
)

type ZoneAuth struct {
	Name string
	Type string
	ID   string
}

func NewZoneAuth(zone eip.DataInnerDnsZoneData) *ZoneAuth {
	return &ZoneAuth{
		Name: zone.GetZoneName(),
		Type: zone.GetZoneType(),
		ID:   zone.GetZoneId(),
	}
}
