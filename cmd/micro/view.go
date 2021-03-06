package main

import (
	"github.com/atotto/clipboard"
	"github.com/gdamore/tcell"
	"io/ioutil"
	"strconv"
	"strings"
	"time"
)

// The View struct stores information about a view into a buffer.
// It has a stores information about the cursor, and the viewport
// that the user sees the buffer from.
type View struct {
	cursor Cursor

	// The topmost line, used for vertical scrolling
	topline int
	// The leftmost column, used for horizontal scrolling
	leftCol int

	// Percentage of the terminal window that this view takes up (from 0 to 100)
	widthPercent  int
	heightPercent int

	// Actual with and height
	width  int
	height int

	// How much to offset because of line numbers
	lineNumOffset int

	// The eventhandler for undo/redo
	eh *EventHandler

	// The buffer
	buf *Buffer
	// The statusline
	sline Statusline

	// Since tcell doesn't differentiate between a mouse release event
	// and a mouse move event with no keys pressed, we need to keep
	// track of whether or not the mouse was pressed (or not released) last event to determine
	// mouse release events
	mouseReleased bool

	// This stores when the last click was
	// This is useful for detecting double and triple clicks
	lastClickTime time.Time

	// Was the last mouse event actually a double click?
	// Useful for detecting triple clicks -- if a double click is detected
	// but the last mouse event was actually a double click, it's a triple click
	doubleClick bool
	// Same here, just to keep track for mouse move events
	tripleClick bool

	// Syntax highlighting matches
	matches SyntaxMatches
	// The matches from the last frame
	lastMatches SyntaxMatches

	// This is the range of lines that should have their syntax highlighting updated
	updateLines [2]int
}

// NewView returns a new fullscreen view
func NewView(buf *Buffer) *View {
	return NewViewWidthHeight(buf, 100, 100)
}

// NewViewWidthHeight returns a new view with the specified width and height percentages
// Note that w and h are percentages not actual values
func NewViewWidthHeight(buf *Buffer, w, h int) *View {
	v := new(View)

	v.buf = buf

	v.widthPercent = w
	v.heightPercent = h
	v.Resize(screen.Size())

	v.topline = 0
	// Put the cursor at the first spot
	v.cursor = Cursor{
		x: 0,
		y: 0,
		v: v,
	}
	v.cursor.ResetSelection()

	v.eh = NewEventHandler(v)

	v.sline = Statusline{
		view: v,
	}

	// Update the syntax highlighting for the entire buffer at the start
	v.UpdateLines(v.topline, v.topline+v.height)
	v.matches = Match(v)

	// Set mouseReleased to true because we assume the mouse is not being pressed when
	// the editor is opened
	v.mouseReleased = true
	v.lastClickTime = time.Time{}

	return v
}

// UpdateLines sets the values for v.updateLines
func (v *View) UpdateLines(start, end int) {
	v.updateLines[0] = start
	v.updateLines[1] = end + 1
}

// Resize recalculates the actual width and height of the view from the width and height
// percentages
// This is usually called when the window is resized, or when a split has been added and
// the percentages have changed
func (v *View) Resize(w, h int) {
	// Always include 1 line for the command line at the bottom
	h--
	v.width = int(float32(w) * float32(v.widthPercent) / 100)
	// We subtract 1 for the statusline
	v.height = int(float32(h)*float32(v.heightPercent)/100) - 1
}

// ScrollUp scrolls the view up n lines (if possible)
func (v *View) ScrollUp(n int) {
	// Try to scroll by n but if it would overflow, scroll by 1
	if v.topline-n >= 0 {
		v.topline -= n
	} else if v.topline > 0 {
		v.topline--
	}
}

// ScrollDown scrolls the view down n lines (if possible)
func (v *View) ScrollDown(n int) {
	// Try to scroll by n but if it would overflow, scroll by 1
	if v.topline+n <= len(v.buf.lines)-v.height {
		v.topline += n
	} else if v.topline < len(v.buf.lines)-v.height {
		v.topline++
	}
}

