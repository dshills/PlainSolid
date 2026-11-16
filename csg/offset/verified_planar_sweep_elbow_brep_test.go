package offset

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/brep"
	"plainsolid/collision"
	"plainsolid/geom"
)

func TestVerifiedPlanarSweepElbowNativeBoundary(t *testing.T) {
	for _, control := range []struct {
		name  string
		shear bool
	}{
		{"perpendicular affine", false},
		{"oblique planes and reversed projective source", true},
	} {
		t.Run(control.name, func(t *testing.T) {
			ctx := context.Background()
			pairs := authoredPlanarSweepElbow(t, control.shear, 1, 0)
			elbow, proof, err := geom.NewVerifiedPlanarNormalOffsetSweepElbow(ctx, pairs[0], pairs[1], geom.SurfacePatchOptions{})
			if err != nil || !proof.Complete {
				t.Fatal("exact sector proof failed", proof, err)
			}
			checkStrictSweepChainRefusals(t, pairs)
			if short, p, err := geom.NewVerifiedPlanarNormalOffsetSweepElbow(ctx, pairs[0], pairs[1],
				geom.SurfacePatchOptions{MaxOperations: proof.ExactOperations - 1}); !errors.Is(err, geom.ErrConvergence) || short != nil || p.Complete {
				t.Fatal("one-short sector proof published a join", p, err)
			}
			canceled, cancel := context.WithCancel(ctx)
			cancel()
			if bad, p, err := geom.NewVerifiedPlanarNormalOffsetSweepElbow(canceled, pairs[0], pairs[1],
				geom.SurfacePatchOptions{}); !errors.Is(err, context.Canceled) || bad != nil || p.Complete {
				t.Fatal("canceled sector proof published a join", p, err)
			}
			r, err := PlanarBRepFromVerifiedPlanarNormalOffsetSweepElbow(ctx, elbow, VerifiedSweepOpenChainBRepOptions{})
			if err != nil {
				t.Fatal(err)
			}
			checkPlanarSweepElbowBRep(t, elbow, r, control.shear)
			// Independently integrate the three footprints of areas 4, 2,
			// and 3 through the common signed thickness 1/2. The shear
			// leaves their physical projected areas unchanged.
			if r.ExactVolume().Cmp(big.NewRat(9, 2)) != 0 {
				t.Fatal("outward exact native volume disagrees with the authoring dimensions", r.ExactVolume())
			}
			o := VerifiedSweepOpenChainBRepOptions{Validation: collision.LocationOptions{MaxCollisionPairs: 1}}
			if bad, err := PlanarBRepFromVerifiedPlanarNormalOffsetSweepElbow(ctx, elbow, o); !errors.Is(err, collision.ErrLimit) ||
				bad.Original() != nil || bad.ExactVolume() != nil || bad.ReentrantEdge() != -1 || len(bad.Solid().Faces()) != 0 {
				t.Fatal("short embedding proof published a native boundary", err)
			}
			t.Logf("16V/28E/14F embedded shell, common swept edge %d, exact volume %s, sector proof %d operations",
				r.ReentrantEdge(), r.ExactVolume(), proof.ExactOperations)
		})
	}
}

