package trimmed

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"reflect"
	"testing"
)

func TestMixedOriginalNonaffinePlanarFragmentFill(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	a, b, p, o := mixedWallFixture(t)
	section, e := SectionSplineShellsMixedAlgebraic(ctx, a, b, p, o)
	if e != nil {
		t.Fatal(e)
	}
	bc := AlgebraicShellBooleanOptions{}
	bc.Imprint.Exact.MaxOperations = 128000000
	bodies, e := section.CombineAlgebraicOperations(ctx, []parametric.BooleanOp{parametric.Intersection}, bc)
	if e != nil {
		t.Fatal(e)
	}
	body := bodies[0]
	n, e := body.TessellateEmbeddedBoundaryCertified(ctx, AlgebraicShellBoundaryEmbeddingOptions{Boundary: AlgebraicShellBoundaryOptions{Deflection: .02, MaxOperations: 128000000, AutoSectionProjection: true, PlanarSectionFallback: true}})
	if e != nil {
		t.Fatal(e)
	}
	var proposals []MixedPlanarFragmentProposal
	for id, f := range body.Faces() {
		q := MixedPlanarFragmentProposal{Face: id}
		switch f.Original {
		case 0:
			z := []float64{-1, -.5}[f.Operand]
			q.Plane = [3]geom.Vec3{{Z: z}, {X: 1, Z: z}, {Y: 1, Z: z}}
			q.ProjectionAxes = [2]int{0, 1}
		case 2, 3:
			x := [][2]float64{{-.125, 1.125}, {0, 1}}[f.Operand][f.Original-2]
			q.Plane = [3]geom.Vec3{{X: x}, {X: x, Y: 1}, {X: x, Z: 1}}
			q.ProjectionAxes = [2]int{1, 2}
		case 4, 5:
			y := [][2]float64{{0, 1}, {-.125, 1.125}}[f.Operand][f.Original-4]
			q.Plane = [3]geom.Vec3{{Y: y}, {X: 1, Y: y}, {Y: y, Z: 1}}
			q.ProjectionAxes = [2]int{0, 2}
		default:
			continue
		}
		proposals = append(proposals, q)
	}
	r, e := n.FillPlanarFragmentsCertified(ctx, proposals, MixedAffineFragmentFillOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(r.Fragments()) < 5 || len(r.Fragments()) != len(proposals) || !reflect.DeepEqual(r.Mesh().Vertices, n.Vertices()) {
		t.Fatal("lost selected original walls")
	}
	for _, id := range r.UnfilledFaces() {
		if body.Faces()[id].Original != 1 {
			t.Fatal("unfilled planar wall", id)
		}
	}
	model := r.Mesh()
	nonaffine := 0
	restricted := 0
	var reversed *MixedPlanarFragmentProposal
	for _, f := range r.Fragments() {
		if f.OriginalUVResolved || f.ErrorUpper.Sign() <= 0 || f.AreaProjected.Sign() <= 0 {
			t.Fatal("fabricated inverse or area")
		}
		source, _ := body.OriginalFace(f.Face)
		patch, _ := exactFaceSupportPatch(source, geom.SurfacePatchOptions{})
		if _, e := geom.NewAffineSurfaceChart(ctx, patch, geom.AffineSurfaceChartOptions{}); e != nil {
			nonaffine++
		}
		for _, use := range body.Faces()[f.Face].Outer {
			cert, e := n.EdgeCertificate(use.Edge)
			if e != nil {
				t.Fatal(e)
			}
			if cert.Kind == MixedBoundaryRestrictedCurve {
				restricted++
			}
		}
		for j, id := range f.BoundaryVertices {
			v := model.Vertices[id]
			xyz := []float64{v.X, v.Y, v.Z}
			for k, axis := range f.ProjectionAxes {
				if f.Coordinates[j][k].Cmp(ratInset(xyz[axis])) != 0 {
					t.Fatal("changed physical projection")
				}
			}
		}
		// Independent exact orientation against projected chart Jacobian.
		triangles := f.Triangles
		for _, tri := range triangles {
			v := model.Vertices
			get := func(i, k int) *big.Rat { x := v[i]; return ratInset([]float64{x.X, x.Y, x.Z}[f.ProjectionAxes[k]]) }
			det := new(big.Rat).Sub(new(big.Rat).Mul(new(big.Rat).Sub(get(tri[1], 0), get(tri[0], 0)), new(big.Rat).Sub(get(tri[2], 1), get(tri[0], 1))), new(big.Rat).Mul(new(big.Rat).Sub(get(tri[1], 1), get(tri[0], 1)), new(big.Rat).Sub(get(tri[2], 0), get(tri[0], 0))))
			want := 1
			if body.Faces()[f.Face].Reversed != f.ProjectionReversed {
				want = -1
			}
			if det.Sign() != want {
				t.Fatal("wrong physical orientation")
			}
		}
		if reversed == nil {
			for _, q := range proposals {
				if q.Face == f.Face {
					copy := q
					copy.ProjectionAxes = [2]int{q.ProjectionAxes[1], q.ProjectionAxes[0]}
					reversed = &copy
					break
				}
			}
		}
	}
	if nonaffine < 3 || restricted == 0 {
		t.Fatal("fixture did not qualify nonlinear wall/restricted seam", nonaffine, restricted)
	}
	rr, e := n.FillPlanarFragmentsCertified(ctx, []MixedPlanarFragmentProposal{*reversed}, MixedAffineFragmentFillOptions{})
	if e != nil || !rr.Fragments()[0].ProjectionReversed {
		t.Fatal("reversed physical projection", e)
	}
	bad := proposals[0]
	for j := range bad.Plane {
		switch 3 - bad.ProjectionAxes[0] - bad.ProjectionAxes[1] {
		case 0:
			bad.Plane[j].X += .125
		case 1:
			bad.Plane[j].Y += .125
		case 2:
			bad.Plane[j].Z += .125
		}
	}
	if got, e := n.FillPlanarFragmentsCertified(ctx, []MixedPlanarFragmentProposal{bad}, MixedAffineFragmentFillOptions{}); got != nil || e == nil {
		t.Fatal("wrong plane accepted")
	}
	for _, id := range r.UnfilledFaces() {
		bad = proposals[0]
		bad.Face = id
		if got, e := n.FillPlanarFragmentsCertified(ctx, []MixedPlanarFragmentProposal{bad}, MixedAffineFragmentFillOptions{}); got != nil || e == nil {
			t.Fatal("roof accepted")
		}
		break
	}
	for _, limits := range []MixedAffineFragmentFillOptions{{MaxOperations: 1}, {MaxPredicates: 1}, {MaxPairs: 1}, {MaxTriangles: 1}} {
		if got, e := n.FillPlanarFragmentsCertified(ctx, proposals, limits); got != nil || !errors.Is(e, geom.ErrConvergence) {
			t.Fatal("partial exhausted fill", e)
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if got, e := n.FillPlanarFragmentsCertified(canceled, proposals, MixedAffineFragmentFillOptions{}); got != nil || !errors.Is(e, context.Canceled) {
		t.Fatal("cancel", e)
	}
	copied := r.Fragments()
	copied[0].Coordinates[0][0].SetInt64(999)
	if r.Fragments()[0].Coordinates[0][0].Cmp(big.NewRat(999, 1)) == 0 {
		t.Fatal("mutable result")
	}
	ops, pred, pairs := r.Work()
	t.Log("planar faces", len(proposals), "nonaffine", nonaffine, "triangles", len(r.Mesh().Triangles), "unfilled roofs", r.UnfilledFaces(), "error", r.Deflection(), "ops", ops, "predicates", pred, "pairs", pairs)
}

func TestMixedPlanarFillRejectsFoldedSupport(t *testing.T) {
	var poles []geom.Vec3
	for _, x := range []float64{0, 1, 0} {
		for _, y := range []float64{0, 1} {
			poles = append(poles, geom.Vec3{X: x, Y: y})
		}
	}
	surface, e := geom.NewBSplineSurface(2, 1, 3, 2, poles, []float64{0, 0, 0, 1, 1, 1}, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	patch, e := geom.NewSurfacePatch(surface, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	proposal := MixedPlanarFragmentProposal{Plane: [3]geom.Vec3{{}, {X: 1}, {Y: 1}}, ProjectionAxes: [2]int{0, 1}}
	if got, e := proveMixedPhysicalPlane(context.Background(), patch, proposal, 1000000); got != nil || e == nil {
		t.Fatal("folded planar chart accepted")
	}
}
