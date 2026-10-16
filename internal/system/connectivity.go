package system

import (
	"bufio"
	"bytes"
	"context"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// Device is a network device NetworkManager manages.
type Device struct {
	Name, Type, State, Connection string
}

// Bluetooth is the adapter's state.
type Bluetooth struct {
	Powered   bool
	Connected []string // device names
}

// Audio is the default output and input.
type Audio struct {
	Sink, Source string
	Volume       int // percent, of the default sink
	Muted        bool
}

// Connectivity is network, bluetooth and audio at a glance. A missing tool is
// flagged rather than reported as an error.
type Connectivity struct {
	Devices        []Device
	WifiSSID       string
	WifiSignal     int // percent
	NetworkErr     error
	NetworkMissing bool

	Bluetooth        Bluetooth
	BluetoothErr     error
	BluetoothMissing bool

	Audio        Audio
	AudioErr     error
	AudioMissing bool
}

// ReadConnectivity asks nmcli, bluetoothctl and wpctl, in parallel.
func ReadConnectivity(ctx context.Context, run Run) Connectivity {
	var c Connectivity
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		out, err := run(ctx, "nmcli", "-t", "-f", "DEVICE,TYPE,STATE,CONNECTION", "device")
		if c.NetworkMissing = missing(err); err != nil {
			if !c.NetworkMissing {
				c.NetworkErr = err
			}
			return
		}
		c.Devices = ParseDevices(out)
		// --rescan no: report what NetworkManager knows rather than scanning.
		if wifi, err := run(ctx, "nmcli", "-t", "-f", "ACTIVE,SSID,SIGNAL", "device", "wifi", "list", "--rescan", "no"); err == nil {
			c.WifiSSID, c.WifiSignal, _ = ParseWifi(wifi)
		}
	}()
	go func() {
		defer wg.Done()
		show, err := run(ctx, "bluetoothctl", "show")
		if c.BluetoothMissing = missing(err); err != nil {
			if !c.BluetoothMissing {
				c.BluetoothErr = err
			}
			return
		}
		connected, _ := run(ctx, "bluetoothctl", "devices", "Connected")
		c.Bluetooth = ParseBluetooth(show, connected)
	}()
	go func() {
		defer wg.Done()
		status, err := run(ctx, "wpctl", "status")
		if c.AudioMissing = missing(err); err != nil {
			if !c.AudioMissing {
				c.AudioErr = err
			}
			return
		}
		vol, _ := run(ctx, "wpctl", "get-volume", "@DEFAULT_AUDIO_SINK@")
		c.Audio = ParseAudio(status, vol)
	}()
	wg.Wait()
	return c
}

// hiddenTypes are devices that aren't the user's connections: docker's
// bridges, loopback, wifi-p2p.
var hiddenTypes = map[string]bool{"bridge": true, "loopback": true, "wifi-p2p": true}

// ParseDevices reads `nmcli -t -f DEVICE,TYPE,STATE,CONNECTION device`,
// keeping the devices worth showing: not unmanaged, not infrastructure.
func ParseDevices(out []byte) []Device {
	var devs []Device
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		f := splitTerse(sc.Text())
		if len(f) < 4 || hiddenTypes[f[1]] || f[2] == "unmanaged" {
			continue
		}
		devs = append(devs, Device{Name: f[0], Type: f[1], State: f[2], Connection: f[3]})
	}
	return devs
}

// ParseWifi finds the active network in `nmcli -t -f ACTIVE,SSID,SIGNAL
// device wifi list`.
func ParseWifi(out []byte) (ssid string, signal int, ok bool) {
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		f := splitTerse(sc.Text())
		if len(f) == 3 && f[0] == "yes" {
			signal, _ = strconv.Atoi(f[2])
			return f[1], signal, true
		}
	}
	return "", 0, false
}

// splitTerse splits nmcli's terse output on colons, honouring \: escapes.
func splitTerse(line string) []string {
	var fields []string
	var cur strings.Builder
	for i := 0; i < len(line); i++ {
		switch {
		case line[i] == '\\' && i+1 < len(line):
			i++
			cur.WriteByte(line[i])
		case line[i] == ':':
			fields = append(fields, cur.String())
			cur.Reset()
		default:
			cur.WriteByte(line[i])
		}
	}
	return append(fields, cur.String())
}

// ParseBluetooth reads `bluetoothctl show` and `bluetoothctl devices
// Connected`.
func ParseBluetooth(show, connected []byte) Bluetooth {
	var bt Bluetooth
	sc := bufio.NewScanner(bytes.NewReader(show))
	for sc.Scan() {
		if k, v, ok := strings.Cut(strings.TrimSpace(sc.Text()), ": "); ok && k == "Powered" {
			bt.Powered = v == "yes"
		}
	}
	sc = bufio.NewScanner(bytes.NewReader(connected))
	for sc.Scan() {
		// "Device 00:1B:66:AA:BB:CC WH-1000XM4"
		f := strings.SplitN(sc.Text(), " ", 3)
		if len(f) == 3 && f[0] == "Device" {
			bt.Connected = append(bt.Connected, f[2])
		}
	}
	return bt
}

var (
	defaultNodeRe = regexp.MustCompile(`\*\s+\d+\.\s+(.*?)\s*(\[vol:.*\])?$`)
	volumeRe      = regexp.MustCompile(`Volume:\s+([\d.]+)(\s+\[MUTED\])?`)
)

// ParseAudio reads the default sink and source from `wpctl status` (the
// entries marked *) and the volume from `wpctl get-volume`.
func ParseAudio(status, volume []byte) Audio {
	var a Audio
	section := ""
	sc := bufio.NewScanner(bytes.NewReader(status))
	for sc.Scan() {
		line := sc.Text()
		switch {
		case line == "Video":
			section = "done" // Video has its own Sinks/Sources
		case section == "done":
		case strings.Contains(line, "Sinks:"):
			section = "sinks"
		case strings.Contains(line, "Sources:"):
			section = "sources"
		case strings.Contains(line, "Filters:"), strings.Contains(line, "Streams:"):
			section = ""
		default:
			m := defaultNodeRe.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			if section == "sinks" && a.Sink == "" {
				a.Sink = m[1]
			}
			if section == "sources" && a.Source == "" {
				a.Source = m[1]
			}
		}
	}
	if m := volumeRe.FindSubmatch(volume); m != nil {
		v, _ := strconv.ParseFloat(string(m[1]), 64)
		a.Volume = int(v*100 + 0.5)
		a.Muted = len(m[2]) > 0
	}
	return a
}
