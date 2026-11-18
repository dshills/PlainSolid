//go:build research

package trimmed

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestExactBezierTrimModelPlaneAndLine(t *testing.T) {
	ctx := context.Background()
	stock := rationalBezierGraphStock(t, 4)
	tool, _ := makeOpenSplineBox(t, [3]float64{-1, -1, -2}, [3]float64{2, 2, 3. / 16}, 0)
	r, e := CombineRationalGraphSplinePlaneClip(ctx, stock, tool, 5, parametric.Intersection, SplineRectangleEmbeddingOptions{})
	if e != nil {
		t.Fatal(e)
	}
	bottom := r.Solid.faces[r.StockFaces[4][0]]
	patch, e := geom.NewSurfacePatch(bottom.Surface.(*geom.BSplineSurface), geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var q ExactBezierUV
	for _, u := range bottom.Outer {
		if trim, ok := u.Curve.(ExactBezierUV); ok {
			q = trim
		}
	}
	compose := geom.SurfaceCurveAgreementOptions{}
	// Compare with the actual cutting plane and then an independent plane.
	qplane := [4]*big.Rat{new(big.Rat), new(big.Rat), big.NewRat(16, 1), big.NewRat(-3, 1)}
	section, e := q.IntersectModelRawPlane(ctx, patch, qplane, compose, geom.ExactCurvePlaneEventOptions{})
	if e != nil || len(section.Hits) != 0 || len(section.Coincident) != 1 ||
		section.Coincident[0].Start.Interval().Lower.Sign() != 0 || section.Coincident[0].End.Interval().Upper.Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal("lost exact cutter-plane coincidence", section, e)
	}
	xplane := [4]*big.Rat{big.NewRat(2, 1), new(big.Rat), new(big.Rat), big.NewRat(-1, 1)}
	section, e = q.IntersectModelRawPlane(ctx, patch, xplane, compose, geom.ExactCurvePlaneEventOptions{})
	if e != nil || len(section.Hits) != 1 || len(section.Coincident) != 0 {
		t.Fatal("isolated transverse plane contact", section, e)
	}
	point, e := section.Hits[0].Event.Enclose(ctx, geom.ComposedCurveEndpointOptions{})
	if e != nil || !point.Converged || point.Bounds.Min.Y > 47./96 || point.Bounds.Max.Y < 47./96 {
		t.Fatal("missing exact plane event image", point, e)
	}
	line, e := q.IntersectModelFiniteLine(ctx, patch, geom.Vec3{X: .5, Z: 3. / 16}, geom.Vec3{Y: 2},
		[2]*big.Rat{big.NewRat(1, 5), big.NewRat(1, 4)}, compose, geom.ComposedCurveLineOptions{})
	if e != nil || len(line.Hits) != 1 || len(line.Coincident) != 0 {
		t.Fatal("finite line misses algebraic image", line, e)
	}
	line, e = q.IntersectModelFiniteLine(ctx, patch, geom.Vec3{X: .5, Z: 3. / 16}, geom.Vec3{Y: 2},
		[2]*big.Rat{new(big.Rat), big.NewRat(1, 5)}, compose, geom.ComposedCurveLineOptions{})
	if e != nil || len(line.Hits) != 0 || len(line.Coincident) != 0 {
		t.Fatal("contact outside closed line range", line, e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = q.IntersectModelRawPlane(canceled, patch, qplane, compose, geom.ExactCurvePlaneEventOptions{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
