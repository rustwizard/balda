package gamecoord

import (
	"testing"

	"github.com/rustwizard/balda/internal/game"
	"github.com/stretchr/testify/assert"
)

func TestFindWinnerByScore(t *testing.T) {
	cases := []struct {
		name       string
		scores     []game.PlayerState
		excludeUID string
		want       string
	}{
		{
			name: "single winner",
			scores: []game.PlayerState{
				{UID: "a", Score: 10},
				{UID: "b", Score: 5},
			},
			want: "a",
		},
		{
			name: "draw equal scores",
			scores: []game.PlayerState{
				{UID: "a", Score: 7},
				{UID: "b", Score: 7},
			},
			want: "",
		},
		{
			name: "draw three way tie",
			scores: []game.PlayerState{
				{UID: "a", Score: 3},
				{UID: "b", Score: 3},
				{UID: "c", Score: 3},
			},
			want: "",
		},
		{
			name: "exclude kicked player",
			scores: []game.PlayerState{
				{UID: "a", Score: 100},
				{UID: "b", Score: 5},
			},
			excludeUID: "a",
			want:       "b",
		},
		{
			name: "exclude not in scores",
			scores: []game.PlayerState{
				{UID: "a", Score: 10},
				{UID: "b", Score: 5},
			},
			excludeUID: "z",
			want:       "a",
		},
		{
			name: "all excluded",
			scores: []game.PlayerState{
				{UID: "a", Score: 10},
			},
			excludeUID: "a",
			want:       "",
		},
		{
			name: "three players winner by score",
			scores: []game.PlayerState{
				{UID: "a", Score: 5},
				{UID: "b", Score: 12},
				{UID: "c", Score: 8},
			},
			want: "b",
		},
		{
			name: "three players draw top two",
			scores: []game.PlayerState{
				{UID: "a", Score: 10},
				{UID: "b", Score: 10},
				{UID: "c", Score: 3},
			},
			want: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, findWinnerByScore(tc.scores, tc.excludeUID))
		})
	}
}
