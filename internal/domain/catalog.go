package domain

// Location represents an available deployment region/location from a provider.
type Location struct {
	ID          string `json:"id"`
	Name        string `json:"name"`        // e.g. "fsn1"
	Description string `json:"description"` // e.g. "Falkenstein"
	Country     string `json:"country"`     // e.g. "DE"
	City        string `json:"city"`        // e.g. "Falkenstein"
}

// ServerTypeSpec describes an available server configuration from a provider.
type ServerTypeSpec struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`        // e.g. "cpx11"
	Description  string   `json:"description"` // e.g. "CPX 11"
	Cores        int      `json:"cores"`
	Memory       float64  `json:"memory"`       // in GB
	Disk         int      `json:"disk"`         // in GB
	Architecture string   `json:"architecture"` // e.g. "x86", "arm"
	PriceMonthly string   `json:"price_monthly"`
	PriceHourly  string   `json:"price_hourly"`
	Locations    []string `json:"locations"` // location names where available
}

// ImageSpec describes an available OS image from a provider.
type ImageSpec struct {
	ID           string `json:"id"`
	Name         string `json:"name"`         // e.g. "ubuntu-24.04"
	Description  string `json:"description"`  // e.g. "Ubuntu 24.04"
	Type         string `json:"type"`         // e.g. "system", "snapshot", "backup"
	OSFlavor     string `json:"os_flavor"`    // e.g. "ubuntu", "debian", "fedora"
	Architecture string `json:"architecture"` // e.g. "x86", "arm"
}

// SSHKeySpec describes an SSH key registered with the provider.
type SSHKeySpec struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Fingerprint string `json:"fingerprint"`
}
