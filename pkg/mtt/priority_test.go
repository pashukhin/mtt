package mtt

import "testing"

func TestPriorityValid(t *testing.T) {
	cases := []struct {
		p    Priority
		want bool
	}{
		{"", true}, // unset is valid
		{PriorityHigh, true},
		{PriorityMedium, true},
		{PriorityLow, true},
		{"urgent", false},
		{"HIGH", false}, // case-sensitive
	}
	for _, c := range cases {
		if got := c.p.Valid(); got != c.want {
			t.Errorf("Priority(%q).Valid() = %v, want %v", c.p, got, c.want)
		}
	}
}

func TestPriorityRank(t *testing.T) {
	cases := []struct {
		p    Priority
		want int
	}{
		{PriorityHigh, 0},
		{PriorityMedium, 1},
		{PriorityLow, 2},
		{"", 1},        // unset ranks as medium
		{"garbage", 1}, // unknown ranks as medium (tolerated)
	}
	for _, c := range cases {
		if got := c.p.Rank(); got != c.want {
			t.Errorf("Priority(%q).Rank() = %d, want %d", c.p, got, c.want)
		}
	}
	// Ordering invariant: high < medium < low.
	if PriorityHigh.Rank() >= PriorityMedium.Rank() || PriorityMedium.Rank() >= PriorityLow.Rank() {
		t.Fatal("want Rank ordering high < medium < low")
	}
}
