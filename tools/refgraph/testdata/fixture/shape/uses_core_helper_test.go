package shape

import "testing"

func TestMk(t *testing.T) {
	if mk().n != 2 {
		t.Fatal("helper")
	}
}
