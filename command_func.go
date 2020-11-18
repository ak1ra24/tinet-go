package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/emicklei/dot"
	"github.com/spf13/viper"
	"github.com/tinynetwork/tinet/internal/pkg/shell"
	"github.com/tinynetwork/tinet/internal/pkg/utils"
	"github.com/urfave/cli/v2"
)

type linkstatus struct {
	leftNodeName  string
	leftInfName   string
	leftIsSet     bool
	leftNodeType  string
	rightNodeName string
	rightInfName  string
	rightIsSet    bool
	rightNodeType string
}

func LoadCfg(c *cli.Context) (tnconfig shell.Tn, verbose bool, err error) {
	cfgFile := c.String("config")
	verbose = c.Bool("verbose")
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
		viper.SetConfigType("yaml")

		if err = viper.ReadInConfig(); err != nil {
			return tnconfig, verbose, err
		}

		if err = viper.Unmarshal(&tnconfig); err != nil {
			return tnconfig, verbose, err
		}
	} else {
		err = fmt.Errorf("not set config file.")
		return tnconfig, verbose, err
	}

	return tnconfig, verbose, nil

}

func CmdBuild(c *cli.Context) error {

	tnconfig, _, err := LoadCfg(c)
	if err != nil {
		return err
	}
	nodes := tnconfig.Nodes

	for _, node := range nodes {
		buildCmd := node.BuildCmd()
		fmt.Fprint(os.Stdout, buildCmd)
	}

	return nil
}

func CmdCheck(c *cli.Context) error {

	tnconfig, _, err := LoadCfg(c)
	if err != nil {
		return err
	}

	nodes := tnconfig.Nodes
	bridges := tnconfig.Switches
	confmap := map[string]string{}

	for _, node := range nodes {
		for _, inf := range node.Interfaces {
			if inf.Type == "direct" {
				host := node.Name + ":" + inf.Name
				peer := strings.Split(inf.Args, "#")
				target := peer[0] + ":" + peer[1]
				confmap[host] = target
			} else if inf.Type == "bridge" {
				host := node.Name + ":" + inf.Name
				target := inf.Args + ":" + node.Name
				confmap[host] = target
			}
		}
	}

	for _, bridge := range bridges {
		for _, inf := range bridge.Interfaces {
			host := bridge.Name + ":" + inf.Args
			target := inf.Args + ":" + inf.Name
			confmap[host] = target
		}
	}

	var matchNum int
	falseConfigMap := map[string]string{}

	for key, value := range confmap {
		if confmap[key] == value && confmap[value] == key {
			matchNum++
		} else {
			falseConfigMap[key] = value
		}
	}

	if len(confmap) == matchNum {
		return nil
	} else {
		var errMsg string
		for key, value := range falseConfigMap {
			errMsg += fmt.Sprintf("%s<->%s\n", key, value)
		}
		return fmt.Errorf(errMsg)
	}
}

