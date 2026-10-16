package tui

// moveKey is the cursor movement every list shares: j/k (or the arrows)
// one row, ctrl+d/ctrl+u half a page, as in vim and less. i is the cursor
// (or scroll offset) among n rows, page the rows on screen. ok is false
// for any other key.
func moveKey(key string, i, n, page int) (int, bool) {
	half := max(page/2, 1)
	switch key {
	case "j", "down":
		i++
	case "k", "up":
		i--
	case "ctrl+d":
		i += half
	case "ctrl+u":
		i -= half
	default:
		return i, false
	}
	return max(0, min(i, n-1)), true
}
