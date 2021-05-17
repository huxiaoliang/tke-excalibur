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
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"time"

	"github.com/gorilla/mux"
	"k8s.io/klog/v2"
)

var (
	apiServerAddress string = "https://" +
		net.JoinHostPort(os.Getenv("KUBERNETES_SERVICE_HOST"), os.Getenv("KUBERNETES_SERVICE_PORT"))
	normServerAddress string = "http://169.254.0.40:80/norm/api"
)

type reverseProxyServer struct {
	mux     *mux.Router
	address string
	port    int
	tlsCfg  *tls.Config
}

type reverseProxyHandler struct {
	reverseProxy string
}

var _ ReverseProxyServer = &reverseProxyServer{}

func (o *reverseProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	remote, err := url.Parse(o.reverseProxy)
	if err != nil {
		panic(err)
	}
	reverseProxy := httputil.NewSingleHostReverseProxy(remote)
	transport := &http.Transport{
		Proxy:           http.ProxyFromEnvironment,
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:        100,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	}
	reverseProxy.Transport = transport
	reverseProxy.FlushInterval = 100 * time.Millisecond
	reverseProxy.ServeHTTP(w, r)
}

func (r *reverseProxyServer) registerHandler() {

	// for healthz check request
	r.mux.HandleFunc("/v1/healthz", r.healthz).Methods("GET")

	// for norm api request
	r.mux.PathPrefix("/norm/api").Handler(&reverseProxyHandler{reverseProxy: normServerAddress})

	// for apiserver request at last
	r.mux.PathPrefix("/").Handler(&reverseProxyHandler{reverseProxy: apiServerAddress})
}

func (o *reverseProxyServer) Run() error {
	o.registerHandler()

	go func() {
		server := http.Server{
			Addr:         fmt.Sprintf("%s:%d", o.address, o.port),
			Handler:      o.mux,
			TLSConfig:    o.tlsCfg,
			TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler)),
		}
		if err := server.ListenAndServeTLS("", ""); err != nil {
			klog.Errorf("failed to serve https request from master at %s:%d: %v", o.address, o.port, err)
		}
	}()
	klog.Infof("start handling https reverse proxy request from master at %s:%d", o.address, o.port)
	return nil
}

func (o *reverseProxyServer) healthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "OK")
}
