/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package kops

import "k8s.io/apimachinery/pkg/api/resource"

// NetworkingSpec allows selection and configuration of a networking plugin
type NetworkingSpec struct {
	Classic    *ClassicNetworkingSpec    `json:"classic,omitempty"`
	Kubenet    *KubenetNetworkingSpec    `json:"kubenet,omitempty"`
	External   *ExternalNetworkingSpec   `json:"external,omitempty"`
	CNI        *CNINetworkingSpec        `json:"cni,omitempty"`
	Kopeio     *KopeioNetworkingSpec     `json:"kopeio,omitempty"`
	Weave      *WeaveNetworkingSpec      `json:"weave,omitempty"`
	Flannel    *FlannelNetworkingSpec    `json:"flannel,omitempty"`
	Calico     *CalicoNetworkingSpec     `json:"calico,omitempty"`
	Canal      *CanalNetworkingSpec      `json:"canal,omitempty"`
	Kuberouter *KuberouterNetworkingSpec `json:"kuberouter,omitempty"`
	Romana     *RomanaNetworkingSpec     `json:"romana,omitempty"`
	AmazonVPC  *AmazonVPCNetworkingSpec  `json:"amazonvpc,omitempty"`
	Cilium     *CiliumNetworkingSpec     `json:"cilium,omitempty"`
	LyftVPC    *LyftVPCNetworkingSpec    `json:"lyftvpc,omitempty"`
	GCE        *GCENetworkingSpec        `json:"gce,omitempty"`
}

// ClassicNetworkingSpec is the specification of classic networking mode, integrated into kubernetes.
// Support been removed since Kubernetes 1.4.
type ClassicNetworkingSpec struct {
}

// KubenetNetworkingSpec is the specification for kubenet networking, largely integrated but intended to replace classic
type KubenetNetworkingSpec struct {
}

// ExternalNetworkingSpec is the specification for networking that is implemented by a user-provided Daemonset that uses the Kubenet kubelet networking plugin.
type ExternalNetworkingSpec struct {
}

// CNINetworkingSpec is the specification for networking that is implemented by a user-provided Daemonset, which uses the CNI kubelet networking plugin.
type CNINetworkingSpec struct {
	UsesSecondaryIP bool `json:"usesSecondaryIP,omitempty"`
}

// KopeioNetworkingSpec declares that we want Kopeio networking
type KopeioNetworkingSpec struct {
}

// WeaveNetworkingSpec declares that we want Weave networking
type WeaveNetworkingSpec struct {
	MTU         *int32 `json:"mtu,omitempty"`
	ConnLimit   *int32 `json:"connLimit,omitempty"`
	NoMasqLocal *int32 `json:"noMasqLocal,omitempty"`

	// MemoryRequest memory request of weave container. Default 200Mi
	MemoryRequest *resource.Quantity `json:"memoryRequest,omitempty"`
	// CPURequest CPU request of weave container. Default 50m
	CPURequest *resource.Quantity `json:"cpuRequest,omitempty"`
	// MemoryLimit memory limit of weave container. Default 200Mi
	MemoryLimit *resource.Quantity `json:"memoryLimit,omitempty"`
	// CPULimit CPU limit of weave container.
	CPULimit *resource.Quantity `json:"cpuLimit,omitempty"`
	// NetExtraArgs are extra arguments that are passed to weave-kube.
	NetExtraArgs string `json:"netExtraArgs,omitempty"`

	// NPCMemoryRequest memory request of weave npc container. Default 200Mi
	NPCMemoryRequest *resource.Quantity `json:"npcMemoryRequest,omitempty"`
	// NPCCPURequest CPU request of weave npc container. Default 50m
	NPCCPURequest *resource.Quantity `json:"npcCPURequest,omitempty"`
	// NPCMemoryLimit memory limit of weave npc container. Default 200Mi
	NPCMemoryLimit *resource.Quantity `json:"npcMemoryLimit,omitempty"`
	// NPCCPULimit CPU limit of weave npc container
	NPCCPULimit *resource.Quantity `json:"npcCPULimit,omitempty"`
	// NPCExtraArgs are extra arguments that are passed to weave-npc.
	NPCExtraArgs string `json:"npcExtraArgs,omitempty"`

	// Version specifies the Weave container image tag. The default depends on the kOps version.
	Version string `json:"version,omitempty"`
}

