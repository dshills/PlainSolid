//go:build research

package trimmed

import (
	"bytes"
	"context"
	"math/big"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"reflect"
	"slices"
	"testing"
)

func TestAnisotropicGenerator(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	r := big.NewRat
	s := anisotropicMassFixture(t, false, geom.Vec3{})
	faces := s.Faces()
	slices.Reverse(faces)
	for i := range faces {
		loop := faces[i].Outer
		faces[i].Outer = append(append([]Use(nil), loop[3:]...), loop[:3]...)
	}
	var e error
	s, e = NewSolidContext(ctx, s.Vertices(), s.Edges(), faces, 1e-8)
	if e != nil {
		t.Fatal(e)
	}
	var raw bytes.Buffer
	if e = WriteJSON(&raw, s); e != nil {
		t.Fatal(e)
	}
	s, e = ReadJSONContext(ctx, &raw, JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	p, e := s.ValidateWrappedAnisotropicStationLoft(ctx, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	wall := p.History.WallFaces[0]
	origin := [3]*big.Rat{r(67, 28), r(3, 14), r(0, 1)}
	d := [3]*big.Rat{r(-10, 7), r(5, 7), r(1, 1)}
	o := WrappedAnisotropicGeneratorOptions{}
	q, e := s.ClipWrappedAnisotropicStationGeneratorRational(ctx, wall, origin, d, r(-1, 1), r(4, 1), o)
	if e != nil {
		t.Fatal(e)
	}
	if q.WallFace != wall || len(q.Boundary) != 1 || len(q.Hits) != 0 {
		t.Fatalf("%+v", q)
	}
	span := q.Boundary[0]
	if !reflect.DeepEqual(span.Faces, []int{wall}) || len(span.Edges) != 0 || span.Start.Parameter.Min.Sign() != 0 || span.End.Parameter.Min.Cmp(r(1, 1)) != 0 || span.Start.Parts != (geom.CylinderLateral|geom.CylinderLowerCap) || span.End.Parts != geom.CylinderLateral || len(span.Start.Faces) != 2 || len(span.End.Faces) != 2 || len(span.Start.RimEdges) != 1 || len(span.End.RimEdges) != 1 {
		t.Fatalf("owners %+v", span)
	}
	// Independent profile midpoint is (5/7,5/7), maintained at every station
	// by x=cx+(3-2z)*5/7, y=cy+(1+z)*5/7. Original remapped rim IDs persist.
	if span.Start.RimEdges[0] != p.History.StationRims[0][0] || span.End.RimEdges[0] != p.History.StationRims[1][0] {
		t.Fatal("wrong source rims")
	}
	rev := [3]*big.Rat{r(20, 7), r(-10, 7), r(-2, 1)}
	v, e := s.ClipWrappedAnisotropicStationGeneratorRational(ctx, wall, origin, rev, r(-2, 1), r(1, 1), o)
	if e != nil || len(v.Boundary) != 1 || v.Boundary[0].Start.Parameter.Min.Cmp(r(-1, 2)) != 0 || v.Boundary[0].End.Parameter.Min.Sign() != 0 || v.Boundary[0].End.Parts != (geom.CylinderLateral|geom.CylinderLowerCap) {
		t.Fatalf("reverse %+v %v", v, e)
	}
	// Independent upper-band generator has opposite Y taper and terminates
	// at the original upper cap, while its start retains both crease walls.
	upperOrigin := [3]*big.Rat{r(17, 28), r(9, 7), r(0, 1)}
	upperD := [3]*big.Rat{r(5, 14), r(-5, 14), r(1, 1)}
	v, e = s.ClipWrappedAnisotropicStationGeneratorRational(ctx, p.History.WallFaces[1], upperOrigin, upperD, r(-1, 1), r(4, 1), o)
	if e != nil || len(v.Boundary) != 1 || v.Boundary[0].Start.Parameter.Min.Cmp(r(1, 1)) != 0 || v.Boundary[0].End.Parameter.Min.Cmp(r(3, 1)) != 0 || len(v.Boundary[0].Start.Faces) != 2 || v.Boundary[0].End.Parts != (geom.CylinderLateral|geom.CylinderUpperCap) {
		t.Fatalf("upper generator %+v %v", v, e)
	}
	// Positive-X is the original periodic native seam, not merely a quadrant
	// curve in UV. Its relative-interior edge occurs twice on the original wall.
	seamOrigin := [3]*big.Rat{r(13, 4), r(-1, 2), r(0, 1)}
	seamD := [3]*big.Rat{r(-2, 1), r(0, 1), r(1, 1)}
	v, e = s.ClipWrappedAnisotropicStationGeneratorRational(ctx, wall, seamOrigin, seamD, r(-1, 1), r(4, 1), o)
	if e != nil || len(v.Boundary) != 1 {
		t.Fatalf("seam %+v %v", v, e)
	}
	seam := v.Boundary[0]
	if len(seam.Edges) != 1 || len(seam.Start.Vertices) != 1 || len(seam.End.Vertices) != 1 || len(seam.Start.RimEdges) != 2 || len(seam.End.RimEdges) != 2 {
		t.Fatalf("seam owners %+v", seam)
	}
	count := 0
	for _, use := range s.Faces()[wall].Outer {
		if use.Edge == seam.Edges[0] {
			count++
		}
	}
	if count != 2 {
		t.Fatal("not the original periodic seam")
	}
	v, e = s.ClipWrappedAnisotropicStationGeneratorRational(ctx, wall, origin, d, r(1, 4), r(3, 4), o)
	if e != nil || len(v.Boundary) != 1 || len(v.Boundary[0].Start.Faces) != 1 || len(v.Boundary[0].End.Faces) != 1 || len(v.Boundary[0].Start.RimEdges) != 0 {
		t.Fatalf("finite %+v %v", v, e)
	}
	v, e = s.ClipWrappedAnisotropicStationGeneratorRational(ctx, wall, origin, d, r(1, 1), r(1, 1), o)
	if e != nil || len(v.Hits) != 1 || len(v.Boundary) != 0 || len(v.Hits[0].Faces) != 2 {
		t.Fatalf("singleton %+v %v", v, e)
	}
	v, e = s.ClipWrappedAnisotropicStationGeneratorRational(ctx, wall, origin, d, r(2, 1), r(3, 1), o)
	if e != nil || len(v.Hits) != 0 || len(v.Boundary) != 0 {
		t.Fatalf("outside selected band %+v %v", v, e)
	}
	exact := o
	exact.MaxOperations = q.ExactOperations
	v, e = s.ClipWrappedAnisotropicStationGeneratorRational(ctx, wall, origin, d, r(-1, 1), r(4, 1), exact)
	if e != nil || !reflect.DeepEqual(q, v) {
		t.Fatalf("exact allowance %v", e)
	}
	for _, budget := range []int{1, q.ExactOperations - 1} {
		bad := o
		bad.MaxOperations = budget
		v, e = s.ClipWrappedAnisotropicStationGeneratorRational(ctx, wall, origin, d, r(-1, 1), r(4, 1), bad)
		if e == nil || len(v.Boundary) != 0 || len(v.Hits) != 0 || v.ExactOperations != 0 {
			t.Fatal("partial quota")
		}
	}
	for _, id := range []int{p.History.CapFaces[0], -1, p.History.WallFaces[1]} {
		if _, e = s.ClipWrappedAnisotropicStationGeneratorRational(ctx, id, origin, d, r(-1, 1), r(4, 1), o); e == nil {
			t.Fatal("wrong selected wall")
		}
	}
	// A sub-ULP directional perturbation must not be accepted as coincidence.
	badD := d
	badD[0] = new(big.Rat).Add(d[0], new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 100)))
	if _, e = s.ClipWrappedAnisotropicStationGeneratorRational(ctx, wall, origin, badD, r(-1, 1), r(4, 1), o); e == nil {
		t.Fatal("approximate generator")
	}
	center := [3]*big.Rat{r(1, 4), r(-1, 2), r(0, 1)}
	axis := [3]*big.Rat{r(0, 1), r(0, 1), r(1, 1)}
	if _, e = s.ClipWrappedAnisotropicStationGeneratorRational(ctx, wall, center, axis, r(-1, 1), r(4, 1), o); e == nil {
		t.Fatal("interior fiber is not boundary")
	}
	q.Boundary[0].Faces[0] = -1
	q.Boundary[0].Start.Parameter.Min.SetInt64(99)
	v, e = s.ClipWrappedAnisotropicStationGeneratorRational(ctx, wall, origin, d, r(-1, 1), r(4, 1), o)
	if e != nil || v.Boundary[0].Faces[0] != wall || v.Boundary[0].Start.Parameter.Min.Sign() != 0 {
		t.Fatal("mutable source")
	}
	c, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = s.ClipWrappedAnisotropicStationGeneratorRational(c, wall, origin, d, r(-1, 1), r(4, 1), o); e == nil {
		t.Fatal("cancel")
	}
}
