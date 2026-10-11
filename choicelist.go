package main

import (
	"github.com/egoist/mygo/ui"
)

// The settings pages offer their choices as a list rather than a drop-down
// where the list is long: a drop-down's list is drawn in an overlay beside
// the window, and mygo's overlays take the wheel nowhere — a scroll box
// inside one cannot be scrolled with the wheel, whatever the app builds.
// The themes, the languages and the pinyin keyboards are long enough to
// need one, so they are shown as what they are: a list of choices, in the
// page, with a scroll box that works.

// choiceListHeight is how tall a list of choices is at most, after which it
// scrolls.
const choiceListHeight = 200

// choiceListStates are the scroll states of the lists of choices, by the
// name of the list, so that a list keeps where it was scrolled between
// frames.
var choiceListStates = map[string]*ui.ScrollState{}

// choiceListState is the scroll state of a list of choices.
func choiceListState(name string) *ui.ScrollState {
	if state, ok := choiceListStates[name]; ok {
		return state
	}
	state := &ui.ScrollState{}
	choiceListStates[name] = state
	return state
}

// choiceListDebug, when set, is given each list as it is built, so that a
// test can watch what the list does: where it is scrolled to, and what it
// holds.
var choiceListDebug func(ui.Element)

// choiceList is a list of choices in the page itself, one of which is
// chosen: clicking a choice chooses it, and the list scrolls.
//
// The box is given both a height and a maximum, and told not to stretch:
// a row lays its children out stretched, which would make the box as tall
// as the row and leave nothing to scroll.
func choiceList(c *ui.Context, name string, choices []string, chosen string, choose func(string)) {
	scroll := ui.Scroll(c).Grow(1).MaxHeight(choiceListHeight)
	if name != "" {
		scroll = scroll.TrackScroll(choiceListState(name))
	}
	if choiceListDebug != nil {
		choiceListDebug(scroll)
	}
	scroll.Children(func() {
		ui.Column(c).Gap(2).Children(func() {
			for _, choice := range choices {
				choice := choice
				row := ui.Row(c).Padding(6, 10).Radius(6).Gap(8).AlignItems(ui.Center)
				switch {
				case choice == chosen:
					row.Background(c.Theme().Accent)
				default:
					row.Background(c.Theme().Surface)
				}
				row.Children(func() {
					// A mark shows which one is chosen, so that the
					// choice is not only a colour.
					if choice == chosen {
						ui.Text(c, "✓").Width(14)
					} else {
						ui.Box(c).Size(14, 14)
					}
					ui.Text(c, choice).SingleLine().Grow(1)
				})
				row.OnClick(func() {
					if choose != nil {
						choose(choice)
					}
				})
			}
		})
	})
}
