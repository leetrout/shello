package main

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Base styles carry colours only; width/height are applied per-render because
// they depend on the live terminal size.
var (
	appTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#ffffff")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1)

	panelBg = lipgloss.Color("#1E1E2A")

	colTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#C9A9FF")).
			Background(panelBg).
			Padding(0, 1)

	colTitleActive = colTitleStyle.
			Foreground(lipgloss.Color("#ffffff")).
			Background(lipgloss.Color("#7D56F4"))

	colTitleDrop = colTitleStyle.
			Foreground(lipgloss.Color("#1a1a1a")).
			Background(lipgloss.Color("#4EF0A5"))

	sepStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#3a3a4a")).
			Background(panelBg)

	cardStyle = lipgloss.NewStyle().
			Padding(0, 1).
			Background(lipgloss.Color("#2E2E42")).
			Foreground(lipgloss.Color("#E6E6F0"))

	cardSelected = cardStyle.
			Background(lipgloss.Color("#7D56F4")).
			Foreground(lipgloss.Color("#ffffff")).
			Bold(true)

	cardGrabbed = cardStyle.
			Background(lipgloss.Color("#4EF0A5")).
			Foreground(lipgloss.Color("#0d0d14")).
			Bold(true)

	cardDragging = cardStyle.
			Faint(true).
			Background(panelBg).
			Foreground(lipgloss.Color("#666677"))

	emptyStyle = lipgloss.NewStyle().
			Background(panelBg).
			Foreground(lipgloss.Color("#55556a")).
			Padding(0, 1)

	helpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#8A8AA0"))

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#4EF0A5")).
			Italic(true)

	grabStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#0d0d14")).
			Background(lipgloss.Color("#4EF0A5")).
			Bold(true).
			Padding(0, 1)
)

func truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= w {
		return s
	}
	r := []rune(s)
	for len(r) > 0 && lipgloss.Width(string(r))+1 > w {
		r = r[:len(r)-1]
	}
	return string(r) + "…"
}

// splitAt splits s so the first part is at most width display cells wide.
func splitAt(s string, width int) (string, string) {
	r := []rune(s)
	w := 0
	for i, c := range r {
		cw := lipgloss.Width(string(c))
		if w+cw > width && i > 0 {
			return string(r[:i]), string(r[i:])
		}
		w += cw
	}
	return s, ""
}

// wrapText word-wraps s to the given cell width, hard-breaking any word longer
// than the width. It is the single source of truth for card line breaking, used
// by both the renderer and the mouse hit-testing so their geometry always agrees.
func wrapText(s string, width int) []string {
	if width < 1 {
		width = 1
	}
	var lines []string
	for _, para := range strings.Split(s, "\n") {
		cur := ""
		for _, word := range strings.Fields(para) {
			for lipgloss.Width(word) > width {
				if cur != "" {
					lines = append(lines, cur)
					cur = ""
				}
				var head string
				head, word = splitAt(word, width)
				lines = append(lines, head)
			}
			switch {
			case cur == "":
				cur = word
			case lipgloss.Width(cur)+1+lipgloss.Width(word) <= width:
				cur += " " + word
			default:
				lines = append(lines, cur)
				cur = word
			}
		}
		if cur != "" {
			lines = append(lines, cur)
		}
	}
	if len(lines) == 0 {
		lines = append(lines, "")
	}
	return lines
}

func (m Model) View() string {
	if m.width == 0 {
		return "starting shello…"
	}

	// ---- app header (spans the full width) ----
	header := appTitleStyle.Width(m.width).Render("shello ▪ trello in your terminal")
	b := &strings.Builder{}
	b.WriteString(header)
	b.WriteString("\n\n") // blank line → columns start at row boardTop (=2)

	// ---- columns ----
	if len(m.board.Columns) == 0 {
		b.WriteString(helpStyle.Render("no columns — press n to make one"))
	} else {
		w := m.colWidth()
		h := m.boardHeight()
		cols := make([]string, len(m.board.Columns))
		for i, col := range m.board.Columns {
			cols[i] = m.renderColumn(i, col, w, h)
		}
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, cols...))
	}

	// ---- footer ----
	b.WriteString("\n\n")
	b.WriteString(m.renderFooter())
	return b.String()
}

