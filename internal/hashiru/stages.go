package hashiru

import (
	"strings"
)

// Stage is one of install.sh's stages.
type Stage struct {
	Num  string // "45", what install.sh takes as a selector
	Name string // "config"
	File string // "45-config.sh"
}

// ParseStages reads `hashiru install --list`: a heading, then one
// "  NN-name.sh" line per stage.
func ParseStages(out []byte) []Stage {
	var stages []Stage
	for _, l := range strings.Split(string(out), "\n") {
		file := strings.TrimSpace(l)
		base, ok := strings.CutSuffix(file, ".sh")
		if !ok {
			continue
		}
		num, name, ok := strings.Cut(base, "-")
		if !ok || num == "" || strings.Trim(num, "0123456789") != "" {
			continue
		}
		stages = append(stages, Stage{Num: num, Name: name, File: file})
	}
	return stages
}
