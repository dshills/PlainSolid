package trimmed

import (
	"context"
	"errors"
	"math/big"
	"slices"
	"testing"

	"plainsolid/geom"
)

func TestWrappedFrustumOpenBoreLine(t *testing.T) {
	ctx := context.Background()
	stock, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, 2, 4, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	stock, _, e = geom.DraftWrappedFrustum(ctx, stock, 0, 3, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	tool, _, e := geom.InsetWrappedCircularFrustumAtCap(ctx, stock, .5, true, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	o := geom.ToroidLineOptions{ParameterTolerance: 1e-8}
	checkRoot := func(r geom.ToroidLineRoot, n, d int64) {
		t.Helper()
		q := big.NewRat(n, d)
		if r.Min == nil || r.Max == nil || r.Min.Cmp(q) > 0 || r.Max.Cmp(q) < 0 ||
			new(big.Rat).Sub(r.Max, r.Min).Cmp(ratInset(o.ParameterTolerance)) > 0 {
			t.Fatal("original line parameter", r, q)
		}
	}
	line := func(p, d geom.Vec3, lo, hi float64) WrappedLoftLineIntersections {
		t.Helper()
		out, e := IntersectWrappedCircularFrustumOpenBoreLine(ctx, stock, tool, p, d, lo, hi, o)
		if e != nil {
			t.Fatal(e)
		}
		return out
	}
	// The axis meets the inward floor and stock base, with no hit on
	// the omitted opening disk. The floor is at original Z=1/2.
	y := line(geom.Vec3{Z: -1}, geom.Vec3{Z: 2}, 0, 3)
	if len(y.Hits) != 2 || len(y.Overlaps) != 0 || !slices.Equal(y.Hits[0].Faces, []int{1}) || !slices.Equal(y.Hits[1].Faces, []int{4}) {
		t.Fatal("axis met a buried cap or lost the floor", y)
	}
	checkRoot(y.Hits[0].Parameter, 1, 2)
	checkRoot(y.Hits[1].Parameter, 3, 4)
	// Exact signed traversal in the annular mouth. Both inner rim
	// hits retain the original annulus and the reversed inner wall.
	r := line(geom.Vec3{X: 6, Z: 4}, geom.Vec3{X: -2}, 0, 6)
	if len(r.Hits) != 4 || len(r.Overlaps) != 2 {
		t.Fatal("annular opening", r)
	}
	for i, p := range [4][2]int64{{1, 2}, {13, 16}, {83, 16}, {11, 2}} {
		checkRoot(r.Hits[i].Parameter, p[0], p[1])
		want := []int{0, 2}
		if i == 1 || i == 2 {
			want = []int{2, 3}
		}
		if !slices.Equal(r.Hits[i].Faces, want) {
			t.Fatal("opening root ancestry", i, r.Hits[i].Faces, want)
		}
	}
	for _, p := range r.Overlaps {
		if p.Face != 2 || p.Part != geom.CylinderUpperCap {
			t.Fatal("opening interval owner", p)
		}
	}
	r = line(geom.Vec3{X: 35.0 / 8, Z: 5}, geom.Vec3{Z: -1}, 0, 2)
	if len(r.Hits) != 2 || !slices.Equal(r.Hits[0].Faces, []int{2, 3}) || !slices.Equal(r.Hits[1].Faces, []int{0}) {
		t.Fatal("transverse exact inner rim must collapse to one hit", r)
	}
	checkRoot(r.Hits[0].Parameter, 1, 1)
	checkRoot(r.Hits[1].Parameter, 11, 6)
	r = line(geom.Vec3{X: 11.0 / 8, Z: 0}, geom.Vec3{X: .75, Z: 1}, .5, 4)
	if len(r.Overlaps) != 1 || r.Overlaps[0].Face != 3 ||
		!slices.Equal(r.Hits[0].Faces, []int{3, 4}) || !slices.Equal(r.Hits[len(r.Hits)-1].Faces, []int{2, 3}) {
		t.Fatal("inner generator and original terminal rims", r)
	}
	checkRoot(r.Overlaps[0].Start, 1, 2)
	checkRoot(r.Overlaps[0].End, 4, 1)
	canceled, stop := context.WithCancel(ctx)
	stop()
	if r, e := IntersectWrappedCircularFrustumOpenBoreLine(canceled, stock, tool, geom.Vec3{}, geom.Vec3{Z: 1}, 0, 1, o); !errors.Is(e, context.Canceled) || len(r.Hits)+len(r.Overlaps) != 0 {
		t.Fatal("failure published partial line results", e)
	}
}
