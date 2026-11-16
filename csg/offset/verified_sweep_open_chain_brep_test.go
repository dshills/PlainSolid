package offset

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/collision"
	"plainsolid/geom"
)

func TestPlanarBRepFromVerifiedNormalOffsetSweepOpenChain(t *testing.T) {
	for _, control := range []struct {
		name      string
		nonlinear bool
		volume    *big.Rat
	}{
		{"exact_affine", false, big.NewRat(3, 2)},
		{"nonparallel_rational_trapezoids", true, big.NewRat(41, 32)},
	} {
		t.Run(control.name, func(t *testing.T) {
			ctx := context.Background()
			chain, cuts := authoredSweepBRepSource(t, control.nonlinear, false, big.NewRat(1, 8))
			result, err := PlanarBRepFromVerifiedNormalOffsetSweepOpenChain(ctx, chain, VerifiedSweepOpenChainBRepOptions{})
			if err != nil {
				t.Fatal(err)
			}
			checkOriginalSweepBRep(t, chain, result, cuts)
			if result.ExactVolume().Cmp(control.volume) != 0 {
				t.Fatal("native volume differs from independently authored footprint times thickness", result.ExactVolume(), control.volume)
			}
			result.ExactVolume().SetInt64(0)
			faces := result.FaceSources()
			faces[0].Owner = -1
			corners := result.VertexSources(0)
			corners[0].Owner = -1
			if result.FaceSources()[0].Owner != 0 || result.VertexSources(0)[0].Owner != 0 || result.ExactVolume().Cmp(control.volume) != 0 {
				t.Fatal("editing external history changed the certified ancestry")
			}
			t.Logf("24 vertices, 44 edges, 22 original quadrilateral faces, exact signed volume %s", control.volume)
		})
	}
}

