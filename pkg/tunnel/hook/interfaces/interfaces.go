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

package interfaces

import (
	k8s "k8s.io/client-go/kubernetes"
)

// TunnelHookProvider is responsible for excute customized hook
// logic during life cycle. For exampe, after tunnel agent get
// started, PostStartTunnelAgent hook will refesh admin token to
// cloud apiserver
// parameter explanation:
//   cloudClient: used to accesss cloud side apiserver
//   localClient: used to accesss local side apiserver
type TunnelHookProvider interface {

	// GetProviderName return provider name for debug ability
	GetProviderName() string

	// PreStartTunnelAgent excute customized logic before agent get started
	PreStartTunnelAgent(clusterName string, cloudClient k8s.Interface, localClient k8s.Interface) error

	// PostStartTunnelAgent excute customized logic after agent get started
	PostStartTunnelAgent(clusterName string, cloudClient k8s.Interface, localClient k8s.Interface) error

	// PreStartTunnelServer excute customized logic before server get started
	PreStartTunnelServer(localClient k8s.Interface) error

	// PostStartTunnelServer excute customized logic after server get started
	PostStartTunnelServer(localClient k8s.Interface) error
}
