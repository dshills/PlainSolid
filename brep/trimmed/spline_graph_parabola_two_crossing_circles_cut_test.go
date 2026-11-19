package trimmed

import (
	"context"
	"errors"
	"testing"

	"plainsolid/geom"
)

func TestNativeParabolaTwoCrossingCirclesCutRefusal(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name string
		f    parabolaTwoCrossingsSourceFixture
		opts ParabolaCircleGraphOptions
		want error
	}{
		{"nonbinary original junction", parabolaTwoCrossingCirclesCoincidentProjection(), ParabolaCircleGraphOptions{}, geom.ErrUnrepresentable},
		{"short exact budget", parabolaTwoCrossingCirclesStaggered(), ParabolaCircleGraphOptions{Proof: geom.GraphPointContactOptions{MaxOperations: 1}}, geom.ErrConvergence},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := tc.f.stocks(t, 0, false)
			r, err := CutParabolaTwoCrossingCirclesSplineGraphs(ctx, src[0], src[1], src[2], 0, 1e-9, tc.opts)
			if r != nil || !errors.Is(err, tc.want) {
				t.Fatal("published Cut with an unresolved source", r, err)
			}
		})
	}
}
