package offset

import (
	"context"
	"errors"
	"math"
	"math/big"
	"reflect"
	"testing"

	"plainsolid/geom"
)

func TestTessellateVerifiedFourChartNormalOffsetSweepChain(t *testing.T) {
	ctx := context.Background()
	q := big.NewRat
	for _, test := range []struct {
		name        string
		nonparallel bool
		volume      *big.Rat
	}{
		{"affine", false, q(5, 4)},
		{"nonparallel", true, q(31, 32)},
	} {
		t.Run(test.name, func(t *testing.T) {
			chain := authoredFourSweepDisplayChain(t, test.nonparallel)
			o := VerifiedSweepMeshOptions{Deflection: .8, NormalAngle: .04}
			got, err := TessellateVerifiedFourChartNormalOffsetSweepChain(ctx, chain, o)
			if err != nil {
				t.Fatal(err)
			}
			checkFourSweepDisplay(t, chain, got, o, test.nonparallel)
			if got.SignedDisplayVolume.Cmp(test.volume) != 0 {
				t.Fatal("wrong independently integrated signed prism volume", got.SignedDisplayVolume, test.volume)
			}
			t.Logf("%d vertices, %d triangles, exact signed volume %s, squared distance %s, squared normal tangent %s, %d work",
				len(got.Mesh.Vertices), len(got.Mesh.Triangles), got.SignedDisplayVolume,
				got.DistanceUpperSquared, got.SquaredFacetNormalTangent, got.GeometryWork)
			o.MaxTriangles = len(got.Mesh.Triangles) - 1
			refused, err := TessellateVerifiedFourChartNormalOffsetSweepChain(ctx, chain, o)
			if !errors.Is(err, ErrLimit) || !reflect.DeepEqual(refused, CertifiedVerifiedFourChartSweepChainMesh{}) {
				t.Fatal("subthreshold triangle budget published a partial proof", err)
			}
		})
	}
}

