// Package cmd tn cli tool
package cmd

import (
	"fmt"
	"log"
	"strings"

	"github.com/ak1ra24/tn/internal/pkg/shell"
	"github.com/spf13/cobra"
)

// reupCmd represents the reup command
var reupCmd = &cobra.Command{
	Use:   "reup",
	Short: "Stop, remove, create and start",
	Run: func(cmd *cobra.Command, args []string) {
		// stop, remove
		for _, node := range tnconfig.Nodes {
			deleteNode := node.DeleteNode()
			fmt.Println(strings.Join(deleteNode, "\n"))
		}
		for _, br := range tnconfig.Switches {
			delBrCmd := br.DeleteSwitch()
			fmt.Println(delBrCmd)
		}

		// create, start
		if len(tnconfig.PreCmd) != 0 {
			for _, preCmds := range tnconfig.PreCmd {
				preExecCmds := shell.ExecCmd(preCmds.Cmds)
				fmt.Println(strings.Join(preExecCmds, "\n"))
			}
		}
		if len(tnconfig.PreInit) != 0 {
			for _, preInitCmds := range tnconfig.PreInit {
				preExecInitCmds := shell.ExecCmd(preInitCmds.Cmds)
				fmt.Println(strings.Join(preExecInitCmds, "\n"))
			}
		}
		for _, node := range tnconfig.Nodes {
			createNodeCmds := node.CreateNode()
			fmt.Println(strings.Join(createNodeCmds, "\n"))

			if node.Type != "netns" {
				mountDockerNetnsCmds := node.Mount_docker_netns()
				fmt.Println(strings.Join(mountDockerNetnsCmds, "\n"))
			}
		}

		if len(tnconfig.Switches) != 0 {
			for _, bridge := range tnconfig.Switches {
				createSwitchCmds := bridge.CreateSwitch()
				fmt.Println(strings.Join(createSwitchCmds, "\n"))
			}
		}

		for _, node := range tnconfig.Nodes {
			for _, inf := range node.Interfaces {
				if inf.Type == "direct" {
					n2nLinkCmds := inf.N2nLink(node.Name)
					fmt.Println(strings.Join(n2nLinkCmds, "\n"))
				} else if inf.Type == "bridge" {
					s2nLinkCmd := inf.S2nLink(node.Name)
					fmt.Println(strings.Join(s2nLinkCmd, "\n"))
				} else if inf.Type == "veth" {
					v2cLinkCmds := inf.V2cLink(node.Name)
					fmt.Println(strings.Join(v2cLinkCmds, "\n"))
				} else if inf.Type == "phys" {
					p2cLinkCmds := inf.P2cLink(node.Name)
					fmt.Println(strings.Join(p2cLinkCmds, "\n"))
				} else {
					err := fmt.Errorf("not supported interface type: %s", inf.Type)
					log.Fatal(err)
				}
			}
		}

		if len(tnconfig.PostInit) != 0 {
			for _, postInitCmds := range tnconfig.PostInit {
				postExecInitCmds := shell.ExecCmd(postInitCmds.Cmds)
				fmt.Println(strings.Join(postExecInitCmds, "\n"))
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(reupCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// reupCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// reupCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