func CmdUp(c *cli.Context) error {

	tnconfig, verbose, err := LoadCfg(c)
	if err != nil {
		return err
	}

	if len(tnconfig.PreCmd) != 0 {
		for _, preCmds := range tnconfig.PreCmd {
			preExecCmds := shell.ExecCmd(preCmds.Cmds)
			utils.PrintCmds(os.Stdout, preExecCmds, verbose)
		}
	}
	if len(tnconfig.PreInit) != 0 {
		for _, preInitCmds := range tnconfig.PreInit {
			preExecInitCmds := shell.ExecCmd(preInitCmds.Cmds)
			utils.PrintCmds(os.Stdout, preExecInitCmds, verbose)
		}
	}
	for _, node := range tnconfig.Nodes {
		createNodeCmds := node.CreateNode()
		utils.PrintCmds(os.Stdout, createNodeCmds, verbose)

		if node.Type != "netns" {
			mountDockerNetnsCmds := node.Mount_docker_netns()
			utils.PrintCmds(os.Stdout, mountDockerNetnsCmds, verbose)
		}
	}

	if len(tnconfig.Switches) != 0 {
		for _, bridge := range tnconfig.Switches {
			createSwitchCmds := bridge.CreateSwitch()
			utils.PrintCmds(os.Stdout, createSwitchCmds, verbose)
		}
	}

	var links []linkstatus

	for _, node := range tnconfig.Nodes {
		for _, inf := range node.Interfaces {
			if inf.Type == "direct" {
				rNodeArgs := strings.Split(inf.Args, "#")
				rNodeName := rNodeArgs[0]
				rInfName := rNodeArgs[1]
				peerFound := false
				for _, link := range links {
					if !link.rightIsSet {
						nodecheck := link.leftNodeName == rNodeName
						infcheck := link.leftInfName == rInfName
						if nodecheck && infcheck {
							link.rightNodeName = node.Name
							link.rightInfName = inf.Name
							link.rightNodeType = node.Type
							link.rightIsSet = true
							peerFound = true
						}
					}
				}
				if !peerFound {
					link := linkstatus{leftNodeName: node.Name, leftInfName: inf.Name, rightNodeName: rNodeName, rightInfName: rInfName}
					link.leftNodeType = node.Type
					link.leftIsSet = true
					links = append(links, link)
					n2nLinkCmds := inf.N2nLink(node.Name)
					utils.PrintCmds(os.Stdout, n2nLinkCmds, verbose)
				}
			} else if inf.Type == "bridge" {
				s2nLinkCmds := inf.S2nLink(node.Name)
				utils.PrintCmds(os.Stdout, s2nLinkCmds, verbose)
			} else if inf.Type == "veth" {
				v2cLinkCmds := inf.V2cLink(node.Name)
				utils.PrintCmds(os.Stdout, v2cLinkCmds, verbose)
			} else if inf.Type == "phys" {
				p2cLinkCmds := inf.P2cLink(node.Name)
				utils.PrintCmds(os.Stdout, p2cLinkCmds, verbose)
			} else {
				err := fmt.Errorf("not supported interface type: %s", inf.Type)
				log.Fatal(err)
			}
		}
	}

	// check
	err = CmdCheck(c)
	if err != nil {
		return err
	}

	for _, node := range tnconfig.Nodes {
		if node.Type == "docker" || node.Type == "" {
			delNsCmd := node.DelNsCmd()
			utils.PrintCmd(os.Stdout, delNsCmd, verbose)
			mountTmplCmd, err := node.MountTmpl()
			utils.PrintCmds(os.Stdout, mountTmplCmd, verbose)
			if err != nil {
				return err
			}
		}
	}

	if len(tnconfig.PostInit) != 0 {
		for _, postInitCmds := range tnconfig.PostInit {
			postExecInitCmds := shell.ExecCmd(postInitCmds.Cmds)
			utils.PrintCmds(os.Stdout, postExecInitCmds, verbose)
		}
	}

	return nil
}

func CmdConf(c *cli.Context) error {
	tnconfig, verbose, err := LoadCfg(c)
	if err != nil {
		return err
	}

	nodeinfo := map[string]string{}
	for _, node := range tnconfig.Nodes {
		nodeinfo[node.Name] = node.Type
	}

	for _, nodeConfig := range tnconfig.NodeConfigs {
		execConfCmds := nodeConfig.ExecConf(nodeinfo[nodeConfig.Name])
		for _, execConfCmd := range execConfCmds {
			utils.PrintCmd(os.Stdout, execConfCmd, verbose)
		}
	}

	return nil
}

func CmdUpConf(c *cli.Context) error {
	// create and start
	if err := CmdUp(c); err != nil {
		return err
	}

	// config
	if err := CmdConf(c); err != nil {
		return err
	}

	return nil
}

func CmdDown(c *cli.Context) error {

	tnconfig, verbose, err := LoadCfg(c)
	if err != nil {
		return err
	}

	for _, node := range tnconfig.Nodes {
		deleteNode := node.DeleteNode()
		utils.PrintCmds(os.Stdout, deleteNode, verbose)
	}
	for _, br := range tnconfig.Switches {
		delBrCmd := br.DeleteSwitch()
		utils.PrintCmd(os.Stdout, delBrCmd, verbose)
	}

	if len(tnconfig.PostFini) != 0 {
		for _, postFiniCmds := range tnconfig.PostFini {
			postExecFiniCmds := shell.ExecCmd(postFiniCmds.Cmds)
			utils.PrintCmds(os.Stdout, postExecFiniCmds, verbose)
		}
	}

	return nil
}

func CmdExec(c *cli.Context) error {

	tnconfig, verbose, err := LoadCfg(c)
	if err != nil {
		return err
	}

	execCmdArgs := c.Args().Slice()
	execCommand := tnconfig.Exec(execCmdArgs[0], execCmdArgs[1:])
	utils.PrintCmd(os.Stdout, execCommand, verbose)

	return nil

}