func checkFourSweepDisplay(t *testing.T, chain *geom.VerifiedFourChartNormalOffsetSweepChain,
	got CertifiedVerifiedFourChartSweepChainMesh, o VerifiedSweepMeshOptions, nonparallel bool) {
	t.Helper()
	if err := got.Mesh.ValidateSolid(); err != nil {
		t.Fatal(err)
	}
	n, nT := len(got.UVStations[0][0])-1, len(got.DisplacementStations)-1
	triangles := 16*n*n + 20*n*nT
	limit := sweepRat(o.Deflection)
	limit.Mul(limit, limit)
	angleLimit := sweepRat(math.Tan(o.NormalAngle))
	angleLimit.Mul(angleLimit, angleLimit)
	if len(got.Mesh.Triangles) != triangles || len(got.Mesh.Vertices) != triangles/2+2 ||
		len(got.VertexStations) != len(got.Mesh.Vertices) || len(got.OriginalFace) != triangles ||
		!got.Collision.Complete || len(got.Collision.Contacts) != 0 || got.SignedDisplayVolume.Sign() <= 0 ||
		got.DistanceUpperSquared.Cmp(limit) > 0 || got.SquaredFacetNormalTangent == nil ||
		got.SquaredFacetNormalTangent.Sign() < 0 || got.SquaredFacetNormalTangent.Cmp(angleLimit) > 0 {
		t.Fatal("incomplete four-owner closed distance and continuous normal certificate", n, nT)
	}
	sweeps, joins := chain.Sweeps(), chain.Joins()
	var faceCounts [4][6]int
	var projectedAreas [4][6]big.Rat
	vertexOwners := make([][4]bool, len(got.Mesh.Vertices))
	for i, triangle := range got.Mesh.Triangles {
		face := got.OriginalFace[i]
		faceCounts[face.Owner][face.Side]++
		sideAxis := int(face.Side) / 2
		segments := [3]int{n, n, nT}
		planeIndex := int(face.Side) % 2 * segments[sideAxis]
		for _, id := range triangle {
			vertexOwners[id][face.Owner] = true
			station := got.VertexStations[id][face.Owner]
			if !station.Present || station.Index[sideAxis] != planeIndex {
				t.Fatal("triangle escaped its original analytic face or left its raw station", i, face, station)
			}
		}
		projected := requireFourSweepDisplayNormal(t, got, triangle, face, nonparallel)
		sum := &projectedAreas[face.Owner][face.Side]
		sum.Add(sum, projected)
	}
	boundary := chain.Boundary()
	if len(boundary) != 18 {
		t.Fatal("expected exactly 18 original outer faces", len(boundary))
	}
	for _, face := range boundary {
		if faceCounts[face.Owner][face.Side] == 0 {
			t.Fatal("lost original outer face", face)
		}
		identity := VerifiedSweepPairMeshFace{Owner: face.Owner, Side: face.Side}
		expected := exactFourSweepDisplayProjectedArea(identity, nonparallel)
		if projectedAreas[face.Owner][face.Side].Cmp(expected) != 0 {
			t.Fatal("represented triangles did not cover the full independently authored convex original face", face, expected)
		}
	}
	for _, join := range joins {
		for i, owner := range join.Owners {
			if faceCounts[owner][join.Original.Edges[i]] != 0 {
				t.Fatal("a shared swept wall appeared among the outer triangles", owner, join.Original)
			}
		}
	}
	seen := make(map[geom.Vec3]bool, len(got.Mesh.Vertices))
	var seamVertices [3]int
	for id, point := range got.Mesh.Vertices {
		if seen[point] {
			t.Fatal("an exact shared seam vertex was duplicated", id, point)
		}
		seen[point] = true
		var owners []int
		for owner, station := range got.VertexStations[id] {
			if station.Present != vertexOwners[id][owner] {
				t.Fatal("original station metadata disagrees with actual owner incidence", id, owner)
			}
			if !station.Present {
				continue
			}
			owners = append(owners, owner)
			u, v := got.UVStations[owner][0][station.Index[0]], got.UVStations[owner][1][station.Index[1]]
			d := got.DisplacementStations[station.Index[2]]
			evaluated, err := sweeps[owner].EvaluateRational(context.Background(), u, v, d,
				geom.SurfacePatchEvaluationOptions{}, geom.SurfaceNormalOffsetEvaluationOptions{})
			if err != nil {
				t.Fatal(err)
			}
			for axis, value := range [3]float64{point.X, point.Y, point.Z} {
				exact := sweepRat(value)
				if evaluated.Lower[axis].Cmp(exact) != 0 || evaluated.Upper[axis].Cmp(exact) != 0 {
					t.Fatal("stored owner station failed exact original world reconstruction", id, owner, station)
				}
			}
		}
		switch len(owners) {
		case 1:
		case 2:
			left, right := owners[0], owners[1]
			if right != left+1 {
				t.Fatal("nonadjacent charts shared an indexed vertex", id, owners)
			}
			join := joins[left]
			if !join.Original.ReverseB || join.Original.BStationRatio != nil {
				t.Fatal("fixture must retain its original reversed axis correspondence", join)
			}
			l, r := got.VertexStations[id][left], got.VertexStations[id][right]
			if l.Index[2] != r.Index[2] {
				t.Fatal("seam indexed two different exact displacement stations", id, l, r)
			}
			var sum big.Rat
			for i, owner := range owners {
				station := got.VertexStations[id][owner]
				edge := join.Original.Edges[i]
				fixed := int(edge) / 2
				free := 1 - fixed
				if station.Index[fixed] != int(edge)%2*n {
					t.Fatal("indexed original station is off its authored shared wall", id, owner, station)
				}
				raw := got.UVStations[owner][free]
				width := new(big.Rat).Sub(raw[n], raw[0])
				normalized := new(big.Rat).Quo(new(big.Rat).Sub(raw[station.Index[free]], raw[0]), width)
				sum.Add(&sum, normalized)
			}
			if sum.Cmp(big.NewRat(1, 1)) != 0 {
				t.Fatal("paired original stations violated the independently read reversed law", id, &sum)
			}
			seamVertices[left]++
		default:
			t.Fatal("an indexed vertex lacks one or two stable original owners", id, owners)
		}
	}
	for i, count := range seamVertices {
		if count != 2*n+2*nT {
			t.Fatal("wrong original perimeter vertex count", i, count, n, nT)
		}
	}
	t.Logf("three original stitched perimeters %v", seamVertices)
}

