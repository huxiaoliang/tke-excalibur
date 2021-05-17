/*
Copyright 2020 The OpenExcalibur Authors.

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

package constants

const (
	TunnelServerReversePorxyPort   = 10261
	TunnelServerAgentPort          = 10262
	TunnelServerMasterPort         = 10263
	TunnelServerMasterInsecurePort = 10264
	TunnelServerServiceName        = "x-tunnel-server-svc"
	TunnelServerAgentPortName      = "tcp"
	TunnelServerExternalAddrKey    = "x-tunnel-server-external-addr"
	TunnelEndpointsName            = "x-tunnel-server-svc"

	// tunnel PKI related constants
	TunnelCSROrg                 = "excalibur:tunnel"
	TunnelServerCSROrg           = "system:masters"
	TunnelServerCSRCN            = "kube-apiserver-kubelet-client"
	TunnelCAFile                 = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
	TunnelTokenFile              = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	TunnelAgentCAFile            = "/var/lib/tunnel-agent/serviceaccount/ca.crt"
	TunnelAgentTokenFile         = "/var/lib/tunnel-agent/serviceaccount/token"
	TunnelServerCertDir          = "/var/lib/%s/pki"
	TunnelAgentCertDir           = "/var/lib/%s/pki"
	TunnelCSRApproverThreadiness = 2

	// name of the environment variables used in pod
	TunnelAgentPodIPEnv = "POD_IP"
	// name of the environment variables used in pod
	TunnelServerNSEnv = "TUNNEL_SERVER_NAMESPACE"
	// probe the client every 10 seconds to ensure the connection is still active
	TunnelANPGrpcKeepAliveTimeSec = 10
	// wait 5 seconds for the probe ack before cutting the connection
	TunnelANPGrpcKeepAliveTimeoutSec = 5
)
