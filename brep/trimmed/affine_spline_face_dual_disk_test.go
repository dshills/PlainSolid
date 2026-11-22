//go:build research

package trimmed

import (
	"bytes"
	"context"
	"math"
	"math/big"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
	"testing"
	"time"
)

func TestAffineSplineFaceDualDiskFloor(t *testing.T) {
	ctx := context.Background()
	frame, e := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	discs := [2]ExtrusionCircle{{Center: geom.Vec2{X: 3.5, Y: 4}, Radius: 2.5}, {Center: geom.Vec2{X: 6.5, Y: 4}, Radius: 2.5}}
	cut, e := CutSideOpenDualDisks(ctx, frame, geom.Vec2{Y: 4}, geom.Vec2{X: 10, Y: 10}, discs, 6, 3, 1e-10, trimvalidate.Options{})
	if e != nil {
		t.Fatal(e)
	}
	q, e := NewAffineSplineFaceQuery(ctx, cut.Solid.Faces()[cut.FloorFace], AffineSplineFaceQueryOptions{})
	if e != nil {
		t.Fatal("floor query", e)
	}
	for _, test := range []struct {
		x, y int64
		want trimvalidate.Location
	}{{5, 5, trimvalidate.Inside}, {5, 4, trimvalidate.Boundary}, {5, 7, trimvalidate.Outside}} {
		p := [3]*big.Rat{big.NewRat(test.x, 1), big.NewRat(test.y, 1), big.NewRat(3, 1)}
		r, e := q.ClassifyRationalPoint(ctx, p, geom.AffineChartInverseOptions{})
		if e != nil || !r.Proven || !r.OnPlane || r.Location != test.want {
			t.Fatal(r, e)
		}
	}
	if q.Plane()[2].Sign() != -1 {
		t.Fatal("floor outward normal", q.Plane())
	}
	var encoded bytes.Buffer
	if e := WriteJSON(&encoded, cut.Solid); e != nil {
		t.Fatal(e)
	}
	restored, e := ReadJSONContext(ctx, &encoded, JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	reload, e := NewAffineSplineFaceQuery(ctx, restored.Faces()[cut.FloorFace], AffineSplineFaceQueryOptions{})
	if e != nil {
		t.Fatal(e)
	}
	rp, e := reload.ClassifyRationalPoint(ctx, [3]*big.Rat{big.NewRat(5, 1), big.NewRat(5, 1), big.NewRat(3, 1)}, geom.AffineChartInverseOptions{})
	if e != nil || rp.Location != trimvalidate.Inside {
		t.Fatal(rp, e)
	}
	start := time.Now()
	area, e := q.AreaBounds(ctx, big.NewRat(1, 10000), 0)
	if e != nil {
		t.Fatal(e)
	}
	lo, _ := area.ModelArea.Lower.Float64()
	hi, _ := area.ModelArea.Upper.Float64()
	want := 6.25*(math.Pi-math.Acos(.6)) + 3
	if lo > want || hi < want || hi-lo > 1e-4 {
		t.Fatal(lo, hi, want)
	}
	t.Logf("floor area width %.8g work %d subdivisions %d elapsed %v", hi-lo, area.Work, area.Subdivisions, time.Since(start))
}
