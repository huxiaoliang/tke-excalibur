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

package server

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/tkestack/tke-excalibur/pkg/tunnel/constants"
	"github.com/tkestack/tke-excalibur/pkg/tunnel/hook/interfaces"
	"github.com/tkestack/tke-excalibur/pkg/tunnel/k8s"
	"github.com/tkestack/tke-excalibur/pkg/tunnel/pki"
	"github.com/tkestack/tke-excalibur/pkg/tunnel/pki/certmanager"
	"github.com/tkestack/tke-excalibur/pkg/version"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"sigs.k8s.io/apiserver-network-proxy/pkg/server"
)

// NewTunnelServerCommand creates a new tunnel-server command
func NewTunnelServerCommand(provider interfaces.TunnelHookProvider, stopCh <-chan struct{}) *cobra.Command {
	o := NewTunnelServerOptions()

	cmd := &cobra.Command{
		Use:   "Launch " + version.GetServerName(),
		Short: version.GetServerName() + " sends requests to " + version.GetAgentName(),
		RunE: func(c *cobra.Command, args []string) error {
			if o.version {
				fmt.Printf("%s: %#v\n", version.GetServerName(), version.Get())
				return nil
			}
			fmt.Printf("%s version: %#v\n", version.GetServerName(), version.Get())

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
		fmt.Sprintf("print the version information of the %s.",
			version.GetServerName()))
	flags.StringVar(&o.kubeConfig, "kube-config", o.kubeConfig,
		"path to the kubeconfig file.")
	flags.StringVar(&o.bindAddr, "bind-address", o.bindAddr,
		fmt.Sprintf("the ip address on which the %s will listen.",
			version.GetServerName()))
	flags.StringVar(&o.insecureBindAddr, "insecure-bind-address", o.insecureBindAddr,
		fmt.Sprintf("the ip address on which the %s will listen without tls.",
			version.GetServerName()))
	flags.StringVar(&o.certDNSNames, "cert-dns-names", o.certDNSNames,
		"DNS names that will be added into server's certificate. (e.g., dns1,dns2)")
	flags.StringVar(&o.certIPs, "cert-ips", o.certIPs,
		"IPs that will be added into server's certificate. (e.g., ip1,ip2)")
	flags.IntVar(&o.serverCount, "server-count", o.serverCount,
		"The number of proxy server instances, should be 1 unless it is an HA server.")
	flags.StringVar(&o.proxyStrategy, "proxy-strategy", o.proxyStrategy,
		"The strategy of proxying requests from tunnel server to agent.")
	flags.StringVar(&o.udsName, "uds-name", o.udsName,
		"uds-name should be empty for TCP traffic. For UDS set to its name.")
	return cmd
}

// TunnelServerOptions has the information that required by the
// tunnel-server
type TunnelServerOptions struct {
	kubeConfig               string
	bindAddr                 string
	insecureBindAddr         string
	certDNSNames             string
	certIPs                  string
	version                  bool
	serverAgentPort          int
	serverMasterPort         int
	serverMasterInsecurePort int
	serverCount              int
	serverAgentAddr          string
	serverMasterAddr         string
	serverMasterInsecureAddr string
	clientSet                kubernetes.Interface
	sharedInformerFactory    informers.SharedInformerFactory
	proxyStrategy            string
	udsName                  string
	hookProvider             interfaces.TunnelHookProvider
}

// NewTunnelServerOptions creates a new ExcaliburNewTunnelServerOptions
func NewTunnelServerOptions() *TunnelServerOptions {
	o := &TunnelServerOptions{
		bindAddr:                 "0.0.0.0",
		insecureBindAddr:         "127.0.0.1",
		serverCount:              1,
		serverAgentPort:          constants.TunnelServerAgentPort,
		serverMasterPort:         constants.TunnelServerMasterPort,
		serverMasterInsecurePort: constants.TunnelServerMasterInsecurePort,
		proxyStrategy:            string(server.ProxyStrategyDestHost),
	}
	return o
}

// validate validates the TunnelServerOptions
func (o *TunnelServerOptions) validate() error {
	if len(o.bindAddr) == 0 {
		return fmt.Errorf("%s's bind address can't be empty",
			version.GetServerName())
	}
	return nil
}

// complete completes all the required options
func (o *TunnelServerOptions) complete(provider interfaces.TunnelHookProvider) error {
	o.serverAgentAddr = fmt.Sprintf("%s:%d", o.bindAddr, o.serverAgentPort)
	o.serverMasterAddr = fmt.Sprintf("%s:%d", o.bindAddr, o.serverMasterPort)
	klog.Infof("server will accept %s requests at: %s, "+
		"server will accept master https requests at: %s ",
		version.GetAgentName(), o.serverAgentAddr, o.serverMasterAddr)

	var err error
	o.clientSet, err = k8s.CreateClientSet(o.kubeConfig)
	if err != nil {
		return err
	}

	o.sharedInformerFactory =
		informers.NewSharedInformerFactory(o.clientSet, 10*time.Second)

	if o.hookProvider != nil {
		o.hookProvider = provider
		klog.Infof("set hook provider to [%s].", provider.GetProviderName())
	}
	return nil
}

// run starts the tunnel-server
func (o *TunnelServerOptions) run(stopCh <-chan struct{}) error {

	// 1. excute pre start tunnel server hook
	if o.hookProvider != nil {
		err := o.hookProvider.PreStartTunnelServer(o.clientSet)
		if err != nil {
			return fmt.Errorf("faild to excute %s provider pre-start tunnel server hook due to %v",
				o.hookProvider.GetProviderName(), err)
		}
	}

	// 2. create a certificate manager for the tunnel server and
	// run the csr approver for both tunnel-server and tunnel-agent
	serverCertMgr, err :=
		certmanager.NewTunnelServerCertManager(
			o.clientSet, o.certDNSNames, o.certIPs, stopCh)
	if err != nil {
		return err
	}
	serverCertMgr.Start()
	go certmanager.NewCSRApprover(o.clientSet, o.sharedInformerFactory.Certificates().V1beta1().CertificateSigningRequests()).
		Run(constants.TunnelCSRApproverThreadiness, stopCh)

	// 3. generate the TLS configuration based on the latest certificate
	rootCertPool, err := pki.GenRootCertPool(o.kubeConfig,
		constants.TunnelCAFile)
	if err != nil {
		return fmt.Errorf("fail to generate the rootCertPool: %s", err)
	}
	tlsCfg, err :=
		pki.GenTLSConfigUseCertMgrAndCertPool(serverCertMgr, rootCertPool)
	if err != nil {
		return err
	}

	// 4. after all of informers are configured completed, start the shared index informer
	o.sharedInformerFactory.Start(stopCh)

	// 5. waiting for the certificate is generated
	_ = wait.PollUntil(5*time.Second, func() (bool, error) {
		// keep polling until the certificate is signed
		if serverCertMgr.Current() != nil {
			return true, nil
		}
		klog.Infof("waiting for the master to sign the %s certificate",
			version.GetServerName())
		return false, nil
	}, stopCh)

	// 6. start reverse proxy
	rps := NewReverseProxyServer(
		o.bindAddr,
		constants.TunnelServerReversePorxyPort,
		tlsCfg,
	)
	if err := rps.Run(); err != nil {
		return err
	}

	// 7. start the tunnel server
	ts := NewTunnelServer(
		o.serverMasterAddr,
		o.serverMasterInsecureAddr,
		o.serverAgentAddr,
		o.serverCount,
		tlsCfg,
		o.proxyStrategy,
		o.udsName)
	if err := ts.Run(); err != nil {
		return err
	}

	// 8. excute pre start tunnel server hook
	if o.hookProvider != nil {
		err := o.hookProvider.PostStartTunnelServer(o.clientSet)
		if err != nil {
			return fmt.Errorf("faild to excute %s provider post-start tunnel server hook due to %v",
				o.hookProvider.GetProviderName(), err)
		}
	}
	<-stopCh
	return nil
}
