package ui

import (
	"reflect"
	"testing"
)

func TestComputeScrollWindow(t *testing.T) {
	tests := []struct {
		name     string
		total    int
		matchSet map[int]bool
		selected int
		height   int
		want     scrollWindow
	}{
		{
			name:   "empty list",
			total:  0,
			height: 5,
			want:   scrollWindow{visible: nil, selPos: 0, startPos: 0, endPos: 0, needsIndicator: false},
		},
		{
			name:     "list shorter than window",
			total:    3,
			selected: 1,
			height:   5,
			want:     scrollWindow{visible: []int{0, 1, 2}, selPos: 1, startPos: 0, endPos: 3, needsIndicator: false},
		},
		{
			name:     "selection within first window",
			total:    10,
			selected: 2,
			height:   5,
			want:     scrollWindow{visible: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}, selPos: 2, startPos: 0, endPos: 5, needsIndicator: true},
		},
		{
			name:     "selection scrolls window down",
			total:    10,
			selected: 7,
			height:   5,
			want:     scrollWindow{visible: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}, selPos: 7, startPos: 3, endPos: 8, needsIndicator: true},
		},
		{
			name:     "selection at end of list",
			total:    10,
			selected: 9,
			height:   5,
			want:     scrollWindow{visible: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}, selPos: 9, startPos: 5, endPos: 10, needsIndicator: true},
		},
		{
			name:     "matchSet filters indices",
			total:    6,
			matchSet: map[int]bool{1: true, 3: true, 5: true},
			selected: 3,
			height:   5,
			want:     scrollWindow{visible: []int{1, 3, 5}, selPos: 1, startPos: 0, endPos: 3, needsIndicator: false},
		},
		{
			name:     "matchSet with no matches",
			total:    5,
			matchSet: map[int]bool{},
			selected: 0,
			height:   5,
			want:     scrollWindow{visible: nil, selPos: 0, startPos: 0, endPos: 0, needsIndicator: false},
		},
		{
			name:     "selected not in visible defaults to selPos 0",
			total:    5,
			matchSet: map[int]bool{2: true, 4: true},
			selected: 0,
			height:   5,
			want:     scrollWindow{visible: []int{2, 4}, selPos: 0, startPos: 0, endPos: 2, needsIndicator: false},
		},
		{
			name:     "height clamped to 1",
			total:    3,
			selected: 2,
			height:   0,
			want:     scrollWindow{visible: []int{0, 1, 2}, selPos: 2, startPos: 2, endPos: 3, needsIndicator: true},
		},
		{
			name:     "negative height clamped to 1",
			total:    3,
			selected: 1,
			height:   -3,
			want:     scrollWindow{visible: []int{0, 1, 2}, selPos: 1, startPos: 1, endPos: 2, needsIndicator: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeScrollWindow(tt.total, tt.matchSet, tt.selected, tt.height)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}
