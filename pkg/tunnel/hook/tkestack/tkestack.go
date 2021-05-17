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

package tkestack

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/tkestack/tke-excalibur/pkg/tunnel/constants"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	k8s "k8s.io/client-go/kubernetes"
)

// TKEStackProvider for execute
type TKEStackProvider struct {
}

// ClusterCredential records the credential information needed to access the cluster.
type ClusterCredential struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	ClusterName       string  `json:"clusterName"`
	Token             *string `json:"token,omitempty"`
}

// ClusterCredentialList is the whole list of all ClusterCredential which owned by a tenant.
type ClusterCredentialList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterCredential `json:"items"`
}

func (hook *TKEStackProvider) GetProviderName() string {
	return "tkestack"
}

func (hook *TKEStackProvider) GetTunnelServerNS(clusterName string) string {
	return clusterName
}

func (hook *TKEStackProvider) PreStartTunnelAgent(clusterName string, cloudclient k8s.Interface, localclient k8s.Interface) error {
	return nil
}

func (hook *TKEStackProvider) PostStartTunnelAgent(clusterName string, cloudclient k8s.Interface, localclient k8s.Interface) error {
	// steps:
	// 1. get cluster credential by cluster name
	// 2. patch token field to cluster credential
	ccl := ClusterCredentialList{}
	restclient := cloudclient.Discovery().RESTClient()
	data, err := restclient.
		Get().
		AbsPath("apis/platform.tkestack.io/v1/clustercredentials").
		Param("fieldSelector", "clusterName="+clusterName).
		SetHeader("Accept", "application/json").
		DoRaw()
	if err != nil {
		return fmt.Errorf("failed to get cluster %s credential: %v", clusterName, err)
	}
	err = json.Unmarshal(data, &ccl)
	if err != nil {
		return err
	}
	if len(ccl.Items) == 0 {
		return fmt.Errorf("cluster credential for cluster: %s is not ready", clusterName)
	}
	tokenByte, err := ioutil.ReadFile(constants.TunnelTokenFile)
	if err != nil {
		return fmt.Errorf("failed to read token from %s: %s", constants.TunnelTokenFile, err)
	}
	patchBody := fmt.Sprintf(`[{"op":"replace","path":"/token","value":"%s"}]`, string(tokenByte))
	err = restclient.Patch(types.JSONPatchType).
		AbsPath("apis/platform.tkestack.io/v1").
		Resource("clustercredentials").
		Name(ccl.Items[0].Name).
		Body([]byte(patchBody)).
		Do().
		Error()
	if err != nil {
		return fmt.Errorf("patch cluster credential for cluster %s faild: %s", clusterName, err)
	}
	return nil
}

func (hook *TKEStackProvider) PreStartTunnelServer(localclient k8s.Interface) error {
	return nil
}

func (hook *TKEStackProvider) PostStartTunnelServer(localclient k8s.Interface) error {
	return nil
}
