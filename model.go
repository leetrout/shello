package main

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// Layout constants (in terminal cells). Column width is computed at runtime
// from the terminal size — see Model.colWidth.
const (
	colGap     = 2 // horizontal gap between columns
	minColW    = 16
	boardTop   = 2 // rows reserved for the app header before columns start
	colHeaderH = 2 // column title + separator line
	cardSlot   = 2 // each card = 1 content row + 1 spacer row
	footerH    = 4 // rows reserved at the bottom for status + help
)

// colWidth is the inner width of each column, sized so the columns fill the
// terminal width. Falls back to a minimum when there are too many to fit.
func (m Model) colWidth() int {
	n := len(m.board.Columns)
	if n <= 0 || m.width <= 0 {
		return minColW
	}
	avail := m.width - (n-1)*colGap
	if avail < n*minColW {
		return minColW
	}
	return avail / n
}

// colOuterWidth is one column plus the gap that follows it.
func (m Model) colOuterWidth() int { return m.colWidth() + colGap }

// boardHeight is the number of rows available for column bodies.
func (m Model) boardHeight() int {
	h := m.height - boardTop - footerH
	if h < 3 {
		h = 3
	}
	return h
}

type inputMode int

const (
	modeNormal inputMode = iota
	modeInput
)

type inputPurpose int

const (
	purposeAddCard inputPurpose = iota
	purposeEditCard
	purposeAddColumn
	purposeRenameColumn
)

// drag holds the state of an in-progress mouse drag.
type drag struct {
	active   bool
	fromCol  int
	fromIdx  int
	overCol  int // column the cursor is currently over (-1 if none)
	overIdx  int // insertion index within overCol
	title    string
}

// Model is the Bubble Tea model for the whole app.
type Model struct {
	board    Board
	path     string
	width    int
	height   int
	curCol   int
	curCard  int
	showHelp bool
	status   string

	mode    inputMode
	purpose inputPurpose
	input   textinput.Model

	drag drag
}

// NewModel builds the initial model.
func NewModel(board Board, path string) Model {
	ti := textinput.New()
	ti.CharLimit = 120
	ti.Prompt = "› "
	return Model{
		board:  board,
		path:   path,
		curCol: 0,
		input:  ti,
		drag:   drag{overCol: -1},
	}
}

func (m Model) Init() tea.Cmd { return nil }

// ---- helpers ----

func (m *Model) clampCursor() {
	if len(m.board.Columns) == 0 {
		m.curCol, m.curCard = 0, 0
		return
	}
	if m.curCol < 0 {
		m.curCol = 0
	}
	if m.curCol >= len(m.board.Columns) {
		m.curCol = len(m.board.Columns) - 1
	}
	n := len(m.board.Columns[m.curCol].Cards)
	if m.curCard < 0 {
		m.curCard = 0
	}
	if m.curCard >= n {
		m.curCard = n - 1
	}
	if n == 0 {
		m.curCard = 0
	}
}

func (m *Model) save() {
	if err := m.board.Save(m.path); err != nil {
		m.status = "save failed: " + err.Error()
		return
	}
	m.status = "saved → " + m.path
}

// cardsTopAbs is the absolute screen row where a column's cards begin.
func cardsTopAbs() int { return boardTop + colHeaderH }

// hitColumn returns the column index for an absolute X, or -1.
func (m Model) hitColumn(x int) int {
	outer := m.colOuterWidth()
	col := x / outer
	if col < 0 || col >= len(m.board.Columns) {
		return -1
	}
	// reject clicks that land in the gap between columns
	if x%outer >= m.colWidth() {
		return -1
	}
	return col
}

// dropIndex returns the insertion index within a column for an absolute Y.
func (m Model) dropIndex(col, y int) int {
	rel := y - cardsTopAbs()
	if rel < 0 {
		return 0
	}
	idx := (rel + cardSlot/2) / cardSlot // round to nearest slot boundary
	n := len(m.board.Columns[col].Cards)
	if idx > n {
		idx = n
	}
	return idx
}

// hitCard returns the card index at absolute (x,y) within column col, or -1.
func (m Model) hitCard(col, y int) int {
	rel := y - cardsTopAbs()
	if rel < 0 || rel%cardSlot != 0 {
		return -1
	}
	idx := rel / cardSlot
	if idx < 0 || idx >= len(m.board.Columns[col].Cards) {
		return -1
	}
	return idx
}

// ---- update ----

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case tea.MouseMsg:
		return m.handleMouse(msg)

	case tea.KeyMsg:
		if m.mode == modeInput {
			return m.handleInputKey(msg)
		}
		return m.handleNormalKey(msg)
	}
	return m, nil
}

func (m Model) handleInputKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.mode = modeNormal
		m.input.Blur()
		return m, nil
	case tea.KeyEnter:
		val := m.input.Value()
		m.applyInput(val)
		m.mode = modeNormal
		m.input.Blur()
		return m, nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *Model) applyInput(val string) {
	switch m.purpose {
	case purposeAddCard:
		if val == "" || len(m.board.Columns) == 0 {
			return
		}
		c := &m.board.Columns[m.curCol]
		c.Cards = append(c.Cards, Card{Title: val})
		m.curCard = len(c.Cards) - 1
	case purposeEditCard:
		if len(m.board.Columns) == 0 {
			return
		}
		c := &m.board.Columns[m.curCol]
		if m.curCard >= 0 && m.curCard < len(c.Cards) {
			if val == "" {
				// empty edit deletes the card
				c.Cards = append(c.Cards[:m.curCard], c.Cards[m.curCard+1:]...)
			} else {
				c.Cards[m.curCard].Title = val
			}
		}
	case purposeAddColumn:
		if val == "" {
			return
		}
		m.board.Columns = append(m.board.Columns, Column{Title: val})
		m.curCol = len(m.board.Columns) - 1
		m.curCard = 0
	case purposeRenameColumn:
		if val != "" && len(m.board.Columns) > 0 {
			m.board.Columns[m.curCol].Title = val
		}
	}
	m.clampCursor()
	m.save()
}