func CmdImg(c *cli.Context) error {

	tnconfig, _, err := LoadCfg(c)
	if err != nil {
		return err
	}

	g := dot.NewGraph(dot.Directed)
	for _, tnnode := range tnconfig.Nodes {
		nodeName := tnnode.Name
		fromNode := g.Node(nodeName)
		fromNode.Label(nodeName)
		ifaceInfos := tnnode.Interfaces
		for _, ifaceInfo := range ifaceInfos {
			var argsName string
			if ifaceInfo.Type == "direct" {
				argsName = strings.Split(ifaceInfo.Args, "#")[0]
				toNode := g.Node(argsName)
				findEdges := g.FindEdges(toNode, fromNode)
				if len(findEdges) == 0 {
					newEdge := g.Edge(fromNode, toNode)
					newEdge.Attr("arrowhead", "none")
					newEdge.Attr("labelfloat", "true")
					newEdge.Attr("headlabel", strings.Split(ifaceInfo.Args, "#")[1])
					newEdge.Attr("taillabel", ifaceInfo.Name)
					newEdge.Attr("fontsize", "8")
				}
			} else if ifaceInfo.Type == "bridge" {
				argsName = ifaceInfo.Args
				toNode := g.Node(argsName)
				findEdges := g.FindEdges(toNode, fromNode)
				if len(findEdges) == 0 {
					newEdge := g.Edge(fromNode, toNode)
					newEdge.Attr("arrowhead", "none")
					newEdge.Attr("labelfloat", "true")
					newEdge.Attr("headlabel", argsName)
					newEdge.Attr("taillabel", ifaceInfo.Name)
					newEdge.Attr("fontsize", "8")
				}
			}
		}
	}

	fmt.Fprintln(os.Stdout, g.String())

	return nil
}

func CmdInit(c *cli.Context) error {
	tnConf, err := shell.GenerateFile()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Fprintln(os.Stdout, tnConf)

	return nil

}

func CmdPs(c *cli.Context) error {

	all := c.Bool("all")
	fmt.Println("echo '---------------------------------------------------------------------------------'")
	fmt.Println("echo '                            Docker Status                                        '")
	fmt.Println("echo '---------------------------------------------------------------------------------'")
	dockerPsCmd := shell.DockerPs(all)
	fmt.Println(dockerPsCmd)
	fmt.Println("echo '---------------------------------------------------------------------------------'")
	fmt.Println("echo '                            IP NETNS LIST                                        '")
	fmt.Println("echo '---------------------------------------------------------------------------------'")
	netnsPsCmd := shell.NetnsPs()
	fmt.Println(netnsPsCmd)

	return nil
}

func CmdPull(c *cli.Context) error {
	tnconfig, verbose, err := LoadCfg(c)
	if err != nil {
		return err
	}
	pullCmds := shell.Pull(tnconfig.Nodes)

	utils.PrintCmds(os.Stdout, pullCmds, verbose)

	return nil
}

func CmdReConf(c *cli.Context) error {
	// stop, remove
	if err := CmdDown(c); err != nil {
		return err
	}

	// create and start
	if err := CmdUp(c); err != nil {
		return err
	}

	// config
	if err := CmdConf(c); err != nil {
		return err
	}

	return nil
}

func CmdReUp(c *cli.Context) error {
	// stop, remove
	if err := CmdDown(c); err != nil {
		return err
	}

	// create and start
	if err := CmdUp(c); err != nil {
		return err
	}

	return nil
}

func CmdTest(c *cli.Context) error {
	tnconfig, _, err := LoadCfg(c)
	if err != nil {
		return err
	}

	testName := c.Args().Get(0)

	var tnTestCmds []string

	if testName == "all" || testName == "" {
		for _, test := range tnconfig.Test {
			tnTestCmds = test.TnTestCmdExec()
		}
	} else {
		for _, test := range tnconfig.Test {
			if testName == test.Name {
				tnTestCmds = test.TnTestCmdExec()
			}
		}
	}

	if len(tnTestCmds) == 0 {
		return fmt.Errorf("not found test name\n")
	}

	fmt.Fprintln(os.Stdout, strings.Join(tnTestCmds, "\n"))

	return nil
}