func requireFourSweepDisplayNormal(t *testing.T, got CertifiedVerifiedFourChartSweepChainMesh,
	triangle [3]int, face VerifiedSweepPairMeshFace, nonparallel bool) *big.Rat {
	t.Helper()
	// The authored cap and y boundary planes have coordinate normals.
	// The last oblique x boundary is x+y/4=3 in the nonparallel fixture.
	// The odd original charts use physical (y,x)=(-u,v) in raw axes.
	expected := [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}
	axis, sign := authoredFourSweepDisplayNormalAxis(face)
	expected[axis].SetInt64(sign)
	if nonparallel && face.Owner == 3 && axis == 0 {
		expected[1].SetFrac64(sign, 4)
	}
	points := [3]geom.Vec3{got.Mesh.Vertices[triangle[0]], got.Mesh.Vertices[triangle[1]], got.Mesh.Vertices[triangle[2]]}
	p := [3]*big.Rat{sweepRat(points[0].X), sweepRat(points[0].Y), sweepRat(points[0].Z)}
	var a, b, actual [3]*big.Rat
	for i, x := range [3]float64{points[1].X, points[1].Y, points[1].Z} {
		a[i] = new(big.Rat).Sub(sweepRat(x), p[i])
	}
	for i, x := range [3]float64{points[2].X, points[2].Y, points[2].Z} {
		b[i] = new(big.Rat).Sub(sweepRat(x), p[i])
	}
	dot := new(big.Rat)
	for i := range actual {
		j, k := (i+1)%3, (i+2)%3
		actual[i] = new(big.Rat).Sub(new(big.Rat).Mul(a[j], b[k]), new(big.Rat).Mul(a[k], b[j]))
		dot.Add(dot, new(big.Rat).Mul(actual[i], expected[i]))
	}
	if dot.Sign() <= 0 {
		t.Fatal("triangle winding disagrees with the independently authored outward supporting plane", face, triangle)
	}
	for i := range actual {
		j, k := (i+1)%3, (i+2)%3
		if new(big.Rat).Mul(actual[j], expected[k]).Cmp(new(big.Rat).Mul(actual[k], expected[j])) != 0 {
			t.Fatal("represented facet departed from the independently known continuous planar normal", face, triangle)
		}
	}
	return new(big.Rat).Mul(actual[axis], big.NewRat(sign, 2))
}

func authoredFourSweepDisplayNormalAxis(face VerifiedSweepPairMeshFace) (axis int, sign int64) {
	sign = int64(2*(int(face.Side)%2) - 1)
	axis = int(face.Side) / 2
	if axis < 2 && face.Owner%2 == 1 {
		axis = 1 - axis
		if axis == 1 {
			sign = -sign
		}
	}
	return axis, sign
}

func exactFourSweepDisplayProjectedArea(face VerifiedSweepPairMeshFace, nonparallel bool) *big.Rat {
	q := big.NewRat
	axis, sign := authoredFourSweepDisplayNormalAxis(face)
	if axis == 0 {
		// Even the oblique end projects onto a 1 by 1/4 rectangle.
		return q(1, 4)
	}
	width := [4]*big.Rat{q(1, 1), q(3, 2), q(2, 1), q(1, 2)}[face.Owner]
	if nonparallel {
		lowWidth := q(1, 1)
		highWidth := [4]*big.Rat{q(1, 1), q(5, 4), q(1, 2), q(1, 1)}[face.Owner]
		switch {
		case axis == 2:
			// Cap footprint area of the original trapezoid.
			return new(big.Rat).Quo(new(big.Rat).Add(lowWidth, highWidth), q(2, 1))
		case sign < 0:
			width = lowWidth
		default:
			width = highWidth
		}
	}
	if axis == 2 {
		return width
	}
	return new(big.Rat).Mul(width, q(1, 4))
}

