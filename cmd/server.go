// Copyright © 2017 NAME HERE <EMAIL ADDRESS>
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
	"net"

	"github.com/juju/errors"
	"github.com/service-exposer/uot/protocal"
	"github.com/spf13/cobra"
)

// serverCmd represents the server command
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Listen tcp address,forward data to target udp address",
}

func init() {
	RootCmd.AddCommand(serverCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// serverCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// serverCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	var (
		listen_addr = "localhost:"
		target_addr = ""
	)
	serverCmd.Flags().StringVarP(&listen_addr, "listen", "l", listen_addr, "listen TCP address")
	serverCmd.Flags().StringVarP(&target_addr, "target", "t", target_addr, "target UDP address")

	serverCmd.Run = func(cmd *cobra.Command, args []string) {
		ln, err := net.Listen("tcp", listen_addr)
		if err != nil {
			Exit(-1, errors.ErrorStack(errors.Trace(err)))
		}

		addr, err := net.ResolveUDPAddr("udp", target_addr)
		if err != nil {
			Exit(-2, errors.ErrorStack(errors.Trace(err)))
		}

		for {
			err = protocal.ServerSide(ln.Accept, addr)
			log.Print(errors.ErrorStack(errors.Trace(err)))
		}

	}
}
