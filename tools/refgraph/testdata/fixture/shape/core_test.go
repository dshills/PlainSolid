package shape

import "testing"

func TestNew(t *testing.T) {
	if New().n != 2 {
		t.Fatal("scale")
	}
}