func authoredFourSweepDisplayChain(t *testing.T, nonparallel bool) *geom.VerifiedFourChartNormalOffsetSweepChain {
	t.Helper()
	ctx := context.Background()
	q := big.NewRat
	cuts := [5][2]int64{{-4, -4}, {0, 0}, {6, 6}, {14, 14}, {16, 16}}
	if nonparallel {
		cuts = [5][2]int64{{-4, -4}, {0, 0}, {4, 5}, {8, 7}, {12, 11}}
	}
	domains := [4]geom.ParameterRectangle{
		{UMin: 0, UMax: 2, VMin: -1, VMax: 1}, {UMin: -2, UMax: 0, VMin: 2, VMax: 3},
		{UMin: 5, UMax: 7, VMin: -4, VMax: -2}, {UMin: 3, UMax: 5, VMin: 8, VMax: 10},
	}
	var sweeps [4]*geom.VerifiedNormalOffsetSweep
	for i, domain := range domains {
		recipe := geom.ExactBezierSurfaceRecipe{Version: 1, DegreeU: 1, DegreeV: 1, Domain: domain}
		projection := geom.NormalOffsetSweepProjection{Axes: [3]int{0, 1, 2}}
		if i%2 == 1 {
			projection = geom.NormalOffsetSweepProjection{Axes: [3]int{1, 0, 2}, Reversed: [3]bool{true}}
		}
		for u := range 2 {
			for v := range 2 {
				x, y := u, v
				if i%2 == 1 {
					x, y = v, 1-u
				}
				recipe.Controls = append(recipe.Controls, [4]string{
					q(cuts[i+x][y], 4).RatString(), q(int64(y), 1).RatString(), "0", "1",
				})
			}
		}
		chart, err := geom.ImportExactBezierSurfaceRecipe(ctx, recipe, geom.SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		sweep, proof, err := geom.NewVerifiedNormalOffsetSweep(ctx, chart, q(-1, 8), q(1, 8), projection, geom.NormalOffsetCoverageOptions{})
		if err != nil || !proof.Complete {
			t.Fatal("original authored chart did not independently seal", i, proof, err)
		}
		sweeps[i] = sweep
	}
	var pairs [3]*geom.VerifiedNormalOffsetSweepPair
	for i := range pairs {
		edges := [2]geom.SurfaceEdge{geom.SurfaceEdgeUMax, geom.SurfaceEdgeVMin}
		if i%2 == 1 {
			edges = [2]geom.SurfaceEdge{geom.SurfaceEdgeVMax, geom.SurfaceEdgeUMin}
		}
		join := geom.VerifiedNormalOffsetSweepJoin{Edges: edges, ReverseB: true, Axis: 0, AInteriorSign: -1}
		if nonparallel && i > 0 {
			join.Covector = &[3]*big.Rat{q(1, 1), q(int64(2*i-3), 4)}
		}
		pair, proof, err := geom.NewVerifiedNormalOffsetSweepPair(ctx, sweeps[i], sweeps[i+1], join, geom.NormalOffsetCoverageOptions{})
		if err != nil || !proof.Complete {
			t.Fatal("original adjacent full walls did not independently match", i, proof, err)
		}
		pairs[i] = pair
	}
	var separators [3]geom.VerifiedNormalOffsetSweepChainSeparator
	levels := [3]*big.Rat{q(1, 2), q(1, 1), q(3, 2)}
	if !nonparallel {
		levels[2] = q(5, 2)
	}
	for i, level := range levels {
		separators[i] = geom.VerifiedNormalOffsetSweepChainSeparator{Covector: [3]*big.Rat{q(1, 1)}, Coordinate: level}
	}
	chain, proof, err := geom.NewVerifiedFourChartNormalOffsetSweepChain(ctx, pairs, separators, geom.NormalOffsetCoverageOptions{})
	if err != nil || !proof.Complete {
		t.Fatal("original four-chart display fixture did not seal", proof, err)
	}
	return chain
}
