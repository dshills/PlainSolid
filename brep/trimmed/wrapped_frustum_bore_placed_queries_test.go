package trimmed

import (
	"context"
	"errors"
	"math/big"
	"slices"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/placement"
	"plainsolid/geom"
)

func TestPlacedFrustumBoreExactObliqueQueries(t *testing.T) {
	ctx := context.Background()
	q := big.NewRat
	zero := q(0, 1)
	// Rational noncardinal orthonormal frame rotates about local Y and
	// translates by nonbinary coordinates. Local axis +Z maps to (4/5,0,3/5).
	matrix := [3][4]*big.Rat{{q(3, 5), zero, q(4, 5), q(1, 7)}, {zero, q(1, 1), zero, q(-2, 7)}, {q(-4, 5), zero, q(3, 5), q(1, 13)}}
	pose, e := placement.NewRationalRigid(matrix, 0)
	if e != nil {
		t.Fatal(e)
	}
	matrix[0][0].SetInt64(0) // The stored pose must own its input.
	patch := geom.SurfacePatchOptions{}
	outer, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, 3, 4, patch)
	if e != nil {
		t.Fatal(e)
	}
	inner, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{X: .25, Y: .25, Z: .5}, 1, 3, patch)
	if e != nil {
		t.Fatal(e)
	}
	s, proof, e := NewWrappedEccentricCircularFrustumBore(ctx, outer, inner, 1e-9, patch)
	if e != nil {
		t.Fatal(e)
	}
	// External analytic formula for the supplied world line. In the local
	// solid it is (1/4,1/4,-1/3)+(0,0,2/3)t.
	origin := [3]*big.Rat{q(11, 420), q(-1, 28), q(-21, 65)}
	dir := [3]*big.Rat{q(8, 15), zero, q(2, 5)}
	r, e := s.IntersectPlacedWrappedFrustumBoreLineRational(ctx, pose, origin, dir, q(1, 7), q(7, 1), geom.ToroidLineOptions{ParameterTolerance: 1e-10})
	if e != nil || len(r.Hits) != 4 || len(r.Overlaps) != 0 {
		t.Fatal(r, e)
	}
	for i, h := range r.Hits {
		want := [4]*big.Rat{q(1, 2), q(5, 4), q(23, 4), q(13, 2)}[i]
		face := [4]int{proof.OuterFaces[1], proof.InnerFaces[1], proof.InnerFaces[2], proof.OuterFaces[2]}[i]
		if h.Parameter.Min.Cmp(want) != 0 || h.Parameter.Max.Cmp(want) != 0 || !slices.Equal(h.Faces, []int{face}) {
			t.Fatal("world placement lost original cap owner or line units", i, h, want)
		}
	}
	var p [3]*big.Rat
	// Evaluate the independently parameterized line at t=13/8, local z=3/4.
	for i := range p {
		p[i] = new(big.Rat).Add(origin[i], new(big.Rat).Mul(q(13, 8), dir[i]))
	}
	loc, e := s.LocatePlacedWrappedFrustumBoreRational(ctx, pose, p, patch)
	if e != nil || loc != parametric.Outside {
		t.Fatal("enclosed void outside material", loc, e)
	}
	dist, e := s.BoundPointPlacedWrappedFrustumBoreDistanceRational(ctx, pose, p, geom.FrustumDistanceOptions{SquaredTolerance: 1e-12})
	if e != nil || dist.Lower.Cmp(q(1, 16)) != 0 || dist.Upper.Cmp(q(1, 16)) != 0 || dist.Witness.Face != proof.InnerFaces[1] {
		t.Fatal(dist, e)
	}
	for i, w := range [3]*big.Rat{q(97, 140), q(-1, 28), q(23, 130)} {
		if dist.Witness.Base[i].Cmp(w) != 0 || dist.Witness.Numerator[i].Sign() != 0 {
			t.Fatal("world exact cap witness", i, dist.Witness, w)
		}
	}
	section, e := s.SectionPlacedWrappedFrustumBoreRational(ctx, pose, q(7, 3), patch)
	if e != nil || !section.Local.HasHole || section.Local.AreaPi.Cmp(q(8, 1)) != 0 || section.Local.OuterFace != proof.OuterFaces[0] || section.Local.InnerFace != proof.InnerFaces[0] || section.Local.Outer.ParameterV.Cmp(q(7, 12)) != 0 || section.Local.Inner.ParameterV.Cmp(q(11, 18)) != 0 {
		t.Fatal("oblique section keeps original circle curves and area", section, e)
	}
	axis := [3]*big.Rat{q(4, 5), zero, q(3, 5)}
	outerCenter := [3]*big.Rat{q(211, 105), q(-2, 7), q(96, 65)}
	innerCenter := [3]*big.Rat{q(907, 420), q(-1, 28), q(83, 65)}
	for i := range axis {
		if section.Axis[i].Cmp(axis[i]) != 0 || section.OuterCenter[i].Cmp(outerCenter[i]) != 0 || section.InnerCenter[i].Cmp(innerCenter[i]) != 0 {
			t.Fatal("world plane or original circle center was rounded", i, section)
		}
	}
	// Independent closed cylinder integrals give V/π=33, M=(-3/4,-3/4,66),
	// Mxx=Myy=1281/16, Mxy=-3/16, Mxz=Myz=-3/2, Mzz=711/4.
	// The listed world values follow by direct substitution of each
	// affine coordinate into those elementary integrals.
	mass, e := s.MeasurePlacedWrappedFrustumBoreRational(ctx, pose, patch)
	if e != nil || mass.VolumePi.Cmp(q(33, 1)) != 0 || len(mass.Area) != 6 {
		t.Fatal(mass, e)
	}
	c := [3]*big.Rat{q(2663, 1540), q(-95, 308), q(926, 715)}
	second := [3][3]*big.Rat{{q(3072753, 19600), q(-69477, 3920), q(261363, 4550)},
		{q(-69477, 3920), q(65217, 784), q(-11847, 910)}, {q(261363, 4550), q(-11847, 910), q(2079543, 16900)}}
	inertia := [3][3]*big.Rat{{q(162519, 1100), q(27, 220), q(4527, 275)},
		{q(27, 220), q(5535, 44), q(-9, 55)}, {q(4527, 275), q(-9, 55), q(37989, 275)}}
	for i := range c {
		if mass.Centroid[i].Cmp(c[i]) != 0 || mass.FirstMomentPi[i].Cmp(new(big.Rat).Mul(c[i], q(33, 1))) != 0 {
			t.Fatal("exact placed centroid/first moment", i, mass)
		}
		for j := range c {
			if mass.SecondMomentPi[i][j].Cmp(second[i][j]) != 0 || mass.InertiaAtCentroidPi[i][j].Cmp(inertia[i][j]) != 0 {
				t.Fatal("wrong placed parallel-axis or rotated inertia term", i, j, mass)
			}
		}
	}

	reflection := [3][4]*big.Rat{{q(-1, 1), zero, zero, zero}, {zero, q(1, 1), zero, zero}, {zero, zero, q(1, 1), zero}}
	if _, e := placement.NewRationalRigid(reflection, 0); !errors.Is(e, placement.ErrInvalid) {
		t.Fatal("improper rotation accepted", e)
	}

	canceled, stop := context.WithCancel(ctx)
	stop()
	if bad, e := s.BoundPointPlacedWrappedFrustumBoreDistanceRational(canceled, pose, p, geom.FrustumDistanceOptions{SquaredTolerance: 1e-12}); !errors.Is(e, context.Canceled) || bad.Upper != nil {
		t.Fatal("canceled query exposed a witness", bad, e)
	}
}
