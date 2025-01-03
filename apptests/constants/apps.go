package constants

const (
	CertManager = "cert-manager"
	Karma       = "karma"
	// CentralizedKubecost runs only on the management cluster.
	CentralizedKubecost = "centralized-kubecost"
	// KubeCost runs only on the managed clusters since 2.14.x (in a lightweight agent mode that depends on the centralized kubecost and a valid object storage configuration).
	KubeCost     = "kubecost"
	Reloader     = "reloader"
	Traefik      = "traefik"
	KarmaTraefik = "karma-traefik"
	Flux         = "kommander-flux"
	ExternalDns  = "external-dns"
	GateKeeper   = "gatekeeper"
	RookCeph     = "rook-ceph"
	Velero       = "velero"
)