// PageUp scrolls the view up a page
func (v *View) PageUp() {
	if v.topline > v.height {
		v.ScrollUp(v.height)
	} else {
		v.topline = 0
	}
}

// PageDown scrolls the view down a page
func (v *View) PageDown() {
	if len(v.buf.lines)-(v.topline+v.height) > v.height {
		v.ScrollDown(v.height)
	} else {
		if len(v.buf.lines) >= v.height {
			v.topline = len(v.buf.lines) - v.height
		}
	}
}

// HalfPageUp scrolls the view up half a page
func (v *View) HalfPageUp() {
	if v.topline > v.height/2 {
		v.ScrollUp(v.height / 2)
	} else {
		v.topline = 0
	}
}

// HalfPageDown scrolls the view down half a page
func (v *View) HalfPageDown() {
	if len(v.buf.lines)-(v.topline+v.height) > v.height/2 {
		v.ScrollDown(v.height / 2)
	} else {
		if len(v.buf.lines) >= v.height {
			v.topline = len(v.buf.lines) - v.height
		}
	}
}

// CanClose returns whether or not the view can be closed
// If there are unsaved changes, the user will be asked if the view can be closed
// causing them to lose the unsaved changes
// The message is what to print after saying "You have unsaved changes. "
func (v *View) CanClose(msg string) bool {
	if v.buf.IsDirty() {
		quit, canceled := messenger.Prompt("You have unsaved changes. " + msg)
		if !canceled {
			if strings.ToLower(quit) == "yes" || strings.ToLower(quit) == "y" {
				return true
			}
		}
	} else {
		return true
	}
	return false
}

// Save the buffer to disk
func (v *View) Save() {
	// If this is an empty buffer, ask for a filename
	if v.buf.path == "" {
		filename, canceled := messenger.Prompt("Filename: ")
		if !canceled {
			v.buf.path = filename
			v.buf.name = filename
		} else {
			return
		}
	}
	err := v.buf.Save()
	if err != nil {
		messenger.Error(err.Error())
	} else {
		messenger.Message("Saved " + v.buf.path)
	}
}

// Copy the selection to the system clipboard
func (v *View) Copy() {
	if v.cursor.HasSelection() {
		if !clipboard.Unsupported {
			clipboard.WriteAll(v.cursor.GetSelection())
		} else {
			messenger.Error("Clipboard is not supported on your system")
		}
	}
}

// Cut the selection to the system clipboard
func (v *View) Cut() {
	if v.cursor.HasSelection() {
		if !clipboard.Unsupported {
			clipboard.WriteAll(v.cursor.GetSelection())
			v.cursor.DeleteSelection()
			v.cursor.ResetSelection()
		} else {
			messenger.Error("Clipboard is not supported on your system")
		}
	}
}

// Paste whatever is in the system clipboard into the buffer
// Delete and paste if the user has a selection
func (v *View) Paste() {
	if !clipboard.Unsupported {
		if v.cursor.HasSelection() {
			v.cursor.DeleteSelection()
			v.cursor.ResetSelection()
		}
		clip, _ := clipboard.ReadAll()
		v.eh.Insert(v.cursor.Loc(), clip)
		v.cursor.SetLoc(v.cursor.Loc() + Count(clip))
	} else {
		messenger.Error("Clipboard is not supported on your system")
	}
}

// SelectAll selects the entire buffer
func (v *View) SelectAll() {
	v.cursor.curSelection[1] = 0
	v.cursor.curSelection[0] = v.buf.Len()
	// Put the cursor at the beginning
	v.cursor.x = 0
	v.cursor.y = 0
}