func (m *Model) startInput(p inputPurpose, prompt, initial string) {
	m.mode = modeInput
	m.purpose = p
	m.input.Placeholder = prompt
	m.input.SetValue(initial)
	m.input.CursorEnd()
	m.input.Focus()
}

func (m Model) handleNormalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		m.save()
		return m, tea.Quit
	case "?":
		m.showHelp = !m.showHelp
		return m, nil

	// selection movement
	case "left", "h":
		m.curCol--
		m.clampCursor()
	case "right", "l":
		m.curCol++
		m.clampCursor()
	case "up", "k":
		m.curCard--
		m.clampCursor()
	case "down", "j":
		m.curCard++
		m.clampCursor()
	case "g":
		m.curCard = 0
		m.clampCursor()
	case "G":
		if len(m.board.Columns) > 0 {
			m.curCard = len(m.board.Columns[m.curCol].Cards) - 1
		}
		m.clampCursor()

	// move card between columns
	case "H", "shift+left":
		m.moveCurrentCard(m.curCol-1, -1)
	case "L", "shift+right":
		m.moveCurrentCard(m.curCol+1, -1)
	// reorder card within column
	case "K", "shift+up":
		m.moveCurrentCard(m.curCol, m.curCard-1)
	case "J", "shift+down":
		m.moveCurrentCard(m.curCol, m.curCard+1)

	// editing
	case "a":
		if len(m.board.Columns) > 0 {
			m.startInput(purposeAddCard, "new card…", "")
		}
	case "e", "enter":
		if len(m.board.Columns) > 0 && len(m.board.Columns[m.curCol].Cards) > 0 {
			m.startInput(purposeEditCard, "edit card…", m.board.Columns[m.curCol].Cards[m.curCard].Title)
		}
	case "d", "x":
		m.deleteCurrentCard()
	case "n":
		m.startInput(purposeAddColumn, "new column…", "")
	case "r":
		if len(m.board.Columns) > 0 {
			m.startInput(purposeRenameColumn, "rename column…", m.board.Columns[m.curCol].Title)
		}
	case "D":
		m.deleteCurrentColumn()
	case "s":
		m.save()
	}
	return m, nil
}

func (m *Model) moveCurrentCard(toCol, toIdx int) {
	if len(m.board.Columns) == 0 {
		return
	}
	if toCol < 0 || toCol >= len(m.board.Columns) {
		return
	}
	// when moving to a different column with unspecified index, append to end
	if toIdx < 0 {
		toIdx = len(m.board.Columns[toCol].Cards)
	}
	nc, ni := m.board.moveCard(m.curCol, m.curCard, toCol, toIdx)
	m.curCol, m.curCard = nc, ni
	m.clampCursor()
	m.save()
}

func (m *Model) deleteCurrentCard() {
	if len(m.board.Columns) == 0 {
		return
	}
	c := &m.board.Columns[m.curCol]
	if m.curCard >= 0 && m.curCard < len(c.Cards) {
		c.Cards = append(c.Cards[:m.curCard], c.Cards[m.curCard+1:]...)
		m.clampCursor()
		m.save()
	}
}

func (m *Model) deleteCurrentColumn() {
	if len(m.board.Columns) == 0 {
		return
	}
	m.board.Columns = append(m.board.Columns[:m.curCol], m.board.Columns[m.curCol+1:]...)
	if m.curCol >= len(m.board.Columns) {
		m.curCol = len(m.board.Columns) - 1
	}
	m.curCard = 0
	m.clampCursor()
	m.save()
}

func (m Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.mode == modeInput {
		return m, nil
	}
	switch msg.Action {
	case tea.MouseActionPress:
		if msg.Button != tea.MouseButtonLeft {
			return m, nil
		}
		col := m.hitColumn(msg.X)
		if col < 0 {
			return m, nil
		}
		card := m.hitCard(col, msg.Y)
		if card < 0 {
			// clicked a column but not a card: just focus the column
			m.curCol = col
			m.clampCursor()
			return m, nil
		}
		// begin drag and focus
		m.curCol, m.curCard = col, card
		m.drag = drag{
			active:  true,
			fromCol: col,
			fromIdx: card,
			overCol: col,
			overIdx: card,
			title:   m.board.Columns[col].Cards[card].Title,
		}
		return m, nil

	case tea.MouseActionMotion:
		if !m.drag.active {
			return m, nil
		}
		col := m.hitColumn(msg.X)
		m.drag.overCol = col
		if col >= 0 {
			m.drag.overIdx = m.dropIndex(col, msg.Y)
		}
		return m, nil

	case tea.MouseActionRelease:
		if !m.drag.active {
			return m, nil
		}
		d := m.drag
		m.drag = drag{overCol: -1}
		if d.overCol < 0 {
			return m, nil // dropped outside any column: cancel
		}
		toIdx := d.overIdx
		// dropping later in the same column: account for the removal shift
		if d.overCol == d.fromCol && toIdx > d.fromIdx {
			toIdx--
		}
		nc, ni := m.board.moveCard(d.fromCol, d.fromIdx, d.overCol, toIdx)
		m.curCol, m.curCard = nc, ni
		m.clampCursor()
		m.save()
		return m, nil
	}
	return m, nil
}
