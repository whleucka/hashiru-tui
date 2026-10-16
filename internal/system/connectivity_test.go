package system

import (
	"context"
	"testing"
)

func TestParseDevices(t *testing.T) {
	devs := ParseDevices(fixture(t, "nmcli-device.txt"))
	// wifi, the wireguard tunnel, and the unplugged ethernet port. Not docker's
	// bridges, loopback, p2p, or the unmanaged veths.
	want := []Device{
		{Name: "wlp194s0", Type: "wifi", State: "connected", Connection: "HomeNet"},
		{Name: "wg_client", Type: "wireguard", State: "connected (externally)", Connection: "wg_client"},
		{Name: "eth0", Type: "ethernet", State: "unavailable"},
	}
	if len(devs) != len(want) {
		t.Fatalf("devices = %+v", devs)
	}
	for i := range want {
		if devs[i] != want[i] {
			t.Errorf("device %d = %+v, want %+v", i, devs[i], want[i])
		}
	}
}

func TestParseWifi(t *testing.T) {
	ssid, signal, ok := ParseWifi(fixture(t, "nmcli-wifi.txt"))
	if !ok || ssid != "HomeNet" || signal != 61 {
		t.Errorf("wifi = %q %d %v", ssid, signal, ok)
	}
	// An SSID can contain colons; nmcli escapes them as \:.
	ssid, _, _ = ParseWifi([]byte(`yes:Cafe\:Guest:40` + "\n"))
	if ssid != "Cafe:Guest" {
		t.Errorf("escaped ssid = %q", ssid)
	}
}

func TestParseBluetooth(t *testing.T) {
	bt := ParseBluetooth(fixture(t, "bt-show.txt"), []byte("Device 00:1B:66:AA:BB:CC WH-1000XM4\nDevice 11:22:33:44:55:66 MX Master 3\n"))
	if !bt.Powered {
		t.Error("Powered = false")
	}
	if len(bt.Connected) != 2 || bt.Connected[0] != "WH-1000XM4" || bt.Connected[1] != "MX Master 3" {
		t.Errorf("Connected = %q", bt.Connected)
	}
}

func TestParseAudio(t *testing.T) {
	a := ParseAudio(fixture(t, "wpctl-status.txt"), fixture(t, "wpctl-volume.txt"))
	if a.Sink != "PCM2902 Audio Codec Analog Stereo" || a.Source != "PCM2902 Audio Codec Analog Stereo" {
		t.Errorf("sink %q source %q", a.Sink, a.Source)
	}
	if a.Volume != 40 || a.Muted {
		t.Errorf("volume %d muted %v", a.Volume, a.Muted)
	}
	if v := ParseAudio(nil, []byte("Volume: 0.75 [MUTED]\n")); v.Volume != 75 || !v.Muted {
		t.Errorf("muted volume = %+v", v)
	}
}

func TestConnectivityMissingTools(t *testing.T) {
	c := ReadConnectivity(context.Background(), fakeRun{}.Run)
	if !c.NetworkMissing || !c.BluetoothMissing || !c.AudioMissing {
		t.Errorf("missing tools should be flagged: %+v", c)
	}
}