// FlannelNetworkingSpec declares that we want Flannel networking
type FlannelNetworkingSpec struct {
	// Backend is the backend overlay type we want to use (vxlan or udp)
	Backend string `json:"backend,omitempty"`
	// DisableTxChecksumOffloading is deprecated as of kOps 1.19 and has no effect.
	DisableTxChecksumOffloading bool `json:"disableTxChecksumOffloading,omitempty"`
	// IptablesResyncSeconds sets resync period for iptables rules, in seconds
	IptablesResyncSeconds *int32 `json:"iptablesResyncSeconds,omitempty"`
}

// CalicoNetworkingSpec declares that we want Calico networking
type CalicoNetworkingSpec struct {
	// Version overrides the Calico container image registry.
	Registry string `json:"registry,omitempty"`
	// Version overrides the Calico container image tag.
	Version string `json:"version,omitempty"`

	// AWSSrcDstCheck enables/disables ENI source/destination checks (AWS only)
	// Options: Disable (default), Enable, or DoNothing
	AWSSrcDstCheck string `json:"awsSrcDstCheck,omitempty"`
	// BPFEnabled enables the eBPF dataplane mode.
	BPFEnabled bool `json:"bpfEnabled,omitempty"`
	// BPFExternalServiceMode controls how traffic from outside the cluster to NodePorts and ClusterIPs is handled.
	// In Tunnel mode, packet is tunneled from the ingress host to the host with the backing pod and back again.
	// In DSR mode, traffic is tunneled to the host with the backing pod and then returned directly;
	// this requires a network that allows direct return.
	// Default: Tunnel (other options: DSR)
	BPFExternalServiceMode string `json:"bpfExternalServiceMode,omitempty"`
	// BPFKubeProxyIptablesCleanupEnabled controls whether Felix will clean up the iptables rules
	// created by the Kubernetes kube-proxy; should only be enabled if kube-proxy is not running.
	BPFKubeProxyIptablesCleanupEnabled bool `json:"bpfKubeProxyIptablesCleanupEnabled,omitempty"`
	// BPFLogLevel controls the log level used by the BPF programs. The logs are emitted
	// to the BPF trace pipe, accessible with the command tc exec BPF debug.
	// Default: Off (other options: Info, Debug)
	BPFLogLevel string `json:"bpfLogLevel,omitempty"`
	// ChainInsertMode controls whether Felix inserts rules to the top of iptables chains, or
	// appends to the bottom. Leaving the default option is safest to prevent accidentally
	// breaking connectivity. Default: 'insert' (other options: 'append')
	ChainInsertMode string `json:"chainInsertMode,omitempty"`
	// CPURequest CPU request of Calico container. Default: 100m
	CPURequest *resource.Quantity `json:"cpuRequest,omitempty"`
	// CrossSubnet is deprecated as of kOps 1.22 and has no effect
	CrossSubnet *bool `json:"crossSubnet,omitempty"`
	// EncapsulationMode specifies the network packet encapsulation protocol for Calico to use,
	// employing such encapsulation at the necessary scope per the related CrossSubnet field. In
	// "ipip" mode, Calico will use IP-in-IP encapsulation as needed. In "vxlan" mode, Calico will
	// encapsulate packets as needed using the VXLAN scheme.
	// Options: ipip (default) or vxlan
	EncapsulationMode string `json:"encapsulationMode,omitempty"`
	// IPIPMode determines when to use IP-in-IP encapsulation for the default Calico IPv4 pool.
	// It is conveyed to the "calico-node" daemon container via the CALICO_IPV4POOL_IPIP
	// environment variable. EncapsulationMode must be set to "ipip".
	// Options: "CrossSubnet", "Always", or "Never".
	// Default: "CrossSubnet" if EncapsulationMode is "ipip", "Never" otherwise.
	IPIPMode string `json:"ipipMode,omitempty"`
	// IPv4AutoDetectionMethod configures how Calico chooses the IP address used to route
	// between nodes.  This should be set when the host has multiple interfaces
	// and it is important to select the interface used.
	// Options: "first-found" (default), "can-reach=DESTINATION",
	// "interface=INTERFACE-REGEX", or "skip-interface=INTERFACE-REGEX"
	IPv4AutoDetectionMethod string `json:"ipv4AutoDetectionMethod,omitempty"`
	// IPv6AutoDetectionMethod configures how Calico chooses the IP address used to route
	// between nodes.  This should be set when the host has multiple interfaces
	// and it is important to select the interface used.
	// Options: "first-found" (default), "can-reach=DESTINATION",
	// "interface=INTERFACE-REGEX", or "skip-interface=INTERFACE-REGEX"
	IPv6AutoDetectionMethod string `json:"ipv6AutoDetectionMethod,omitempty"`
	// IptablesBackend controls which variant of iptables binary Felix uses
	// Default: Auto (other options: Legacy, NFT)
	IptablesBackend string `json:"iptablesBackend,omitempty"`
	// LogSeverityScreen lets us set the desired log level. (Default: info)
	LogSeverityScreen string `json:"logSeverityScreen,omitempty"`
	// MTU to be set in the cni-network-config for calico.
	MTU *int32 `json:"mtu,omitempty"`
	// PrometheusMetricsEnabled can be set to enable the experimental Prometheus
	// metrics server (default: false)
	PrometheusMetricsEnabled bool `json:"prometheusMetricsEnabled,omitempty"`
	// PrometheusMetricsPort is the TCP port that the experimental Prometheus
	// metrics server should bind to (default: 9091)
	PrometheusMetricsPort int32 `json:"prometheusMetricsPort,omitempty"`
	// PrometheusGoMetricsEnabled enables Prometheus Go runtime metrics collection
	PrometheusGoMetricsEnabled bool `json:"prometheusGoMetricsEnabled,omitempty"`
	// PrometheusProcessMetricsEnabled enables Prometheus process metrics collection
	PrometheusProcessMetricsEnabled bool `json:"prometheusProcessMetricsEnabled,omitempty"`
	// MajorVersion is deprecated as of kOps 1.20 and has no effect
	MajorVersion string `json:"majorVersion,omitempty"`
	// TyphaPrometheusMetricsEnabled enables Prometheus metrics collection from Typha
	// (default: false)
	TyphaPrometheusMetricsEnabled bool `json:"typhaPrometheusMetricsEnabled,omitempty"`
	// TyphaPrometheusMetricsPort is the TCP port the typha Prometheus metrics server
	// should bind to (default: 9093)
	TyphaPrometheusMetricsPort int32 `json:"typhaPrometheusMetricsPort,omitempty"`
	// TyphaReplicas is the number of replicas of Typha to deploy
	TyphaReplicas int32 `json:"typhaReplicas,omitempty"`
	// VXLANMode determines when to use VXLAN encapsulation for the default Calico IPv4 pool.
	// It is conveyed to the "calico-node" daemon container via the CALICO_IPV4POOL_VXLAN
	// environment variable. EncapsulationMode must be set to "vxlan".
	// Options: "CrossSubnet", "Always", or "Never".
	// Default: "CrossSubnet" if EncapsulationMode is "vxlan", "Never" otherwise.
	VXLANMode string `json:"vxlanMode,omitempty"`
	// WireguardEnabled enables WireGuard encryption for all on-the-wire pod-to-pod traffic
	// (default: false)
	WireguardEnabled bool `json:"wireguardEnabled,omitempty"`
}

