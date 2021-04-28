// Package shell generate shell script
package shell

import (
	"fmt"
	"os"
	"strings"
	"text/template"

	l "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"

	"github.com/tinynetwork/tinet/internal/pkg/utils"
)

var log = l.New()

// Tn tinet config
type Tn struct {
	PreCmd      []PreCmd     `yaml:"precmd"`
	PreInit     []PreInit    `yaml:"preinit"`
	PostInit    []PostInit   `yaml:"postinit"`
	PostFini    []PostFini   `yaml:"postfini"`
	Nodes       []Node       `yaml:"nodes" mapstructure:"nodes"`
	Switches    []Switch     `yaml:"switches" mapstructure:"switches"`
	NodeConfigs []NodeConfig `yaml:"node_configs" mapstructure:"node_configs"`
	Test        []Test       `yaml:"test"`
}

// PreCmd
type PreCmd struct {
	// Cmds []Cmd `yaml:"cmds"`
	Cmds []Cmd `yaml:"cmds" mapstructure:"cmds"`
}

// PreInit
type PreInit struct {
	Cmds []Cmd `yaml:"cmds" mapstructure:"cmds"`
}

// PostInit
type PostInit struct {
	Cmds []Cmd `yaml:"cmds" mapstructure:"cmds"`
}

// PostFini
type PostFini struct {
	Cmds []Cmd `yaml:"cmds" mapstructure:"cmds"`
}

// Node
type Node struct {
	Name           string                 `yaml:"name" mapstructure:"name"`
	Type           string                 `yaml:"type" mapstructure:"type"`
	NetBase        string                 `yaml:"net_base" mapstructure:"net_base"`
	VolumeBase     string                 `yaml:"volume" mapstructure:"volume"`
	Image          string                 `yaml:"image" mapstructure:"image"`
	BuildFile      string                 `yaml:"buildfile" mapstructure:"buildfile"`
	Interfaces     []Interface            `yaml:"interfaces" mapstructure:"interfaces"`
	Sysctls        []Sysctl               `yaml:"sysctls" mapstructure:"sysctls"`
	Mounts         []string               `yaml:"mounts,flow" mapstructure:"mounts,flow"`
	DNS            []string               `yaml:"dns,flow" mapstructure:"dns,flow"`
	DNSSearches    []string               `yaml:"dns_search,flow" mapstructure:"dns_search,flow"`
	HostNameIgnore bool                   `yaml:"hostname_ignore" mapstructure:"hostname_ignore"`
	EntryPoint     string                 `yaml:"entrypoint" mapstructure:"entrypoint"`
	ExtraArgs      string                 `yaml:"docker_run_extra_args" mapstructure:"docker_run_extra_args"`
	Vars           map[string]interface{} `yaml:"vars" mapstructure:"vars"`
	Templates      []Template             `yaml:"templates" mapstructure:"templates"`
}

// Interface
type Interface struct {
	Name string `yaml:"name"`
	Type string `yaml:"type"`
	Args string `yaml:"args"`
	Addr string `yaml:"addr"`
}

// Sysctl
type Sysctl struct {
	Sysctl string `yaml:"string"`
}

type Template struct {
	Src     string `yaml:"src"`
	Dst     string `yaml:"dst"`
	Content string `yaml:"content"`
}

// Switch
type Switch struct {
	Name       string      `yaml:"name"`
	Interfaces []Interface `yaml:"interfaces" mapstructure:"interfaces"`
}

// NodeConfig
type NodeConfig struct {
	Name string `yaml:"name"`
	Cmds []Cmd  `yaml:"cmds" mapstructure:"cmds"`
}

// Cmd
type Cmd struct {
	Cmd string `yaml:"cmd"`
}

// Test
type Test struct {
	Name string
	Cmds []Cmd `yaml:"cmds" mapstructure:"cmds"`
}

// BuildCmd
func (node *Node) BuildCmd() (buildCmd string) {

	if node.BuildFile != "" {
		buildCmd = fmt.Sprintf("docker build -t %s %s", node.Image, node.BuildFile)
	}

	return buildCmd
}

