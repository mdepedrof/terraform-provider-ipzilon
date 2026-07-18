package client

// Hub

type Hub struct {
	ID           int64   `json:"id"`
	SiteID       int64   `json:"site_id"`
	Name         string  `json:"name"`
	AddressSpace *string `json:"address_space"`
	Location     *string `json:"location"`
	Description  *string `json:"description"`
}

type HubCreate struct {
	SiteID       int64   `json:"site_id"`
	Name         string  `json:"name"`
	AddressSpace *string `json:"address_space,omitempty"`
	Location     *string `json:"location,omitempty"`
	Description  *string `json:"description,omitempty"`
}

type HubUpdate struct {
	Name         *string `json:"name,omitempty"`
	AddressSpace *string `json:"address_space"`
	Location     *string `json:"location"`
	Description  *string `json:"description"`
}

// LandingZone

type LandingZone struct {
	ID          int64   `json:"id"`
	HubID       int64   `json:"hub_id"`
	ParentID    *int64  `json:"parent_id"`
	Name        string  `json:"name"`
	CIDR        *string `json:"cidr"`
	Description *string `json:"description"`
}

type LandingZoneCreate struct {
	HubID       int64   `json:"hub_id"`
	ParentID    *int64  `json:"parent_id,omitempty"`
	Name        string  `json:"name"`
	CIDR        *string `json:"cidr,omitempty"`
	Description *string `json:"description,omitempty"`
}

type LandingZoneUpdate struct {
	ParentID    *int64  `json:"parent_id"`
	Name        *string `json:"name,omitempty"`
	CIDR        *string `json:"cidr"`
	Description *string `json:"description"`
}

// Network

type Network struct {
	ID            int64   `json:"id"`
	LandingZoneID int64   `json:"landing_zone_id"`
	Name          string  `json:"name"`
	CIDR          string  `json:"cidr"`
	Description   *string `json:"description"`
}

type NetworkCreate struct {
	LandingZoneID int64   `json:"landing_zone_id"`
	Name          string  `json:"name"`
	CIDR          string  `json:"cidr"`
	Description   *string `json:"description,omitempty"`
}

type NetworkUpdate struct {
	LandingZoneID *int64  `json:"landing_zone_id,omitempty"`
	Name          *string `json:"name,omitempty"`
	CIDR          *string `json:"cidr,omitempty"`
	Description   *string `json:"description"`
}

// Subnet

type Subnet struct {
	ID          int64   `json:"id"`
	NetworkID   int64   `json:"network_id"`
	Name        string  `json:"name"`
	CIDR        string  `json:"cidr"`
	Description *string `json:"description"`
}

type SubnetCreate struct {
	NetworkID   int64   `json:"network_id"`
	Name        string  `json:"name"`
	CIDR        string  `json:"cidr"`
	Description *string `json:"description,omitempty"`
}

type SubnetUpdate struct {
	Name        *string `json:"name,omitempty"`
	CIDR        *string `json:"cidr,omitempty"`
	Description *string `json:"description"`
}

type AllocateSubnetBody struct {
	PrefixLength int64   `json:"prefix_length"`
	Name         *string `json:"name,omitempty"`
	Description  *string `json:"description,omitempty"`
}

// IPAddress

type IPAddress struct {
	ID              int64   `json:"id"`
	SubnetID        int64   `json:"subnet_id"`
	Address         string  `json:"address"`
	Status          string  `json:"status"`
	IsAzureReserved bool    `json:"is_azure_reserved"`
	Hostname        *string `json:"hostname"`
	Description     *string `json:"description"`
}

type IPAddressUpdate struct {
	Status      *string `json:"status,omitempty"`
	Hostname    *string `json:"hostname"`
	Description *string `json:"description"`
}
