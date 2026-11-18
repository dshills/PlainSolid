package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"reflect"
	"testing"
)

func TestAnisotropicCapLine(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	r := big.NewRat
	s := anisotropicMassFixture(t, false, geom.Vec3{})
	o := WrappedAnisotropicMultiBandLineOptions{Geometry: geom.ToroidLineOptions{ParameterTolerance: 1e-9}}
	p, e := s.ValidateWrappedAnisotropicStationLoft(ctx, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	origin := [3]*big.Rat{r(1, 4), r(-1, 2), r(0, 1)}
	d := [3]*big.Rat{r(2, 1), r(0, 1), r(0, 1)}
	q, e := s.ClipWrappedAnisotropicStationCapLineRational(ctx, origin, d, r(-2, 1), r(2, 1), o)
	if e != nil {
		t.Fatal(e)
	}
	if len(q.Hits) != 2 || len(q.Boundary) != 1 {
		t.Fatalf("%+v", q)
	}
	for i, t0 := range []*big.Rat{r(-3, 2), r(3, 2)} {
		h := q.Hits[i]
		if h.Parameter.Min.Cmp(t0) != 0 || h.Parameter.Max.Cmp(t0) != 0 || len(h.Faces) != 2 || len(h.RimEdges) != 2 || len(h.Vertices) != 1 || h.Parts != (geom.CylinderLateral|geom.CylinderLowerCap) {
			t.Fatalf("cardinal %+v", h)
		}
	}
	span := q.Boundary[0]
	if !reflect.DeepEqual(span.Faces, []int{p.History.CapFaces[0]}) || len(span.StartFaces) != 2 || len(span.EndFaces) != 2 {
		t.Fatalf("span %+v", span)
	}
	// Exact independently evaluated weighted-quadratic midpoint is (5/7,5/7).
	// Both non-dyadic root enclosures retain the correct original rim quadrant.
	d = [3]*big.Rat{r(3, 1), r(1, 1), r(0, 1)}
	q, e = s.ClipWrappedAnisotropicStationCapLineRational(ctx, origin, d, r(-1, 1), r(1, 1), o)
	if e != nil {
		t.Fatal(e)
	}
	if len(q.Hits) != 2 || len(q.Boundary) != 1 {
		t.Fatalf("diagonal %+v", q)
	}
	for i, t0 := range []*big.Rat{r(-5, 7), r(5, 7)} {
		h := q.Hits[i]
		quarter := 2
		if i == 1 {
			quarter = 0
		}
		if h.Parameter.Min.Cmp(t0) > 0 || h.Parameter.Max.Cmp(t0) < 0 || !reflect.DeepEqual(h.RimEdges, []int{p.History.StationRims[0][quarter]}) {
			t.Fatalf("rim %+v", h)
		}
	}
	// A finite piece strictly inside the cap is only boundary, with no rim hits.
	q, e = s.ClipWrappedAnisotropicStationCapLineRational(ctx, origin, d, r(-1, 10), r(1, 10), o)
	if e != nil || len(q.Hits) != 0 || len(q.Boundary) != 1 || len(q.Boundary[0].StartFaces) != 1 || len(q.Boundary[0].EndFaces) != 1 {
		t.Fatalf("clipped %+v %v", q, e)
	}
	q, e = s.ClipWrappedAnisotropicStationCapLineRational(ctx, origin, d, r(0, 1), r(0, 1), o)
	if e != nil || len(q.Hits) != 1 || len(q.Boundary) != 0 || !reflect.DeepEqual(q.Hits[0].Faces, []int{p.History.CapFaces[0]}) {
		t.Fatalf("singleton %+v %v", q, e)
	}
	// Upper-cap isolated tangent: both original rim edges and cap/wall faces,
	// but no positive boundary run, and never a volume-interior interval.
	origin[2] = r(3, 1)
	origin[1] = r(1, 2)
	d = [3]*big.Rat{r(1, 1), r(0, 1), r(0, 1)}
	q, e = s.ClipWrappedAnisotropicStationCapLineRational(ctx, origin, d, r(-1, 1), r(1, 1), o)
	if e != nil || len(q.Hits) != 1 || len(q.Boundary) != 0 {
		t.Fatalf("tangent %+v %v", q, e)
	}
	if q.Hits[0].Parts != (geom.CylinderUpperCap|geom.CylinderLateral) || len(q.Hits[0].RimEdges) != 2 || len(q.Hits[0].Vertices) != 1 {
		t.Fatalf("upper owners %+v", q.Hits[0])
	}
	// Upper actual secant and opposite direction preserve source cap identity.
	origin[1] = r(-1, 2)
	d[0] = r(-2, 1)
	q, e = s.ClipWrappedAnisotropicStationCapLineRational(ctx, origin, d, r(-2, 1), r(2, 1), o)
	if e != nil || len(q.Boundary) != 1 || !reflect.DeepEqual(q.Boundary[0].Faces, []int{p.History.CapFaces[1]}) {
		t.Fatalf("upper %+v %v", q, e)
	}
	exact := o
	exact.MaxOperations = q.ExactOperations
	v, e := s.ClipWrappedAnisotropicStationCapLineRational(ctx, origin, d, r(-2, 1), r(2, 1), exact)
	if e != nil || !reflect.DeepEqual(q, v) {
		t.Fatalf("exact budget %v", e)
	}
	for _, bad := range []WrappedAnisotropicMultiBandLineOptions{{Geometry: o.Geometry, MaxOperations: q.ExactOperations - 1}, {Geometry: o.Geometry, MaxEvents: 2}} {
		v, e = s.ClipWrappedAnisotropicStationCapLineRational(ctx, origin, d, r(-2, 1), r(2, 1), bad)
		if e == nil || len(v.Hits) != 0 || len(v.Boundary) != 0 || v.ExactOperations != 0 {
			t.Fatal("partial")
		}
	}
	origin[2] = r(1, 1)
	if _, e = s.ClipWrappedAnisotropicStationCapLineRational(ctx, origin, d, r(-2, 1), r(2, 1), o); e == nil {
		t.Fatal("internal station cap")
	}
	origin[2] = r(0, 1)
	d[2] = r(1, 1)
	if _, e = s.ClipWrappedAnisotropicStationCapLineRational(ctx, origin, d, r(-2, 1), r(2, 1), o); e == nil {
		t.Fatal("nonhorizontal")
	}
	d[2] = r(0, 1)
	c, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = s.ClipWrappedAnisotropicStationCapLineRational(c, origin, d, r(-2, 1), r(2, 1), o); e == nil {
		t.Fatal("cancel")
	}
}
