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

package k8s

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/transport"
	certutil "k8s.io/client-go/util/cert"
	"k8s.io/klog/v2"

	"github.com/tkestack/tke-excalibur/pkg/tunnel/constants"
)

// CreateClientSet creates a clientset based on:
// 1. given kubeConfig (kubeConfig is not empty)
// 2. in-cluster config (kubeConfig is empty)
func CreateClientSet(kubeConfig string) (*kubernetes.Clientset, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfig)
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(config)
}

// CreateClientSet creates a clientset based on the given kubeconfig,
// otherwise raise err for the reason
func CreateClientSetKubeConfig(kubeConfig string) (*kubernetes.Clientset, error) {
	var (
		cfg *rest.Config
		err error
	)
	if kubeConfig == "" {
		return nil, errors.New("kubeconfig is not set")
	}
	if _, err := os.Stat(kubeConfig); err != nil && os.IsNotExist(err) {
		return nil, err
	}
	cfg, err = clientcmd.BuildConfigFromFlags("", kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("fail to create the clientset based on %s: %v",
			kubeConfig, err)
	}
	cliSet, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return cliSet, nil
}

// CreateClientSetApiserverAddr creates a clientset based on the given apiserverAddr.
// The clientset uses the serviceaccount's CA and Token for authentication and
// authorization
func CreateClientSetApiserverAddr(apiserverAddr string) (*kubernetes.Clientset, error) {
	if apiserverAddr == "" {
		return nil, errors.New("apiserver addr can't be empty")
	}

	token, err := ioutil.ReadFile(constants.TunnelAgentTokenFile)
	if err != nil {
		return nil, err
	}

	tlsClientConfig := rest.TLSClientConfig{}

	if _, err := certutil.NewPool(constants.TunnelAgentCAFile); err != nil {
		klog.Errorf("Expected to load root CA config from %s, but got err: %v",
			constants.TunnelAgentCAFile, err)
	} else {
		tlsClientConfig.CAFile = constants.TunnelAgentCAFile

	}

	restConfig := rest.Config{
		Host:            "https://" + apiserverAddr,
		TLSClientConfig: tlsClientConfig,
		BearerToken:     string(token),
		BearerTokenFile: constants.TunnelAgentTokenFile,
	}

	transportConfig, err := restConfig.TransportConfig()
	if err != nil {
		return nil, err
	}
	restTLSConfig, err := transport.TLSConfigFor(transportConfig)
	if err != nil {
		return nil, err
	}

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:        100,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig:     restTLSConfig,
	}
	restConfig.Transport = transport
	// reset tls config to use transport tls config
	restConfig.TLSClientConfig = rest.TLSClientConfig{}
	return kubernetes.NewForConfig(&restConfig)
}
