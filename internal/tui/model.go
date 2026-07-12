// Package tui implements shello's Bubble Tea terminal UI: the kanban board
// model, its update logic (keyboard + mouse), and rendering.
package tui

import (
	"reflect"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/leetrout/shello/internal/board"
)

// Layout constants (in terminal cells). Column width is computed at runtime
// from the terminal size — see Model.colWidth.
const (
	colGap     = 2 // horizontal gap between columns
	minColW    = 16
	boardTop   = 2 // rows reserved for the app header before columns start
	colHeaderH = 2 // column title + separator line
	cardGap    = 1 // spacer row between cards
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
	modeConfirm
)

type inputPurpose int

const (
	purposeAddCard inputPurpose = iota
	purposeEditCard
	purposeAddColumn
	purposeRenameColumn
)

// confirmAction is the pending action a yes/no confirmation will carry out.
type confirmAction int

const (
	confirmNone confirmAction = iota
	confirmDeleteColumn
)

// drag holds the state of an in-progress mouse drag.
type drag struct {
	active  bool
	fromCol int
	fromIdx int
	overCol int // column the cursor is currently over (-1 if none)
	overIdx int // insertion index within overCol
	title   string
}

// Model is the Bubble Tea model for the whole app.
type Model struct {
	board    board.Board
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

	// confirm is the action awaiting a yes/no answer while mode == modeConfirm.
	confirm confirmAction

	drag drag

	// grabbed is true when the selected card has been "picked up" for
	// keyboard move: arrows/hjkl then relocate it until it is dropped.
	grabbed bool

	// colScroll[i] is the number of card rows scrolled off the top of column i.
	colScroll []int

	// undo/redo hold board snapshots. Every mutation pushes the pre-image onto
	// undo (see Update); undoing moves it to redo and vice versa.
	undo []snapshot
	redo []snapshot

	// grabStart is the pre-grab snapshot captured when a card is picked up. The
	// whole grab-and-move is recorded as a single undo entry when it is dropped,
	// so undo can't step through it a column at a time.
	grabStart *snapshot
}

// snapshot is a point-in-time copy of the board and cursor for undo/redo. The
// board is deep-cloned so it can't alias live state.
type snapshot struct {
	board   board.Board
	curCol  int
	curCard int
}

// maxUndo bounds how many snapshots each stack retains.
const maxUndo = 100

// New builds the initial model for the given board and save path.
func New(b board.Board, path string) Model {
	ti := textinput.New()
	ti.CharLimit = 120
	ti.Prompt = "› "
	return Model{
		board:  b,
		path:   path,
		curCol: 0,
		input:  ti,
		drag:   drag{overCol: -1},
	}
}

// Init implements tea.Model; shello needs no startup command.
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
	m.ensureVisible()
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

// columnCardLayout returns, for each card in col, its top row (relative to
// cardsTopAbs) and its rendered height in rows. It uses the same wrapping as the
// renderer so mouse geometry and the display never disagree.
func (m Model) columnCardLayout(col int) (tops, heights []int) {
	if col < 0 || col >= len(m.board.Columns) {
		return nil, nil
	}
	innerW := m.colWidth() - 2
	offset := 0
	for _, c := range m.board.Columns[col].Cards {
		h := len(wrapText(c.Title, innerW))
		if h < 1 {
			h = 1
		}
		tops = append(tops, offset)
		heights = append(heights, h)
		offset += h + cardGap
	}
	return tops, heights
}

// cardsViewH is the number of card rows visible in a column (below its header).
func (m Model) cardsViewH() int {
	h := m.boardHeight() - colHeaderH
	if h < 1 {
		h = 1
	}
	return h
}

// contentHeight is the total number of rows a column's cards occupy.
func (m Model) contentHeight(col int) int {
	tops, heights := m.columnCardLayout(col)
	if len(tops) == 0 {
		return 0
	}
	last := len(tops) - 1
	return tops[last] + heights[last]
}

// maxScroll is the furthest a column can scroll before its last card sits at the
// bottom of the viewport.
func (m Model) maxScroll(col int) int {
	over := m.contentHeight(col) - m.cardsViewH()
	if over < 0 {
		return 0
	}
	return over
}

// scrollFor returns the clamped scroll offset (in rows) for a column.
func (m Model) scrollFor(col int) int {
	s := 0
	if col >= 0 && col < len(m.colScroll) {
		s = m.colScroll[col]
	}
	if limit := m.maxScroll(col); s > limit {
		s = limit
	}
	if s < 0 {
		s = 0
	}
	return s
}

// setScroll stores a clamped scroll offset for a column, growing the slice.
func (m *Model) setScroll(col, v int) {
	if col < 0 {
		return
	}
	for len(m.colScroll) <= col {
		m.colScroll = append(m.colScroll, 0)
	}
	if limit := m.maxScroll(col); v > limit {
		v = limit
	}
	if v < 0 {
		v = 0
	}
	m.colScroll[col] = v
}

