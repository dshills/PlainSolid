package trimmed

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/geom"
)

func TestNativeParabolaCircleFourCrossingsCutUnrepresentableCap(t *testing.T) {
	src := parabolaCircleCurvatureStock(t, 0, big.NewRat(1, 2), big.NewRat(447, 8192), big.NewRat(42025, 262144), big.NewRat(2, 1), false)
	for order := range 2 {
		out, err := CutParabolaCircleFourCrossingsSplineGraphs(context.Background(), src[0], src[1+order], src[2-order], 0, 1e-9, ParabolaCircleGraphOptions{})
		if out != nil || !errors.Is(err, geom.ErrConvergence) {
			t.Fatal("published a partial Cut with an unresolved original cap", out, err)
		}
	}
}
