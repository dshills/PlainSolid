package trimmed

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
)

func TestWrappedCylinderIntersectionTopology(t *testing.T) {
	ctx := context.Background()
	a := wrappedOverlapSupport(t)
	b, _, e := a.InsertKnot(ctx, 0, .125, 1, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	ar := [2][2]*big.Rat{{big.NewRat(0, 1), big.NewRat(3, 1)}, {big.NewRat(-1, 1), big.NewRat(2, 1)}}
	br := [2][2]*big.Rat{{big.NewRat(2, 1), big.NewRat(5, 1)}, {big.NewRat(0, 1), big.NewRat(3, 1)}}
	for _, reverse := range []bool{false, true} {
		target := b
		rectB := br
		opts := WrappedCylinderIntersectionOptions{}
		if reverse {
			target, _, e = b.Reparameterize(ctx, geom.ParameterRectangle{UMin: 2, UMax: 10, VMin: -3, VMax: 7}, true, false, geom.SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
			opts.NormalizeDomains = true
			opts.ReverseBU = true
			rectB = [2][2]*big.Rat{{big.NewRat(0, 1), big.NewRat(6, 1)}, {big.NewRat(1, 1), big.NewRat(7, 1)}}
		}
		r, e := NewWrappedCylinderIntersectionTopology(ctx, a, target, ar, rectB, opts)
		if e != nil {
			t.Fatal(e)
		}
		orientation := 1
		if reverse {
			orientation = -1
		}
		if r.OwnerOrientations() != [2]int{1, orientation} {
			t.Fatal("lost owner orientation")
		}
		seen := map[geom.Vec3]bool{}
		for ci, graph := range r.Components() {
			if graph == nil || len(graph.Vertices()) != 4 || len(graph.Edges()) != 4 || len(graph.Faces()) != 1 || len(graph.BoundaryCycles()) != 1 || wrappedCycleArea(graph.Exterior()).Cmp(big.NewRat(2, 1)) != 0 {
				t.Fatal("wrong native component", ci)
			}
			for _, p := range graph.Vertices() {
				if seen[p] {
					t.Fatal("disjoint components welded")
				}
				seen[p] = true
			}
			for _, list := range graph.EdgeAncestry() {
				if len(list) != 0 {
					t.Fatal("invented native source edge IDs")
				}
			}
			open := Solid{vertices: graph.Vertices(), edges: graph.Edges(), faces: graph.Faces(), tolerance: 1e-9}
			agreement, e := open.ValidateEdgeAgreement(ctx, edgecheck.Options{})
			if e != nil || !agreement.Complete {
				t.Fatal("native component edge proof", e)
			}
			history := graph.CellAncestry()
			if history[0].Owners != 3 || history[0].SourceCell != ci {
				t.Fatal("component owner")
			}
			sides := r.RectangleBoundaryAncestry(ci)
			if len(sides) != 4 {
				t.Fatal("missing conceptual sides")
			}
			for edgeID, list := range sides {
				if len(list) == 0 {
					t.Fatal("unowned component side")
				}
				edge := graph.Edges()[edgeID]
				for _, owner := range list {
					bounds := ar
					support := a
					if owner.Owner == 1 {
						bounds = rectB
						support = target
					}
					corners := [4][2]*big.Rat{{bounds[0][0], bounds[1][0]}, {bounds[0][1], bounds[1][0]}, {bounds[0][1], bounds[1][1]}, {bounds[0][0], bounds[1][1]}}
					for end, parameter := range owner.Parameters {
						var uv [2]*big.Rat
						for axis := 0; axis < 2; axis++ {
							start, finish := corners[owner.Side][axis], corners[(owner.Side+1)%4][axis]
							uv[axis] = new(big.Rat).Add(start, new(big.Rat).Mul(parameter, new(big.Rat).Sub(finish, start)))
							if uv[axis].Cmp(owner.UV[end][axis]) != 0 {
								t.Fatal("wrong normalized source side unit")
							}
						}
						jet, e := support.EvaluateRational(ctx, uv[0], uv[1], geom.SurfacePatchEvaluationOptions{})
						if e != nil {
							t.Fatal(e)
						}
						point, e := geom.RepresentedExactPoint(jet.Point)
						if e != nil {
							t.Fatal(e)
						}
						id := edge.Start
						if end == 1 {
							id = edge.End
						}
						if point != graph.Vertices()[id] {
							t.Fatal("different source side model endpoint")
						}
					}
				}
			}
			sides[0][0].Parameters[0].SetInt64(999)
			sides[0][0].UV[0][0].SetInt64(999)
			if r.RectangleBoundaryAncestry(ci)[0][0].Parameters[0].Cmp(big.NewRat(999, 1)) == 0 {
				t.Fatal("mutable owner storage")
			}
		}
		shifts := r.PeriodShifts()
		if shifts[0][0].Cmp(shifts[1][0]) == 0 {
			t.Fatal("period-shift ancestry collapsed")
		}
		shifts[0][0].SetInt64(999)
		if r.PeriodShifts()[0][0].Int64() == 999 {
			t.Fatal("mutable shift storage")
		}
		opts.MaxOperations = r.ExactOperations() - 1
		if out, e := NewWrappedCylinderIntersectionTopology(ctx, a, target, ar, rectB, opts); e == nil || out != nil {
			t.Fatal("partially published components", e)
		}
	}
	// The exact quotient proof accepts 10/3; native UV storage must not round it.
	nonbinary := ar
	nonbinary[0][1] = big.NewRat(10, 3)
	relation, _, e := geom.CompareWrappedSurfaceCharts(ctx, a, b, geom.ExactSurfaceComparisonOptions{})
	if e != nil {
		t.Fatal(e)
	}
	proof, e := relation.CertifyCylinderTwoRectangleOverlap(ctx, nonbinary, br, geom.WrappedCylinderTwoOverlapOptions{})
	if e != nil || !proof.Complete {
		t.Fatal("exact fixture not qualified", e)
	}
	if out, e := NewWrappedCylinderIntersectionTopology(ctx, a, b, nonbinary, br, WrappedCylinderIntersectionOptions{}); !errors.Is(e, geom.ErrUnrepresentable) || out != nil {
		t.Fatal("nonbinary source cut silently rounded", e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if out, e := NewWrappedCylinderIntersectionTopology(canceled, a, b, ar, br, WrappedCylinderIntersectionOptions{}); e == nil || out != nil {
		t.Fatal("canceled components", e)
	}
}
