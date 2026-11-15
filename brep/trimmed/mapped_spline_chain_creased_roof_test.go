package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math"
	"math/big"
	"strings"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func mappedChainCreasedSupport(t *testing.T, swapped bool) *geom.BSplineSurface {
	t.Helper()
	x, y := []float64{0, .5, 1}, []float64{0, 1}
	ku, kv := []float64{0, 0, .5, 1, 1}, []float64{0, 0, 1, 1}
	if swapped {
		x, y, ku, kv = y, x, kv, ku
	}
	poles := make([]geom.Vec3, 0, len(x)*len(y))
	for _, u := range x {
		for _, v := range y {
			a, b := u, v
			if swapped {
				a, b = b, a
			}
			poles = append(poles, geom.Vec3{X: a, Y: b, Z: a*b + .5*max(0, a-.5)})
		}
	}
	s, err := geom.NewBSplineSurface(1, 1, len(x), len(y), poles, ku, kv, nil)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestMappedSplineChainCreasedRoof(t *testing.T) {
	ctx := context.Background()
	r, closing, planar, seams := mappedChainClosedFixtureWithKnot(t, true)
	shell, err := CertifyMappedSplineChainShell(ctx, r, closing, [2]bool{false, true}, planar, seams, MappedChainShellProofOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var recipe bytes.Buffer
	if err := shell.WriteJSON(ctx, &recipe, MappedChainShellNativeOptions{}); err != nil {
		t.Fatal(err)
	}
	shell, err = ReadMappedSplineChainShellJSON(ctx, &recipe, MappedChainShellNativeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	embedded, err := shell.CertifyRectangularEmbedding(ctx, geom.SurfacePatchOptions{})
	if err != nil || embedded.Clearance.Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal("complete affine-XY embedding on original knot cells", err)
	}
	for owner, s := range shell.boundary.supports {
		if !geom.SameSplineSurface(s.(*geom.BSplineSurface), r.supports[owner].(*geom.BSplineSurface)) {
			t.Fatal("original roof coefficients or knots changed during native reopen")
		}
		patch, err := exactSplineSupportPatch(s, geom.SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		split := patch.SplitU
		uv := func(x, y *big.Rat) [2]*big.Rat { return [2]*big.Rat{x, y} }
		if owner == 1 {
			split = patch.SplitV
			uv = func(x, y *big.Rat) [2]*big.Rat { return [2]*big.Rat{y, x} }
		}
		left, right, err := split(.5)
		if err != nil {
			t.Fatal(err)
		}
		knot := big.NewRat(1, 2)
		var lifts [2]*geom.ExactSplineCurve
		for side, p := range [2]*geom.SurfacePatch{left, right} {
			a, b := uv(knot, new(big.Rat)), uv(knot, big.NewRat(3, 4))
			lifts[side], err = p.AffineParameterCurve(ctx, a, b, geom.TubeAffineLineOptions{})
			if err != nil {
				t.Fatal(err)
			}
			point := uv(knot, big.NewRat(1, 4))
			jet, err := p.EvaluateRational(ctx, point[0], point[1], geom.SurfacePatchEvaluationOptions{})
			if err != nil {
				t.Fatal(err)
			}
			tangent := jet.Du
			if owner == 1 {
				tangent = jet.Dv
			}
			if tangent[0].Cmp(big.NewRat(1, 1)) != 0 || tangent[1].Sign() != 0 || tangent[2].Cmp(big.NewRat(int64(1+2*side), 4)) != 0 {
				t.Fatal("original one-sided derivative was averaged or rescaled", owner, side, tangent)
			}
		}
		proof, err := geom.CompareExactCurvesContext(ctx, lifts[0], lifts[1], geom.ExactCurveComparisonOptions{})
		if err != nil || !proof.Complete || !proof.Equal {
			t.Fatal("one-sided original support seam curves must coincide", owner, err)
		}
	}

	out, err := shell.BuildRectangularSolid(ctx, 1e-9, geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Solid.Faces()) != 7 || len(out.Solid.Edges()) != 19 || len(out.Solid.Vertices()) != 14 {
		t.Fatal("lost the true roof knot events on the original closing edges")
	}
	var native bytes.Buffer
	if err := WriteJSON(&native, out.Solid); err != nil {
		t.Fatal(err)
	}
	solid, err := ReadJSON(&native, JSONLimits{})
	if err != nil {
		t.Fatal(err)
	}
	all, err := solid.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if err != nil || !all.Complete || len(all.Uses) != 38 {
		t.Fatal("original owner agreement on every native coedge", err)
	}
	faces := solid.Faces()
	for i := range out.ChainEdges {
		a, b := faces[0].Outer[i], faces[1].Outer[i]
		model, err := ExactTrimModelBezier(ctx, faces[0].Surface, a.Curve, false, geom.SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		owner := b.Curve.(MappedSplineUV)
		lift, err := ProveMappedSplineTransfer(ctx, faces[1].Surface, owner.Source(), model, geom.SurfacePatchOptions{})
		if err != nil || lift.ParameterWeight().Cmp(owner.ParameterWeight()) != 0 {
			t.Fatal("original spline coedge transfer across roof knot cells", i, err)
		}
	}
	crossing, err := NewExactQuadraticUV([3][3]*big.Rat{
		{new(big.Rat), big.NewRat(3, 4), big.NewRat(1, 1)},
		{big.NewRat(1, 2), big.NewRat(3, 4), big.NewRat(1, 1)},
		{big.NewRat(1, 1), big.NewRat(3, 4), big.NewRat(1, 1)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if a, err := ExactTrimModelBezier(ctx, faces[0].Surface, crossing, false, geom.SurfacePatchOptions{}); a != nil ||
		!errors.Is(err, geom.ErrConvergence) || !strings.Contains(err.Error(), "spans support knot cells") {
		t.Fatal("unresolved cross-knot trim must fail closed", err)
	}
	mass, err := solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
	// Integral of 1+x*y+(x-1/2)_+/2 over the exact rectangular footprint.
	if err != nil || math.Abs(mass.Volume-15.0/16) > 1e-8 {
		t.Fatal("original creased-roof mass", mass.Volume, err)
	}
	face := MixedExactMultiSpanMeshOptions{Face: MixedExactCurvedMeshOptions{
		BoundaryModelBudget: .005,
		Interior:            CurvedQuadraticTrimMeshOptions{Deflection: .055},
	}}
	outMesh, err := shell.TessellateClosedCertified(ctx, nil, MappedChainClosedMeshOptions{
		Boundary: MappedChainPolylineOptions{Deflection: .005},
		Curved:   [2]MixedExactMultiSpanMeshOptions{face, face},
	})
	if err != nil || !outMesh.DisplayCollision.Complete || len(outMesh.DisplayCollision.Contacts) != 0 {
		t.Fatal("closed representation across original roof knot", err)
	}
	t.Logf("crease: 7 faces 19 edges 14 vertices, exact volume %.12g, closed display %d triangles", mass.Volume, len(outMesh.Mesh.Triangles))
}