// ExecConf Execute NodeConfig command
func (nodeConfig *NodeConfig) ExecConf(nodeType string) (execConfCmds []string) {

	for _, nodeConfigCmd := range nodeConfig.Cmds {
		var execConfCmd string
		if nodeType == "docker" {
			execConfCmd = fmt.Sprintf("docker exec %s %s", nodeConfig.Name, nodeConfigCmd.Cmd)
		} else if nodeType == "netns" {
			execConfCmd = fmt.Sprintf("ip netns exec %s %s", nodeConfig.Name, nodeConfigCmd.Cmd)
		} else if nodeType == "" {
			execConfCmd = fmt.Sprintf("docker exec %s %s", nodeConfig.Name, nodeConfigCmd.Cmd)
		} else {
			// err := fmt.Errorf("not supported node type...")
			// log.Fatal(err)
			return []string{""}
		}
		// fmt.Println(execConfigCmd)
		execConfCmds = append(execConfCmds, execConfCmd)
	}

	return execConfCmds
}

// DeleteNode Delete docker and netns
func (node *Node) DeleteNode() (deleteNodeCmds []string) {

	var deleteCmd string

	if node.Type == "docker" || node.Type == "" {
		deleteCmd = fmt.Sprintf("docker rm -f %s || true", node.Name)
	} else if node.Type == "netns" {
		deleteCmd = fmt.Sprintf("ip netns del %s", node.Name)
	} else {
		return []string{""}
	}

	deleteNsCmd := fmt.Sprintf("rm -rf /var/run/netns/%s", node.Name)

	deleteNodeCmds = []string{deleteCmd, deleteNsCmd}

	return deleteNodeCmds
}

// DeleteSwitch Delete bridge
func (br *Switch) DeleteSwitch() (deleteBrCmd string) {

	deleteBrCmd = fmt.Sprintf("ovs-vsctl del-br %s", br.Name)

	return deleteBrCmd
}

// Exec Select Node exec command
func (tnconfig *Tn) Exec(nodeName string, Cmds []string) (execCommand string) {

	var selectedNode *Node

	for i, node := range tnconfig.Nodes {
		if node.Name == nodeName {
			selectedNode = &tnconfig.Nodes[i]
		}
	}

	if selectedNode == nil {
		err := fmt.Errorf("no such node...\n")
		log.Error(err)
	} else {
		if selectedNode.Type == "docker" {
			execCommand = fmt.Sprintf("docker exec %s", nodeName)
		} else if selectedNode.Type == "netns" {
			execCommand = fmt.Sprintf("ip netns exec %s", nodeName)
		} else if selectedNode.Type == "" {
			execCommand = fmt.Sprintf("docker exec %s", nodeName)
		} else {
			err := fmt.Errorf("no such node type...\n")
			log.Error(err)
		}

		var cmdStr string
		for _, cmd := range Cmds {
			cmdStr += fmt.Sprintf(" %s", cmd)
			execCommand += cmdStr
		}
	}

	return execCommand
}

