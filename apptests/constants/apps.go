package constants

const (
	CertManager = "cert-manager"
	Karma       = "karma"
	// KubeCost runs only on the managed clusters (in a lightweight agent mode that depends on the centralized kubecost and a valid object storage configuration).
	// Centralized kubecost runs only on the management cluster.
	KubeCost     = "centralized-kubecost"
	Reloader     = "reloader"
	Traefik      = "traefik"
	KarmaTraefik = "karma-traefik"
	Flux         = "kommander-flux"
	ExternalDns  = "external-dns"
	GateKeeper   = "gatekeeper"
	RookCeph     = "rook-ceph"
	Velero       = "velero"
)