// OpenFile opens a new file in the current view
// It makes sure that the current buffer can be closed first (unsaved changes)
func (v *View) OpenFile() {
	if v.CanClose("Continue? ") {
		filename, canceled := messenger.Prompt("File to open: ")
		if canceled {
			return
		}
		file, err := ioutil.ReadFile(filename)

		if err != nil {
			messenger.Error(err.Error())
			return
		}
		v.buf = NewBuffer(string(file), filename)
	}
}

// Relocate moves the view window so that the cursor is in view
// This is useful if the user has scrolled far away, and then starts typing
func (v *View) Relocate() bool {
	ret := false
	cy := v.cursor.y
	if cy < v.topline {
		v.topline = cy
		ret = true
	}
	if cy > v.topline+v.height-1 {
		v.topline = cy - v.height + 1
		ret = true
	}

	cx := v.cursor.GetVisualX()
	if cx < v.leftCol {
		v.leftCol = cx
		ret = true
	}
	if cx+v.lineNumOffset+1 > v.leftCol+v.width {
		v.leftCol = cx - v.width + v.lineNumOffset + 1
		ret = true
	}
	return ret
}

// MoveToMouseClick moves the cursor to location x, y assuming x, y were given
// by a mouse click
func (v *View) MoveToMouseClick(x, y int) {
	if y-v.topline > v.height-1 {
		v.ScrollDown(1)
		y = v.height + v.topline - 1
	}
	if y >= len(v.buf.lines) {
		y = len(v.buf.lines) - 1
	}
	if y < 0 {
		y = 0
	}
	if x < 0 {
		x = 0
	}

	x = v.cursor.GetCharPosInLine(y, x)
	if x > Count(v.buf.lines[y]) {
		x = Count(v.buf.lines[y])
	}
	v.cursor.x = x
	v.cursor.y = y
	v.cursor.lastVisualX = v.cursor.GetVisualX()
}

