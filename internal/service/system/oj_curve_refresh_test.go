package system

import (
	"testing"

	"personal_assistant/internal/model/entity"
)

func TestShouldRefreshLeetcodeCurve(t *testing.T) {
	t.Parallel()

	if !shouldRefreshLeetcodeCurve(nil, 10) {
		t.Fatal("expected nil leetcode detail to require refresh")
	}

	u := &entity.LeetcodeUserDetail{TotalNumber: 10}
	if shouldRefreshLeetcodeCurve(u, 10) {
		t.Fatal("expected unchanged leetcode total to skip refresh")
	}
	if !shouldRefreshLeetcodeCurve(u, 11) {
		t.Fatal("expected changed leetcode total to require refresh")
	}
}

func TestShouldRefreshLuoguCurve(t *testing.T) {
	t.Parallel()

	if !shouldRefreshLuoguCurve(nil, 5, 0) {
		t.Fatal("expected nil luogu detail to require refresh")
	}

	u := &entity.LuoguUserDetail{PassedNumber: 8}
	if shouldRefreshLuoguCurve(u, 8, 0) {
		t.Fatal("expected unchanged luogu total to skip refresh")
	}
	if !shouldRefreshLuoguCurve(u, 9, 0) {
		t.Fatal("expected changed luogu total to require refresh")
	}
	if shouldRefreshLuoguCurve(u, 0, 8) {
		t.Fatal("expected fallback passedLen with unchanged total to skip refresh")
	}
}
