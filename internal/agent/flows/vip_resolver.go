package flows

import (
	"fmt"
	"sync"
)

// Endpoint describes a backing workload behind a virtual IP.
type Endpoint struct {
	WorkloadName string `json:"workload_name"` // e.g. "deployment:billing-api", "pod:billing-api-74bf"
	IP           string `json:"ip"`
	Port         int    `json:"port"`
}

// ServiceVIPMapping stores the virtual IP and its resolved backing endpoints.
type ServiceVIPMapping struct {
	ServiceName string     `json:"service_name"` // e.g. "k8s_service:billing-svc"
	VirtualIP   string     `json:"virtual_ip"`   // e.g. "10.96.0.100"
	Port        int        `json:"port"`
	Endpoints   []Endpoint `json:"endpoints"`
}

// VIPResolver resolves Kubernetes ClusterIPs and Docker Swarm VIPs to true workload identities.
type VIPResolver struct {
	services map[string]*ServiceVIPMapping // "VIP:Port" -> Mapping
	mu       sync.RWMutex
}

// NewVIPResolver creates a new virtual IP resolver.
func NewVIPResolver() *VIPResolver {
	return &VIPResolver{
		services: make(map[string]*ServiceVIPMapping),
	}
}

// RegisterService adds or updates a virtual IP mapping.
func (r *VIPResolver) RegisterService(serviceName, vip string, port int, endpoints []Endpoint) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := fmt.Sprintf("%s:%d", vip, port)
	r.services[key] = &ServiceVIPMapping{
		ServiceName: serviceName,
		VirtualIP:   vip,
		Port:        port,
		Endpoints:   endpoints,
	}
}

// Resolve resolves a destination IP and port to its backing workload.
// It returns (resolvedWorkload, serviceVIP, confidence).
func (r *VIPResolver) Resolve(destIP string, destPort int) (string, string, float64) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	key := fmt.Sprintf("%s:%d", destIP, destPort)
	mapping, exists := r.services[key]
	if !exists {
		// Not a registered virtual IP, return destination IP directly
		return destIP, "", 1.0
	}

	if len(mapping.Endpoints) == 1 {
		// Exactly one backing workload: deterministic resolution
		return mapping.Endpoints[0].WorkloadName, mapping.VirtualIP, 1.0
	} else if len(mapping.Endpoints) > 1 {
		// Multiple backing pods: resolve to primary or service collection with slight uncertainty
		return mapping.Endpoints[0].WorkloadName, mapping.VirtualIP, 0.85
	}

	// Registered VIP with no endpoints available
	return mapping.ServiceName, mapping.VirtualIP, 0.70
}
