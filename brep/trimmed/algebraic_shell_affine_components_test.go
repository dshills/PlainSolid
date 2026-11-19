//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
	"testing"
)

func TestAffineShellMaterialComponents(t *testing.T) {
	ctx := context.Background()
	a, b := pairedAffineShellFixture(t)
	r, e := CombineConvexAffineSplineShellComponents(ctx, b, a, parametric.Difference, ConvexAffineSplineShellCollectionOptions{})
	if e != nil {
		t.Fatal(e)
	}
	components, volumes := r.Components(), r.ComponentVolumes()
	if len(components) != 2 || len(r.Separations()) != 1 {
		t.Fatal("incomplete component proof")
	}
	// Independent raw-coordinate lower-Y lobe: (3/4)*(1/4+1/24)=7/32.
	lower := new(big.Rat).Mul(big.NewRat(3, 4), new(big.Rat).Add(big.NewRat(1, 4), big.NewRat(1, 24)))
	total := new(big.Rat).Sub(big.NewRat(6, 1), affineMaterialFixtureCommonVolume())
	if volumes[0].Cmp(lower) != 0 || volumes[1].Cmp(new(big.Rat).Sub(total, lower)) != 0 || r.ExactVolume().Cmp(total) != 0 {
		t.Fatal("component mass")
	}
	proof := r.Separations()[0]
	if proof.Components != [2]int{0, 1} || proof.PlaneOwner != 1 || proof.PlaneFace != 2 || proof.PointSide != 1 || proof.PairedNode < 0 || proof.Point[0].Sign() != 0 || proof.Point[1].Cmp(big.NewRat(-1, 1)) != 0 || proof.Point[2].Sign() != 0 {
		t.Fatal("wrong point ownership")
	}
	source := r.OriginalImprint()
	node := source.PairedNodes()[proof.PairedNode]
	point := source.Vertices()[node.Vertex]
	for k := range point {
		if point[k].Cmp(proof.Point[k]) != 0 {
			t.Fatal("unbound paired contact")
		}
	}
	faceCount := 0
	for ci, solid := range components {
		if len(solid.vertices)-len(solid.edges)+len(solid.faces) != 2 {
			t.Fatal("component Euler")
		}
		if e = affineMaterialVertexLinks(ctx, solid); e != nil {
			t.Fatal(e)
		}
		zero, shared := 0, 0
		budget := &nativePrismAffineBudget{ctx: ctx, max: 100000, bits: 16384}
		plane := affineMaterialPlane{n: proof.Normal, d: proof.Offset}
		for _, v := range solid.vertices {
			q := affineMaterialXYZ(v)
			sign := affineMaterialValue(budget, plane, q).Sign()
			if sign*proof.Sides[ci] < 0 {
				t.Fatal("wrong closed halfspace")
			}
			if sign == 0 {
				zero++
			}
			same := true
			for k := range q {
				same = same && q[k].Cmp(point[k]) == 0
			}
			if same {
				shared++
			}
		}
		if shared != 1 || (ci == proof.PointSide && zero != 1) || (ci != proof.PointSide && zero < 3) {
			t.Fatal("unproved isolated zero set")
		}
		sources := r.FaceSources(ci)
		if len(sources) != len(solid.faces) {
			t.Fatal("missing original face")
		}
		faceCount += len(sources)
		for fi, f := range sources {
			original := source.sources[f.Owner].faces[f.Face]
			if solid.faces[fi].Surface != original.Surface || solid.faces[fi].Reversed != (original.Reversed != f.Reversed) || r.SourceFaceFragments(f.Owner, f.Face) == nil {
				t.Fatal("source support lost")
			}
		}
	}
	retained := 0
	for _, f := range r.Fragments() {
		if f.Retained {
			retained++
		}
	}
	if faceCount != retained {
		t.Fatal("lost selected fragments")
	}
	proof.Point[0].SetInt64(9)
	volumes[0].SetInt64(9)
	if r.Separations()[0].Point[0].Sign() != 0 || r.ComponentVolumes()[0].Cmp(lower) != 0 {
		t.Fatal("mutable collection proof")
	}
	if _, e = source.CombineConvexComponents(ctx, intersect.AffineRingDifference, AffineShellMaterialCollectionOptions{Material: AffineShellMaterialOptions{MaxOperations: r.ExactOperations()}}); e != nil {
		t.Fatal("exact quota", e)
	}
	if v, e := source.CombineConvexComponents(ctx, intersect.AffineRingDifference, AffineShellMaterialCollectionOptions{Material: AffineShellMaterialOptions{MaxOperations: r.ExactOperations() - 1}}); v != nil || e == nil {
		t.Fatal("partial quota publication")
	}
	if _, e = source.CombineConvex(ctx, intersect.AffineRingDifference, AffineShellMaterialOptions{}); e == nil {
		t.Fatal("strict contract widened")
	}
	if v, e := CombineConvexAffineSplineShellComponents(ctx, a, b, parametric.Difference, ConvexAffineSplineShellCollectionOptions{}); v != nil || e == nil {
		t.Fatal("connected pinched component accepted")
	}
}
func TestAffineShellMaterialComponentRejects(t *testing.T) {
	ctx := context.Background()
	a, b := pairedAffineShellFixture(t)
	s, e := AuthorizeAffineSplineShellPairedImprint(ctx, b, a, AffineShellPairedImprintOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for _, o := range []AffineShellMaterialCollectionOptions{{MaxComponents: 1}, {MaxComponents: 33}, {Material: AffineShellMaterialOptions{MaxOperations: 1}}} {
		if r, e := s.CombineConvexComponents(ctx, intersect.AffineRingDifference, o); r != nil || e == nil {
			t.Fatal("non-atomic bound")
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if r, e := s.CombineConvexComponents(canceled, intersect.AffineRingDifference, AffineShellMaterialCollectionOptions{}); r != nil || e == nil {
		t.Fatal("cancellation")
	}
	budget := func() *nativePrismAffineBudget { return &nativePrismAffineBudget{ctx: ctx, max: 1000000, bits: 16384} }
	if _, ok := affineComponentSeparator(budget(), s, a, a); ok {
		t.Fatal("area overlap accepted")
	}
	// Two exact separating vertex sets share a nonzero interval: no original
	// affine plane can prove a singleton zero set on either side.
	lo := Solid{vertices: []geom.Vec3{{X: -.5, Y: -1}, {X: .5, Y: -1}, {Y: -2, Z: -1}, {Y: -2, Z: 1}}}
	hi := Solid{vertices: []geom.Vec3{{X: -.5, Y: -1}, {X: .5, Y: -1}, {Z: -1}, {Z: 1}}}
	if _, ok := affineComponentSeparator(budget(), s, lo, hi); ok {
		t.Fatal("interval contact accepted")
	}
	r, e := s.CombineConvexComponents(ctx, intersect.AffineRingDifference, AffineShellMaterialCollectionOptions{})
	if e != nil {
		t.Fatal(e)
	}
	damaged := *s
	damaged.paired = nil
	parts := r.Components()
	if _, ok := affineComponentSeparator(budget(), &damaged, parts[0], parts[1]); ok {
		t.Fatal("unowned contact accepted")
	}
}
