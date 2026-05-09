package ui

// scrollWindow describes which slice of a filtered list should be drawn
// for a given selection and viewport height.
type scrollWindow struct {
	visible        []int // raw indices that pass the filter
	selPos         int   // position of `selected` within visible (0 if not found)
	startPos       int   // first index into visible to render (inclusive)
	endPos         int   // last index into visible to render (exclusive)
	needsIndicator bool  // true when len(visible) > height
}

// computeScrollWindow filters indices [0, total) by matchSet, locates the
// position of `selected` within the filtered list, and computes the visible
// range so that the selected row stays inside a viewport of `height` rows.
//
// If matchSet is nil, every index in [0, total) is considered visible.
// `height` is clamped to a minimum of 1.
func computeScrollWindow(total int, matchSet map[int]bool, selected, height int) scrollWindow {
	if height < 1 {
		height = 1
	}

	var visible []int
	for i := 0; i < total; i++ {
		if matchSet == nil || matchSet[i] {
			visible = append(visible, i)
		}
	}

	selPos := 0
	for j, idx := range visible {
		if idx == selected {
			selPos = j
			break
		}
	}

	startPos := 0
	if selPos >= height {
		startPos = selPos - height + 1
	}
	endPos := startPos + height
	if endPos > len(visible) {
		endPos = len(visible)
	}

	return scrollWindow{
		visible:        visible,
		selPos:         selPos,
		startPos:       startPos,
		endPos:         endPos,
		needsIndicator: len(visible) > height,
	}
}