// HandleEvent handles an event passed by the main loop
func (v *View) HandleEvent(event tcell.Event) {
	// This bool determines whether the view is relocated at the end of the function
	// By default it's true because most events should cause a relocate
	relocate := true

	// By default we don't update and syntax highlighting
	v.UpdateLines(-2, 0)
	switch e := event.(type) {
	case *tcell.EventResize:
		// Window resized
		v.Resize(e.Size())
	case *tcell.EventKey:
		switch e.Key() {
		case tcell.KeyUp:
			// Cursor up
			v.cursor.ResetSelection()
			v.cursor.Up()
		case tcell.KeyDown:
			// Cursor down
			v.cursor.ResetSelection()
			v.cursor.Down()
		case tcell.KeyLeft:
			// Cursor left
			v.cursor.ResetSelection()
			v.cursor.Left()
		case tcell.KeyRight:
			// Cursor right
			v.cursor.ResetSelection()
			v.cursor.Right()
		case tcell.KeyEnter:
			// Insert a newline
			if v.cursor.HasSelection() {
				v.cursor.DeleteSelection()
				v.cursor.ResetSelection()
			}
			v.eh.Insert(v.cursor.Loc(), "\n")
			v.cursor.Right()
			// Rehighlight the entire buffer
			v.UpdateLines(v.topline, v.topline+v.height)
			v.cursor.lastVisualX = v.cursor.GetVisualX()
			// v.UpdateLines(v.cursor.y-1, v.cursor.y)
		case tcell.KeySpace:
			// Insert a space
			if v.cursor.HasSelection() {
				v.cursor.DeleteSelection()
				v.cursor.ResetSelection()
			}
			v.eh.Insert(v.cursor.Loc(), " ")
			v.cursor.Right()
			v.UpdateLines(v.cursor.y, v.cursor.y)
		case tcell.KeyBackspace2, tcell.KeyBackspace:
			// Delete a character
			if v.cursor.HasSelection() {
				v.cursor.DeleteSelection()
				v.cursor.ResetSelection()
				// Rehighlight the entire buffer
				v.UpdateLines(v.topline, v.topline+v.height)
			} else if v.cursor.Loc() > 0 {
				// We have to do something a bit hacky here because we want to
				// delete the line by first moving left and then deleting backwards
				// but the undo redo would place the cursor in the wrong place
				// So instead we move left, save the position, move back, delete
				// and restore the position
				v.cursor.Left()
				cx, cy := v.cursor.x, v.cursor.y
				v.cursor.Right()
				loc := v.cursor.Loc()
				v.eh.Remove(loc-1, loc)
				v.cursor.x, v.cursor.y = cx, cy
				// Rehighlight the entire buffer
				v.UpdateLines(v.topline, v.topline+v.height)
				// v.UpdateLines(v.cursor.y, v.cursor.y+1)
			}
			v.cursor.lastVisualX = v.cursor.GetVisualX()
		case tcell.KeyTab:
			// Insert a tab
			if v.cursor.HasSelection() {
				v.cursor.DeleteSelection()
				v.cursor.ResetSelection()
			}
			if settings.TabsToSpaces {
				v.eh.Insert(v.cursor.Loc(), Spaces(settings.TabSize))
				for i := 0; i < settings.TabSize; i++ {
					v.cursor.Right()
				}
			} else {
				v.eh.Insert(v.cursor.Loc(), "\t")
				v.cursor.Right()
			}
			v.UpdateLines(v.cursor.y, v.cursor.y)
		case tcell.KeyCtrlS:
			v.Save()
		case tcell.KeyCtrlF:
			if v.cursor.HasSelection() {
				searchStart = v.cursor.curSelection[1]
			} else {
				searchStart = ToCharPos(v.cursor.x, v.cursor.y, v.buf)
			}
			BeginSearch()
		case tcell.KeyCtrlN:
			if v.cursor.HasSelection() {
				searchStart = v.cursor.curSelection[1]
			} else {
				searchStart = ToCharPos(v.cursor.x, v.cursor.y, v.buf)
			}
			messenger.Message("Find: " + lastSearch)
			Search(lastSearch, v, true)
		case tcell.KeyCtrlP:
			if v.cursor.HasSelection() {
				searchStart = v.cursor.curSelection[0]
			} else {
				searchStart = ToCharPos(v.cursor.x, v.cursor.y, v.buf)
			}
			messenger.Message("Find: " + lastSearch)
			Search(lastSearch, v, false)
		case tcell.KeyCtrlZ:
			v.eh.Undo()
			// Rehighlight the entire buffer
			v.UpdateLines(v.topline, v.topline+v.height)
		case tcell.KeyCtrlY:
			v.eh.Redo()
			// Rehighlight the entire buffer
			v.UpdateLines(v.topline, v.topline+v.height)
		case tcell.KeyCtrlC:
			v.Copy()
			// Rehighlight the entire buffer
			v.UpdateLines(v.topline, v.topline+v.height)
		case tcell.KeyCtrlX:
			v.Cut()
			// Rehighlight the entire buffer
			v.UpdateLines(v.topline, v.topline+v.height)
		case tcell.KeyCtrlV:
			v.Paste()
			// Rehighlight the entire buffer
			v.UpdateLines(v.topline, v.topline+v.height)
		case tcell.KeyCtrlA:
			v.SelectAll()
		case tcell.KeyCtrlO:
			v.OpenFile()
			// Rehighlight the entire buffer
			v.UpdateLines(v.topline, v.topline+v.height)
		case tcell.KeyHome:
			v.topline = 0
			relocate = false
		case tcell.KeyEnd:
			if v.height > len(v.buf.lines) {
				v.topline = 0
			} else {
				v.topline = len(v.buf.lines) - v.height
			}
			relocate = false
		case tcell.KeyPgUp:
			v.PageUp()
			relocate = false
		case tcell.KeyPgDn:
			v.PageDown()
			relocate = false
		case tcell.KeyCtrlU:
			v.HalfPageUp()
			relocate = false
		case tcell.KeyCtrlD:
			v.HalfPageDown()
			relocate = false
		case tcell.KeyRune:
			// Insert a character
			if v.cursor.HasSelection() {
				v.cursor.DeleteSelection()
				v.cursor.ResetSelection()
				// Rehighlight the entire buffer
				v.UpdateLines(v.topline, v.topline+v.height)
			} else {
				v.UpdateLines(v.cursor.y, v.cursor.y)
			}
			v.eh.Insert(v.cursor.Loc(), string(e.Rune()))
			v.cursor.Right()
		}
	case *tcell.EventMouse:
		x, y := e.Position()
		x -= v.lineNumOffset - v.leftCol
		y += v.topline
		// Position always seems to be off by one
		x--
		y--

		button := e.Buttons()

		switch button {
		case tcell.Button1:
			// Left click
			origX, origY := v.cursor.x, v.cursor.y
			v.MoveToMouseClick(x, y)

			if v.mouseReleased {
				if (time.Since(v.lastClickTime)/time.Millisecond < doubleClickThreshold) &&
					(origX == v.cursor.x && origY == v.cursor.y) {
					if v.doubleClick {
						// Triple click
						v.lastClickTime = time.Now()

						v.tripleClick = true
						v.doubleClick = false

						v.cursor.SelectLine()
					} else {
						// Double click
						v.lastClickTime = time.Now()

						v.doubleClick = true
						v.tripleClick = false

						v.cursor.SelectWord()
					}
				} else {
					v.doubleClick = false
					v.tripleClick = false
					v.lastClickTime = time.Now()

					loc := v.cursor.Loc()
					v.cursor.curSelection[0] = loc
					v.cursor.curSelection[1] = loc
				}
			} else {
				if v.tripleClick {
					v.cursor.AddLineToSelection()
				} else if v.doubleClick {
					v.cursor.AddWordToSelection()
				} else {
					v.cursor.curSelection[1] = v.cursor.Loc()
				}
			}
			v.mouseReleased = false
		case tcell.ButtonNone:
			// Mouse event with no click
			if !v.mouseReleased {
				// Mouse was just released

				// Relocating here isn't really necessary because the cursor will
				// be in the right place from the last mouse event
				// However, if we are running in a terminal that doesn't support mouse motion
				// events, this still allows the user to make selections, except only after they
				// release the mouse

				if !v.doubleClick && !v.tripleClick {
					v.MoveToMouseClick(x, y)
					v.cursor.curSelection[1] = v.cursor.Loc()
				}
				v.mouseReleased = true
			}
			// We don't want to relocate because otherwise the view will be relocated
			// every time the user moves the cursor
			relocate = false
		case tcell.WheelUp:
			// Scroll up two lines
			v.ScrollUp(2)
			// We don't want to relocate if the user is scrolling
			relocate = false
			// Rehighlight the entire buffer
			v.UpdateLines(v.topline, v.topline+v.height)
		case tcell.WheelDown:
			// Scroll down two lines
			v.ScrollDown(2)
			// We don't want to relocate if the user is scrolling
			relocate = false
			// Rehighlight the entire buffer
			v.UpdateLines(v.topline, v.topline+v.height)
		}
	}

	if relocate {
		v.Relocate()
	}
	if settings.Syntax {
		v.matches = Match(v)
	}
}

