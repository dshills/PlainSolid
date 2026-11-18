package trimmed

import (
	"testing"
)

func TestDualDiskRimScheduleLimits(t *testing.T) {
	for _, s := range [][]int{{0}, {-1}, {2, 1}, {2, 2}, {257}, {64, 128, 192, 256}, {1, 2, 3, 4, 5, 6, 7, 8, 9, 10}} {
		if _, e := dualDiskRimSchedule(s, 256); e == nil {
			t.Fatal(s)
		}
	}
	if _, e := dualDiskRimSchedule([]int{96}, 64); e == nil {
		t.Fatal("interval cap")
	}
	s := []int{64, 96}
	r, e := dualDiskRimSchedule(s, 256)
	if e != nil {
		t.Fatal(e)
	}
	s[0] = 1
	if r[0] != 64 {
		t.Fatal("caller alias")
	}
}
