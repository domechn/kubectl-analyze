// Copyright © 2020 NAME HERE <EMAIL ADDRESS>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"log"

	"github.com/domgoer/kubectl-analyze/pkg/podusage"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// Options of command
type Options struct {
	configFlags *genericclioptions.ConfigFlags

	podName string

	namespace string

	nodeName string

	multiple float64
}

func NewCmd() *cobra.Command {
	o := NewPodUsageOptions()

	// podusageCmd represents the podusage command
	var podusageCmd = &cobra.Command{
		Use:   "podusage [NAME | -n namespace | -node node-name] [flags]",
		Short: "analyze the resources of pod ",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) > 0 {
				o.podName = args[0]
			}
			if err := o.Run(); err != nil {
				log.Fatal(err)
			}
		},
	}
	podusageCmd.Flags().StringVarP(&o.namespace, "namespace", "n", o.namespace, "namespace")
	podusageCmd.Flags().StringVarP(&o.nodeName, "node-name", "N", o.nodeName, "nodeName")
	podusageCmd.Flags().Float64VarP(&o.multiple, "multiple", "m", o.multiple, "multiple")

	flags := podusageCmd.PersistentFlags()
	o.configFlags.AddFlags(flags)

	return podusageCmd
}

func NewPodUsageOptions() *Options {
	return &Options{
		configFlags: genericclioptions.NewConfigFlags(true),
		multiple:    1.5,
	}
}

func (o *Options) Run() error {
	restConfig, err := o.configFlags.ToRESTConfig()
	if err != nil {
		return err
	}
	pu := podusage.MustNew(restConfig)
	data, err := pu.FindUsageNotMatchRequest(o.podName, o.namespace, o.nodeName, o.multiple)
	if err != nil {
		return err
	}
	return pu.Print(data)
}

func init() {
	rootCmd.AddCommand(NewCmd())
}
