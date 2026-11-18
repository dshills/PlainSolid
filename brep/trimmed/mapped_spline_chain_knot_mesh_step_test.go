package trimmed_test

import (
	"context"
	"math/big"
	"os"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/brep/trimmed"
	"plainsolid/collision"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func orderedMappedDisplaySTEP(t *testing.T, ctx context.Context) step.MappedChainSTEPModel {
	t.Helper()
	in, err := os.Open("../../exchange/step/testdata/occt_mapped_chain_ordered_joined_independent_roofs.step")
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	model, err := step.ReadOCCTMappedChainRectangularShell(ctx, in, step.Limits{}, step.MappedChainShellSTEPOptions{
		AllowMultiSpanRoofs: true, AllowTwoInteriorRoofKnots: true, AllowUnsplitSupportKnot: true,
		AllowMixedOwnerBases: true, AllowJoinedSupportKnotCells: true, AllowOrderedJoinedSupportKnotCells: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return model
}

func TestOrderedMappedSTEPKnotAlignedDisplay(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	model := orderedMappedDisplaySTEP(t, ctx)
	fill := trimmed.MixedExactMultiSpanMeshOptions{
		Span: trimmed.MultiSpanTriangleBoundOptions{MaxDegree: 4}, MaxResidualOperations: 512 << 20,
		Face: trimmed.MixedExactCurvedMeshOptions{
			BoundaryModelBudget: .065,
			Interior:            trimmed.CurvedQuadraticTrimMeshOptions{Deflection: .30, MaxSubdivisions: 64},
		},
	}
	out, err := model.Shell.TessellateClosedCertified(ctx, nil, trimmed.MappedChainClosedMeshOptions{
		AlignSupportKnots: true, MaxTriangles: 6000,
		Boundary:  trimmed.MappedChainPolylineOptions{Deflection: .01875},
		Curved:    [2]trimmed.MixedExactMultiSpanMeshOptions{fill, fill},
		Collision: collision.Options{MaxPairs: 4_000_000},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Mesh.Triangles) != 4162 || len(out.ChainJoints) != 3 ||
		out.SquaredDistance.Cmp(big.NewRat(9, 100)) > 0 ||
		!out.DisplayCollision.Complete || len(out.DisplayCollision.Contacts) != 0 {
		t.Fatal("authored knot geometry should be closed within the tighter bound", len(out.Mesh.Triangles), out.SquaredDistance.FloatString(9))
	}
	if err := out.Mesh.ValidateSolid(); err != nil {
		t.Fatal(err)
	}
	t.Logf("triangles=%d distance²=%s pairs=%d rounds=%d", len(out.Mesh.Triangles),
		out.SquaredDistance.FloatString(12), out.DisplayCollision.BroadPhasePairs, out.RefinementRounds)
	for owner, roof := range out.Curved {
		patch, err := geom.NewSurfacePatch(model.Native.Solid.Faces()[owner].Surface.(*geom.BSplineSurface), geom.SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		us, vs := patch.KnotBreaks()
		axis, knots := 0, us
		if len(vs) > len(us) {
			axis, knots = 1, vs
		}
		cells := make([]*geom.SurfacePatch, len(knots)-1)
		d := patch.Domain()
		for i := range cells {
			if axis == 0 {
				d.UMin, d.UMax = knots[i], knots[i+1]
			} else {
				d.VMin, d.VMax = knots[i], knots[i+1]
			}
			cells[i], err = patch.Trim(d)
			if err != nil {
				t.Fatal(err)
			}
		}
		edges := make(map[[2]int]int)
		usedKnots := make(map[int]bool)
		creased, exceedsDefault := false, false
		for triangle, tri := range roof.Mesh.Triangles {
			uv := [3][2]*big.Rat{roof.ExactUV[tri[0]], roof.ExactUV[tri[1]], roof.ExactUV[tri[2]]}
			i := -1
			for span := range cells {
				lo, hi := new(big.Rat).SetFloat64(knots[span]), new(big.Rat).SetFloat64(knots[span+1])
				if uv[0][axis].Cmp(lo) >= 0 && uv[1][axis].Cmp(lo) >= 0 && uv[2][axis].Cmp(lo) >= 0 &&
					uv[0][axis].Cmp(hi) <= 0 && uv[1][axis].Cmp(hi) <= 0 && uv[2][axis].Cmp(hi) <= 0 {
					i = span
					break
				}
			}
			if i < 0 || out.RoofSupportCells[owner][triangle][axis] != i ||
				out.RoofSupportCells[owner][triangle][1-axis] != 0 {
				t.Fatal("a geometric triangle crosses an original knot or lost its original support slot", owner, tri, i)
			}
			if !exceedsDefault {
				a, b, c := exactMeshVector(roof.Mesh.Vertices[tri[0]]), exactMeshVector(roof.Mesh.Vertices[tri[1]]), exactMeshVector(roof.Mesh.Vertices[tri[2]])
				facet := exactMeshCross(exactMeshSub(b, a), exactMeshSub(c, a))
				u, v := new(big.Rat), new(big.Rat)
				for _, p := range uv {
					u.Add(u, p[0])
					v.Add(v, p[1])
				}
				u.Quo(u, big.NewRat(3, 1))
				v.Quo(v, big.NewRat(3, 1))
				j, err := cells[i].EvaluateRational(ctx, u, v, geom.SurfacePatchEvaluationOptions{})
				if err != nil {
					t.Fatal(err)
				}
				normal := exactMeshCross(j.Du, j.Dv)
				dot := exactMeshDot(facet, normal)
				off := exactMeshCross(facet, normal)
				// sin(.2) <= .2 and cos(.2) >= .98 imply tan(.2) <= 10/49.
				upper := new(big.Rat).Mul(big.NewRat(100, 2401), new(big.Rat).Mul(dot, dot))
				exceedsDefault = dot.Sign() <= 0 || exactMeshDot(off, off).Cmp(upper) > 0
			}
			for side, a := range tri {
				b := tri[(side+1)%3]
				key := [2]int{min(a, b), max(a, b)}
				other, paired := edges[key]
				edges[key] = i
				if !paired || other == i {
					continue
				}
				knot := max(i, other)
				station := new(big.Rat).SetFloat64(knots[knot])
				p, q := roof.ExactUV[a], roof.ExactUV[b]
				if p[axis].Cmp(station) != 0 || q[axis].Cmp(station) != 0 || max(i, other)-min(i, other) != 1 {
					t.Fatal("disjoint source spans cannot share a display edge", owner, i, other)
				}
				if usedKnots[knot] {
					continue
				}
				usedKnots[knot] = true
				u := new(big.Rat).Quo(new(big.Rat).Add(p[0], q[0]), big.NewRat(2, 1))
				v := new(big.Rat).Quo(new(big.Rat).Add(p[1], q[1]), big.NewRat(2, 1))
				left, aErr := cells[knot-1].EvaluateRational(ctx, u, v, geom.SurfacePatchEvaluationOptions{})
				right, bErr := cells[knot].EvaluateRational(ctx, u, v, geom.SurfacePatchEvaluationOptions{})
				if aErr != nil || bErr != nil {
					t.Fatal(aErr, bErr)
				}
				for k := range left.Point {
					if left.Point[k].Cmp(right.Point[k]) != 0 {
						t.Fatal("adjacent one-sided supports must lift the same edge point", owner, knot)
					}
				}
				difference := exactMeshCross(exactMeshCross(left.Du, left.Dv), exactMeshCross(right.Du, right.Dv))
				creased = creased || exactMeshDot(difference, difference).Sign() > 0
			}
		}
		if len(usedKnots) != len(knots)-2 || !creased || !exceedsDefault {
			t.Fatal("fixture must exercise every original cut, distinct one-sided derivatives and a truly excessive facet angle",
				owner, usedKnots, creased, exceedsDefault)
		}
	}
}

func exactMeshVector(p geom.Vec3) [3]*big.Rat {
	return [3]*big.Rat{new(big.Rat).SetFloat64(p.X), new(big.Rat).SetFloat64(p.Y), new(big.Rat).SetFloat64(p.Z)}
}
func exactMeshSub(a, b [3]*big.Rat) (out [3]*big.Rat) {
	for k := range out {
		out[k] = new(big.Rat).Sub(a[k], b[k])
	}
	return out
}
func exactMeshCross(a, b [3]*big.Rat) (out [3]*big.Rat) {
	for k := range out {
		i, j := (k+1)%3, (k+2)%3
		out[k] = new(big.Rat).Sub(new(big.Rat).Mul(a[i], b[j]), new(big.Rat).Mul(a[j], b[i]))
	}
	return out
}
func exactMeshDot(a, b [3]*big.Rat) *big.Rat {
	out := new(big.Rat)
	for i := range a {
		out.Add(out, new(big.Rat).Mul(a[i], b[i]))
	}
	return out
}

func TestOrderedMappedSTEPProjectedQuality(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	model := orderedMappedDisplaySTEP(t, ctx)
	fill := trimmed.MixedExactMultiSpanMeshOptions{
		Span: trimmed.MultiSpanTriangleBoundOptions{MaxDegree: 4}, MaxResidualOperations: 512 << 20,
		Face: trimmed.MixedExactCurvedMeshOptions{
			BoundaryModelBudget: .065,
			Interior:            trimmed.CurvedQuadraticTrimMeshOptions{Deflection: .30, MaxSubdivisions: 64},
		},
	}
	out, err := model.Shell.TessellateClosedCertified(ctx, nil, trimmed.MappedChainClosedMeshOptions{
		AlignSupportKnots: true, MaxKnotCellFlips: 100000, MaxTriangles: 3000,
		Boundary:  trimmed.MappedChainPolylineOptions{Deflection: .01875},
		Curved:    [2]trimmed.MixedExactMultiSpanMeshOptions{fill, fill},
		Collision: collision.Options{MaxPairs: 4_000_000},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := out.Mesh.ValidateSolid(); err != nil {
		t.Fatal(err)
	}
	if len(out.Mesh.Triangles) != 1914 || len(out.ChainJoints) != 3 ||
		!out.DisplayCollision.Complete || len(out.DisplayCollision.Contacts) != 0 ||
		out.SquaredDistance.Cmp(big.NewRat(1, 25)) >= 0 {
		t.Fatal("projected triangulation must retain the closed source with distance below .2 and fewer facets", len(out.Mesh.Triangles))
	}
	t.Logf("triangles=%d distance²=%s pairs=%d", len(out.Mesh.Triangles), out.SquaredDistance.FloatString(12), out.DisplayCollision.BroadPhasePairs)
	owner := 1
	roof := out.Curved[owner]
	p, err := geom.NewSurfacePatch(model.Native.Solid.Faces()[owner].Surface.(*geom.BSplineSurface), geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	u, v := p.KnotBreaks()
	verifyMappedProtectedCoedgeAngle(t, ctx, p, roof)
	cells := make(map[[2]int]*geom.SurfacePatch)
	for i, tri := range roof.Mesh.Triangles {
		cell := out.RoofSupportCells[owner][i]
		original, found := cells[cell]
		if !found {
			original, err = p.Trim(geom.ParameterRectangle{
				UMin: u[cell[0]], UMax: u[cell[0]+1], VMin: v[cell[1]], VMax: v[cell[1]+1],
			})
			if err != nil {
				t.Fatal(err)
			}
			cells[cell] = original
		}
		a, b, c := exactMeshVector(roof.Mesh.Vertices[tri[0]]), exactMeshVector(roof.Mesh.Vertices[tri[1]]), exactMeshVector(roof.Mesh.Vertices[tri[2]])
		facet := exactMeshCross(exactMeshSub(b, a), exactMeshSub(c, a))
		for _, id := range tri {
			uv := roof.ExactUV[id]
			j, err := original.EvaluateRational(ctx, uv[0], uv[1], geom.SurfacePatchEvaluationOptions{})
			if err != nil {
				t.Fatal(err)
			}
			normal := exactMeshCross(j.Du, j.Dv)
			dot := exactMeshDot(facet, normal)
			off := exactMeshCross(facet, normal)
			// sin(.35) <= .35 and cos(.35) >= 1-.35²/2 bound tan(.35) above by 280/751.
			upper := new(big.Rat).Mul(big.NewRat(280*280, 751*751), new(big.Rat).Mul(dot, dot))
			if dot.Sign() <= 0 || exactMeshDot(off, off).Cmp(upper) > 0 {
				t.Logf("roof %d original cell %v triangle %d exact .35-radian counterexample at %s,%s", owner, cell, i, uv[0], uv[1])
				return
			}
		}
	}
	t.Fatal("the distance-only projected mesh must still exercise an exact excessive angle")
}

func verifyMappedProtectedCoedgeAngle(t *testing.T, ctx context.Context, p *geom.SurfacePatch, roof trimmed.CertifiedMixedExactCurvedMesh) {
	t.Helper()
	var boundaryEdge bool
	for _, loop := range roof.BoundaryLoops {
		for i, a := range loop {
			x, y := roof.ExactUV[a], roof.ExactUV[loop[(i+1)%len(loop)]]
			if x[1].Cmp(y[1]) > 0 {
				x, y = y, x
			}
			boundaryEdge = boundaryEdge || (x[0].Sign() == 0 && y[0].Sign() == 0 &&
				x[1].Cmp(big.NewRat(1, 4)) == 0 && y[1].Cmp(big.NewRat(3, 4)) == 0)
		}
	}
	if !boundaryEdge {
		t.Fatal("the projected fallback must retain the original unsplit closing display edge")
	}
	knots, _ := p.KnotBreaks()
	d := p.Domain()
	d.UMax = knots[1]
	oneSide, err := p.Trim(d)
	if err != nil {
		t.Fatal(err)
	}
	var normals [2][3]*big.Rat
	for i, y := range []*big.Rat{big.NewRat(1, 4), big.NewRat(19, 32)} {
		j, err := oneSide.EvaluateRational(ctx, new(big.Rat), y, geom.SurfacePatchEvaluationOptions{})
		if err != nil {
			t.Fatal(err)
		}
		normals[i] = exactMeshCross(j.Du, j.Dv)
	}
	dot := exactMeshDot(normals[0], normals[1])
	cross := exactMeshCross(normals[0], normals[1])
	// The alternating sine upper bound at .7 is <13/20, and cosine
	// is >3/4 by 1-.7²/2. Thus tan(.7)<13/15. The geodesic triangle
	// inequality rules out any single facet within .35 of both normals.
	if dot.Sign() <= 0 || exactMeshDot(cross, cross).Cmp(new(big.Rat).Mul(big.NewRat(169, 225), new(big.Rat).Mul(dot, dot))) <= 0 {
		t.Fatal("the unsplit closing edge must require angular subdivision", normals)
	}
	t.Logf("protected closing edge has source normal separation >.7 at raw (0,1/4) and (0,19/32)")
}

func TestOrderedMappedSTEPFiniteAngleDisplay(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	model := orderedMappedDisplaySTEP(t, ctx)
	fill := trimmed.MixedExactMultiSpanMeshOptions{
		Span: trimmed.MultiSpanTriangleBoundOptions{MaxDegree: 4}, MaxResidualOperations: 850000000,
		Face: trimmed.MixedExactCurvedMeshOptions{
			BoundaryModelBudget: .065,
			Interior:            trimmed.CurvedQuadraticTrimMeshOptions{Deflection: .30, MaxSubdivisions: 128},
		},
	}
	out, err := model.Shell.TessellateClosedCertified(ctx, nil, trimmed.MappedChainClosedMeshOptions{
		AlignSupportKnots: true, MaxKnotCellFlips: 100000, NormalAngle: .35, MaxTriangles: 3000,
		Boundary:  trimmed.MappedChainPolylineOptions{Deflection: .01875},
		Curved:    [2]trimmed.MixedExactMultiSpanMeshOptions{fill, fill},
		Collision: collision.Options{MaxPairs: 4_000_000},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := out.Mesh.ValidateSolid(); err != nil {
		t.Fatal(err)
	}
	a := big.NewRat(7, 20)
	limit := new(big.Rat).Mul(new(big.Rat).Mul(a, a), a)
	limit.Quo(limit, big.NewRat(3, 1))
	limit.Add(limit, a)
	limit.Mul(limit, limit)
	if len(out.Mesh.Triangles) != 2126 || len(out.ChainJoints) != 3 || len(out.Planar) != 5 ||
		out.SquaredDistance.Cmp(big.NewRat(81, 10000)) >= 0 ||
		out.SquaredNormalTangent == nil || out.SquaredNormalTangent.Cmp(limit) > 0 ||
		!out.DisplayCollision.Complete || len(out.DisplayCollision.Contacts) != 0 {
		t.Fatal("finite angular request must preserve closed original ownership and prove both bounds", len(out.Mesh.Triangles))
	}
	for i, roof := range out.Curved {
		if roof.SquaredNormalTangent == nil || roof.SquaredNormalTangent.Cmp(out.SquaredNormalTangent) > 0 ||
			len(out.RoofSupportCells[i]) != len(roof.Mesh.Triangles) {
			t.Fatal("lost one-sided roof cone or original support cell", i)
		}
		t.Logf("roof=%d triangles=%d tangent²<=%s interior passes=%d", i, len(roof.Mesh.Triangles),
			roof.SquaredNormalTangent.FloatString(12), roof.InteriorRefinementPasses)
	}
	t.Logf("shell triangles=%d distance²=%s tangent²<=%s pairs=%d seam rounds=%d", len(out.Mesh.Triangles),
		out.SquaredDistance.FloatString(12), out.SquaredNormalTangent.FloatString(12), out.DisplayCollision.BroadPhasePairs, out.RefinementRounds)
}

func TestOrderedMappedSTEPNormalTurnDisplay(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	model := orderedMappedDisplaySTEP(t, ctx)
	fill := trimmed.MixedExactMultiSpanMeshOptions{
		Span: trimmed.MultiSpanTriangleBoundOptions{MaxDegree: 4}, MaxResidualOperations: 1000000000,
		Face: trimmed.MixedExactCurvedMeshOptions{
			BoundaryModelBudget: .065,
			Interior:            trimmed.CurvedQuadraticTrimMeshOptions{Deflection: .30, MaxSubdivisions: 128},
		},
	}
	out, err := model.Shell.TessellateClosedCertified(ctx, nil, trimmed.MappedChainClosedMeshOptions{
		AlignSupportKnots: true, MaxKnotCellFlips: 100000, NormalTurn: .35, MaxTriangles: 10000,
		Boundary:  trimmed.MappedChainPolylineOptions{Deflection: .01875},
		Curved:    [2]trimmed.MixedExactMultiSpanMeshOptions{fill, fill},
		Collision: collision.Options{MaxPairs: 4_000_000},
	})
	if err != nil {
		t.Fatal(err)
	}
	half := big.NewRat(7, 40)
	tanHalf := new(big.Rat).Mul(new(big.Rat).Mul(half, half), half)
	tanHalf.Quo(tanHalf, big.NewRat(3, 1))
	tanHalf.Add(tanHalf, half)
	q := new(big.Rat).Mul(tanHalf, tanHalf)
	d := new(big.Rat).Sub(big.NewRat(1, 1), q)
	limit := new(big.Rat).Quo(new(big.Rat).Mul(big.NewRat(4, 1), q), d.Mul(d, d))
	if out.SquaredNormalTurnTangent == nil || out.SquaredNormalTurnTangent.Cmp(limit) > 0 ||
		out.SquaredDistance.Cmp(big.NewRat(9, 100)) > 0 || len(out.ChainJoints) != 3 ||
		len(out.Planar) != 5 || !out.DisplayCollision.Complete || len(out.DisplayCollision.Contacts) != 0 ||
		out.Mesh.ValidateSolid() != nil {
		t.Fatal("original two-sided distance and full source normal-turn bounds must form one verified closed shell")
	}

	roof := out.Curved[1]
	closing := roof.UVRegion.Samples[len(roof.UVRegion.Samples)-1]
	wall := model.Native.Embedding.WallFaces[0]
	wallVertices := make(map[int]bool)
	for _, id := range out.FaceVertices[wall] {
		wallVertices[id] = true
	}
	want := []*big.Rat{big.NewRat(13, 16), big.NewRat(7, 8), big.NewRat(15, 16)}
	for _, station := range want {
		found := false
		for k, actual := range closing.Parameters {
			if station.Cmp(actual) != 0 {
				continue
			}
			local := closing.Vertices[k]
			global := out.FaceVertices[1][local]
			x, y := roof.ExactUV[local][0], roof.ExactUV[local][1]
			s := new(big.Rat).Sub(big.NewRat(3, 4), new(big.Rat).Quo(station, big.NewRat(2, 1)))
			if x.Sign() != 0 || y.Cmp(s) != 0 || !wallVertices[global] {
				t.Fatal("the original closing station and its planar mate must share one exact physical vertex", station, global)
			}
			found = true
		}
		if !found {
			t.Fatal("lost the obstructing closing edge's original station", station)
		}
	}
	t.Logf("shell triangles=%d distance²=%s normal-turn tangent²<=%s contacts=%d seam rounds=%d",
		len(out.Mesh.Triangles), out.SquaredDistance.FloatString(12), out.SquaredNormalTurnTangent.FloatString(12),
		out.DisplayCollision.BroadPhasePairs, out.RefinementRounds)
}
