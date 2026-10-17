package main

import (
	"github.com/egoist/mygo/ui"
)

// dropDown is a select whose list of choices scrolls, for the lists that
// are longer than the window: the themes, the languages, the pinyin
// keyboards, the log levels.
//
// mygo's own Select draws its choices in an overlay beside the window, and
// an overlay takes no wheel: a list inside one cannot be scrolled with the
// wheel, whatever the app builds. The list here takes the wheel itself,
// and keeps where it is scrolled in a state of its own, which also lets
// the arrow keys move through it: the highlight follows them and the list
// follows the highlight.

// dropDownRows is how many choices are shown at once, after which the list
// scrolls.
const dropDownRows = 8

// dropDownRowHeight is how tall one choice is.
const dropDownRowHeight = 30

// dropDownStates are the places the drop-down lists are scrolled to, by
// the name of the list, so that a list stays where the user left it
// between frames.
var dropDownStates = map[string]*ui.ScrollState{}

// dropDownState is where a list of choices is scrolled to.
func dropDownState(name string) *ui.ScrollState {
	if state, ok := dropDownStates[name]; ok {
		return state
	}
	state := &ui.ScrollState{}
	dropDownStates[name] = state
	return state
}

// dropDown draws a select of choices into *selected, calling changed when
// the user chooses one.
func dropDown(c *ui.Context, name string, selected *string, choices []string, changed func()) {
	sel := ui.SelectBase(c, selected)
	sel.Trigger.Width(220).Padding(6, 10).Radius(8).
		Border(1, c.Theme().Border).Background(c.Theme().Surface)
	sel.Trigger.Children(func() {
		ui.Text(c, *selected).SingleLine().Grow(1)
		// The mark of a list of choices, which the select of mygo draws itself
		// and this one has to draw for itself.
		ui.Icon(c, chevronIcon()).FontSize(12).TextColor(c.Theme().TextMuted)
	})

	state := dropDownState(name)
	rows := len(choices)
	if rows > dropDownRows {
		rows = dropDownRows
	}

	sel.Popup(func(panel ui.Element) {
		panel.Padding(4).Radius(cardRadius).Border(1, c.Theme().Border).Background(c.Theme().Background)

		list := ui.Scroll(c).
			Height(float32(rows) * dropDownRowHeight).
			TrackScroll(state)
		// The wheel, which an overlay does not hand to the list itself:
		// the list takes it and moves where it is scrolled to.
		list.HandleInput(func(event ui.InputEvent) bool {
			if event.Kind != ui.InputScroll || event.DY == 0 {
				return false
			}
			state.Y = clampScroll(state.Y+event.DY, state.MaxY)
			return true
		})
		list.Children(func() {
			ui.Column(c).Gap(2).Children(func() {
				for _, choice := range choices {
					item := sel.Item(choice).Padding(6, 10).Radius(rowRadius - 2).Height(dropDownRowHeight)
					switch {
					case item.Highlighted():
						item.Background(selectionTint(c.Theme())).TextColor(c.Theme().Text)
					case choice == *selected:
						item.Background(c.Theme().Surface)
					}
					item.Children(func() {
						ui.Text(c, choice).SingleLine()
					})
				}
			})
		})
	})

	if sel.Trigger.Changed() && changed != nil {
		changed()
	}
}

// clampScroll keeps a place within the content it is scrolled through.
func clampScroll(y, maxY float32) float32 {
	if y < 0 {
		return 0
	}
	if maxY > 0 && y > maxY {
		return maxY
	}
	return y
}
