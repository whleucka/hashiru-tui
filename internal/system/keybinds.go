package system

import (
	"bufio"
	"bytes"
	"sort"
	"strconv"
	"strings"
)

// Bind is one Hyprland keybind that has a description.
type Bind struct {
	Mods        string // "Super+Shift", or "" for none
	Key         string // as you'd read it: "/", "Return", "T"
	Description string
}

// Combo is the full chord: "Super+Shift+T".
func (b Bind) Combo() string {
	if b.Mods == "" {
		return b.Key
	}
	return b.Mods + "+" + b.Key
}

// Matches reports whether every word appears in the combo or description.
// words must be lower case.
func (b Bind) Matches(words []string) bool {
	hay := strings.ToLower(b.Combo() + " " + strings.ReplaceAll(b.Mods, "+", " ") + " " + b.Description)
	for _, w := range words {
		if !strings.Contains(hay, w) {
			return false
		}
	}
	return true
}

// ParseBinds reads `hyprctl binds`. The plain text, not -j: on Hyprland 0.56
// the JSON is malformed for Lua-registered binds (see Hashiru's
// hypr/scripts/keybinds). Binds inside a submap, and binds without a
// description, are left out, as that script does.
func ParseBinds(out []byte) []Bind {
	var binds []Bind
	var cur map[string]string
	flush := func() {
		if cur == nil || cur["description"] == "" || cur["submap"] != "" {
			return
		}
		mask, _ := strconv.Atoi(cur["modmask"])
		key := cur["key"]
		if key == "" && cur["keycode"] != "" && cur["keycode"] != "0" {
			key = "code:" + cur["keycode"]
		}
		binds = append(binds, Bind{Mods: mods(mask), Key: keyName(key), Description: cur["description"]})
	}

	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "bind") {
			flush()
			cur = map[string]string{}
			continue
		}
		if cur == nil || !strings.HasPrefix(line, "\t") {
			continue
		}
		k, v, ok := strings.Cut(strings.TrimPrefix(line, "\t"), ": ")
		if !ok {
			k, v = strings.TrimSuffix(strings.TrimPrefix(line, "\t"), ":"), ""
		}
		cur[k] = v
	}
	flush()

	sort.SliceStable(binds, func(i, j int) bool {
		if binds[i].Mods != binds[j].Mods {
			return binds[i].Mods < binds[j].Mods
		}
		return binds[i].Key < binds[j].Key
	})
	return binds
}

// mods turns Hyprland's modmask into names. The bits are X11's: Shift 1,
// Ctrl 4, Alt (Mod1) 8, Super (Mod4) 64.
func mods(mask int) string {
	var m []string
	for _, b := range []struct {
		bit  int
		name string
	}{{64, "Super"}, {4, "Ctrl"}, {8, "Alt"}, {1, "Shift"}} {
		if mask&b.bit != 0 {
			m = append(m, b.name)
		}
	}
	return strings.Join(m, "+")
}

var keyNames = map[string]string{
	"slash": "/", "comma": ",", "period": ".", "return": "Return",
	"space": "Space", "escape": "Esc", "backspace": "Backspace",
	"minus": "-", "equal": "=", "code:20": "-", "code:21": "=",
}

func keyName(k string) string {
	if n, ok := keyNames[strings.ToLower(k)]; ok {
		return n
	}
	if len(k) == 1 {
		return strings.ToUpper(k)
	}
	return k
}
