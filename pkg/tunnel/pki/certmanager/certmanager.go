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

package certmanager

import (
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/tkestack/tke-excalibur/pkg/tunnel/constants"
	"github.com/tkestack/tke-excalibur/pkg/tunnel/server/serveraddr"
	"github.com/tkestack/tke-excalibur/pkg/version"

	certificates "k8s.io/api/certificates/v1beta1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	clicert "k8s.io/client-go/kubernetes/typed/certificates/v1beta1"
	"k8s.io/client-go/util/certificate"
	"k8s.io/klog/v2"
)

// NewTunnelServerCertManager creates a certificate manager for
// the tunnel-server
func NewTunnelServerCertManager(
	clientset kubernetes.Interface,
	clCertNames,
	clIPs string,
	stopCh <-chan struct{}) (certificate.Manager, error) {
	// get server DNS names and IPs
	var (
		dnsNames = []string{}
		ips      = []net.IP{}
		err      error
	)
	_ = wait.PollUntil(5*time.Second, func() (bool, error) {
		dnsNames, ips, err = serveraddr.GetExcaliburTunelServerDNSandIP(clientset)
		if err == nil {
			return true, nil
		}
		klog.Errorf("failed to get DNS names and ips: %s", err)
		return false, nil
	}, stopCh)
	// add user specified DNS anems and IP addresses
	if clCertNames != "" {
		dnsNames = append(dnsNames, strings.Split(clCertNames, ",")...)
	}
	if clIPs != "" {
		for _, ipstr := range strings.Split(clIPs, ",") {
			ips = append(ips, net.ParseIP(ipstr))
		}
	}
	return newCertManager(
		clientset,
		version.GetServerName(),
		fmt.Sprintf(constants.TunnelServerCertDir, version.GetServerName()),
		constants.TunnelServerCSRCN,
		[]string{constants.TunnelServerCSROrg, constants.TunnelCSROrg},
		dnsNames,
		ips)
}

// NewTunnelAgentCertManager creates a certificate manager for
// the tunnel-agent
func NewTunnelAgentCertManager(
	clientset kubernetes.Interface,
	clusterName string) (certificate.Manager, error) {
	podIP := os.Getenv(constants.TunnelAgentPodIPEnv)
	if podIP == "" {
		return nil, fmt.Errorf("env %s is not set",
			constants.TunnelAgentPodIPEnv)
	}
	return newCertManager(
		clientset,
		version.GetAgentName(),
		fmt.Sprintf(constants.TunnelAgentCertDir, version.GetAgentName()),
		clusterName,
		[]string{constants.TunnelCSROrg},
		[]string{clusterName},
		[]net.IP{net.ParseIP(podIP)})
}

// NewCertManager creates a certificate manager that will generates a
// certificate by sending a csr to the apiserver
func newCertManager(
	clientset kubernetes.Interface,
	componentName,
	certDir,
	commonName string,
	organizations,
	dnsNames []string,
	ipAddrs []net.IP) (certificate.Manager, error) {
	certificateStore, err :=
		certificate.NewFileStore(componentName, certDir, certDir, "", "")
	if err != nil {
		return nil, fmt.Errorf("failed to initialize the server certificate store: %v", err)
	}

	getTemplate := func() *x509.CertificateRequest {
		return &x509.CertificateRequest{
			Subject: pkix.Name{
				CommonName:   commonName,
				Organization: organizations,
			},
			DNSNames:    dnsNames,
			IPAddresses: ipAddrs,
		}
	}

	certManager, err := certificate.NewManager(&certificate.Config{
		ClientFn: func(current *tls.Certificate) (clicert.CertificateSigningRequestInterface, error) {
			return clientset.CertificatesV1beta1().CertificateSigningRequests(), nil
		},
		GetTemplate: getTemplate,
		Usages: []certificates.KeyUsage{
			certificates.UsageAny,
		},
		CertificateStore: certificateStore,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize server certificate manager: %v", err)
	}

	return certManager, nil
}