// GenerateFile Generate tinet template config file
func GenerateFile() (genContent string, err error) {

	precmd := PreCmd{
		Cmds: []Cmd{
			Cmd{
				Cmd: "",
			},
		},
	}

	preinit := PreInit{
		Cmds: []Cmd{
			Cmd{
				Cmd: "",
			},
		},
	}
	postinit := PostInit{
		Cmds: []Cmd{
			Cmd{
				Cmd: "echo hoge",
			},
		},
	}

	postfini := PostFini{
		Cmds: []Cmd{
			Cmd{
				Cmd: "",
			},
		},
	}

	nodes := Node{
		Name:    "R1",
		Image:   "slankdev/ubuntu18.04",
		NetBase: "",
		Interfaces: []Interface{
			Interface{
				Name: "net0",
				Type: "direct",
				Args: "C1#net0",
			},
			Interface{
				Name: "net1",
				Type: "bridge",
				Args: "B0",
			},
			Interface{
				Name: "net2",
				Type: "veth",
				Args: "peer0",
			},
			Interface{
				Name: "net3",
				Type: "phys",
			},
		},
	}

	switches := Switch{
		Name: "B0",
		Interfaces: []Interface{
			Interface{
				Name: "net0",
				Type: "docker",
				Args: "R1",
			},
			Interface{
				Name: "net0",
				Type: "netns",
				Args: "R2",
			},
		},
	}

	nodeConfigs := []NodeConfig{
		NodeConfig{
			Name: "C0",
			Cmds: []Cmd{
				Cmd{
					Cmd: "ip link set dev net0 up",
				},
			},
		},
		NodeConfig{
			Name: "C1",
			Cmds: []Cmd{
				Cmd{
					Cmd: "echo slankdev slankdev",
				},
				Cmd{
					Cmd: "echo slankdev &&\necho slankdev",
				},
			},
		},
	}

	tests := []Test{
		Test{
			Name: "p2p",
			Cmds: []Cmd{
				Cmd{
					Cmd: "docker exec C0 ping -c2 10.0.0.2",
				},
				Cmd{
					Cmd: "echo hoge",
				},
			},
		},
		Test{
			Name: "lo",
			Cmds: []Cmd{
				Cmd{
					Cmd: "docker exec C0 ping -c2 10.255.0.1",
				},
				Cmd{
					Cmd: "echo hoge",
				},
			},
		},
	}

	tnconfig := &Tn{
		PreCmd:      []PreCmd{precmd},
		PreInit:     []PreInit{preinit},
		PostInit:    []PostInit{postinit},
		PostFini:    []PostFini{postfini},
		Nodes:       []Node{nodes},
		Switches:    []Switch{switches},
		NodeConfigs: nodeConfigs,
		Test:        tests,
	}

	data, err := yaml.Marshal(tnconfig)
	if err != nil {
		return "", err
	}

	genContent = string(data)

	return genContent, err
}

// DockerPs Show docker ps
func DockerPs(all bool) (dockerPsCmd string) {

	dockerPsCmd = "docker ps --format 'table {{.ID}}\\t{{.Names}}\\t{{.Status}}\\t{{.Networks}}'"

	if all {
		dockerPsCmd += " -a"
	}

	return dockerPsCmd
}

// NetnsPs Show ip netns list
func NetnsPs() (netnsListCmd string) {

	netnsListCmd = "ip netns list"

	return netnsListCmd
}

// Pull pull Docker Image
func Pull(nodes []Node) (pullCmds []string) {

	var images []string

	for _, node := range nodes {
		images = append(images, node.Image)
	}

	images = utils.RemoveDuplicatesString(images)

	for _, image := range images {
		pullCmd := fmt.Sprintf("docker pull %s", image)
		pullCmds = append(pullCmds, pullCmd)
	}

	return pullCmds
}

// TnTestCmdExec Execute test cmds
func (t *Test) TnTestCmdExec() (tnTestCmds []string) {

	for _, testCmd := range t.Cmds {
		tnTestCmds = append(tnTestCmds, testCmd.Cmd)
	}

	return tnTestCmds
}

// ExecCmd Execute cmds
func ExecCmd(cmds []Cmd) (execCmds []string) {

	for _, cmd := range cmds {
		execCmds = append(execCmds, cmd.Cmd)
	}

	return execCmds
}

