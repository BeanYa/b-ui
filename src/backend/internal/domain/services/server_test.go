package service

import (
	"testing"

	psnet "github.com/shirou/gopsutil/v4/net"
)

func TestIsVirtualNetworkInterfaceMatches3xUIStyleNames(t *testing.T) {
	virtualNames := []string{"lo", "lo0", "loopback0", "docker0", "br-lan", "veth123", "virbr0", "tun0", "tap0", "wg0", "tailscale0", "ztabc"}
	for _, name := range virtualNames {
		if !isVirtualNetworkInterface(name) {
			t.Fatalf("expected %q to be treated as virtual", name)
		}
	}

	physicalNames := []string{"eth0", "ens3", "enp1s0", "wlan0"}
	for _, name := range physicalNames {
		if isVirtualNetworkInterface(name) {
			t.Fatalf("expected %q to be treated as physical", name)
		}
	}
}

func TestAggregateNetworkCountersSkipsVirtualInterfaces(t *testing.T) {
	result := aggregateNetworkCounters([]psnet.IOCountersStat{
		{Name: "eth0", BytesSent: 100, BytesRecv: 200, PacketsSent: 3, PacketsRecv: 4},
		{Name: "wlan0", BytesSent: 50, BytesRecv: 75, PacketsSent: 5, PacketsRecv: 6},
		{Name: "lo", BytesSent: 1000, BytesRecv: 1000, PacketsSent: 100, PacketsRecv: 100},
		{Name: "docker0", BytesSent: 2000, BytesRecv: 2000, PacketsSent: 200, PacketsRecv: 200},
	})

	if result["sent"] != uint64(150) {
		t.Fatalf("expected sent=150, got %#v", result["sent"])
	}
	if result["recv"] != uint64(275) {
		t.Fatalf("expected recv=275, got %#v", result["recv"])
	}
	if result["psent"] != uint64(8) {
		t.Fatalf("expected psent=8, got %#v", result["psent"])
	}
	if result["precv"] != uint64(10) {
		t.Fatalf("expected precv=10, got %#v", result["precv"])
	}
}
