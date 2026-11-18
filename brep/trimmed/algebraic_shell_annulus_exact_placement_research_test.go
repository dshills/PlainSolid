//go:build research

package trimmed

import (
	"plainsolid/internal/slowtest"
	"testing"
)

func TestCertifiedExactPlacedXZBooleanAnnulusFill(t *testing.T) {
	slowtest.Skip(t)
	checkCertifiedBooleanAnnulusOwnerFill(t, 2, true)
}
