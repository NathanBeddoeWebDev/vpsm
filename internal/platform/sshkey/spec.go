package sshkey

// Spec describes an SSH key registered with a provider.
type Spec struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Fingerprint string `json:"fingerprint"`
}
