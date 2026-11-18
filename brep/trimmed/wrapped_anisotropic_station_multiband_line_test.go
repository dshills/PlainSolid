package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"reflect"
	"testing"
)

func TestAnisotropicMultiBandLine(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	r := big.NewRat
	s := anisotropicMassFixture(t, false, geom.Vec3{})
	o := WrappedAnisotropicMultiBandLineOptions{Geometry: geom.ToroidLineOptions{ParameterTolerance: 1e-9}}
	proof, e := s.ValidateWrappedAnisotropicStationLoft(ctx, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	origin := [3]*big.Rat{r(1, 4), r(-1, 2), r(0, 1)}
	d := [3]*big.Rat{r(0, 1), r(0, 1), r(1, 1)}
	q, e := s.ClipWrappedAnisotropicStationLineRational(ctx, origin, d, r(-1, 1), r(4, 1), o)
	if e != nil {
		t.Fatal(e)
	}
	if len(q.Hits) != 2 || len(q.Interior) != 1 {
		t.Fatalf("axis %+v", q)
	}
	for i, z := range []*big.Rat{r(0, 1), r(3, 1)} {
		h := q.Hits[i]
		if h.Parameter.Min.Cmp(z) != 0 || h.Parameter.Max.Cmp(z) != 0 || !reflect.DeepEqual(h.Faces, []int{proof.History.CapFaces[i]}) || len(h.RimEdges) != 0 {
			t.Fatalf("cap %+v", h)
		}
	}
	// The waist cardinal point is reached at the exact original crease. Both
	// neighboring bands contain the line on their interior side; this boundary
	// event must retain both source walls, two rim edges and the original vertex.
	origin[0] = r(5, 4)
	q, e = s.ClipWrappedAnisotropicStationLineRational(ctx, origin, d, r(-1, 1), r(4, 1), o)
	if e != nil {
		t.Fatal(e)
	}
	if len(q.Hits) != 3 || len(q.Interior) != 2 {
		t.Fatalf("crease %+v", q)
	}
	h := q.Hits[1]
	if h.Parameter.Min.Cmp(r(1, 1)) != 0 || h.Parameter.Max.Cmp(r(1, 1)) != 0 || len(h.Faces) != 2 || len(h.RimEdges) != 2 || len(h.Vertices) != 1 || h.Parts != geom.CylinderLateral {
		t.Fatalf("crease owners %+v", h)
	}
	for _, face := range proof.History.WallFaces {
		found := false
		for _, id := range h.Faces {
			found = found || id == face
		}
		if !found {
			t.Fatal("missing wall")
		}
	}
	// Reverse authored t with the same world line and finite range.
	reverse := [3]*big.Rat{r(0, 1), r(0, 1), r(-2, 1)}
	v, e := s.ClipWrappedAnisotropicStationLineRational(ctx, origin, reverse, r(-2, 1), r(1, 2), o)
	if e != nil || len(v.Hits) != 3 || len(v.Interior) != 2 {
		t.Fatalf("reverse %+v %v", v, e)
	}
	for i, want := range []*big.Rat{r(-3, 2), r(-1, 2), r(0, 1)} {
		if v.Hits[i].Parameter.Min.Cmp(want) != 0 || v.Hits[i].Parameter.Max.Cmp(want) != 0 {
			t.Fatalf("t %+v", v.Hits[i])
		}
	}
	// Exact terminal cardinal contact owns cap and lateral support together.
	origin[0] = r(13, 4)
	q, e = s.ClipWrappedAnisotropicStationLineRational(ctx, origin, d, r(-1, 1), r(4, 1), o)
	if e != nil {
		t.Fatal(e)
	}
	if len(q.Hits) != 1 || len(q.Interior) != 0 || len(q.Hits[0].Faces) != 2 || len(q.Hits[0].RimEdges) != 2 || q.Hits[0].Parts != (geom.CylinderLateral|geom.CylinderLowerCap) {
		t.Fatalf("terminal %+v", q)
	}
	// Independent affine cardinal equations give contacts in two different
	// bands: -7/2+3z=-(3-2z) at z=1/2 and
	// -7/2+3z=(z+1)/2 at z=8/5. The retained interval crosses
	// the internal station disk, which contributes no boundary event.
	obliqueOrigin := [3]*big.Rat{r(-13, 4), r(-1, 2), r(0, 1)}
	obliqueDirection := [3]*big.Rat{r(3, 1), r(0, 1), r(1, 1)}
	v, e = s.ClipWrappedAnisotropicStationLineRational(ctx, obliqueOrigin, obliqueDirection, r(-1, 1), r(4, 1), o)
	if e != nil || len(v.Hits) != 2 || len(v.Interior) != 1 {
		t.Fatalf("two bands %+v %v", v, e)
	}
	for i, want := range []*big.Rat{r(1, 2), r(8, 5)} {
		h := v.Hits[i]
		if h.Parameter.Min.Cmp(want) != 0 || h.Parameter.Max.Cmp(want) != 0 || !reflect.DeepEqual(h.Faces, []int{proof.History.WallFaces[i]}) {
			t.Fatalf("band contact %+v", h)
		}
	}
	// One ledger spans all bands and exact gap/plane classifications.
	origin[0] = r(5, 4)
	q, e = s.ClipWrappedAnisotropicStationLineRational(ctx, origin, d, r(-1, 1), r(4, 1), o)
	if e != nil {
		t.Fatal(e)
	}
	exact := o
	exact.MaxOperations = q.ExactOperations
	v, e = s.ClipWrappedAnisotropicStationLineRational(ctx, origin, d, r(-1, 1), r(4, 1), exact)
	if e != nil || !reflect.DeepEqual(q, v) {
		t.Fatalf("exact work %+v %v", v, e)
	}
	for _, limit := range []int{1, q.ExactOperations - 1} {
		bad := o
		bad.MaxOperations = limit
		v, e = s.ClipWrappedAnisotropicStationLineRational(ctx, origin, d, r(-1, 1), r(4, 1), bad)
		if e == nil || len(v.Hits) != 0 || len(v.Interior) != 0 || v.ExactOperations != 0 {
			t.Fatal("partial work")
		}
	}
	bad := o
	bad.MaxEvents = 1
	if v, e = s.ClipWrappedAnisotropicStationLineRational(ctx, origin, d, r(-1, 1), r(4, 1), bad); e == nil || len(v.Hits) != 0 {
		t.Fatal("events")
	}
	// A generator on even one clipped band prevents an isolated ledger.
	goOrigin := [3]*big.Rat{r(67, 28), r(3, 14), r(0, 1)}
	goDirection := [3]*big.Rat{r(-10, 7), r(5, 7), r(1, 1)}
	v, e = s.ClipWrappedAnisotropicStationLineRational(ctx, goOrigin, goDirection, r(-1, 1), r(4, 1), o)
	if e == nil || len(v.Hits) != 0 || len(v.Interior) != 0 || v.ExactOperations != 0 {
		t.Fatal("partial generator ledger")
	}
	c, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = s.ClipWrappedAnisotropicStationLineRational(c, origin, d, r(-1, 1), r(4, 1), o); e == nil {
		t.Fatal("cancel")
	}
	d[2] = r(0, 1)
	d[0] = r(1, 1)
	if _, e = s.ClipWrappedAnisotropicStationLineRational(ctx, origin, d, r(-1, 1), r(4, 1), o); e == nil {
		t.Fatal("coincident plane scope")
	}
}