// CreateNode Create nodes set in config
func (node *Node) CreateNode() (createNodeCmds []string) {

	var createNodeCmd string

	if node.NetBase == "" {
		node.NetBase = "none"
	}

	if node.Type == "docker" || node.Type == "" {
		createNodeCmd = fmt.Sprintf("docker run -td --net %s --name %s --rm --privileged ", node.NetBase, node.Name)

		if !node.HostNameIgnore {
			createNodeCmd += fmt.Sprintf("--hostname %s ", node.Name)
		}

		if len(node.Sysctls) != 0 {
			for _, sysctl := range node.Sysctls {
				createNodeCmd += fmt.Sprintf("--sysctl %s ", sysctl.Sysctl)
			}
		}

		if node.EntryPoint != "" {
			createNodeCmd += fmt.Sprintf("--entrypoint %s ", node.EntryPoint)
		}

		if node.VolumeBase == "" {
			createNodeCmd += "-v /tmp/tinet:/tinet "
		} else {
			createNodeCmd += fmt.Sprintf("-v %s:/tinet ", node.VolumeBase)
		}

		if len(node.Mounts) != 0 {
			for _, mount := range node.Mounts {
				createNodeCmd += fmt.Sprintf("-v %s ", mount)
			}
		}

		if len(node.DNS) != 0 {
			for _, dns := range node.DNS {
				createNodeCmd += fmt.Sprintf("--dns=%s ", dns)
			}
		}

		if len(node.DNSSearches) != 0 {
			for _, dns_search := range node.DNSSearches {
				createNodeCmd += fmt.Sprintf("--dns-search=%s ", dns_search)
			}
		}

		if node.ExtraArgs != "" {
			createNodeCmd += fmt.Sprintf("%s ", node.ExtraArgs)
		}

		createNodeCmd += node.Image
	} else if node.Type == "netns" {
		createNodeCmd = fmt.Sprintf("ip netns add %s", node.Name)
	} else {
		createNodeCmd = fmt.Sprintf("unknown nodetype %s", node.Type)
	}

	createNodeCmds = append(createNodeCmds, createNodeCmd)

	if node.Type == "netns" {
		if len(node.Sysctls) != 0 {
			for _, sysctl := range node.Sysctls {
				sysctlNsCmd := fmt.Sprintf("ip netns exec %s sysctl -w %s", node.Name, sysctl.Sysctl)
				createNodeCmds = append(createNodeCmds, sysctlNsCmd)
			}
		}
		infloUpCmd := fmt.Sprintf("ip netns exec %s ip link set lo up", node.Name)
		createNodeCmds = append(createNodeCmds, infloUpCmd)
	}

	return createNodeCmds
}

// HostLinkUp Link up link of host
func HostLinkUp(linkName string) (linkUpCmd string) {

	linkUpCmd = fmt.Sprintf("ip link set %s up", linkName)

	return linkUpCmd
}

// NetnsLinkUp Link up link of netns
func NetnsLinkUp(netnsName string, linkName string) (netnsLinkUpCmd string) {

	netnsLinkUpCmd = fmt.Sprintf("ip netns exec %s ip link set %s up", netnsName, linkName)

	return netnsLinkUpCmd
}

// CreateSwitch Create bridge set in config
func (bridge *Switch) CreateSwitch() (createSwitchCmds []string) {

	addSwitchCmd := fmt.Sprintf("ovs-vsctl add-br %s", bridge.Name)
	createSwitchCmds = append(createSwitchCmds, addSwitchCmd)

	bridgeUpCmd := HostLinkUp(bridge.Name)
	createSwitchCmds = append(createSwitchCmds, bridgeUpCmd)

	return createSwitchCmds
}

// N2nLink Connect links between nodes
func (inf *Interface) N2nLink(nodeName string) (n2nLinkCmds []string) {

	nodeinf := inf.Name
	peerinfo := strings.Split(inf.Args, "#")
	peerNode := peerinfo[0]
	peerinf := peerinfo[1]
	n2nlinkCmd := fmt.Sprintf("ip link add %s netns %s type veth peer name %s netns %s", nodeinf, nodeName, peerinf, peerNode)
	n2nLinkCmds = append(n2nLinkCmds, n2nlinkCmd)
	n2nLinkCmds = append(n2nLinkCmds, NetnsLinkUp(nodeName, nodeinf))
	n2nLinkCmds = append(n2nLinkCmds, NetnsLinkUp(peerNode, peerinf))

	if len(inf.Addr) != 0 {
		addrSetCmd := fmt.Sprintf("ip netns exec %s ip link set %s address %s", nodeName, inf.Name, inf.Addr)
		n2nLinkCmds = append(n2nLinkCmds, addrSetCmd)
	}

	return n2nLinkCmds
}

// S2nLink Connect links between nodes and switches
func (inf *Interface) S2nLink(nodeName string) (s2nLinkCmds []string) {

	nodeinf := inf.Name
	peerBr := inf.Args
	peerBrInf := fmt.Sprintf("%s-%s", peerBr, nodeName)
	s2nLinkCmd := fmt.Sprintf("ip link add %s netns %s type veth peer name %s", nodeinf, nodeName, peerBrInf)
	s2nLinkCmds = append(s2nLinkCmds, s2nLinkCmd)
	s2nLinkCmds = append(s2nLinkCmds, NetnsLinkUp(nodeName, nodeinf))
	s2nLinkCmds = append(s2nLinkCmds, HostLinkUp(peerBrInf))
	setBrLinkCmd := fmt.Sprintf("ovs-vsctl add-port %s %s", peerBr, peerBrInf)
	s2nLinkCmds = append(s2nLinkCmds, setBrLinkCmd)

	return s2nLinkCmds
}