func TestVerifiedPlanarSweepElbowRefusesExtraEndIntersection(t *testing.T) {
	ctx := context.Background()
	q := big.NewRat
	for _, control := range []struct {
		name              string
		leftHigh          int64
		point             [3]*big.Rat
		leftRaw, rightRaw [2]*big.Rat
	}{
		{"positive end overlap", 5, [3]*big.Rat{q(-1, 2), q(7, 4), q(0, 1)},
			[2]*big.Rat{q(5, 1), q(-1, 6)}, [2]*big.Rat{q(43, 4), q(-57, 7)}},
		{"extra swept face contact", 3, [3]*big.Rat{q(-1, 2), q(3, 2), q(0, 1)},
			[2]*big.Rat{q(5, 1), q(0, 1)}, [2]*big.Rat{q(21, 2), q(-8, 1)}},
	} {
		t.Run(control.name, func(t *testing.T) {
			pairs := authoredPlanarSweepElbow(t, false, control.leftHigh, -3)
			ends := [2]*geom.VerifiedNormalOffsetSweep{pairs[0].Sweeps()[0], pairs[1].Sweeps()[1]}
			for i, uv := range [2][2]*big.Rat{control.leftRaw, control.rightRaw} {
				domain, _ := ends[i].Domain()
				atBoundary := uv[1].Cmp(new(big.Rat).SetFloat64(domain.VMax)) == 0
				if uv[0].Cmp(new(big.Rat).SetFloat64(domain.UMin)) <= 0 ||
					uv[0].Cmp(new(big.Rat).SetFloat64(domain.UMax)) >= 0 ||
					uv[1].Cmp(new(big.Rat).SetFloat64(domain.VMin)) <= 0 ||
					uv[1].Cmp(new(big.Rat).SetFloat64(domain.VMax)) > 0 || atBoundary != (control.leftHigh == 3) {
					t.Fatal("invalid exact authored overlap or contact station", i, uv)
				}
				p, err := ends[i].EvaluateRational(ctx, uv[0], uv[1], control.point[2],
					geom.SurfacePatchEvaluationOptions{}, geom.SurfaceNormalOffsetEvaluationOptions{})
				if err != nil {
					t.Fatal(err)
				}
				for axis, want := range control.point {
					if p.Lower[axis].Cmp(want) != 0 || p.Upper[axis].Cmp(want) != 0 {
						t.Fatal("the two independently joined end sweeps must genuinely meet at the extra station", i, p, want)
					}
				}
			}
			bad, proof, err := geom.NewVerifiedPlanarNormalOffsetSweepElbow(ctx, pairs[0], pairs[1], geom.SurfacePatchOptions{})
			if !errors.Is(err, geom.ErrDomain) || bad != nil || proof.Complete {
				t.Fatal("valid adjacent wall joins and extra end intersection published an elbow", proof, err)
			}
			t.Logf("both exact original end stations evaluate to %v; sector proof refused in %d operations", control.point, proof.ExactOperations)
		})
	}
}

