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

package agent

import (
	"crypto/tls"
	"time"

	"github.com/tkestack/tke-excalibur/pkg/version"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"k8s.io/klog/v2"
	anpagent "sigs.k8s.io/apiserver-network-proxy/pkg/agent"
)

// anpTunnelAgent implements the TunnelAgent using the
// apiserver-network-proxy package
type anpTunnelAgent struct {
	tlsCfg           *tls.Config
	tunnelServerAddr string
	clusterName      string
	agentIdentifiers string
}

var _ TunnelAgent = &anpTunnelAgent{}

// RunAgent runs the tunnel-agent which will try to connect tunnel-server
func (ata *anpTunnelAgent) Run(stopChan <-chan struct{}) {
	dialOption := grpc.WithTransportCredentials(credentials.NewTLS(ata.tlsCfg))
	cc := &anpagent.ClientSetConfig{
		Address:                 ata.tunnelServerAddr,
		AgentID:                 ata.clusterName,
		AgentIdentifiers:        ata.agentIdentifiers,
		SyncInterval:            5 * time.Second,
		ProbeInterval:           5 * time.Second,
		DialOptions:             []grpc.DialOption{dialOption},
		ServiceAccountTokenPath: "",
	}

	cs := cc.NewAgentClientSet(stopChan)
	cs.Serve()
	klog.Infof("start serving grpc request redirected from %s: %s",
		version.GetServerName(), ata.tunnelServerAddr)
}