// V2cLink Connect links between veth and container
func (inf *Interface) V2cLink(nodeName string) (v2cLinkCmds []string) {

	nodeinf := inf.Name
	peerName := inf.Args
	v2cLinkCmd := fmt.Sprintf("ip link add %s type veth peer name %s", nodeinf, peerName)
	v2cLinkCmds = append(v2cLinkCmds, v2cLinkCmd)

	v2cLinkCmds = append(v2cLinkCmds, inf.P2cLink(nodeName)...)
	v2cLinkCmds = append(v2cLinkCmds, HostLinkUp(peerName))

	return v2cLinkCmds
}

// P2cLink Connect links between phys-eth and container
func (inf *Interface) P2cLink(nodeName string) (p2cLinkCmds []string) {

	physInf := inf.Name
	setNsCmd := fmt.Sprintf("ip link set dev %s netns %s", physInf, nodeName)
	p2cLinkCmds = append(p2cLinkCmds, setNsCmd)
	execNsCmd := fmt.Sprintf("ip netns exec %s ip link set %s up", nodeName, physInf)
	p2cLinkCmds = append(p2cLinkCmds, execNsCmd)
	delNsCmd := fmt.Sprintf("ip netns del %s", nodeName)
	p2cLinkCmds = append(p2cLinkCmds, delNsCmd)

	return p2cLinkCmds
}

// Mount_docker_netns Mount docker netns to ip netns
func (node *Node) Mount_docker_netns() (mountDockerNetnsCmds []string) {

	netnsDir := "/var/run/netns"
	mkdirCmd := fmt.Sprintf("mkdir -p %s", netnsDir)
	mountDockerNetnsCmds = append(mountDockerNetnsCmds, mkdirCmd)
	dockerPid := GetContainerPid(node.Name)
	mountDockerNetnsCmds = append(mountDockerNetnsCmds, dockerPid)
	mountDockerNetnsCmd := fmt.Sprintf("ln -s /proc/$PID/ns/net /var/run/netns/%s", node.Name)
	mountDockerNetnsCmds = append(mountDockerNetnsCmds, mountDockerNetnsCmd)

	return mountDockerNetnsCmds
}

// GetContainerPid func is Output get Docker PID Command
func GetContainerPid(nodename string) (getpidCmd string) {

	getpidCmd = fmt.Sprintf("PID=`docker inspect %s --format '{{.State.Pid}}'`", nodename)

	return getpidCmd
}

// DelNsCmds
func (node *Node) DelNsCmd() (delNsCmd string) {

	delNsCmd = fmt.Sprintf("ip netns del %s", node.Name)

	return delNsCmd
}

// MountTmpl
func (node *Node) MountTmpl() (mountTmplCmd []string, err error) {
	for _, tmpl := range node.Templates {
		dest := strings.Split(tmpl.Dst, "/")
		destfile := node.Name + "_" + dest[len(dest)-1]
		f, err := os.Create(destfile)
		if err != nil {
			return nil, err
		}
		defer f.Close()

		if len(tmpl.Src) > 0 {
			tpl, err := template.ParseFiles(tmpl.Src)
			if err != nil {
				return nil, err
			}

			err = tpl.Execute(f, node.Vars)
			if err != nil {
				return nil, err
			}
		} else if len(tmpl.Content) > 0 {
			tpl, err := template.New("").Parse(tmpl.Content)
			if err != nil {
				return nil, err
			}
			err = tpl.Execute(f, node.Vars)
			if err != nil {
				return nil, err
			}
		}

		tmplCmd := fmt.Sprintf("docker cp %s %s:%s", destfile, node.Name, tmpl.Dst)
		mountTmplCmd = append(mountTmplCmd, tmplCmd)
		removeFileCmd := fmt.Sprintf("rm -rf %s", destfile)
		mountTmplCmd = append(mountTmplCmd, removeFileCmd)
	}

	return mountTmplCmd, nil
}