// ensureVisible scrolls the current column so the selected card is fully in view.
func (m *Model) ensureVisible() {
	col := m.curCol
	tops, heights := m.columnCardLayout(col)
	if m.curCard < 0 || m.curCard >= len(tops) {
		return
	}
	top := tops[m.curCard]
	bottom := top + heights[m.curCard]
	view := m.cardsViewH()
	s := m.scrollFor(col)
	if top < s {
		s = top
	}
	if bottom > s+view {
		s = bottom - view
	}
	m.setScroll(col, s)
}

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

// dropIndex returns the insertion index within a column for an absolute Y,
// based on which card the cursor's row is above the vertical midpoint of.
func (m Model) dropIndex(col, y int) int {
	rel := y - cardsTopAbs() + m.scrollFor(col)
	if rel < 0 {
		return 0
	}
	tops, heights := m.columnCardLayout(col)
	for j := range tops {
		mid := tops[j] + (heights[j]+1)/2
		if rel < mid {
			return j
		}
	}
	return len(tops)
}

// hitCard returns the card index at absolute (x,y) within column col, or -1 if
// the row falls on a spacer or below the last card.
func (m Model) hitCard(col, y int) int {
	if y < cardsTopAbs() || y >= cardsTopAbs()+m.cardsViewH() {
		return -1 // outside the scrollable card viewport
	}
	rel := y - cardsTopAbs() + m.scrollFor(col)
	if rel < 0 {
		return -1
	}
	tops, heights := m.columnCardLayout(col)
	for j := range tops {
		if rel >= tops[j] && rel < tops[j]+heights[j] {
			return j
		}
	}
	return -1
}

// ---- update ----

// Update implements tea.Model. It intercepts undo/redo, then dispatches the
// message and records an undo snapshot if the board changed as a result. This
// single choke point means individual mutations don't have to remember to push
// their own undo state.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Undo/redo restore state directly and must not themselves be recorded.
	if k, ok := msg.(tea.KeyMsg); ok && m.mode == modeNormal && !m.grabbed {
		switch k.String() {
		case "u":
			return m.undoLast(), nil
		case "ctrl+r":
			return m.redoLast(), nil
		}
	}

	wasGrabbed := m.grabbed
	before := m.snapshot()
	updated, cmd := m.dispatch(msg)
	mm := updated.(Model)

	switch {
	case !wasGrabbed && mm.grabbed:
		// Card picked up: stash the pre-grab state; recording waits for the drop.
		mm.grabStart = &before
	case wasGrabbed && mm.grabbed:
		// Mid-grab movement: individual steps are not recorded.
	case wasGrabbed && !mm.grabbed:
		// Dropped: record the entire grab as a single undoable action.
		if mm.grabStart != nil && !reflect.DeepEqual(mm.grabStart.board, mm.board) {
			mm.pushUndo(*mm.grabStart)
		}
		mm.grabStart = nil
	default:
		if !reflect.DeepEqual(before.board, mm.board) {
			mm.pushUndo(before)
		}
	}
	return mm, cmd
}

// pushUndo records a pre-mutation snapshot, capping the stack and invalidating
// the redo branch (a fresh edit forks history).
func (m *Model) pushUndo(s snapshot) {
	m.undo = append(m.undo, s)
	if len(m.undo) > maxUndo {
		m.undo = m.undo[len(m.undo)-maxUndo:]
	}
	m.redo = nil
}

// snapshot captures the current board (deep-cloned) and cursor.
func (m Model) snapshot() snapshot {
	return snapshot{board: m.board.Clone(), curCol: m.curCol, curCard: m.curCard}
}

// restore replaces the board and cursor from a snapshot, cloning so the stack
// entry can't alias live state, then reconciles cursor/scroll and persists.
func (m *Model) restore(s snapshot) {
	m.board = s.board.Clone()
	m.curCol, m.curCard = s.curCol, s.curCard
	m.clampCursor()
	m.save()
}

// undoLast reverts to the most recent snapshot, pushing the current state onto
// the redo stack.
func (m Model) undoLast() Model {
	if len(m.undo) == 0 {
		m.status = "nothing to undo"
		return m
	}
	m.redo = append(m.redo, m.snapshot())
	last := m.undo[len(m.undo)-1]
	m.undo = m.undo[:len(m.undo)-1]
	m.restore(last)
	m.status = "undo"
	return m
}

// redoLast re-applies the most recently undone snapshot.
func (m Model) redoLast() Model {
	if len(m.redo) == 0 {
		m.status = "nothing to redo"
		return m
	}
	m.undo = append(m.undo, m.snapshot())
	next := m.redo[len(m.redo)-1]
	m.redo = m.redo[:len(m.redo)-1]
	m.restore(next)
	m.status = "redo"
	return m
}

// dispatch routes a message to the right handler by mode.
func (m Model) dispatch(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case tea.MouseMsg:
		return m.handleMouse(msg)

	case tea.KeyMsg:
		switch m.mode {
		case modeInput:
			return m.handleInputKey(msg)
		case modeConfirm:
			return m.handleConfirmKey(msg)
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

// handleConfirmKey answers a pending yes/no confirmation. Any key other than
// y/Y/enter cancels.
func (m Model) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y", "enter":
		switch m.confirm {
		case confirmDeleteColumn:
			m.deleteCurrentColumn()
		}
	}
	m.mode = modeNormal
	m.confirm = confirmNone
	return m, nil
}