// DisplayView renders the view to the screen
func (v *View) DisplayView() {
	// matches := make(SyntaxMatches, len(v.buf.lines))
	//
	// viewStart := v.topline
	// viewEnd := v.topline + v.height
	// if viewEnd > len(v.buf.lines) {
	// 	viewEnd = len(v.buf.lines)
	// }
	//
	// lines := v.buf.lines[viewStart:viewEnd]
	// for i, line := range lines {
	// 	matches[i] = make([]tcell.Style, len(line))
	// }

	// The character number of the character in the top left of the screen

	charNum := ToCharPos(0, v.topline, v.buf)

	// Convert the length of buffer to a string, and get the length of the string
	// We are going to have to offset by that amount
	maxLineLength := len(strconv.Itoa(len(v.buf.lines)))
	// + 1 for the little space after the line number
	v.lineNumOffset = maxLineLength + 1

	var highlightStyle tcell.Style

	for lineN := 0; lineN < v.height; lineN++ {
		var x int
		// If the buffer is smaller than the view height
		// and we went too far, break
		if lineN+v.topline >= len(v.buf.lines) {
			break
		}
		line := v.buf.lines[lineN+v.topline]

		// Write the line number
		lineNumStyle := defStyle
		if style, ok := colorscheme["line-number"]; ok {
			lineNumStyle = style
		}
		// Write the spaces before the line number if necessary
		lineNum := strconv.Itoa(lineN + v.topline + 1)
		for i := 0; i < maxLineLength-len(lineNum); i++ {
			screen.SetContent(x, lineN, ' ', nil, lineNumStyle)
			x++
		}
		// Write the actual line number
		for _, ch := range lineNum {
			screen.SetContent(x, lineN, ch, nil, lineNumStyle)
			x++
		}
		// Write the extra space
		screen.SetContent(x, lineN, ' ', nil, lineNumStyle)
		x++

		// Write the line
		tabchars := 0
		runes := []rune(line)
		for colN := v.leftCol; colN < v.leftCol+v.width; colN++ {
			if colN >= len(runes) {
				break
			}
			ch := runes[colN]
			var lineStyle tcell.Style
			// Does the current character need to be syntax highlighted?

			// if lineN >= v.updateLines[0] && lineN < v.updateLines[1] {
			if settings.Syntax {
				highlightStyle = v.matches[lineN][colN]
			}
			// } else if lineN < len(v.lastMatches) && colN < len(v.lastMatches[lineN]) {
			// highlightStyle = v.lastMatches[lineN][colN]
			// } else {
			// highlightStyle = defStyle
			// }

			if v.cursor.HasSelection() &&
				(charNum >= v.cursor.curSelection[0] && charNum < v.cursor.curSelection[1] ||
					charNum < v.cursor.curSelection[0] && charNum >= v.cursor.curSelection[1]) {

				lineStyle = defStyle.Reverse(true)

				if style, ok := colorscheme["selection"]; ok {
					lineStyle = style
				}
			} else {
				lineStyle = highlightStyle
			}
			// matches[lineN][colN] = highlightStyle

			if ch == '\t' {
				screen.SetContent(x+tabchars, lineN, ' ', nil, lineStyle)
				tabSize := settings.TabSize
				for i := 0; i < tabSize-1; i++ {
					tabchars++
					if x-v.leftCol+tabchars >= v.lineNumOffset {
						screen.SetContent(x-v.leftCol+tabchars, lineN, ' ', nil, lineStyle)
					}
				}
			} else {
				if x-v.leftCol+tabchars >= v.lineNumOffset {
					screen.SetContent(x-v.leftCol+tabchars, lineN, ch, nil, lineStyle)
				}
			}
			charNum++
			x++
		}
		// Here we are at a newline

		// The newline may be selected, in which case we should draw the selection style
		// with a space to represent it
		if v.cursor.HasSelection() &&
			(charNum >= v.cursor.curSelection[0] && charNum < v.cursor.curSelection[1] ||
				charNum < v.cursor.curSelection[0] && charNum >= v.cursor.curSelection[1]) {

			selectStyle := defStyle.Reverse(true)

			if style, ok := colorscheme["selection"]; ok {
				selectStyle = style
			}
			screen.SetContent(x-v.leftCol+tabchars, lineN, ' ', nil, selectStyle)
		}

		charNum++
	}
	// v.lastMatches = matches
}

// Display renders the view, the cursor, and statusline
func (v *View) Display() {
	v.DisplayView()
	v.cursor.Display()
	v.sline.Display()
}
