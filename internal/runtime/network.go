package runtime

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	// Ensure fmt and strconv are imported for CleanupNetworking
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

func ipSuffixFromID(id string) int {
	if len(id) < 2 {
		return 2
	}
	b, err := strconv.ParseUint(id[:2], 16, 8)
	if err != nil {
		return 2
	}
	return int(b%250) + 2
}

// PortMap represents a published port mapping
type PortMap struct {
	Host      int
	Container int
}

// SetupNetworking configures veth pair and iptables rules for the container
func SetupNetworking(pid int, id string, ports []PortMap, r CmdRunner) error {
	if r == nil {
		r = defaultRunner()
	}
	short := id
	if len(short) > 8 {
		short = id[:8]
	}
	hostVeth := "veth" + short
	contVeth := hostVeth + "_c"
	ipSuffix := ipSuffixFromID(id)

	if err := r.Run("ip", "link", "add", hostVeth, "type", "veth", "peer", "name", contVeth); err != nil {
		return err
	}
	_ = r.Run("ip", "addr", "add", "10.42.0.1/24", "dev", hostVeth)
	if err := r.Run("ip", "link", "set", hostVeth, "up"); err != nil {
		return err
	}
	if err := r.Run("ip", "link", "set", contVeth, "netns", strconv.Itoa(pid)); err != nil {
		return err
	}
	if err := r.Run("nsenter", "--target", strconv.Itoa(pid), "--net", "ip", "link", "set", "lo", "up"); err != nil {
		return err
	}
	if err := r.Run("nsenter", "--target", strconv.Itoa(pid), "--net", "ip", "link", "set", contVeth, "up"); err != nil {
		return err
	}
	addr := fmt.Sprintf("10.42.0.%d/24", ipSuffix)
	if err := r.Run("nsenter", "--target", strconv.Itoa(pid), "--net", "ip", "addr", "add", addr, "dev", contVeth); err != nil {
		return err
	}
	if err := r.Run("nsenter", "--target", strconv.Itoa(pid), "--net", "ip", "route", "add", "default", "via", "10.42.0.1"); err != nil {
		return err
	}
	// enable IP forwarding
	_ = os.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1"), 0644)
	// allow packets to be forwarded to / from this veth
	if err := r.Run("iptables", "-A", "FORWARD", "-o", hostVeth, "-j", "ACCEPT"); err != nil {
		return err
	}
	if err := r.Run("iptables", "-A", "FORWARD", "-i", hostVeth, "-j", "ACCEPT"); err != nil {
		return err
	}

	for _, pm := range ports {
		h := strconv.Itoa(pm.Host)
		c := strconv.Itoa(pm.Container)
		dest := fmt.Sprintf("10.42.0.%d:%s", ipSuffix, c)
		if err := r.Run("iptables", "-t", "nat", "-A", "PREROUTING",
			"-p", "tcp", "-m", "tcp", "--dport", h,
			"-j", "DNAT", "--to-destination", dest); err != nil {
			return err
		}
		// Add OUTPUT rule for local packets to published port
		if err := r.Run("iptables", "-t", "nat", "-A", "OUTPUT",
			"-p", "tcp", "-m", "tcp", "--dport", h,
			"-j", "DNAT", "--to-destination", dest); err != nil {
			return err
		}
		if err := r.Run("iptables", "-t", "nat", "-A", "POSTROUTING", "-s", fmt.Sprintf("10.42.0.%d/32", ipSuffix), "-j", "MASQUERADE"); err != nil {
			return err
		}
	}
	return nil
}

// CleanupNetworking removes veth interface and iptables rules for the container.
func CleanupNetworking(id string, ports []PortMap) error {
	short := id
	if len(short) > 8 {
		short = id[:8]
	}
	hostVeth := "veth" + short

	// delete host veth interface
	_, _ = exec.Command("ip", "link", "del", hostVeth).CombinedOutput()

	// remove FORWARD chain rules
	_, _ = exec.Command("iptables", "-D", "FORWARD", "-o", hostVeth, "-j", "ACCEPT").CombinedOutput()
	_, _ = exec.Command("iptables", "-D", "FORWARD", "-i", hostVeth, "-j", "ACCEPT").CombinedOutput()

	// remove NAT rules
	for _, pm := range ports {
		h := strconv.Itoa(pm.Host)
		ipSuffix := ipSuffixFromID(id)
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
	return nil
}
