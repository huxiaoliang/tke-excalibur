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

package main

import (
	"flag"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"

	"github.com/tkestack/tke-excalibur/pkg/tunnel/agent"
	"github.com/tkestack/tke-excalibur/pkg/tunnel/hook/tkestack"
	"github.com/tkestack/tke-excalibur/pkg/version"
)

func main() {
	klog.InitFlags(nil)
	defer klog.Flush()
	// set hook provider for testing
	provider := &tkestack.TKEStackProvider{}
	cmd := agent.NewTunnelAgentCommand(provider, wait.NeverStop)
	cmd.Flags().AddGoFlagSet(flag.CommandLine)
	if err := cmd.Execute(); err != nil {
		klog.Fatalf("%s failed: %s", version.GetAgentName(), err)
	}
}
