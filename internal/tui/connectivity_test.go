package tui

import (
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func fixtureText(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("../system/testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func connModel(t *testing.T, opts Options) *Model {
	t.Helper()
	opts.Source = fakeSource{}
	opts.Probe = probe{
		"nmcli -t -f DEVICE,TYPE,STATE,CONNECTION device":             fixtureText(t, "nmcli-device.txt"),
		"nmcli -t -f ACTIVE,SSID,SIGNAL device wifi list --rescan no": fixtureText(t, "nmcli-wifi.txt"),
		"bluetoothctl show":                     fixtureText(t, "bt-show.txt"),
		"bluetoothctl devices Connected":        "Device 00:1B:66:AA:BB:CC WH-1000XM4\n",
		"wpctl status":                          fixtureText(t, "wpctl-status.txt"),
		"wpctl get-volume @DEFAULT_AUDIO_SINK@": "Volume: 0.40\n",
	}.run
	m := New(opts)
	load(t, m)
	return m
}

func TestConnectivityShown(t *testing.T) {
	m := connModel(t, Options{})
	if out := ansi.Strip(m.View().Content); !strings.Contains(out, "HomeNet") {
		t.Errorf("Overview should show the network:\n%s", out)
	}

	goTo(t, m, "Connectivity")
	out := ansi.Strip(m.View().Content)
	for _, want := range []string{"HomeNet", "61%", "wg_client", "WH-1000XM4", "PCM2902 Audio Codec Analog Stereo", "40%"} {
		if !strings.Contains(out, want) {
			t.Errorf("Connectivity missing %q:\n%s", want, out)
		}
	}
}

func TestConnectivityLaunchers(t *testing.T) {
	var ran, opened []string
	m := connModel(t, Options{
		Exec:   recordExec(&ran),
		Detach: func(argv []string) error { opened = append(opened, strings.Join(argv, " ")); return nil },
	})
	goTo(t, m, "Connectivity")
	press(m, "w", "b", "B", "v")
	if strings.Join(ran, ",") != "nmtui,bluetoothctl" {
		t.Errorf("terminal tools ran %v", ran)
	}
	if strings.Join(opened, ",") != "blueman-manager,pavucontrol" {
		t.Errorf("GUI apps opened %v", opened)
	}
}
