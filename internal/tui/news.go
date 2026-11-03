package tui

import (
	"fmt"
)

// newsPreview is how many news items Updates lists.
const newsPreview = 5

// newsLine is Overview's one line about Arch news.
func newsLine(st *State) string {
	switch {
	case st.News == nil && st.NewsBusy:
		return styleMuted.Render("fetching…")
	case st.News == nil && st.NewsErr != nil:
		return styleMuted.Render("unavailable (" + st.NewsErr.Error() + ")")
	}
	if n := st.UnreadNews(); n > 0 {
		return styleWarn.Render(fmt.Sprintf("%d since your last update, read before updating", n))
	}
	return styleMuted.Render("nothing new since your last update")
}

// newsRows lists the latest items, flagging those since the last update.
func newsRows(st *State) []string {
	if st.News == nil {
		if st.NewsErr != nil {
			return []string{styleFail.Render("  " + st.NewsErr.Error())}
		}
		return nil
	}
	unread := st.UnreadNews()
	var rows []string
	for i, it := range st.News {
		if i == newsPreview {
			break
		}
		mark := "  "
		if i < unread {
			mark = styleWarn.Render("● ")
		}
		rows = append(rows, mark+styleMuted.Render(it.Date.Format("2006-01-02"))+"  "+it.Title)
	}
	return rows
}