// CanalNetworkingSpec declares that we want Canal networking
type CanalNetworkingSpec struct {
	// ChainInsertMode controls whether Felix inserts rules to the top of iptables chains, or
	// appends to the bottom. Leaving the default option is safest to prevent accidentally
	// breaking connectivity. Default: 'insert' (other options: 'append')
	ChainInsertMode string `json:"chainInsertMode,omitempty"`
	// CPURequest CPU request of Canal container. Default: 100m
	CPURequest *resource.Quantity `json:"cpuRequest,omitempty"`
	// DefaultEndpointToHostAction allows users to configure the default behaviour
	// for traffic between pod to host after calico rules have been processed.
	// Default: ACCEPT (other options: DROP, RETURN)
	DefaultEndpointToHostAction string `json:"defaultEndpointToHostAction,omitempty"`
	// DisableFlannelForwardRules configures Flannel to NOT add the
	// default ACCEPT traffic rules to the iptables FORWARD chain
	DisableFlannelForwardRules bool `json:"disableFlannelForwardRules,omitempty"`
	// DisableTxChecksumOffloading is deprecated as of kOps 1.19 and has no effect.
	DisableTxChecksumOffloading bool `json:"disableTxChecksumOffloading,omitempty"`
	// IptablesBackend controls which variant of iptables binary Felix uses
	// Default: Auto (other options: Legacy, NFT)
	IptablesBackend string `json:"iptablesBackend,omitempty"`
	// LogSeveritySys the severity to set for logs which are sent to syslog
	// Default: INFO (other options: DEBUG, WARNING, ERROR, CRITICAL, NONE)
	LogSeveritySys string `json:"logSeveritySys,omitempty"`
	// MTU to be set in the cni-network-config (default: 1500)
	MTU *int32 `json:"mtu,omitempty"`
	// PrometheusGoMetricsEnabled enables Prometheus Go runtime metrics collection
	PrometheusGoMetricsEnabled bool `json:"prometheusGoMetricsEnabled,omitempty"`
	// PrometheusMetricsEnabled can be set to enable the experimental Prometheus
	// metrics server (default: false)
	PrometheusMetricsEnabled bool `json:"prometheusMetricsEnabled,omitempty"`
	// PrometheusMetricsPort is the TCP port that the experimental Prometheus
	// metrics server should bind to (default: 9091)
	PrometheusMetricsPort int32 `json:"prometheusMetricsPort,omitempty"`
	// PrometheusProcessMetricsEnabled enables Prometheus process metrics collection
	PrometheusProcessMetricsEnabled bool `json:"prometheusProcessMetricsEnabled,omitempty"`
	// TyphaPrometheusMetricsEnabled enables Prometheus metrics collection from Typha
	// (default: false)
	TyphaPrometheusMetricsEnabled bool `json:"typhaPrometheusMetricsEnabled,omitempty"`
	// TyphaPrometheusMetricsPort is the TCP port the typha Prometheus metrics server
	// should bind to (default: 9093)
	TyphaPrometheusMetricsPort int32 `json:"typhaPrometheusMetricsPort,omitempty"`
	// TyphaReplicas is the number of replicas of Typha to deploy
	TyphaReplicas int32 `json:"typhaReplicas,omitempty"`
}

