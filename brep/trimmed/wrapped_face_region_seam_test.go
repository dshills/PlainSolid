//go:build research

package trimmed

import (
	"context"
	"math/big"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
)

func TestWrappedFaceRegionsTrueSeam(t *testing.T) {
	ctx := context.Background()
	w := wrappedOverlapSupport(t)
	refined, _, e := w.InsertKnot(ctx, 0, .125, 1, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	opts := WrappedRectangleFaceOverlapOptions{AllowSeamCrossing: true, ProjectionAxes: [2]int{1, 2}}
	for _, reversed := range []bool{false, true} {
		a := wrappedOverlapFace(t, w, geom.ParameterRectangle{UMin: -.25, UMax: .25, VMin: -1, VMax: 1})
		a.Face.Reversed = reversed
		b := wrappedOverlapFace(t, refined, geom.ParameterRectangle{UMin: 3.875, UMax: 4.375, VMin: 0, VMax: 2})
		b.Face.Reversed = !reversed
		legacy := opts
		legacy.AllowSeamCrossing = false
		if out, e := CertifyWrappedRectangleFaceOverlap(ctx, a, b, legacy); e == nil || out.Complete {
			t.Fatal("legacy seam guard changed", e)
		}
		proof, e := CertifyWrappedRectangleFaceOverlap(ctx, a, b, opts)
		if e != nil || !proof.Complete || proof.LiftedEmbedding == nil || !proof.LiftedEmbedding.Embedding.ProvenInjective || proof.Embedding.Arrangement.Complete {
			t.Fatal("missing distinct lifted proof", e)
		}
		lifted := proof.LiftedEmbedding.Embedding
		if lifted.FundamentalPieces != 2 || lifted.ProjectionOrientation != 1 || lifted.Domain.UMin != -.25 || lifted.Domain.UMax != .375 {
			t.Fatal(lifted)
		}
		// Original native edges genuinely cross the seam, with exact dyadic
		// parameter witnesses. They retain the wrapper, not a spliced approximation.
		for owner, f := range []SewFace{a, b} {
			parameter := .5
			if owner == 1 {
				parameter = .25
			}
			point, e := f.Edges[0].Curve.Eval(parameter)
			if e != nil {
				t.Fatal(e)
			}
			v := f.Face.Outer[0].UVStart.Y
			jet, e := w.EvaluateRational(ctx, big.NewRat(0, 1), ratInset(v), geom.SurfacePatchEvaluationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			exact, e := geom.RepresentedExactPoint(jet.Point)
			if e != nil || point != exact {
				t.Fatal("native seam evaluator", owner, point, exact, e)
			}
		}
		for _, selection := range []WrappedFaceRegionSelection{WrappedFaceRegionUnion, WrappedFaceRegionIntersection, WrappedFaceRegionDifference} {
			r, e := NewWrappedFaceRegionTopology(ctx, a, b, selection, opts)
			if e != nil || r.Empty() {
				t.Fatal(selection, e)
			}
			// Exercise the established native edge agreement validator only; this
			// private open container is not published or validated as a Solid.
			open := Solid{vertices: r.Vertices(), edges: r.Edges(), faces: r.Faces(), tolerance: 1e-9}
			agreement, err := open.ValidateEdgeAgreement(ctx, edgecheck.Options{})
			if err != nil || !agreement.Complete {
				t.Fatal("native continuous edge proof rejected seam", err)
			}
			want := big.NewRat(13, 8)
			if selection == WrappedFaceRegionIntersection {
				want = big.NewRat(3, 8)
			}
			if selection == WrappedFaceRegionDifference {
				want = big.NewRat(5, 8)
			}
			if reversed {
				want.Neg(want)
			}
			if len(r.BoundaryCycles()) != 1 || wrappedCycleArea(r.Exterior()).Cmp(want) != 0 {
				t.Fatal("wrong seam region orientation", selection, wrappedCycleArea(r.Exterior()), want)
			}
			for _, edge := range r.Edges() {
				line, ok := analyticModelLine(edge)
				if !ok || line.surface != w {
					t.Fatal("lost original model support")
				}
				for _, parameter := range []float64{0, .25, .5, .75, 1} {
					var uv [2]*big.Rat
					for axis := 0; axis < 2; axis++ {
						uv[axis] = new(big.Rat).Add(line.uv[0][axis], new(big.Rat).Mul(ratInset(parameter), new(big.Rat).Sub(line.uv[1][axis], line.uv[0][axis])))
					}
					jet, e := w.EvaluateRational(ctx, uv[0], uv[1], geom.SurfacePatchEvaluationOptions{})
					if e != nil {
						t.Fatal(e)
					}
					exact, e := geom.RepresentedExactPoint(jet.Point)
					if e != nil {
						t.Fatal(e)
					}
					native, e := edge.Curve.Eval(parameter)
					if e != nil || native != exact {
						t.Fatal("native/exact edge disagreement", native, exact, e)
					}
				}
			}
			for _, cell := range r.CellAncestry() {
				if cell.Owners&2 != 0 {
					for _, uv := range cell.OwnerUV[1] {
						if uv[0].Cmp(big.NewRat(31, 8)) < 0 || uv[0].Cmp(big.NewRat(35, 8)) > 0 {
							t.Fatal("B chart was canonicalized", uv)
						}
					}
				}
			}
			bad := opts
			bad.MaxOperations = r.ExactOperations() - 1
			if out, e := NewWrappedFaceRegionTopology(ctx, a, b, selection, bad); e == nil || out != nil {
				t.Fatal("partial seam topology", e)
			}
		}
		bad := opts
		bad.ProjectionAxes = [2]int{0, 1}
		if out, e := NewWrappedFaceRegionTopology(ctx, a, b, WrappedFaceRegionUnion, bad); e == nil || out != nil {
			t.Fatal("singular projection accepted", e)
		}
	}
}

func TestWrappedFaceRegionsExactSeamEndpoints(t *testing.T) {
	ctx := context.Background()
	w := wrappedOverlapSupport(t)
	a := wrappedOverlapFace(t, w, geom.ParameterRectangle{UMin: -.25, UMax: .25, VMin: -1, VMax: 1})
	b := wrappedOverlapFace(t, w, geom.ParameterRectangle{UMin: 4, UMax: 4.5, VMin: 0, VMax: 2})
	opts := WrappedRectangleFaceOverlapOptions{AllowSeamCrossing: true, ProjectionAxes: [2]int{1, 2}}
	r, e := NewWrappedFaceRegionTopology(ctx, a, b, WrappedFaceRegionUnion, opts)
	if e != nil {
		t.Fatal(e)
	}
	atSeam := 0
	for _, face := range r.Faces() {
		for _, u := range face.Outer {
			if u.UVStart.X == 0 {
				atSeam++
				edge := r.Edges()[u.Edge]
				id := edge.Start
				if u.Reversed {
					id = edge.End
				}
				jet, e := w.EvaluateRational(ctx, big.NewRat(4, 1), ratInset(u.UVStart.Y), geom.SurfacePatchEvaluationOptions{})
				if e != nil {
					t.Fatal(e)
				}
				point, e := geom.RepresentedExactPoint(jet.Point)
				if e != nil || point != r.Vertices()[id] {
					t.Fatal("seam endpoint has inconsistent global vertex")
				}
			}
		}
	}
	if atSeam < 2 {
		t.Fatal("seam endpoint fixture missing")
	}
	// A real hole can straddle the same seam; all two-owner ancestry remains
	// lifted even though the inner physical boundary traverses both sides.
	a = wrappedOverlapFace(t, w, geom.ParameterRectangle{UMin: -.25, UMax: .5, VMin: -1, VMax: 2})
	b = wrappedOverlapFace(t, w, geom.ParameterRectangle{UMin: 3.875, UMax: 4.25, VMin: 0, VMax: 1})
	r, e = NewWrappedFaceRegionTopology(ctx, a, b, WrappedFaceRegionDifference, opts)
	if e != nil || len(r.BoundaryCycles()) != 2 || wrappedCycleArea(r.BoundaryCycles()[1]).Cmp(big.NewRat(-3, 8)) != 0 {
		t.Fatal("seam-crossing hole", e)
	}
}
