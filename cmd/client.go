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

// clientCmd represents the client command
var clientCmd = &cobra.Command{
	Use:   "client",
	Short: "Listen udp address,forward data to server over tcp",
}

func init() {
	RootCmd.AddCommand(clientCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// clientCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// clientCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	var (
		listen_addr = "localhost:"
		remote_addr = ""
	)

	clientCmd.Flags().StringVarP(&listen_addr, "listen", "l", listen_addr, "local listen UDP address")
	clientCmd.Flags().StringVarP(&remote_addr, "remote", "r", remote_addr, "remote server TCP address")

	clientCmd.Run = func(cmd *cobra.Command, args []string) {
		pconn, err := net.ListenPacket("udp", listen_addr)
		if err != nil {
			Exit(-1, errors.ErrorStack(errors.Trace(err)))
		}

		nat := protocal.NewNAT()
		nat.Dial = func() (net.Conn, error) {
			return net.Dial("tcp", remote_addr)
		}

		for {
			err = protocal.ClientSide(nat, pconn)
			log.Print(errors.ErrorStack(errors.Trace(err)))
		}
	}
}
