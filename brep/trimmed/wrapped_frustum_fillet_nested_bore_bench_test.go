package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

// Measures exact rejection cases without including native preparation. Neither
// an exact band endpoint nor an equal singleton distance is a strict winner.
func BenchmarkNestedBoreUnresolved(b *testing.B) {
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	w, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, 8, 6, o)
	if e != nil {
		b.Fatal(e)
	}
	w, _, e = geom.DraftWrappedFrustum(ctx, w, 0, 8, o)
	if e != nil {
		b.Fatal(e)
	}
	s, _, e := NewWrappedFrustum(ctx, w, 1e-9, o)
	if e != nil {
		b.Fatal(e)
	}
	q, e := PrepareFilletedFrustumMaterial(ctx, s, 15./16, 15./8, []FilletedFrustumBoreCut{{Center: geom.Vec2{X: 6}, Radius: 1}}, o)
	if e != nil {
		b.Fatal(e)
	}
	r := big.NewRat
	for _, tc := range []struct {
		name string
		p    [3]*big.Rat
	}{{"endpoint", [3]*big.Rat{r(63, 8), r(0, 1), r(11, 16)}}, {"tie", [3]*big.Rat{r(991, 128), r(0, 1), r(3, 4)}}} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			var got FilletedFrustumNestedBoreFaces
			for i := 0; i < b.N; i++ {
				var e error
				got, e = q.ClassifyNestedTorusBoreFacesRational(ctx, tc.p, FilletedFrustumNestedBoreOptions{})
				if e != nil || got.Complete || got.Lower == nil || got.RefinementChecks > 6 {
					b.Fatal(got, e)
				}
			}
			b.ReportMetric(float64(got.RefinementChecks), "refinements/op")
		})
	}
}
