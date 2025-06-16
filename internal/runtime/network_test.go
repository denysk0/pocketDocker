package runtime

import (
	"reflect"
	"strconv"
	"testing"
)

type fakeNetRunner struct{ cmds [][]string }

func (f *fakeNetRunner) Run(cmd string, args ...string) error {
	c := append([]string{cmd}, args...)
	f.cmds = append(f.cmds, c)
	return nil
}

type mockIptablesChecker struct{}

func (mockIptablesChecker) CheckRule(args ...string) bool {
	return false
}

func TestSetupNetworkingCommands(t *testing.T) {
	f := &fakeNetRunner{}
	ports := []PortMap{{Host: 8080, Container: 80}}
	_, _, err := SetupNetworkingWithChecker(123, "abcdef0123456789", ports, f, mockIptablesChecker{})
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"ip", "link", "add", "vethabcdef01", "type", "veth", "peer", "name", "vethabcdef01_c"},
		{"ip", "addr", "add", "10.42.0.1/24", "dev", "vethabcdef01"},
		{"ip", "link", "set", "vethabcdef01", "up"},
		{"ip", "link", "set", "vethabcdef01_c", "netns", "123"},
		{"nsenter", "--target", "123", "--net", "ip", "link", "set", "lo", "up"},
		{"nsenter", "--target", "123", "--net", "ip", "link", "set", "vethabcdef01_c", "up"},
		{"nsenter", "--target", "123", "--net", "ip", "addr", "add", "10.42.0." +
			strconv.Itoa(ipSuffixFromID("abcdef0123456789")) + "/24", "dev", "vethabcdef01_c"},
		{"nsenter", "--target", "123", "--net", "ip", "route", "add", "default", "via", "10.42.0.1"},
		{"iptables", "-A", "FORWARD", "-o", "vethabcdef01", "-j", "ACCEPT"},
		{"iptables", "-A", "FORWARD", "-i", "vethabcdef01", "-j", "ACCEPT"},
		{"iptables", "-t", "nat", "-A", "PREROUTING", "-p", "tcp", "-m", "tcp", "--dport", "8080", "-j", "DNAT", "--to-destination", "10.42.0." + strconv.Itoa(ipSuffixFromID("abcdef0123456789")) + ":80"},
		{"iptables", "-t", "nat", "-A", "OUTPUT", "-p", "tcp", "-m", "tcp", "--dport", "8080", "-j", "DNAT", "--to-destination", "10.42.0." + strconv.Itoa(ipSuffixFromID("abcdef0123456789")) + ":80"},
		{"iptables", "-t", "nat", "-A", "POSTROUTING", "-s", "10.42.0." + strconv.Itoa(ipSuffixFromID("abcdef0123456789")) + "/32", "-j", "MASQUERADE"},
	}
	if !reflect.DeepEqual(f.cmds, want) {
		t.Fatalf("commands mismatch\nwant=%v\n got=%v", want, f.cmds)
	}
}