func (m Model) renderColumn(i int, col Column, w, h int) string {
	container := lipgloss.NewStyle().
		Width(w).
		Height(h).
		Background(panelBg)
	if i < len(m.board.Columns)-1 {
		container = container.MarginRight(colGap)
	}

	// build the flat list of card rows (wrapped content + a spacer per card)
	cardRows := make([]string, 0, len(col.Cards)*3)
	for j, card := range col.Cards {
		style := cardStyle
		switch {
		case m.drag.active && i == m.drag.fromCol && j == m.drag.fromIdx:
			style = cardDragging
		case m.grabbed && i == m.curCol && j == m.curCard:
			style = cardGrabbed
		case i == m.curCol && j == m.curCard && !m.drag.active:
			style = cardSelected
		}
		wrapped := strings.Join(wrapText(card.Title, w-2), "\n")
		for _, ln := range strings.Split(style.Width(w).Render(wrapped), "\n") {
			cardRows = append(cardRows, ln)
		}
		cardRows = append(cardRows, sepStyle.Width(w).Render("")) // spacer row
	}
	if len(col.Cards) == 0 {
		cardRows = append(cardRows, emptyStyle.Width(w).Render("(empty)"))
	}

	// apply the scroll window
	view := m.cardsViewH()
	scroll := m.scrollFor(i)
	end := scroll + view
	if end > len(cardRows) {
		end = len(cardRows)
	}
	visible := cardRows
	if scroll < len(cardRows) {
		visible = cardRows[scroll:end]
	} else {
		visible = nil
	}

	// header: title (with scroll hints) + separator
	titleStyle := colTitleStyle
	if m.drag.active && m.drag.overCol == i {
		titleStyle = colTitleDrop
	} else if i == m.curCol {
		titleStyle = colTitleActive
	}
	hint := ""
	if scroll > 0 {
		hint += " ↑"
	}
	if scroll < m.maxScroll(i) {
		hint += " ↓"
	}
	title := truncate(col.Title, w-8)
	lines := make([]string, 0, view+colHeaderH)
	lines = append(lines, titleStyle.Width(w).Render(title+" "+countBadge(len(col.Cards))+hint))
	lines = append(lines, sepStyle.Width(w).Render(strings.Repeat("─", w)))
	lines = append(lines, visible...)

	body := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return container.Render(body)
}

func countBadge(n int) string {
	if n == 0 {
		return ""
	}
	return lipgloss.NewStyle().Faint(true).Render("•" + itoa(n))
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

func (m Model) renderFooter() string {
	if m.mode == modeInput {
		return m.input.View()
	}

	var b strings.Builder
	switch {
	case m.grabbed:
		b.WriteString(grabStyle.Render("◆ moving card — ←→ column · ↑↓ reorder · space/enter to drop"))
		b.WriteString("\n")
	case m.drag.active:
		target := "—"
		if m.drag.overCol >= 0 {
			target = m.board.Columns[m.drag.overCol].Title
		}
		b.WriteString(statusStyle.Render("⇢ dragging \"" + truncate(m.drag.title, 30) + "\" → " + target))
		b.WriteString("\n")
	case m.status != "":
		b.WriteString(statusStyle.Render(m.status))
		b.WriteString("\n")
	}

	if m.showHelp {
		b.WriteString(helpStyle.Render(fullHelp))
	} else {
		b.WriteString(helpStyle.Render("hjkl/↑↓←→ move cursor · space grab & move card · a add · e edit · d del · n col · mouse drag · ? help · q quit"))
	}
	return b.String()
}

const fullHelp = `navigate   ←/→/h/l columns   ↑/↓/j/k cards   g/G top/bottom
move card  space to grab, then ←→ column / ↑↓ reorder, space to drop
           (H/L/J/K also move the selected card directly)
mouse      click a card and drag it to any column to drop it
cards      a add   e/enter edit   d/x delete   (empty edit deletes)
columns    n new   r rename   D delete
other      s save (auto-saves on every change)   ? toggle help   q quit`