func checkPlanarSweepElbowBRep(t *testing.T, elbow *geom.VerifiedPlanarNormalOffsetSweepElbow, r VerifiedPlanarSweepElbowBRep, shear bool) {
	t.Helper()
	ctx := context.Background()
	if r.Original() != elbow {
		t.Fatal("native result lost the original pair certificates")
	}
	solid := r.Solid()
	if err := solid.Validate(); err != nil {
		t.Fatal(err)
	}
	if got := solid.Topology(); got != (brep.Topology{Vertices: 16, Edges: 28, Faces: 14, Shells: 1, EulerCharacteristic: 2}) {
		t.Fatal("unexpected indexed L topology", got)
	}
	locator, err := collision.NewPointLocator(ctx, solid, collision.LocationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	shells, report := locator.Shells(), locator.BoundaryCollision()
	if len(shells) != 1 || shells[0].SignedVolume.Cmp(big.NewRat(9, 2)) != 0 || !report.Complete || len(report.Contacts) != 0 {
		t.Fatal("expected one embedded outward shell", shells, report)
	}
	contact := elbow.Contact()
	expectedCorners := [3][2]int{{1, 1}, {0, 1}, {0, 1}}
	if contact.Corners != expectedCorners {
		t.Fatal("incorrect endpoints under the original signed and reversed wall maps", contact.Corners)
	}
	// Both joins fix their normalized endpoints exactly, even where
	// the second original correspondence is reversed and projective.
	x := 0.0
	if shear {
		x = .25
	}
	expectEnds := [2]geom.Vec3{{X: x, Y: 1, Z: -.125}, {X: x, Y: 1, Z: .375}}
	vertexIDs := [2]int{-1, -1}
	vertices, edges, faces := solid.Vertices(), solid.Edges(), solid.Faces()
	sources, sweeps := r.FaceSources(), elbow.Sweeps()
	for id, point := range vertices {
		history := r.VertexSources(id)
		if len(history) < 1 || len(history) > 3 {
			t.Fatal("invalid original corner ancestry", id, history)
		}
		for _, source := range history {
			domain, layer := sweeps[source.Owner].Domain()
			u := new(big.Rat).SetFloat64([2]float64{domain.UMin, domain.UMax}[source.Corner[0]])
			v := new(big.Rat).SetFloat64([2]float64{domain.VMin, domain.VMax}[source.Corner[1]])
			d := [2]*big.Rat{layer.Lower, layer.Upper}[source.Corner[2]]
			p, err := sweeps[source.Owner].EvaluateRational(ctx, u, v, d,
				geom.SurfacePatchEvaluationOptions{}, geom.SurfaceNormalOffsetEvaluationOptions{})
			if err != nil {
				t.Fatal(err)
			}
			for axis, value := range [3]float64{point.X, point.Y, point.Z} {
				want := new(big.Rat).SetFloat64(value)
				if p.Lower[axis].Cmp(want) != 0 || p.Upper[axis].Cmp(want) != 0 {
					t.Fatal("native corner cannot be replayed by its original owner", id, source, axis)
				}
			}
		}
		if len(history) == 3 {
			d := history[0].Corner[2]
			if vertexIDs[d] != -1 || point != expectEnds[d] {
				t.Fatal("an unauthenticated corner received three owners", id, point)
			}
			vertexIDs[d] = id
			for i, source := range history {
				expected := expectedCorners[i]
				if source.Owner != i || source.Corner != [3]int{expected[0], expected[1], d} {
					t.Fatal("contact lost its three original station indices", source)
				}
			}
			for axis, value := range [3]float64{point.X, point.Y, point.Z} {
				if contact.Endpoints[d][axis].Cmp(new(big.Rat).SetFloat64(value)) != 0 {
					t.Fatal("exact source edge endpoint changed", d, axis)
				}
			}
		}
	}
	if vertexIDs[0] < 0 || vertexIDs[1] < 0 || r.ReentrantEdge() < 0 {
		t.Fatal("the authenticated shared edge is absent")
	}
	e := edges[r.ReentrantEdge()]
	if e.Start != min(vertexIDs[0], vertexIDs[1]) || e.End != max(vertexIDs[0], vertexIDs[1]) {
		t.Fatal("a different native edge claimed the exact common source edge", e, vertexIDs)
	}
	uses := make([][2]int, len(edges))
	var endUses [3]int
	retained := elbow.Boundary()
	for i, face := range faces {
		source := sources[i]
		if source.Owner != retained[i].Owner || source.Side != retained[i].Side || len(face.Boundary) != 4 {
			t.Fatal("lost original retained face identity", i, source)
		}
		for _, use := range face.Boundary {
			orientation := 0
			if use.Reversed {
				orientation = 1
			}
			uses[use.Edge][orientation]++
			if use.Edge == r.ReentrantEdge() {
				endUses[source.Owner]++
			}
		}
	}
	for i, uses := range uses {
		if uses != [2]int{1, 1} {
			t.Fatal("native edge lacks opposite coedges", i, uses)
		}
	}
	if endUses != [3]int{1, 0, 1} {
		t.Fatal("reentrant edge must join the two independently retained end faces", endUses)
	}
	for _, check := range []struct {
		x, y float64
		want collision.Location
	}{
		{0, 0, collision.Inside}, {.5, 1, collision.Inside},
		{0, 1, collision.OnBoundary}, {-.5, 1.5, collision.Outside},
	} {
		x := check.x
		if shear {
			x += check.y / 4
		}
		got, err := locator.Locate(ctx, geom.Vec3{X: x, Y: check.y})
		if err != nil || got.Location != check.want {
			t.Fatal("native boundary disagrees at an original wall or its fourth sector", check, got, err)
		}
	}
	contact.Endpoints[0][0].SetInt64(99)
	if elbow.Contact().Endpoints[0][0].Cmp(big.NewRat(99, 1)) == 0 {
		t.Fatal("caller mutation changed the immutable contact certificate")
	}
}

func checkStrictSweepChainRefusals(t *testing.T, pairs [2]*geom.VerifiedNormalOffsetSweepPair) {
	t.Helper()
	ctx := context.Background()
	sep := geom.VerifiedNormalOffsetSweepChainSeparator{Covector: [3]*big.Rat{big.NewRat(1, 1)}, Coordinate: new(big.Rat)}
	if s, proof, err := geom.NewVerifiedNonparallelNormalOffsetSweepChain(ctx, pairs[0], pairs[1], sep,
		geom.NormalOffsetCoverageOptions{}); !errors.Is(err, geom.ErrDomain) || s != nil || proof.Complete {
		t.Fatal("the original strict nonparallel chain admitted a contacting elbow", proof, err)
	}
	if s, proof, err := geom.NewVerifiedNormalOffsetSweepOpenChain(ctx, pairs[:], []geom.VerifiedNormalOffsetSweepNonadjacentSeparator{
		{Owners: [2]int{0, 2}, Separator: sep},
	}, geom.NormalOffsetCoverageOptions{}); !errors.Is(err, geom.ErrDomain) || s != nil || proof.Complete {
		t.Fatal("the original strict open chain admitted a contacting elbow", proof, err)
	}
}

func authoredPlanarSweepElbow(t *testing.T, shear bool, leftHigh, rightLow int64) [2]*geom.VerifiedNormalOffsetSweepPair {
	t.Helper()
	ctx := context.Background()
	q := big.NewRat
	xy := [3][2][2][2]int64{
		{{{-2, -1}, {-2, leftHigh}}, {{0, -1}, {0, 1}}},
		{{{0, -1}, {0, 1}}, {{1, -1}, {1, 1}}},
		{{{1, 1}, {0, 1}}, {{1, 4}, {rightLow, 4}}},
	}
	domains := [3]geom.ParameterRectangle{
		{UMin: 2, UMax: 6, VMin: -2, VMax: 0},
		{UMin: -7, UMax: -6, VMin: 5, VMax: 7},
		{UMin: 10, UMax: 13, VMin: -9, VMax: -8},
	}
	var sweeps [3]*geom.VerifiedNormalOffsetSweep
	for owner, domain := range domains {
		recipe := geom.ExactBezierSurfaceRecipe{Version: 1, DegreeU: 1, DegreeV: 1, Domain: domain}
		for u := range 2 {
			for v := range 2 {
				x, y := xy[owner][u][v][0], xy[owner][u][v][1]
				p, weight := q(x, 1), q(1, 1)
				if shear {
					p.Add(p, q(y, 4))
					if owner == 2 && v == 1 {
						weight.SetFrac64(3, 2)
					}
				}
				recipe.Controls = append(recipe.Controls, [4]string{
					new(big.Rat).Mul(p, weight).RatString(), new(big.Rat).Mul(q(y, 1), weight).RatString(), "0", weight.RatString(),
				})
			}
		}
		chart, err := geom.ImportExactBezierSurfaceRecipe(ctx, recipe, geom.SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		projection := geom.NormalOffsetSweepProjection{Axes: [3]int{0, 1, 2}}
		if owner == 2 {
			projection = geom.NormalOffsetSweepProjection{Axes: [3]int{1, 0, 2}, Reversed: [3]bool{false, true}}
		}
		sweeps[owner] = startVerifiedSweep(t, chart, q(-1, 8), q(3, 8), projection)
	}
	joins := [2]geom.VerifiedNormalOffsetSweepJoin{
		{Edges: [2]geom.SurfaceEdge{geom.SurfaceEdgeUMax, geom.SurfaceEdgeUMin}, Axis: 0, AInteriorSign: -1},
		{Edges: [2]geom.SurfaceEdge{geom.SurfaceEdgeVMax, geom.SurfaceEdgeUMin}, ReverseB: true, Axis: 1, AInteriorSign: -1},
	}
	if shear {
		joins[0].Covector = &[3]*big.Rat{q(1, 1), q(-1, 4)}
		joins[1].BStationRatio = q(3, 2)
	}
	var pairs [2]*geom.VerifiedNormalOffsetSweepPair
	for i, join := range joins {
		pair, proof, err := geom.NewVerifiedNormalOffsetSweepPair(ctx, sweeps[i], sweeps[i+1], join, geom.NormalOffsetCoverageOptions{})
		if err != nil || !proof.Complete {
			t.Fatal("original adjacent walls must independently qualify", i, proof, err)
		}
		pairs[i] = pair
	}
	return pairs
}