func (m *Model) applyInput(val string) {
	switch m.purpose {
	case purposeAddCard:
		if val == "" || len(m.board.Columns) == 0 {
			return
		}
		c := &m.board.Columns[m.curCol]
		c.Cards = append(c.Cards, board.Card{Title: val})
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
		m.board.Columns = append(m.board.Columns, board.Column{Title: val})
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
	// While a card is grabbed, movement keys carry the card instead of the
	// cursor. This is the keyboard equivalent of a mouse drag.
	if m.grabbed {
		return m.handleGrabbedKey(msg)
	}

	switch msg.String() {
	case "ctrl+c", "q":
		m.save()
		return m, tea.Quit
	case "?":
		m.showHelp = !m.showHelp
		return m, nil

	// pick up the selected card for a keyboard move
	case " ":
		if len(m.board.Columns) > 0 && len(m.board.Columns[m.curCol].Cards) > 0 {
			m.grabbed = true
		}
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

	// move the whole column left / right
	case "<":
		m.moveColumnBy(-1)
	case ">":
		m.moveColumnBy(1)

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
		if len(m.board.Columns) > 0 {
			m.mode = modeConfirm
			m.confirm = confirmDeleteColumn
		}
	case "s":
		m.save()
	}
	return m, nil
}

// moveColumnBy shifts the current column left (delta -1) or right (delta +1),
// carrying its scroll offset and keeping it selected.
func (m *Model) moveColumnBy(delta int) {
	to := m.curCol + delta
	if to < 0 || to >= len(m.board.Columns) {
		return
	}
	m.ensureScrollLen()
	m.board.Columns[m.curCol], m.board.Columns[to] = m.board.Columns[to], m.board.Columns[m.curCol]
	m.colScroll[m.curCol], m.colScroll[to] = m.colScroll[to], m.colScroll[m.curCol]
	m.curCol = to
	m.clampCursor()
	m.save()
}

// ensureScrollLen grows colScroll so it has an entry per column.
func (m *Model) ensureScrollLen() {
	for len(m.colScroll) < len(m.board.Columns) {
		m.colScroll = append(m.colScroll, 0)
	}
}

// handleGrabbedKey processes keys while a card is picked up: movement keys
// relocate the card, and space/enter/esc drop it.
func (m Model) handleGrabbedKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case " ", "enter", "esc":
		m.grabbed = false
	case "ctrl+c", "q":
		m.grabbed = false
		m.save()
		return m, tea.Quit

	case "left", "h", "H":
		m.grabMove(-1, 0)
	case "right", "l", "L":
		m.grabMove(1, 0)
	case "up", "k", "K":
		m.grabMove(0, -1)
	case "down", "j", "J":
		m.grabMove(0, 1)
	}
	return m, nil
}

// grabMove relocates the grabbed card by colDelta columns and/or rowDelta rows,
// keeping the cursor on the card so it can keep being moved.
func (m *Model) grabMove(colDelta, rowDelta int) {
	if len(m.board.Columns) == 0 {
		return
	}
	toCol := m.curCol + colDelta
	if toCol < 0 || toCol >= len(m.board.Columns) {
		return // at the edge; nothing to do
	}

	var toIdx int
	if colDelta != 0 {
		// switching columns: keep a similar vertical slot
		toIdx = m.curCard
		if n := len(m.board.Columns[toCol].Cards); toIdx > n {
			toIdx = n
		}
	} else {
		toIdx = m.curCard + rowDelta
		if toIdx < 0 || toIdx >= len(m.board.Columns[m.curCol].Cards) {
			return // already at top/bottom of this column
		}
	}

	nc, ni := m.board.MoveCard(m.curCol, m.curCard, toCol, toIdx)
	m.curCol, m.curCard = nc, ni
	m.clampCursor()
	m.save()
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
	nc, ni := m.board.MoveCard(m.curCol, m.curCard, toCol, toIdx)
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
	if m.curCol < len(m.colScroll) {
		m.colScroll = append(m.colScroll[:m.curCol], m.colScroll[m.curCol+1:]...)
	}
	if m.curCol >= len(m.board.Columns) {
		m.curCol = len(m.board.Columns) - 1
	}
	m.curCard = 0
	m.clampCursor()
	m.save()
}

func (m Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.mode != modeNormal {
		return m, nil
	}

	// mouse wheel scrolls whichever column the cursor is over
	if msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown {
		col := m.hitColumn(msg.X)
		if col < 0 {
			col = m.curCol
		}
		delta := 2
		if msg.Button == tea.MouseButtonWheelUp {
			delta = -2
		}
		m.setScroll(col, m.scrollFor(col)+delta)
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
		nc, ni := m.board.MoveCard(d.fromCol, d.fromIdx, d.overCol, toIdx)
		m.curCol, m.curCard = nc, ni
		m.clampCursor()
		m.save()
		return m, nil
	}
	return m, nil
}
