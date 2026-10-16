package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/whleucka/hashiru-tui/internal/system"
)

// connectivity is network, bluetooth and audio: what's connected, and the
// tools that change it. It shows; nmtui, bluetoothctl and the mixers do.
type connectivity struct{ e *env }

func newConnectivity(e *env) *connectivity { return &connectivity{e: e} }

func (c *connectivity) Name() string           { return "Connectivity" }
func (c *connectivity) Update(tea.Msg) tea.Cmd { return nil }

func (c *connectivity) Actions() []Action {
	return []Action{
		{Title: "Network: connect to wifi, edit connections (nmtui)", Key: "w", Run: c.tool("nmtui", Interactive)},
		{Title: "Bluetooth: pair and connect (bluetoothctl)", Key: "b", Run: c.tool("bluetoothctl", Interactive)},
		{Title: "Bluetooth manager (blueman)", Key: "B", Run: c.tool("blueman-manager", GUI)},
		{Title: "Audio mixer (pavucontrol)", Key: "v", Run: c.tool("pavucontrol", GUI)},
	}
}

func (c *connectivity) tool(name string, kind Kind) func() tea.Cmd {
	return func() tea.Cmd {
		// Refresh: back from nmtui, the connection may well have changed.
		return c.e.launch(Launch{Argv: []string{name}, Kind: kind, Refresh: kind == Interactive})
	}
}

func (c *connectivity) HandleKey(msg tea.KeyPressMsg) (tea.Cmd, bool) {
	for _, a := range c.Actions() {
		if msg.String() == a.Key {
			return a.Run(), true
		}
	}
	return nil, false
}

func (c *connectivity) Keys() []Key {
	return []Key{
		{Keys: "w", Desc: "nmtui"},
		{Keys: "b", Desc: "bluetoothctl"},
		{Keys: "B", Desc: "blueman"},
		{Keys: "v", Desc: "mixer"},
	}
}

func (c *connectivity) View(width, height int) string {
	conn := c.e.state.Conn
	if conn == nil {
		return styleMuted.Render("reading network, bluetooth and audio…")
	}
	var b strings.Builder
	row := func(k, v string) { b.WriteString(styleKey.Render(k) + v + "\n") }

	b.WriteString(styleTitle.Render("Network") + styleMuted.Render("  w nmtui") + "\n")
	switch {
	case conn.NetworkMissing:
		b.WriteString(styleMuted.Render("nmcli not installed") + "\n")
	case conn.NetworkErr != nil:
		b.WriteString(styleFail.Render(conn.NetworkErr.Error()) + "\n")
	}
	for _, d := range conn.Devices {
		state := styleMuted.Render(d.State)
		if strings.HasPrefix(d.State, "connected") {
			state = styleOK.Render(d.State)
		}
		what := d.Connection
		if d.Type == "wifi" && conn.WifiSSID != "" && d.Connection != "" {
			what = fmt.Sprintf("%s  %d%%", conn.WifiSSID, conn.WifiSignal)
		}
		row(d.Type, fmt.Sprintf("%-12s %s  %s", d.Name, state, what))
	}

	b.WriteString("\n" + styleTitle.Render("Bluetooth") + styleMuted.Render("  b bluetoothctl · B blueman") + "\n")
	row("adapter", bluetoothLine(conn))
	for _, dev := range conn.Bluetooth.Connected {
		row("", dev)
	}

	b.WriteString("\n" + styleTitle.Render("Audio") + styleMuted.Render("  v mixer") + "\n")
	switch {
	case conn.AudioMissing:
		b.WriteString(styleMuted.Render("wpctl not installed") + "\n")
	case conn.AudioErr != nil:
		b.WriteString(styleFail.Render(conn.AudioErr.Error()) + "\n")
	default:
		row("output", audioLine(conn))
		row("input", orUnknown(conn.Audio.Source))
	}
	return b.String()
}

// networkLine is Overview's: the connected wifi or wired connection.
func networkLine(c *system.Connectivity) string {
	switch {
	case c.NetworkMissing:
		return styleMuted.Render("nmcli not installed")
	case c.NetworkErr != nil:
		return styleFail.Render(c.NetworkErr.Error())
	}
	var up []string
	for _, d := range c.Devices {
		if !strings.HasPrefix(d.State, "connected") {
			continue
		}
		if d.Type == "wifi" && c.WifiSSID != "" {
			up = append(up, fmt.Sprintf("%s (%d%%)", c.WifiSSID, c.WifiSignal))
		} else {
			up = append(up, d.Connection)
		}
	}
	if len(up) == 0 {
		return styleWarn.Render("offline")
	}
	return strings.Join(up, styleMuted.Render(" · "))
}

func bluetoothLine(c *system.Connectivity) string {
	switch {
	case c.BluetoothMissing:
		return styleMuted.Render("bluetoothctl not installed")
	case c.BluetoothErr != nil:
		return styleFail.Render(c.BluetoothErr.Error())
	case !c.Bluetooth.Powered:
		return styleMuted.Render("off")
	}
	return fmt.Sprintf("on, %d connected", len(c.Bluetooth.Connected))
}

func audioLine(c *system.Connectivity) string {
	switch {
	case c.AudioMissing:
		return styleMuted.Render("wpctl not installed")
	case c.AudioErr != nil:
		return styleFail.Render(c.AudioErr.Error())
	}
	vol := fmt.Sprintf("%d%%", c.Audio.Volume)
	if c.Audio.Muted {
		vol = styleWarn.Render(vol + " muted")
	}
	return orUnknown(c.Audio.Sink) + "  " + vol
}
