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
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/certificate"
	"k8s.io/klog/v2"
	"sigs.k8s.io/apiserver-network-proxy/pkg/agent"
	"yunion.io/x/pkg/util/wait"

	"github.com/tkestack/tke-excalibur/pkg/tunnel/constants"
	"github.com/tkestack/tke-excalibur/pkg/tunnel/hook/interfaces"
	"github.com/tkestack/tke-excalibur/pkg/tunnel/k8s"
	"github.com/tkestack/tke-excalibur/pkg/tunnel/pki"
	"github.com/tkestack/tke-excalibur/pkg/tunnel/pki/certmanager"
	"github.com/tkestack/tke-excalibur/pkg/tunnel/server/serveraddr"
	"github.com/tkestack/tke-excalibur/pkg/version"
)

// NewTunnelAgentCommand creates a new tunnel-agent command
func NewTunnelAgentCommand(provider interfaces.TunnelHookProvider, stopCh <-chan struct{}) *cobra.Command {
	o := &TunnelAgentOptions{}
	cmd := &cobra.Command{
		Short: fmt.Sprintf("Launch %s", version.GetAgentName()),
		RunE: func(c *cobra.Command, args []string) error {
			if o.version {
				fmt.Printf("%s: %#v\n", version.GetAgentName(), version.Get())
				return nil
			}
			fmt.Printf("%s version: %#v\n", version.GetAgentName(), version.Get())

			if err := o.validate(); err != nil {
				return err
			}
			if err := o.complete(provider); err != nil {
				return err
			}
			if err := o.run(stopCh); err != nil {
				return err
			}
			return nil
		},
	}

	flags := cmd.Flags()
	flags.BoolVar(&o.version, "version", o.version,
		"print the version information.")
	flags.StringVar(&o.clusterName, "cluster-name", o.clusterName,
		"The name of the cluster.")
	flags.StringVar(&o.tunnelServerAddr, "tunnelserver-addr", o.tunnelServerAddr,
		fmt.Sprintf("The address of %s", version.GetServerName()))
	flags.StringVar(&o.apiserverAddr, "apiserver-addr", o.tunnelServerAddr,
		"A reachable address of the apiserver.")
	flags.StringVar(&o.kubeConfig, "kube-config", o.kubeConfig,
		"Path to the kubeconfig file.")
	flags.StringVar(&o.agentIdentifiers, "agent-identifiers", o.agentIdentifiers,
		"The identifiers of the agent, which will be used by the server when choosing agent.")
	return cmd
}

// TunnelAgentOptions has the information that required by the
// tunnel-agent
type TunnelAgentOptions struct {
	clusterName      string
	tunnelServerAddr string
	apiserverAddr    string
	kubeConfig       string
	version          bool
	// the clinet to access cloud k8s api server
	cloudClientSet kubernetes.Interface
	// the clinet to access local k8s api server
	localClientSet   kubernetes.Interface
	agentIdentifiers string
	hookProvider     interfaces.TunnelHookProvider
}

// validate validates the TunnelServerOptions
func (o *TunnelAgentOptions) validate() error {
	if o.clusterName == "" {
		return errors.New("--cluster-name is not set")
	}

	if !agentIdentifiersAreValid(o.agentIdentifiers) {
		return errors.New("--agent-identifiers are invalid, format should be host={cluster-name}")
	}

	return nil
}

// complete completes all the required options
func (o *TunnelAgentOptions) complete(provider interfaces.TunnelHookProvider) error {
	var err error

	if len(o.agentIdentifiers) == 0 {
		o.agentIdentifiers = fmt.Sprintf("host=%s", o.clusterName)
	}
	klog.Infof("%s is set for agent identifies", o.agentIdentifiers)

	if o.apiserverAddr != "" {
		klog.Infof("create the clientset based on the apiserver address(%s).", o.apiserverAddr)
		o.cloudClientSet, err = k8s.CreateClientSetApiserverAddr(o.apiserverAddr)
		return err
	}

	if o.kubeConfig != "" {
		klog.Infof("create the clientset based on the kubeconfig(%s).", o.kubeConfig)
		o.localClientSet, err = k8s.CreateClientSetKubeConfig(o.kubeConfig)
		return err
	} else {
		klog.Infof("create the clientset based on the kubeconfig(in-cluster config).")
		o.localClientSet, err = k8s.CreateClientSet(o.kubeConfig)
	}

	if o.hookProvider != nil {
		o.hookProvider = provider
		klog.Infof("set hook provider to [%s].", o.hookProvider.GetProviderName())
	}
	return err
}

// run starts the tunnel-agent
func (o *TunnelAgentOptions) run(stopCh <-chan struct{}) error {
	var (
		tunnelServerAddr string
		err              error
		agentCertMgr     certificate.Manager
	)

	// 1. excute pre start tunnel agent hook
	if o.hookProvider != nil {
		err = o.hookProvider.PreStartTunnelAgent(o.clusterName, o.cloudClientSet, o.localClientSet)
		if err != nil {
			return fmt.Errorf("faild to excute %s provider pre-start tunnel agent hook due to %v",
				o.hookProvider.GetProviderName(), err)
		}
	}

	// 2. get the address of the tunnel-server
	tunnelServerAddr = o.tunnelServerAddr
	if o.tunnelServerAddr == "" {
		if tunnelServerAddr, err = serveraddr.GetTunnelServerAddr(o.cloudClientSet); err != nil {
			return err
		}
	}
	klog.Infof("%s address: %s", version.GetServerName(), tunnelServerAddr)

	// 3. create a certificate manager
	agentCertMgr, err =
		certmanager.NewTunnelAgentCertManager(o.cloudClientSet, o.clusterName)
	if err != nil {
		return err
	}
	agentCertMgr.Start()

	// 4. generate a TLS configuration for securing the connection to server
	tlsCfg, err := pki.GenTLSConfigUseCertMgrAndCA(agentCertMgr,
		tunnelServerAddr, constants.TunnelAgentCAFile)
	if err != nil {
		return err
	}
	// 5. waiting for the certificate is generated
	_ = wait.PollUntil(5*time.Second, func() (bool, error) {
		// keep polling until the certificate is signed
		if agentCertMgr.Current() != nil {
			return true, nil
		}
		klog.Infof("waiting for the master to sign the %s certificate",
			version.GetAgentName())
		return false, nil
	}, stopCh)

	// 6. start the tunnel-agent
	ta := NewTunnelAgent(tlsCfg, tunnelServerAddr, o.clusterName, o.agentIdentifiers)
	ta.Run(stopCh)

	// 7. excute post start tunnel agent hook
	if o.hookProvider != nil {
		err = o.hookProvider.PostStartTunnelAgent(o.clusterName, o.cloudClientSet, o.localClientSet)
		if err != nil {
			return fmt.Errorf("faild to excute %s provider post-start tunnel agent hook due to %v",
				o.hookProvider.GetProviderName(), err)
		}
	}
	<-stopCh
	return nil
}

// agentIdentifiersIsValid verify agent identifiers are valid or not
func agentIdentifiersAreValid(agentIdentifiers string) bool {
	if len(agentIdentifiers) == 0 {
		return true
	}

	entries := strings.Split(agentIdentifiers, ",")
	for i := range entries {
		parts := strings.Split(entries[i], "=")
		if len(parts) != 2 {
			return false
		}

		switch agent.IdentifierType(parts[0]) {
		case agent.Host, agent.CIDR, agent.IPv4, agent.IPv6, agent.UID:
			// valid agent identifier
		default:
			return false
		}
	}

	return true
}