func TestPlanarBRepFromVerifiedNormalOffsetSweepOpenChainRefusals(t *testing.T) {
	ctx := context.Background()
	chain, _ := authoredSweepBRepSource(t, false, false, big.NewRat(1, 8))
	curved, _ := authoredSweepBRepSource(t, false, true, big.NewRat(1, 8))
	nonbinary, _ := authoredSweepBRepSource(t, false, false, big.NewRat(1, 10))
	prisms, err := nonbinary.PlanarPrismsRational(ctx, geom.SurfacePatchOptions{})
	if err != nil || len(prisms) != 5 || prisms[0].Corners[0][0][0][2].Cmp(big.NewRat(-1, 10)) != 0 {
		t.Fatal("nonbinary control must be a genuine exactly proved planar image", err, prisms)
	}
	domain, shift := curved.Sweeps()[4].Domain()
	far, err := curved.Sweeps()[4].EvaluateRational(ctx,
		new(big.Rat).SetFloat64(domain.UMax), new(big.Rat).SetFloat64(domain.VMax), new(big.Rat),
		geom.SurfacePatchEvaluationOptions{}, geom.SurfaceNormalOffsetEvaluationOptions{})
	if err != nil || far.Lower[2].Cmp(big.NewRat(1, 512)) != 0 || shift.Upper.Sign() <= 0 {
		t.Fatal("the curved negative must retain an original nonplanar face in a sealed chain", far, err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	for _, control := range []struct {
		name  string
		ctx   context.Context
		chain *geom.VerifiedNormalOffsetSweepOpenChain
		o     VerifiedSweepOpenChainBRepOptions
		err   error
	}{
		{"curved original", ctx, curved, VerifiedSweepOpenChainBRepOptions{}, geom.ErrDomain},
		{"nonbinary world corner", ctx, nonbinary, VerifiedSweepOpenChainBRepOptions{}, ErrPrecision},
		{"source arithmetic", ctx, chain, VerifiedSweepOpenChainBRepOptions{Source: geom.SurfacePatchOptions{MaxOperations: 1}}, geom.ErrConvergence},
		{"collision work", ctx, chain, VerifiedSweepOpenChainBRepOptions{Validation: collision.LocationOptions{MaxCollisionPairs: 1}}, collision.ErrLimit},
		{"cancellation", canceled, chain, VerifiedSweepOpenChainBRepOptions{}, context.Canceled},
	} {
		t.Run(control.name, func(t *testing.T) {
			got, err := PlanarBRepFromVerifiedNormalOffsetSweepOpenChain(control.ctx, control.chain, control.o)
			if !errors.Is(err, control.err) || got.Original() != nil || len(got.Solid().Faces()) != 0 ||
				len(got.FaceSources()) != 0 || len(got.VertexSources(0)) != 0 || got.ExactVolume() != nil {
				t.Fatal("failure published a planar solid or receipt", err)
			}
		})
	}
}

func checkOriginalSweepBRep(t *testing.T, chain *geom.VerifiedNormalOffsetSweepOpenChain, r VerifiedSweepOpenChainBRep, cuts [6][2]int64) {
	t.Helper()
	ctx := context.Background()
	solid := r.Solid()
	if r.Original() != chain {
		t.Fatal("source ownership does not reverse to the original immutable chain")
	}
	if err := solid.Validate(); err != nil {
		t.Fatal(err)
	}
	vertices, edges, faces := solid.Vertices(), solid.Edges(), solid.Faces()
	ancestry := r.FaceSources()
	if len(vertices) != 24 || len(edges) != 44 || len(faces) != 22 || len(ancestry) != len(faces) {
		t.Fatal("unexpected planar chain topology", len(vertices), len(edges), len(faces))
	}
	locator, err := collision.NewPointLocator(ctx, solid, collision.LocationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	shells, contacts := locator.Shells(), locator.BoundaryCollision()
	if len(shells) != 1 || shells[0].Depth != 0 || shells[0].Parent != -1 || shells[0].SignedVolume.Cmp(r.ExactVolume()) != 0 ||
		!contacts.Complete || len(contacts.Contacts) != 0 {
		t.Fatal("planar boundary did not certify one embedded outward shell", shells, contacts)
	}
	counts := make([][2]int, len(edges))
	bySource := make(map[SweepBRepFaceSource]int)
	boundary := chain.Boundary()
	for i, face := range faces {
		source := ancestry[i]
		original := boundary[i]
		if source.Owner != original.Owner || source.Side != original.Side || len(face.Boundary) != 4 || len(face.Holes) != 0 {
			t.Fatal("native face does not retain the original analytic face", i, source, original)
		}
		if _, duplicate := bySource[source]; duplicate {
			t.Fatal("split or duplicated an original face ID", source)
		}
		bySource[source] = i
		for _, use := range face.Boundary {
			winding := 0
			if use.Reversed {
				winding = 1
			}
			counts[use.Edge][winding]++
			side := int(source.Side)
			edge := edges[use.Edge]
			for _, vertex := range [2]int{edge.Start, edge.End} {
				onSource := false
				for _, ref := range r.VertexSources(vertex) {
					onSource = onSource || (ref.Owner == source.Owner && ref.Corner[side/2] == side%2)
				}
				if !onSource {
					t.Fatal("face coedge endpoint lacks its original raw source station", source, use, vertex)
				}
			}
		}
	}
	for i, count := range counts {
		if count != [2]int{1, 1} {
			t.Fatal("native edge does not have opposite coedges", i, count)
		}
	}

	sweeps, joins := chain.Sweeps(), chain.Joins()
	var shared [4]int
	for id, point := range vertices {
		refs := r.VertexSources(id)
		for _, ref := range refs {
			d, shift := sweeps[ref.Owner].Domain()
			u := new(big.Rat).SetFloat64([2]float64{d.UMin, d.UMax}[ref.Corner[0]])
			v := new(big.Rat).SetFloat64([2]float64{d.VMin, d.VMax}[ref.Corner[1]])
			distance := [2]*big.Rat{shift.Lower, shift.Upper}[ref.Corner[2]]
			p, err := sweeps[ref.Owner].EvaluateRational(ctx, u, v, distance, geom.SurfacePatchEvaluationOptions{}, geom.SurfaceNormalOffsetEvaluationOptions{})
			if err != nil {
				t.Fatal(err)
			}
			for axis, x := range [3]float64{point.X, point.Y, point.Z} {
				q := new(big.Rat).SetFloat64(x)
				if p.Lower[axis].Cmp(q) != 0 || p.Upper[axis].Cmp(q) != 0 {
					t.Fatal("native vertex differs from exact original station", id, ref, axis)
				}
			}
		}
		if len(refs) == 1 {
			continue
		}
		if len(refs) != 2 || refs[1].Owner != refs[0].Owner+1 {
			t.Fatal("native vertex does not carry valid original owner incidence", refs)
		}
		join := joins[refs[0].Owner]
		a, b := int(join.Original.Edges[0]), int(join.Original.Edges[1])
		an, bn := refs[0].Corner, refs[1].Corner
		if an[a/2] != a%2 || bn[b/2] != b%2 || an[2] != bn[2] || an[1-a/2]+bn[1-b/2] != 1 {
			t.Fatal("stitched native corner failed original reversed endpoint map", refs, join)
		}
		shared[refs[0].Owner]++
	}
	for i, join := range joins {
		if shared[i] != 4 {
			t.Fatal("expected four exact shared corners per original wall", i, shared)
		}
		for j, owner := range join.Owners {
			if _, found := bySource[SweepBRepFaceSource{Owner: owner, Side: geom.NormalOffsetSweepSide(join.Original.Edges[j])}]; found {
				t.Fatal("internal joined wall still has a native face", owner, join)
			}
		}
		cut := cuts[i+1]
		point := geom.Vec3{X: float64(cut[0]+cut[1]) / 8, Y: .5}
		inside, err := locator.Locate(ctx, point)
		if err != nil || inside.Location != collision.Inside {
			t.Fatal("relative interior of omitted wall did not become interior material", i, inside, err)
		}
		point.Z = .125
		rim, err := locator.Locate(ctx, point)
		if err != nil || rim.Location != collision.OnBoundary || ancestry[rim.Face].Side != geom.NormalOffsetSweepUpper {
			t.Fatal("retained upper face lost its exact native seam rim", i, rim, err)
		}
	}
}

func authoredSweepBRepSource(t *testing.T, nonparallel, curved bool, halfThickness *big.Rat) (*geom.VerifiedNormalOffsetSweepOpenChain, [6][2]int64) {
	t.Helper()
	ctx := context.Background()
	q := big.NewRat
	cuts := [6][2]int64{{-4, -4}, {0, 0}, {6, 6}, {14, 14}, {16, 16}, {20, 20}}
	if nonparallel {
		cuts = [6][2]int64{{-4, -4}, {0, 0}, {4, 5}, {8, 7}, {12, 11}, {16, 17}}
	}
	domains := [5]geom.ParameterRectangle{
		{UMin: 0, UMax: 2, VMin: -1, VMax: 1}, {UMin: -2, UMax: 0, VMin: 2, VMax: 3},
		{UMin: 5, UMax: 7, VMin: -4, VMax: -2}, {UMin: 3, UMax: 5, VMin: 8, VMax: 10},
		{UMin: -5, UMax: -3, VMin: 11, VMax: 12},
	}
	weights := [5]*big.Rat{q(1, 1), q(1, 1), q(1, 1), q(1, 1), q(1, 1)}
	if nonparallel {
		weights = [5]*big.Rat{q(1, 1), q(3, 2), q(5, 4), q(7, 6), q(9, 8)}
	}
	var sweeps [5]*geom.VerifiedNormalOffsetSweep
	for owner, domain := range domains {
		recipe := geom.ExactBezierSurfaceRecipe{Version: 1, DegreeU: 1, DegreeV: 1, Domain: domain}
		projection := geom.NormalOffsetSweepProjection{Axes: [3]int{0, 1, 2}}
		if owner%2 == 1 {
			projection = geom.NormalOffsetSweepProjection{Axes: [3]int{1, 0, 2}, Reversed: [3]bool{true}}
		}
		if curved && owner == 4 {
			recipe.DegreeU = 2
		}
		for u := 0; u <= recipe.DegreeU; u++ {
			for v := range 2 {
				x, y, free := u, v, v
				if owner%2 == 1 {
					x, y, free = v, 1-u, u
				}
				weight := q(1, 1)
				if free == 1 {
					weight = weights[owner]
				}
				start, end := cuts[owner][y], cuts[owner+1][y]
				xPoint := q(start+int64(x)*(end-start), 4)
				z := q(0, 1)
				if recipe.DegreeU == 2 {
					xPoint = q(2*start+int64(x)*(end-start), 8)
					// The noncoplanar support leaves its incoming
					// original edge and first cross derivative exact.
					if u == 2 {
						z = q(1, 512)
					}
				}
				recipe.Controls = append(recipe.Controls, [4]string{
					new(big.Rat).Mul(xPoint, weight).RatString(), new(big.Rat).Mul(q(int64(y), 1), weight).RatString(),
					new(big.Rat).Mul(z, weight).RatString(), weight.RatString(),
				})
			}
		}
		patch, err := geom.ImportExactBezierSurfaceRecipe(ctx, recipe, geom.SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		sweeps[owner] = startVerifiedSweep(t, patch, new(big.Rat).Neg(halfThickness), halfThickness, projection)
	}
	pairs := make([]*geom.VerifiedNormalOffsetSweepPair, 4)
	for i := range pairs {
		edges := [2]geom.SurfaceEdge{geom.SurfaceEdgeUMax, geom.SurfaceEdgeVMin}
		if i%2 == 1 {
			edges = [2]geom.SurfaceEdge{geom.SurfaceEdgeVMax, geom.SurfaceEdgeUMin}
		}
		cut := cuts[i+1]
		join := geom.VerifiedNormalOffsetSweepJoin{Edges: edges, ReverseB: true, Axis: -1, AInteriorSign: -1,
			BStationRatio: new(big.Rat).Mul(weights[i], weights[i+1]), Covector: &[3]*big.Rat{q(1, 1), q(cut[0]-cut[1], 4)}}
		pair, proof, err := geom.NewVerifiedNormalOffsetSweepPair(ctx, sweeps[i], sweeps[i+1], join, geom.NormalOffsetCoverageOptions{})
		if err != nil || !proof.Complete {
			t.Fatal("original full-wall proof failed", i, proof, err)
		}
		pairs[i] = pair
	}
	var separators []geom.VerifiedNormalOffsetSweepNonadjacentSeparator
	for i := 0; i < len(sweeps)-2; i++ {
		for j := i + 2; j < len(sweeps); j++ {
			a, b := cuts[i+1], cuts[j]
			separators = append(separators, geom.VerifiedNormalOffsetSweepNonadjacentSeparator{
				Owners: [2]int{i, j}, Separator: geom.VerifiedNormalOffsetSweepChainSeparator{
					Covector: [3]*big.Rat{q(1, 1)}, Coordinate: q(max(a[0], a[1])+min(b[0], b[1]), 8),
				},
			})
		}
	}
	chain, proof, err := geom.NewVerifiedNormalOffsetSweepOpenChain(ctx, pairs, separators, geom.NormalOffsetCoverageOptions{})
	if err != nil || !proof.Complete {
		t.Fatal("independently authored open source failed to seal", proof, err)
	}
	return chain, cuts
}
