package shape

import "testing"

// TestLooksCore reaches research code only through helpers in two other files.
func TestLooksCore(t *testing.T) {
	if viaA() != 6 {
		t.Fatal("chain")
	}
}
