package main

import (
	"github.com/gdamore/tcell"
	"strings"
)

const helpTxt = `Press Ctrl-q to quit help

Micro keybindings:

Ctrl-q:   Quit
Ctrl-s:   Save
Ctrl-o:   Open file

Ctrl-z:   Undo
Ctrl-y:   Redo

Ctrl-f:   Find
Ctrl-n:   Find next
Ctrl-p:   Find previous

Ctrl-a:   Select all

Ctrl-c:   Copy
Ctrl-x:   Cut
Ctrl-v:   Paste

Ctrl-g:   Open this help screen

Ctrl-u:   Half page up
Ctrl-d:   Half page down
PageUp:   Page up
PageDown: Page down

Home:     Go to beginning
End:      Go to end

Ctrl-e:   Execute a command

Possible commands:

'quit': Quits micro
'save': saves the current buffer

'replace "search" "value"': This will replace 'search' with 'value'.
Note that 'search' must be a valid regex.  If one of the arguments
does not have any spaces in it, you may omit the quotes.

'set option value': sets the option to value. Please see the next section for a list of options you can set

Micro options:

Configuration directory:

Micro uses the $XDG_CONFIG_HOME/micro as the configuration directory. As per the XDG spec,
if $XDG_CONFIG_HOME is not set, ~/.config/micro is used as the config directory.

colorscheme: loads the colorscheme stored in $(configDir)/colorschemes/'option'.micro
	default value: 'default'
	Note that the default colorschemes (default, solarized, and solarized-tc) are not located in configDir,
	because they are embedded in the micro binary

tabsize: sets the tab size to 'option'
	default value: '4'

syntax: turns syntax on or off
	default value: 'on'

tabsToSpaces: use spaces instead of tabs
	default value: 'off'
`

// DisplayHelp displays the help txt
// It blocks the main loop
func DisplayHelp() {
	topline := 0
	_, height := screen.Size()
	screen.HideCursor()
	totalLines := strings.Split(helpTxt, "\n")
	for {
		screen.Clear()

		lineEnd := topline + height
		if lineEnd > len(totalLines) {
			lineEnd = len(totalLines)
		}
		lines := totalLines[topline:lineEnd]
		for y, line := range lines {
			for x, ch := range line {
				st := defStyle
				screen.SetContent(x, y, ch, nil, st)
			}
		}

		screen.Show()

		event := screen.PollEvent()
		switch e := event.(type) {
		case *tcell.EventResize:
			_, height = e.Size()
		case *tcell.EventKey:
			switch e.Key() {
			case tcell.KeyUp:
				if topline > 0 {
					topline--
				}
			case tcell.KeyDown:
				if topline < len(totalLines)-height {
					topline++
				}
			case tcell.KeyCtrlQ, tcell.KeyCtrlW, tcell.KeyEscape, tcell.KeyCtrlC:
				return
			}
		}
	}
}
