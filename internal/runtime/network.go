//go:build linux

package runtime

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"bytes"
	"strings"
)

type CmdRunner interface {
	Run(cmd string, args ...string) error
}

type realRunner struct{}

func (realRunner) Run(cmd string, args ...string) error {
	c := exec.Command(cmd, args...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

func defaultRunner() CmdRunner {
	return realRunner{}
}

// IptablesChecker interface allows mocking iptables rule checking
type IptablesChecker interface {
	CheckRule(args ...string) bool
}

type realIptablesChecker struct{}

func (realIptablesChecker) CheckRule(args ...string) bool {
	return checkIptablesRuleReal(args...)
}

func checkIptablesRuleReal(args ...string) bool {
	checkArgs := make([]string, len(args))
	copy(checkArgs, args)
	for i, arg := range checkArgs {
		if arg == "-A" {
			checkArgs[i] = "-C"
			break
		}
	}
	
	cmd := exec.Command("iptables", checkArgs...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	return err == nil
}

func ipSuffixFromID(id string) int {
	return ipSuffixFromIDWithCollisionCheck(id, nil)
}

func ipSuffixFromIDWithCollisionCheck(id string, checker func(int) bool) int {
	if len(id) < 6 {
		id = id + "000000"
	}
	v, err := strconv.ParseUint(id[:6], 16, 32)
	if err != nil {
		return 2
	}
	suffix := int(v%250) + 2
	
	if checker == nil {
		return suffix
	}
	
	for i := 0; i < 250; i++ {
		candidate := suffix + i
		if candidate > 254 {
			candidate = (candidate % 253) + 2
		}
		if !checker(candidate) {
			return candidate
		}
	}
	
	return 2
}

// checkIPInUse checks if a given IP suffix is already assigned to a veth interface
func checkIPInUse(suffix int) bool {
	targetIP := fmt.Sprintf("10.42.0.%d", suffix)
	cmd := exec.Command("ip", "addr", "show")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(output), targetIP)
}

// PortMap represents a published port mapping
type PortMap struct {
	Host      int
	Container int
}

// SetupNetworking configures veth pair and iptables rules for the container
// Returns the original ip_forward value and actual IP suffix used
func SetupNetworking(pid int, id string, ports []PortMap, r CmdRunner) (string, int, error) {
	return SetupNetworkingWithChecker(pid, id, ports, r, realIptablesChecker{})
}

// SetupNetworkingWithChecker configures veth pair and iptables rules for the container with custom checker
// Returns the original ip_forward value and actual IP suffix used
func SetupNetworkingWithChecker(pid int, id string, ports []PortMap, r CmdRunner, checker IptablesChecker) (string, int, error) {
	origBytes, _ := os.ReadFile("/proc/sys/net/ipv4/ip_forward")
	origValue := strings.TrimSpace(string(origBytes))
	
	success := false
	ipSuffix := ipSuffixFromIDWithCollisionCheck(id, checkIPInUse)
	defer func() {
		if !success {
			_ = CleanupNetworkingWithIPSuffix(id, ipSuffix, ports, origValue)
		}
	}()
	if r == nil {
		r = defaultRunner()
	}
	short := id
	if len(short) > 8 {
		short = id[:8]
	}
	hostVeth := "veth" + short
	if len(hostVeth) > 13 {
		hostVeth = hostVeth[:13]
	}
	contVeth := hostVeth + "_c"
	
	if err := r.Run("ip", "link", "add", hostVeth, "type", "veth", "peer", "name", contVeth); err != nil {
		return "", 0, err
	}
	_ = r.Run("ip", "addr", "add", "10.42.0.1/24", "dev", hostVeth)
	if err := r.Run("ip", "link", "set", hostVeth, "up"); err != nil {
		return "", 0, err
	}
	if err := r.Run("ip", "link", "set", contVeth, "netns", strconv.Itoa(pid)); err != nil {
		return "", 0, err
	}
	if err := r.Run("nsenter", "--target", strconv.Itoa(pid), "--net", "ip", "link", "set", "lo", "up"); err != nil {
		return "", 0, err
	}
	if err := r.Run("nsenter", "--target", strconv.Itoa(pid), "--net", "ip", "link", "set", contVeth, "up"); err != nil {
		return "", 0, err
	}
	addr := fmt.Sprintf("10.42.0.%d/24", ipSuffix)
	if err := r.Run("nsenter", "--target", strconv.Itoa(pid), "--net", "ip", "addr", "add", addr, "dev", contVeth); err != nil {
		return "", 0, err
	}
	if err := r.Run("nsenter", "--target", strconv.Itoa(pid), "--net", "ip", "route", "add", "default", "via", "10.42.0.1"); err != nil {
		return "", 0, err
	}
	
	if os.Geteuid() == 0 {
		if err := os.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1"), 0644); err != nil {
			return "", 0, fmt.Errorf("enable ip_forward: %w", err)
		}
	}
	
	forwardOutArgs := []string{"-A", "FORWARD", "-o", hostVeth, "-j", "ACCEPT"}
	if !checker.CheckRule(forwardOutArgs...) {
		if err := r.Run("iptables", forwardOutArgs...); err != nil {
			return "", 0, err
		}
	}
	
	forwardInArgs := []string{"-A", "FORWARD", "-i", hostVeth, "-j", "ACCEPT"}
	if !checker.CheckRule(forwardInArgs...) {
		if err := r.Run("iptables", forwardInArgs...); err != nil {
			return "", 0, err
		}
	}

	for _, pm := range ports {
		h := strconv.Itoa(pm.Host)
		c := strconv.Itoa(pm.Container)
		dest := fmt.Sprintf("10.42.0.%d:%s", ipSuffix, c)
		
		preroutingArgs := []string{"-t", "nat", "-A", "PREROUTING",
			"-p", "tcp", "-m", "tcp", "--dport", h,
			"-j", "DNAT", "--to-destination", dest}
		if !checker.CheckRule(preroutingArgs...) {
			if err := r.Run("iptables", preroutingArgs...); err != nil {
				return "", 0, err
			}
		}
		
		outputArgs := []string{"-t", "nat", "-A", "OUTPUT",
			"-p", "tcp", "-m", "tcp", "--dport", h,
			"-j", "DNAT", "--to-destination", dest}
		if !checker.CheckRule(outputArgs...) {
			if err := r.Run("iptables", outputArgs...); err != nil {
				return "", 0, err
			}
		}
		
		postroutingArgs := []string{"-t", "nat", "-A", "POSTROUTING", 
			"-s", fmt.Sprintf("10.42.0.%d/32", ipSuffix), "-j", "MASQUERADE"}
		if !checker.CheckRule(postroutingArgs...) {
			if err := r.Run("iptables", postroutingArgs...); err != nil {
				return "", 0, err
			}
		}
	}
	success = true
	return origValue, ipSuffix, nil
}

// CleanupNetworking removes veth interface and iptables rules for the container.
func CleanupNetworking(id string, ports []PortMap) error {
	ipSuffix := ipSuffixFromID(id)
	return CleanupNetworkingWithIPSuffix(id, ipSuffix, ports, "")
}

// CleanupNetworkingWithRestore removes networking resources and restores ip_forward
func CleanupNetworkingWithRestore(id string, ports []PortMap, ipForwardOrig string) error {
	ipSuffix := ipSuffixFromID(id)
	return CleanupNetworkingWithIPSuffix(id, ipSuffix, ports, ipForwardOrig)
}

// CleanupNetworkingWithIPSuffix removes networking resources using specific IP suffix
func CleanupNetworkingWithIPSuffix(id string, ipSuffix int, ports []PortMap, ipForwardOrig string) error {
	short := id
	if len(short) > 8 {
		short = id[:8]
	}
	hostVeth := "veth" + short

	_, _ = exec.Command("ip", "link", "del", hostVeth).CombinedOutput()

	_, _ = exec.Command("iptables", "-D", "FORWARD", "-o", hostVeth, "-j", "ACCEPT").CombinedOutput()
	_, _ = exec.Command("iptables", "-D", "FORWARD", "-i", hostVeth, "-j", "ACCEPT").CombinedOutput()

	for _, pm := range ports {
		h := strconv.Itoa(pm.Host)
		dest := fmt.Sprintf("10.42.0.%d:%d", ipSuffix, pm.Container)
		_, _ = exec.Command("iptables", "-t", "nat", "-D", "PREROUTING",
			"-p", "tcp", "-m", "tcp", "--dport", h,
			"-j", "DNAT", "--to-destination", dest).CombinedOutput()
		_, _ = exec.Command("iptables", "-t", "nat", "-D", "OUTPUT",
			"-p", "tcp", "-m", "tcp", "--dport", h,
			"-j", "DNAT", "--to-destination", dest).CombinedOutput()
		_, _ = exec.Command("iptables", "-t", "nat", "-D", "POSTROUTING",
			"-s", fmt.Sprintf("10.42.0.%d/32", ipSuffix),
			"-j", "MASQUERADE").CombinedOutput()
	}
	
	if ipForwardOrig != "" {
		_ = os.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte(ipForwardOrig), 0644)
	}
	
	return nil
}
