package offset

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/internal/slowtest"
	"reflect"
	"testing"

	"plainsolid/collision"
	"plainsolid/geom"
)

var openSweepDisplayCuts = [9][2]int64{{-4, -4}, {0, 0}, {4, 5}, {8, 7}, {12, 11}, {16, 17}, {20, 20}, {24, 25}, {28, 28}}

func TestTessellateVerifiedNormalOffsetSweepOpenChain(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	chain := authoredOpenSweepDisplayChain(t, 5)
	o := VerifiedSweepMeshOptions{Deflection: .8, NormalAngle: .15}
	got, err := TessellateVerifiedNormalOffsetSweepOpenChain(ctx, chain, o)
	if err != nil {
		t.Fatal(err)
	}
	checkOpenSweepDisplay(t, chain, got, o)
	if got.SignedDisplayVolume.Cmp(big.NewRat(41, 32)) != 0 {
		t.Fatal("expected the exact original trapezoid times the signed layer thickness", got.SignedDisplayVolume)
	}
	t.Logf("%d vertices, %d triangles, volume %s, squared distance %s, squared normal tangent %s, %d geometry work",
		len(got.Mesh.Vertices), len(got.Mesh.Triangles), got.SignedDisplayVolume,
		got.DistanceUpperSquared.FloatString(12), got.SquaredFacetNormalTangent.FloatString(12), got.GeometryWork)
	for _, control := range []struct {
		name string
		edit func(*VerifiedSweepMeshOptions)
		want error
	}{
		{"triangle limit", func(o *VerifiedSweepMeshOptions) { o.MaxTriangles = len(got.Mesh.Triangles) - 1 }, ErrLimit},
		{"work limit", func(o *VerifiedSweepMeshOptions) { o.MaxWork = 4095 }, ErrLimit},
		{"depth limit", func(o *VerifiedSweepMeshOptions) { o.MaxDepth = 1 }, geom.ErrConvergence},
		{"collision limit", func(o *VerifiedSweepMeshOptions) { o.Deflection, o.NormalAngle, o.MaxCollisionPairs = 3, 0, 1 }, collision.ErrLimit},
	} {
		t.Run(control.name, func(t *testing.T) {
			short := o
			control.edit(&short)
			refused, err := TessellateVerifiedNormalOffsetSweepOpenChain(ctx, chain, short)
			if !errors.Is(err, control.want) || !reflect.DeepEqual(refused, CertifiedVerifiedSweepOpenChainMesh{}) {
				t.Fatal("exhausted budget published part of the original boundary or returned the wrong error", err)
			}
		})
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if got, err := TessellateVerifiedNormalOffsetSweepOpenChain(canceled, chain, o); !errors.Is(err, context.Canceled) ||
		!reflect.DeepEqual(got, CertifiedVerifiedSweepOpenChainMesh{}) {
		t.Fatal("canceled chain published a display", err)
	}
}

func TestTessellateVerifiedNormalOffsetSweepOpenChainOwnerLimits(t *testing.T) {
	ctx := context.Background()
	for _, count := range []int{2, 8} {
		chain := authoredOpenSweepDisplayChain(t, count)
		got, err := TessellateVerifiedNormalOffsetSweepOpenChain(ctx, chain, VerifiedSweepMeshOptions{Deflection: 4})
		if err != nil {
			t.Fatal(count, err)
		}
		if err := got.Mesh.ValidateSolid(); err != nil {
			t.Fatal(count, err)
		}
		faces := make(map[VerifiedSweepPairMeshFace]bool, 4*count+2)
		for _, face := range got.OriginalFace {
			faces[face] = true
		}
		last := openSweepDisplayCuts[count]
		volume := big.NewRat(last[0]+last[1]+8, 32)
		if len(got.UVStations) != count || len(faces) != 4*count+2 || !got.Collision.Complete ||
			len(got.Collision.Contacts) != 0 || got.SignedDisplayVolume.Cmp(volume) != 0 || got.SquaredFacetNormalTangent != nil {
			t.Fatal("distance-only dynamic display changed original ownership or signed material", count, len(faces), got.SignedDisplayVolume)
		}
		for _, row := range got.VertexStations {
			if len(row) != count {
				t.Fatal("original station ledger does not follow the owner count", count, len(row))
			}
		}
		t.Logf("%d original owners, %d retained faces, %d indexed triangles, exact volume %s",
			count, len(faces), len(got.Mesh.Triangles), volume)
	}
}

func checkOpenSweepDisplay(t *testing.T, chain *geom.VerifiedNormalOffsetSweepOpenChain,
	got CertifiedVerifiedSweepOpenChainMesh, o VerifiedSweepMeshOptions) {
	t.Helper()
	if err := got.Mesh.ValidateSolid(); err != nil {
		t.Fatal(err)
	}
	n, nT := len(got.UVStations[0][0])-1, len(got.DisplacementStations)-1
	wantTriangles := 20*n*n + 24*n*nT
	limit := sweepRat(o.Deflection)
	limit.Mul(limit, limit)
	angleLimit := sweepRat(math.Tan(o.NormalAngle))
	angleLimit.Mul(angleLimit, angleLimit)
	if len(got.UVStations) != 5 || len(got.Mesh.Triangles) != wantTriangles || len(got.Mesh.Vertices) != wantTriangles/2+2 ||
		len(got.VertexStations) != len(got.Mesh.Vertices) || len(got.OriginalFace) != wantTriangles ||
		got.DistanceUpperSquared.Sign() <= 0 || got.DistanceUpperSquared.Cmp(limit) > 0 ||
		got.SquaredFacetNormalTangent == nil || got.SquaredFacetNormalTangent.Sign() < 0 ||
		got.SquaredFacetNormalTangent.Cmp(angleLimit) > 0 ||
		!got.Collision.Complete || len(got.Collision.Contacts) != 0 {
		t.Fatal("incomplete five-owner indexed distance and continuous normal proof",
			n, nT, got.DistanceUpperSquared, got.SquaredFacetNormalTangent, got.Collision.Complete, len(got.Collision.Contacts))
	}
	faces := chain.Boundary()
	if len(faces) != 22 {
		t.Fatal("expected 22 retained original faces", faces)
	}
	type edgeUse struct{ count, winding int }
	edges := make(map[[2]int]edgeUse)
	areas := make(map[VerifiedSweepPairMeshFace]*big.Rat, len(faces))
	incidence := make([][5]bool, len(got.Mesh.Vertices))
	segments := [3]int{n, n, nT}
	for _, face := range faces {
		areas[VerifiedSweepPairMeshFace{Owner: face.Owner, Side: face.Side}] = new(big.Rat)
	}
	for i, tri := range got.Mesh.Triangles {
		face := got.OriginalFace[i]
		area := areas[face]
		if area == nil {
			t.Fatal("an omitted wall or invalid original face supplied a triangle", i, face)
		}
		fixedAxis := int(face.Side) / 2
		for j, id := range tri {
			next := tri[(j+1)%3]
			key := [2]int{min(id, next), max(id, next)}
			use := edges[key]
			use.count++
			if id < next {
				use.winding++
			} else {
				use.winding--
			}
			edges[key] = use
			incidence[id][face.Owner] = true
			station := got.VertexStations[id][face.Owner]
			if !station.Present || station.Index[fixedAxis] != int(face.Side)%2*segments[fixedAxis] {
				t.Fatal("triangulated vertices lost their exact raw analytic face", i, face, station)
			}
		}
		area.Add(area, checkOpenSweepDisplayFacet(t, got, tri, face))
	}
	for edge, use := range edges {
		if use.count != 2 || use.winding != 0 {
			t.Fatal("edge was not used twice in opposite directions", edge, use)
		}
	}
	for face, area := range areas {
		if expected := openSweepDisplayProjectedArea(face); area.Cmp(expected) != 0 {
			t.Fatal("continuous source face and projected facets cover different areas", face, area, expected)
		}
	}

	seen := make(map[geom.Vec3]bool, len(got.Mesh.Vertices))
	sweeps, joins := chain.Sweeps(), chain.Joins()
	var counts [4]int
	var nonbinary [4]bool
	for id, point := range got.Mesh.Vertices {
		if seen[point] || len(got.VertexStations[id]) != 5 {
			t.Fatal("duplicate physical seam or incomplete dynamic owner row", id, point)
		}
		seen[point] = true
		var owners []int
		for owner, station := range got.VertexStations[id] {
			if station.Present != incidence[id][owner] {
				t.Fatal("source station ownership differs from indexed facet incidence", id, owner)
			}
			if !station.Present {
				continue
			}
			owners = append(owners, owner)
			u := got.UVStations[owner][0][station.Index[0]]
			v := got.UVStations[owner][1][station.Index[1]]
			d := got.DisplacementStations[station.Index[2]]
			raw, err := sweeps[owner].EvaluateRational(context.Background(), u, v, d,
				geom.SurfacePatchEvaluationOptions{}, geom.SurfaceNormalOffsetEvaluationOptions{})
			if err != nil {
				t.Fatal(err)
			}
			for axis, value := range [3]float64{point.X, point.Y, point.Z} {
				want := sweepRat(value)
				if raw.Lower[axis].Cmp(want) != 0 || raw.Upper[axis].Cmp(want) != 0 {
					t.Fatal("raw source station does not exactly reconstruct the indexed world point", id, owner, station)
				}
			}
		}
		switch len(owners) {
		case 1:
		case 2:
			left, right := owners[0], owners[1]
			if right != left+1 {
				t.Fatal("nonadjacent source stations share a mesh vertex", id, owners)
			}
			join := joins[left]
			a, b := got.VertexStations[id][left], got.VertexStations[id][right]
			if a.Index[2] != b.Index[2] || !join.Original.ReverseB {
				t.Fatal("seam lost its original reversed wall map or shared displacement", id, join)
			}
			station := [2]*big.Rat{}
			for i, owner := range owners {
				edge := join.Original.Edges[i]
				fixed := int(edge) / 2
				ref := got.VertexStations[id][owner].Index
				if ref[fixed] != int(edge)%2*n {
					t.Fatal("seam sample is off its original wall", id, owner, ref)
				}
				raw := got.UVStations[owner][1-fixed]
				value := raw[ref[1-fixed]]
				if _, exact := value.Float64(); !exact {
					nonbinary[left] = true
				}
				station[i] = new(big.Rat).Quo(new(big.Rat).Sub(value, raw[0]), new(big.Rat).Sub(raw[n], raw[0]))
			}
			ratio := new(big.Rat).Mul(openSweepDisplayWeight(left), openSweepDisplayWeight(right))
			mapped := new(big.Rat).Mul(station[0], ratio)
			denom := new(big.Rat).Add(new(big.Rat).Sub(mapped, station[0]), big.NewRat(1, 1))
			want := new(big.Rat).Sub(big.NewRat(1, 1), mapped.Quo(mapped, denom))
			if ratio.Cmp(join.Original.BStationRatio) != 0 || station[1].Cmp(want) != 0 {
				t.Fatal("original raw station did not follow the independently authored reversed projective law", id, left, station, want)
			}
			counts[left]++
		default:
			t.Fatal("indexed vertex needs one or two original owners", id, owners)
		}
	}
	for i, count := range counts {
		if count != 2*n+2*nT || !nonbinary[i] {
			t.Fatal("each original stitched perimeter needs its complete exact nonbinary station map", i, count, nonbinary[i])
		}
	}
	t.Logf("four original reversed projective join perimeters %v, each with exact nonbinary stations", counts)
}

func checkOpenSweepDisplayFacet(t *testing.T, got CertifiedVerifiedSweepOpenChainMesh, tri [3]int, face VerifiedSweepPairMeshFace) *big.Rat {
	t.Helper()
	var points [3][3]*big.Rat
	for i, id := range tri {
		p := got.Mesh.Vertices[id]
		points[i] = [3]*big.Rat{sweepRat(p.X), sweepRat(p.Y), sweepRat(p.Z)}
	}
	var d, e [3]*big.Rat
	for i := range d {
		d[i] = new(big.Rat).Sub(points[1][i], points[0][i])
		e[i] = new(big.Rat).Sub(points[2][i], points[0][i])
	}
	var normal [3]*big.Rat
	for i := range normal {
		j, k := (i+1)%3, (i+2)%3
		normal[i] = new(big.Rat).Sub(new(big.Rat).Mul(d[j], e[k]), new(big.Rat).Mul(d[k], e[j]))
	}
	axis, sign := authoredFourSweepDisplayNormalAxis(face)
	expected := [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}
	expected[axis].SetInt64(sign)
	if axis == 0 && face.Owner == 4 {
		expected[1].SetFrac64(-1, 4)
	}
	if normal[axis].Sign() != int(sign) {
		t.Fatal("represented triangle opposes its independently authored outward face", face, tri)
	}
	for i := range normal {
		if new(big.Rat).Mul(normal[i], expected[axis]).Cmp(new(big.Rat).Mul(normal[axis], expected[i])) != 0 {
			t.Fatal("represented facet is not exactly aligned with the original planar normal", face, tri)
		}
	}
	return new(big.Rat).Mul(normal[axis], big.NewRat(sign, 2))
}

func openSweepDisplayProjectedArea(face VerifiedSweepPairMeshFace) *big.Rat {
	axis, sign := authoredFourSweepDisplayNormalAxis(face)
	if axis == 0 {
		return big.NewRat(1, 4)
	}
	left, right := openSweepDisplayCuts[face.Owner], openSweepDisplayCuts[face.Owner+1]
	low, high := right[0]-left[0], right[1]-left[1]
	if axis == 2 {
		return big.NewRat(low+high, 8)
	}
	if sign < 0 {
		return big.NewRat(low, 16)
	}
	return big.NewRat(high, 16)
}

func openSweepDisplayWeight(owner int) *big.Rat {
	w := [8][2]int64{{1, 1}, {3, 2}, {5, 4}, {7, 6}, {9, 8}, {8, 7}, {9, 7}, {7, 6}}[owner]
	return big.NewRat(w[0], w[1])
}

func authoredOpenSweepDisplayChain(t *testing.T, count int) *geom.VerifiedNormalOffsetSweepOpenChain {
	t.Helper()
	ctx := context.Background()
	q := big.NewRat
	domains := [8]geom.ParameterRectangle{
		{UMin: 0, UMax: 2, VMin: -1, VMax: 1}, {UMin: -2, UMax: 0, VMin: 2, VMax: 3},
		{UMin: 5, UMax: 7, VMin: -4, VMax: -2}, {UMin: 3, UMax: 5, VMin: 8, VMax: 10},
		{UMin: -5, UMax: -3, VMin: 11, VMax: 12}, {UMin: 9, UMax: 10, VMin: 3, VMax: 5},
		{UMin: -7, UMax: -2, VMin: 2, VMax: 3}, {UMin: 1, UMax: 2, VMin: -13, VMax: -11},
	}
	sweeps := make([]*geom.VerifiedNormalOffsetSweep, count)
	for i := range sweeps {
		recipe := geom.ExactBezierSurfaceRecipe{Version: 1, DegreeU: 1, DegreeV: 1, Domain: domains[i]}
		view := geom.NormalOffsetSweepProjection{Axes: [3]int{0, 1, 2}}
		if i%2 == 1 {
			view = geom.NormalOffsetSweepProjection{Axes: [3]int{1, 0, 2}, Reversed: [3]bool{true}}
		}
		for u := range 2 {
			for v := range 2 {
				x, y, free := u, v, v
				if i%2 == 1 {
					x, y, free = v, 1-u, u
				}
				weight := q(1, 1)
				if free == 1 {
					weight = openSweepDisplayWeight(i)
				}
				recipe.Controls = append(recipe.Controls, [4]string{
					new(big.Rat).Mul(q(openSweepDisplayCuts[i+x][y], 4), weight).RatString(),
					new(big.Rat).Mul(q(int64(y), 1), weight).RatString(), "0", weight.RatString(),
				})
			}
		}
		chart, err := geom.ImportExactBezierSurfaceRecipe(ctx, recipe, geom.SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		sweeps[i] = startVerifiedSweep(t, chart, q(-1, 8), q(1, 8), view)
	}
	pairs := make([]*geom.VerifiedNormalOffsetSweepPair, count-1)
	for i := range pairs {
		edges := [2]geom.SurfaceEdge{geom.SurfaceEdgeUMax, geom.SurfaceEdgeVMin}
		if i%2 == 1 {
			edges = [2]geom.SurfaceEdge{geom.SurfaceEdgeVMax, geom.SurfaceEdgeUMin}
		}
		cut := openSweepDisplayCuts[i+1]
		join := geom.VerifiedNormalOffsetSweepJoin{
			Edges: edges, ReverseB: true, Axis: -1, AInteriorSign: -1,
			BStationRatio: new(big.Rat).Mul(openSweepDisplayWeight(i), openSweepDisplayWeight(i+1)),
			Covector:      &[3]*big.Rat{q(1, 1), q(cut[0]-cut[1], 4)},
		}
		pair, proof, err := geom.NewVerifiedNormalOffsetSweepPair(ctx, sweeps[i], sweeps[i+1], join, geom.NormalOffsetCoverageOptions{})
		if err != nil || !proof.Complete {
			t.Fatal("original adjacent full-wall proof failed", i, proof, err)
		}
		pairs[i] = pair
	}
	var separators []geom.VerifiedNormalOffsetSweepNonadjacentSeparator
	for i := 0; i < count-2; i++ {
		for j := i + 2; j < count; j++ {
			a, b := openSweepDisplayCuts[i+1], openSweepDisplayCuts[j]
			separators = append(separators, geom.VerifiedNormalOffsetSweepNonadjacentSeparator{
				Owners: [2]int{i, j},
				Separator: geom.VerifiedNormalOffsetSweepChainSeparator{
					Covector: [3]*big.Rat{q(1, 1)}, Coordinate: q(max(a[0], a[1])+min(b[0], b[1]), 8),
				},
			})
		}
	}
	chain, proof, err := geom.NewVerifiedNormalOffsetSweepOpenChain(ctx, pairs, separators, geom.NormalOffsetCoverageOptions{})
	if err != nil || !proof.Complete {
		t.Fatal("original open chain did not independently verify", proof, err)
	}
	return chain
}