// KuberouterNetworkingSpec declares that we want Kube-router networking
type KuberouterNetworkingSpec struct {
}

// RomanaNetworkingSpec declares that we want Romana networking
// Romana is deprecated as of kOps 1.18 and removed as of kOps 1.19.
type RomanaNetworkingSpec struct {
	// DaemonServiceIP is the Kubernetes Service IP for the romana-daemon pod
	DaemonServiceIP string `json:"daemonServiceIP,omitempty"`
	// EtcdServiceIP is the Kubernetes Service IP for the etcd backend used by Romana
	EtcdServiceIP string `json:"etcdServiceIP,omitempty"`
}

// AmazonVPCNetworkingSpec declares that we want Amazon VPC CNI networking
type AmazonVPCNetworkingSpec struct {
	// ImageName is the container image name to use.
	ImageName string `json:"imageName,omitempty"`
	// InitImageName is the init container image name to use.
	InitImageName string `json:"initImageName,omitempty"`
	// Env is a list of environment variables to set in the container.
	Env []EnvVar `json:"env,omitempty"`
}

const CiliumIpamEni = "eni"

// CiliumNetworkingSpec declares that we want Cilium networking
type CiliumNetworkingSpec struct {
	// Version is the version of the Cilium agent and the Cilium Operator.
	Version string `json:"version,omitempty"`

	// MemoryRequest memory request of Cilium agent + operator container. (default: 128Mi)
	MemoryRequest *resource.Quantity `json:"memoryRequest,omitempty"`
	// CPURequest CPU request of Cilium agent + operator container. (default: 25m)
	CPURequest *resource.Quantity `json:"cpuRequest,omitempty"`

	// AccessLog is not implemented and may be removed in the future.
	// Setting this has no effect.
	AccessLog string `json:"accessLog,omitempty"`
	// AgentLabels is not implemented and may be removed in the future.
	// Setting this has no effect.
	AgentLabels []string `json:"agentLabels,omitempty"`
	// AgentPrometheusPort is the port to listen to for Prometheus metrics.
	// Defaults to 9090.
	AgentPrometheusPort int `json:"agentPrometheusPort,omitempty"`
	// AllowLocalhost is not implemented and may be removed in the future.
	// Setting this has no effect.
	AllowLocalhost string `json:"allowLocalhost,omitempty"`
	// AutoIpv6NodeRoutes is not implemented and may be removed in the future.
	// Setting this has no effect.
	AutoIpv6NodeRoutes bool `json:"autoIpv6NodeRoutes,omitempty"`
	// BPFRoot is not implemented and may be removed in the future.
	// Setting this has no effect.
	BPFRoot string `json:"bpfRoot,omitempty"`
	// ContainerRuntime is not implemented and may be removed in the future.
	// Setting this has no effect.
	ContainerRuntime []string `json:"containerRuntime,omitempty"`
	// ContainerRuntimeEndpoint is not implemented and may be removed in the future.
	// Setting this has no effect.
	ContainerRuntimeEndpoint map[string]string `json:"containerRuntimeEndpoint,omitempty"`
	// Debug runs Cilium in debug mode.
	Debug bool `json:"debug,omitempty"`
	// DebugVerbose is not implemented and may be removed in the future.
	// Setting this has no effect.
	DebugVerbose []string `json:"debugVerbose,omitempty"`
	// Device is not implemented and may be removed in the future.
	// Setting this has no effect.
	Device string `json:"device,omitempty"`
	// DisableConntrack is not implemented and may be removed in the future.
	// Setting this has no effect.
	DisableConntrack bool `json:"disableConntrack,omitempty"`
	// DisableEndpointCRD disables usage of CiliumEndpoint CRD.
	// Default: false
	DisableEndpointCRD bool `json:"disableEndpointCRD,omitempty"`
	// DisableIpv4 is deprecated: Use EnableIpv4 instead.
	// Setting this flag has no effect.
	DisableIpv4 bool `json:"disableIpv4,omitempty"`
	// DisableK8sServices is not implemented and may be removed in the future.
	// Setting this has no effect.
	DisableK8sServices bool `json:"disableK8sServices,omitempty"`
	// EnablePolicy specifies the policy enforcement mode.
	// "default": Follows Kubernetes policy enforcement.
	// "always": Cilium restricts all traffic if no policy is in place.
	// "never": Cilium allows all traffic regardless of policies in place.
	// If unspecified, "default" policy mode will be used.
	EnablePolicy string `json:"enablePolicy,omitempty"`
	// EnableL7Proxy enables L7 proxy for L7 policy enforcement.
	// Default: true
	EnableL7Proxy *bool `json:"enableL7Proxy,omitempty"`
	// EnableBPFMasquerade enables masquerading packets from endpoints leaving the host with BPF instead of iptables.
	// Default: false
	EnableBPFMasquerade *bool `json:"enableBPFMasquerade,omitempty"`
	// EnableEndpointHealthChecking enables connectivity health checking between virtual endpoints.
	// Default: true
	EnableEndpointHealthChecking *bool `json:"enableEndpointHealthChecking,omitempty"`
	// EnableTracing is not implemented and may be removed in the future.
	// Setting this has no effect.
	EnableTracing bool `json:"enableTracing,omitempty"`
	// EnablePrometheusMetrics enables the Cilium "/metrics" endpoint for both the agent and the operator.
	EnablePrometheusMetrics bool `json:"enablePrometheusMetrics,omitempty"`
	// EnableEncryption enables Cilium Encryption.
	// Default: false
	EnableEncryption bool `json:"enableEncryption,omitempty"`
	// EnvoyLog is not implemented and may be removed in the future.
	// Setting this has no effect.
	EnvoyLog string `json:"envoyLog,omitempty"`
	// IdentityAllocationMode specifies in which backend identities are stored ("crd", "kvstore").
	// Default: crd
	IdentityAllocationMode string `json:"identityAllocationMode,omitempty"`
	// IdentityChangeGracePeriod specifies the duration to wait before using a changed identity.
	// Default: 5s
	IdentityChangeGracePeriod string `json:"identityChangeGracePeriod,omitempty"`
	// Ipv4ClusterCIDRMaskSize is not implemented and may be removed in the future.
	// Setting this has no effect.
	Ipv4ClusterCIDRMaskSize int `json:"ipv4ClusterCidrMaskSize,omitempty"`
	// Ipv4Node is not implemented and may be removed in the future.
	// Setting this has no effect.
	Ipv4Node string `json:"ipv4Node,omitempty"`
	// Ipv4Range is not implemented and may be removed in the future.
	// Setting this has no effect.
	Ipv4Range string `json:"ipv4Range,omitempty"`
	// Ipv4ServiceRange is not implemented and may be removed in the future.
	// Setting this has no effect.
	Ipv4ServiceRange string `json:"ipv4ServiceRange,omitempty"`
	// Ipv6ClusterAllocCidr is not implemented and may be removed in the future.
	// Setting this has no effect.
	Ipv6ClusterAllocCidr string `json:"ipv6ClusterAllocCidr,omitempty"`
	// Ipv6Node is not implemented and may be removed in the future.
	// Setting this has no effect.
	Ipv6Node string `json:"ipv6Node,omitempty"`
	// Ipv6Range is not implemented and may be removed in the future.
	// Setting this has no effect.
	Ipv6Range string `json:"ipv6Range,omitempty"`
	// Ipv6ServiceRange is not implemented and may be removed in the future.
	// Setting this has no effect.
	Ipv6ServiceRange string `json:"ipv6ServiceRange,omitempty"`
	// K8sAPIServer is not implemented and may be removed in the future.
	// Setting this has no effect.
	K8sAPIServer string `json:"k8sApiServer,omitempty"`
	// K8sKubeconfigPath is not implemented and may be removed in the future.
	// Setting this has no effect.
	K8sKubeconfigPath string `json:"k8sKubeconfigPath,omitempty"`
	// KeepBPFTemplates is not implemented and may be removed in the future.
	// Setting this has no effect.
	KeepBPFTemplates bool `json:"keepBpfTemplates,omitempty"`
	// KeepConfig is not implemented and may be removed in the future.
	// Setting this has no effect.
	KeepConfig bool `json:"keepConfig,omitempty"`
	// LabelPrefixFile is not implemented and may be removed in the future.
	// Setting this has currently no effect
	LabelPrefixFile string `json:"labelPrefixFile,omitempty"`
	// Labels is not implemented and may be removed in the future.
	// Setting this has no effect.
	Labels []string `json:"labels,omitempty"`
	// LB is not implemented and may be removed in the future.
	// Setting this has no effect.
	LB string `json:"lb,omitempty"`
	// LibDir is not implemented and may be removed in the future.
	// Setting this has no effect.
	LibDir string `json:"libDir,omitempty"`
	// LogDrivers is not implemented and may be removed in the future.
	// Setting this has no effect.
	LogDrivers []string `json:"logDriver,omitempty"`
	// LogOpt is not implemented and may be removed in the future.
	// Setting this has no effect.
	LogOpt map[string]string `json:"logOpt,omitempty"`
	// Logstash is not implemented and may be removed in the future.
	// Setting this has no effect.
	Logstash bool `json:"logstash,omitempty"`
	// LogstashAgent is not implemented and may be removed in the future.
	// Setting this has no effect.
	LogstashAgent string `json:"logstashAgent,omitempty"`
	// LogstashProbeTimer is not implemented and may be removed in the future.
	// Setting this has no effect.
	LogstashProbeTimer uint32 `json:"logstashProbeTimer,omitempty"`
	// DisableMasquerade disables masquerading traffic to external destinations behind the node IP.
	DisableMasquerade *bool `json:"disableMasquerade,omitempty"`
	// Nat6Range is not implemented and may be removed in the future.
	// Setting this has no effect.
	Nat46Range string `json:"nat46Range,omitempty"`
	// Pprof is not implemented and may be removed in the future.
	// Setting this has no effect.
	Pprof bool `json:"pprof,omitempty"`
	// PrefilterDevice is not implemented and may be removed in the future.
	// Setting this has no effect.
	PrefilterDevice string `json:"prefilterDevice,omitempty"`
	// PrometheusServeAddr is deprecated. Use EnablePrometheusMetrics and AgentPrometheusPort instead.
	// Setting this has no effect.
	PrometheusServeAddr string `json:"prometheusServeAddr,omitempty"`
	// Restore is not implemented and may be removed in the future.
	// Setting this has no effect.
	Restore bool `json:"restore,omitempty"`
	// SingleClusterRoute is not implemented and may be removed in the future.
	// Setting this has no effect.
	SingleClusterRoute bool `json:"singleClusterRoute,omitempty"`
	// SocketPath is not implemented and may be removed in the future.
	// Setting this has no effect.
	SocketPath string `json:"socketPath,omitempty"`
	// StateDir is not implemented and may be removed in the future.
	// Setting this has no effect.
	StateDir string `json:"stateDir,omitempty"`
	// TracePayloadLen is not implemented and may be removed in the future.
	// Setting this has no effect.
	TracePayloadLen int `json:"tracePayloadlen,omitempty"`
	// Tunnel specifies the Cilium tunnelling mode. Possible values are "vxlan", "geneve", or "disabled".
	// Default: vxlan
	Tunnel string `json:"tunnel,omitempty"`
	// EnableIpv6 is not implemented and may be removed in the future.
	// Setting this has no effect.
	EnableIpv6 bool `json:"enableipv6,omitempty"`
	// EnableIpv4 is not implemented and may be removed in the future.
	// Setting this has no effect.
	EnableIpv4 bool `json:"enableipv4,omitempty"`
	// MonitorAggregation sets the level of packet monitoring. Possible values are "low", "medium", or "maximum".
	// Default: medium
	MonitorAggregation string `json:"monitorAggregation,omitempty"`
	// BPFCTGlobalTCPMax is the maximum number of entries in the TCP CT table.
	// Default: 524288
	BPFCTGlobalTCPMax int `json:"bpfCTGlobalTCPMax,omitempty"`
	// BPFCTGlobalAnyMax is the maximum number of entries in the non-TCP CT table.
	// Default: 262144
	BPFCTGlobalAnyMax int `json:"bpfCTGlobalAnyMax,omitempty"`
	// BPFLBAlgorithm is the load balancing algorithm ("random", "maglev").
	// Default: random
	BPFLBAlgorithm string `json:"bpfLBAlgorithm,omitempty"`
	// BPFLBMaglevTableSize is the per service backend table size when going with Maglev (parameter M).
	// Default: 16381
	BPFLBMaglevTableSize string `json:"bpfLBMaglevTableSize,omitempty"`
	// BPFNATGlobalMax is the the maximum number of entries in the BPF NAT table.
	// Default: 524288
	BPFNATGlobalMax int `json:"bpfNATGlobalMax,omitempty"`
	// BPFNeighGlobalMax is the the maximum number of entries in the BPF Neighbor table.
	// Default: 524288
	BPFNeighGlobalMax int `json:"bpfNeighGlobalMax,omitempty"`
	// BPFPolicyMapMax is the maximum number of entries in endpoint policy map.
	// Default: 16384
	BPFPolicyMapMax int `json:"bpfPolicyMapMax,omitempty"`
	// BPFLBMapMax is the maximum number of entries in bpf lb service, backend and affinity maps.
	// Default: 65536
	BPFLBMapMax int `json:"bpfLBMapMax,omitempty"`
	// PreallocateBPFMaps reduces the per-packet latency at the expense of up-front memory allocation.
	// Default: true
	PreallocateBPFMaps bool `json:"preallocateBPFMaps,omitempty"`
	// SidecarIstioProxyImage is the regular expression matching compatible Istio sidecar istio-proxy
	// container image names.
	// Default: cilium/istio_proxy
	SidecarIstioProxyImage string `json:"sidecarIstioProxyImage,omitempty"`
	// ClusterName is the name of the cluster. It is only relevant when building a mesh of clusters.
	ClusterName string `json:"clusterName,omitempty"`
	// ToFqdnsDNSRejectResponseCode sets the DNS response code for rejecting DNS requests.
	// Possible values are "nameError" or "refused".
	// Default: refused
	ToFqdnsDNSRejectResponseCode string `json:"toFqdnsDnsRejectResponseCode,omitempty"`
	// ToFqdnsEnablePoller replaces the DNS proxy-based implementation of FQDN policies
	// with the less powerful legacy implementation.
	// Default: false
	ToFqdnsEnablePoller bool `json:"toFqdnsEnablePoller,omitempty"`
	// ContainerRuntimeLabels enables fetching of container-runtime labels from the specified container runtime and associating them with endpoints.
	// Supported values are: "none", "containerd", "crio", "docker", "auto"
	// As of Cilium 1.7.0, Cilium no longer fetches information from the
	// container runtime and this field is ignored.
	// Default: none
	ContainerRuntimeLabels string `json:"containerRuntimeLabels,omitempty"`
	// Ipam specifies the IP address allocation mode to use.
	// Possible values are "crd" and "eni".
	// "eni" will use AWS native networking for pods. Eni requires masquerade to be set to false.
	// "crd" will use CRDs for controlling IP address management.
	// "hostscope" will use hostscope IPAM mode.
	// "kubernetes" will use addersing based on node pod CIDR.
	// Empty value will use hostscope for cilum <= 1.7 and "kubernetes" otherwise.
	Ipam string `json:"ipam,omitempty"`
	// IPTablesRulesNoinstall disables installing the base IPTables rules used for masquerading and kube-proxy.
	// Default: false
	IPTablesRulesNoinstall bool `json:"IPTablesRulesNoinstall,omitempty"`
	// AutoDirectNodeRoutes adds automatic L2 routing between nodes.
	// Default: false
	AutoDirectNodeRoutes bool `json:"autoDirectNodeRoutes,omitempty"`
	// EnableHostReachableServices configures Cilium to enable services to be
	// reached from the host namespace in addition to pod namespaces.
	// https://docs.cilium.io/en/v1.9/gettingstarted/host-services/
	// Default: false
	EnableHostReachableServices bool `json:"enableHostReachableServices,omitempty"`
	// EnableNodePort replaces kube-proxy with Cilium's BPF implementation.
	// Requires spec.kubeProxy.enabled be set to false.
	// Default: false
	EnableNodePort bool `json:"enableNodePort,omitempty"`
	// EtcdManagd installs an additional etcd cluster that is used for Cilium state change.
	// The cluster is operated by cilium-etcd-operator.
	// Default: false
	EtcdManaged bool `json:"etcdManaged,omitempty"`
	// EnableRemoteNodeIdentity enables the remote-node-identity added in Cilium 1.7.0.
	// Default: true
	EnableRemoteNodeIdentity *bool `json:"enableRemoteNodeIdentity,omitempty"`
	// Hubble configures the Hubble service on the Cilium agent.
	Hubble *HubbleSpec `json:"hubble,omitempty"`

	// RemoveCbrBridge is not implemented and may be removed in the future.
	// Setting this has no effect.
	RemoveCbrBridge bool `json:"removeCbrBridge,omitempty"`
	// RestartPods is not implemented and may be removed in the future.
	// Setting this has no effect.
	RestartPods bool `json:"restartPods,omitempty"`
	// ReconfigureKubelet is not implemented and may be removed in the future.
	// Setting this has no effect.
	ReconfigureKubelet bool `json:"reconfigureKubelet,omitempty"`
	// NodeInitBootstrapFile is not implemented and may be removed in the future.
	// Setting this has no effect.
	NodeInitBootstrapFile string `json:"nodeInitBootstrapFile,omitempty"`
	// CniBinPath is not implemented and may be removed in the future.
	// Setting this has no effect.
	CniBinPath string `json:"cniBinPath,omitempty"`
}

// HubbleSpec configures the Hubble service on the Cilium agent.
type HubbleSpec struct {
	// Enabled decides if Hubble is enabled on the agent or not
	Enabled *bool `json:"enabled,omitempty"`

	// Metrics is a list of metrics to collect. If empty or null, metrics are disabled.
	// See https://docs.cilium.io/en/stable/configuration/metrics/#hubble-exported-metrics
	Metrics []string `json:"metrics,omitempty"`
}

// LyftVPCNetworkingSpec declares that we want to use the cni-ipvlan-vpc-k8s CNI networking.
type LyftVPCNetworkingSpec struct {
	SubnetTags map[string]string `json:"subnetTags,omitempty"`
}

// GCENetworkingSpec is the specification of GCE's native networking mode, using IP aliases
type GCENetworkingSpec struct {
}
