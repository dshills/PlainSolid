package offset

import (
	"context"
	"errors"
	"math"
	"math/big"
	"reflect"
	"testing"

	"plainsolid/collision"
	"plainsolid/geom"
)

func TestTessellateVerifiedNormalOffsetSweepChainPlacedClocks(t *testing.T) {
	chain := authoredPlacedSweepMeshChain(t)
	options := VerifiedSweepMeshOptions{Deflection: .43}
	got, err := TessellateVerifiedNormalOffsetSweepChain(context.Background(), chain, options)
	if err != nil {
		t.Fatal(err)
	}
	checkVerifiedSweepChainMesh(t, chain, got, options)
	n, nT := len(got.UVStations[0][0])-1, len(got.DisplacementStations)-1
	var seamCount, nonbinary [2]int
	for id, stations := range got.VertexStations {
		left, middle, right := stations[0], stations[1], stations[2]
		if !middle.Present || (!left.Present && !right.Present) {
			continue
		}
		m := middle.Index
		physicalStation := big.NewRat(int64(n-m[0]), int64(n))
		num := new(big.Rat).Sub(big.NewRat(1, 1), physicalStation)
		den := new(big.Rat).Add(big.NewRat(1, 1), new(big.Rat).Quo(physicalStation, big.NewRat(2, 1)))
		num.Quo(num, den).Mul(num, big.NewRat(2, 1))
		middleRaw := got.UVStations[1][0][m[0]]
		if middleRaw.Cmp(new(big.Rat).Add(big.NewRat(2, 1), num)) != 0 {
			t.Fatal("lost exact original reversed middle clock on an outer face", id, middleRaw)
		}
		point := got.Mesh.Vertices[id]
		projection := (48*point.X + 36*point.Y - 25*point.Z) / 7
		if left.Present {
			seamCount[0]++
			if right.Present || m[1] != 0 || left.Index[0] != n || left.Index[1]+m[0] != n ||
				left.Index[2] != m[2] || math.Abs(projection-477.0/112.0) > 5e-15 {
				t.Fatal("first wall should retain matching original indices on one shared ID", id, stations, projection)
			}
			if _, binary := middleRaw.Float64(); !binary {
				nonbinary[0]++
			}
		} else {
			seamCount[1]++
			if m[1] != n || right.Index[0] != 0 || right.Index[1]+m[0] != n ||
				right.Index[2] != m[2] || math.Abs(projection-1517.0/112.0) > 5e-15 {
				t.Fatal("second wall should retain its own original owners and physical station", id, stations, projection)
			}
			raw := got.UVStations[2][1][right.Index[1]]
			// Composing the two independently authored maps gives
			// C's normalized parameter 4s/(5-s), distinctly nonuniform.
			want := new(big.Rat).Mul(big.NewRat(8, 1), physicalStation)
			want.Quo(want, new(big.Rat).Sub(big.NewRat(5, 1), physicalStation)).Add(want, big.NewRat(5, 1))
			if raw.Cmp(want) != 0 {
				t.Fatal("the second seam used a regenerated grid instead of the actual middle clock", id, raw, want)
			}
			if _, binary := raw.Float64(); !binary {
				nonbinary[1]++
			}
		}
	}
	if seamCount != [2]int{2*n + 2*nT, 2*n + 2*nT} || nonbinary[0] == 0 || nonbinary[1] == 0 {
		t.Fatal("expected distinct complete perimeters with actual nonbinary stations", seamCount, nonbinary)
	}

	want := referencePlacedSweepChainVolume()
	reported, _ := got.SignedDisplayVolume.Float64()
	if math.Abs(reported-want) > .002*want {
		t.Fatal("actual mesh volume differs from independently integrated analytic source", reported, want)
	}
	bound, _ := got.DistanceUpperSquared.Float64()
	t.Logf("%d triangles, seam vertices %v, nonbinary owners %v, distance %.12g, volume %.15g vs independent %.15g, work %d",
		len(got.Mesh.Triangles), seamCount, nonbinary, math.Sqrt(bound), reported, want, got.GeometryWork)
	nearLimit := VerifiedSweepMeshOptions{Deflection: 8, NormalAngle: 1e-7, MaxDepth: 1}
	limited, err := TessellateVerifiedNormalOffsetSweepChain(context.Background(), chain, nearLimit)
	if !errors.Is(err, geom.ErrConvergence) || !reflect.DeepEqual(limited, CertifiedVerifiedSweepChainMesh{}) {
		t.Fatal("insufficient normal refinement published a chain mesh", err)
	}
}

func TestTessellateVerifiedNormalOffsetSweepChainPlanarAndLimits(t *testing.T) {
	ctx := context.Background()
	chain := planarVerifiedSweepMeshChain(t, 0)
	options := VerifiedSweepMeshOptions{Deflection: .36, NormalAngle: .04}
	got, err := TessellateVerifiedNormalOffsetSweepChain(ctx, chain, options)
	if err != nil {
		t.Fatal(err)
	}
	checkVerifiedSweepChainMesh(t, chain, got, options)
	displacement, _ := big.NewRat(1, 9).Float64()
	expected := new(big.Rat).Mul(sweepRat(displacement), big.NewRat(6, 1))
	if got.SignedDisplayVolume.Cmp(expected) != 0 || got.SquaredFacetNormalTangent == nil || got.SquaredFacetNormalTangent.Sign() != 0 {
		t.Fatal("planar chain changed its represented material volume or outward normals", got.SignedDisplayVolume, got.SquaredFacetNormalTangent)
	}
	do := func(ctx context.Context, chain *geom.VerifiedNormalOffsetSweepChain, o VerifiedSweepMeshOptions, want error) {
		t.Helper()
		got, err := TessellateVerifiedNormalOffsetSweepChain(ctx, chain, o)
		if !errors.Is(err, want) || !reflect.DeepEqual(got, CertifiedVerifiedSweepChainMesh{}) {
			t.Fatal("unqualified chain display should fail atomically", o, err, want)
		}
	}
	do(ctx, &geom.VerifiedNormalOffsetSweepChain{}, VerifiedSweepMeshOptions{}, geom.ErrInvalid)
	do(ctx, chain, VerifiedSweepMeshOptions{MaxTriangles: 27}, ErrLimit)
	do(ctx, chain, VerifiedSweepMeshOptions{MaxWork: 1}, ErrLimit)
	do(ctx, chain, VerifiedSweepMeshOptions{Deflection: .01, MaxDepth: 1}, geom.ErrConvergence)
	do(ctx, chain, VerifiedSweepMeshOptions{Deflection: 4, MaxCollisionPairs: 1}, collision.ErrLimit)
	do(ctx, planarVerifiedSweepMeshChain(t, 1<<60), VerifiedSweepMeshOptions{Deflection: 4}, collision.ErrInvalid)
	stopped, cancel := context.WithCancel(ctx)
	cancel()
	do(stopped, chain, VerifiedSweepMeshOptions{}, context.Canceled)
}

func checkVerifiedSweepChainMesh(t *testing.T, chain *geom.VerifiedNormalOffsetSweepChain,
	got CertifiedVerifiedSweepChainMesh, o VerifiedSweepMeshOptions) {
	t.Helper()
	if err := got.Mesh.ValidateSolid(); err != nil {
		t.Fatal(err)
	}
	n, nT := len(got.UVStations[0][0])-1, len(got.DisplacementStations)-1
	triangles := 12*n*n + 16*n*nT
	limit := sweepRat(o.Deflection)
	limit.Mul(limit, limit)
	if triangles != len(got.Mesh.Triangles) || len(got.Mesh.Vertices) != triangles/2+2 ||
		len(got.VertexStations) != len(got.Mesh.Vertices) || len(got.OriginalFace) != triangles ||
		!got.Collision.Complete || len(got.Collision.Contacts) != 0 || got.SignedDisplayVolume.Sign() <= 0 ||
		got.DistanceUpperSquared.Cmp(limit) > 0 || got.GeometryWork <= len(got.VertexStations)*4096 {
		t.Fatal("incomplete embedded source mesh certificate", n, nT)
	}
	faceCounts := [3][6]int{}
	original := chain.Sweeps()
	vertexOwners := make([][3]bool, len(got.Mesh.Vertices))
	ends := [3]int{n, n, nT}
	for i, face := range got.OriginalFace {
		faceCounts[face.Owner][face.Side]++
		axis := int(face.Side) / 2
		free := [3][2]int{{1, 2}, {0, 2}, {0, 1}}[axis]
		var indexes [3][3]int
		for j, id := range got.Mesh.Triangles[i] {
			vertexOwners[id][face.Owner] = true
			owner := got.VertexStations[id][face.Owner]
			if !owner.Present || owner.Index[axis] != int(face.Side)%2*ends[axis] {
				t.Fatal("original face escaped its own side or station", i, face, owner)
			}
			indexes[j] = owner.Index
		}
		a, b, c := indexes[0], indexes[1], indexes[2]
		u, v := free[0], free[1]
		area := (b[u]-a[u])*(c[v]-a[v]) - (b[v]-a[v])*(c[u]-a[u])
		sourceFace, _ := original[face.Owner].Boundary(face.Side)
		if area == 0 || (area < 0) != sourceFace.ReverseWinding() {
			t.Fatal("mesh changed an original outward orientation", i, face)
		}
	}
	for _, face := range chain.Boundary() {
		if faceCounts[face.Owner][face.Side] == 0 {
			t.Fatal("missing original outer face", face)
		}
	}
	for _, join := range chain.Joins() {
		for i, owner := range join.Owners {
			if faceCounts[owner][join.Original.Edges[i]] != 0 {
				t.Fatal("an internal wall was meshed", owner, join.Original.Edges[i])
			}
		}
	}
	seen := make(map[geom.Vec3]bool, len(got.Mesh.Vertices))
	for id, point := range got.Mesh.Vertices {
		if seen[point] {
			t.Fatal("duplicate represented vertex survived indexed stitching", id, point)
		}
		seen[point] = true
		for owner, ref := range got.VertexStations[id] {
			if ref.Present != vertexOwners[id][owner] {
				t.Fatal("vertex source metadata disagrees with actual facet incidence", id, owner)
			}
		}
	}
}

func authoredPlacedSweepMeshChain(t *testing.T) *geom.VerifiedNormalOffsetSweepChain {
	t.Helper()
	ctx := context.Background()
	rotation := [3][3]int64{{48, -39, 20}, {36, 52, 15}, {-25, 0, 60}}
	xHeights := [3][5][2]int64{
		{{4, 1}, {1, 1}, {1, 6}, {0, 1}, {0, 1}},
		{{0, 1}, {0, 1}, {1, 6}, {0, 1}, {0, 1}},
		{{0, 1}, {0, 1}, {1, 6}, {1, 1}, {4, 1}},
	}
	var sweeps [3]*geom.VerifiedNormalOffsetSweep
	for owner := range sweeps {
		recipe := geom.ExactBezierSurfaceRecipe{
			Version: 1, DegreeU: 4, DegreeV: 2, Domain: geom.ParameterRectangle{UMin: 2, UMax: 3, VMin: -1, VMax: 1},
		}
		view := geom.NormalOffsetSweepProjection{Axes: [3]int{0, 1, 2}}
		if owner == 1 {
			recipe.DegreeU, recipe.DegreeV = 2, 4
			recipe.Domain = geom.ParameterRectangle{UMin: 2, UMax: 4, VMin: -1, VMax: 0}
			view = geom.NormalOffsetSweepProjection{Axes: [3]int{1, 0, 2}, Reversed: [3]bool{true}}
		}
		if owner == 2 {
			recipe.Domain.VMin, recipe.Domain.VMax = 5, 7
		}
		for i := 0; i <= recipe.DegreeU; i++ {
			for j := 0; j <= recipe.DegreeV; j++ {
				xi, yi, weight := i, j, big.NewRat(1, 1)
				if owner == 1 {
					xi, yi = j, 2-i
					weight = [3]*big.Rat{big.NewRat(1, 1), big.NewRat(3, 2), big.NewRat(9, 4)}[i]
				}
				if owner == 2 {
					weight = [3]*big.Rat{big.NewRat(1, 1), big.NewRat(5, 4), big.NewRat(25, 16)}[j]
				}
				x, y := big.NewRat(int64(5+8*(owner-1)+2*xi), 8), big.NewRat(int64(yi-1), 1)
				zx := xHeights[owner][xi]
				z := big.NewRat(zx[0], 512*zx[1])
				z.Add(z, big.NewRat([3]int64{1, -1, 1}[yi], 64))
				world := [3]*big.Rat{big.NewRat(1, 8), big.NewRat(-1, 4), big.NewRat(5, 16)}
				for k, row := range rotation {
					for l, value := range []*big.Rat{x, y, z} {
						world[k].Add(world[k], new(big.Rat).Mul(big.NewRat(row[l], 65), value))
					}
				}
				recipe.Controls = append(recipe.Controls, [4]string{
					world[0].Mul(world[0], weight).RatString(), world[1].Mul(world[1], weight).RatString(),
					world[2].Mul(world[2], weight).RatString(), weight.RatString(),
				})
			}
		}
		source, err := geom.ImportExactBezierSurfaceRecipe(ctx, recipe, geom.SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		sweeps[owner] = startVerifiedSweep(t, source, big.NewRat(-1, 11), big.NewRat(2, 13), view)
	}
	joins := [2]geom.VerifiedNormalOffsetSweepJoin{
		{Edges: [2]geom.SurfaceEdge{geom.SurfaceEdgeUMax, geom.SurfaceEdgeVMin}, ReverseB: true,
			BStationRatio: big.NewRat(3, 2), Axis: -1, AInteriorSign: -1,
			Covector: &[3]*big.Rat{big.NewRat(48, 7), big.NewRat(36, 7), big.NewRat(-25, 7)}},
		{Edges: [2]geom.SurfaceEdge{geom.SurfaceEdgeVMax, geom.SurfaceEdgeUMin}, ReverseB: true,
			BStationRatio: big.NewRat(15, 8), Axis: -1, AInteriorSign: 1,
			Covector: &[3]*big.Rat{big.NewRat(-72, 7), big.NewRat(-54, 7), big.NewRat(75, 14)}},
	}
	return composeVerifiedSweepMeshChain(t, sweeps, joins)
}

func planarVerifiedSweepMeshChain(t *testing.T, elevation int64) *geom.VerifiedNormalOffsetSweepChain {
	t.Helper()
	ctx := context.Background()
	var sweeps [3]*geom.VerifiedNormalOffsetSweep
	for owner := range sweeps {
		var controls [][4]*big.Rat
		for u := range 2 {
			for v := range 2 {
				controls = append(controls, [4]*big.Rat{
					big.NewRat(int64(owner+u), 1), big.NewRat(int64(v), 1), big.NewRat(elevation, 1), big.NewRat(1, 1),
				})
			}
		}
		source, err := geom.NewExactHomogeneousBezierSurface(ctx, 1, 1, controls, geom.SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		sweeps[owner] = startVerifiedSweep(t, source, big.NewRat(-1, 9), big.NewRat(1, 9), geom.NormalOffsetSweepProjection{Axes: [3]int{0, 1, 2}})
	}
	join := geom.VerifiedNormalOffsetSweepJoin{Edges: [2]geom.SurfaceEdge{geom.SurfaceEdgeUMax, geom.SurfaceEdgeUMin}, Axis: 0, AInteriorSign: -1}
	return composeVerifiedSweepMeshChain(t, sweeps, [2]geom.VerifiedNormalOffsetSweepJoin{join, join})
}

func composeVerifiedSweepMeshChain(t *testing.T, sweeps [3]*geom.VerifiedNormalOffsetSweep,
	joins [2]geom.VerifiedNormalOffsetSweepJoin) *geom.VerifiedNormalOffsetSweepChain {
	t.Helper()
	ctx := context.Background()
	var pairs [2]*geom.VerifiedNormalOffsetSweepPair
	for i, join := range joins {
		pair, proof, err := geom.NewVerifiedNormalOffsetSweepPair(ctx, sweeps[i], sweeps[i+1], join, geom.NormalOffsetCoverageOptions{})
		if err != nil || !proof.Complete {
			t.Fatal("both original independent joins must qualify", i, proof, err)
		}
		pairs[i] = pair
	}
	chain, proof, err := geom.NewVerifiedNormalOffsetSweepChain(ctx, pairs[0], pairs[1], geom.SurfacePatchOptions{})
	if err != nil || !proof.Complete {
		t.Fatal("expected analytic three-chart volume", proof, err)
	}
	return chain
}

func referencePlacedSweepChainVolume() float64 {
	lo, hi := -1.0/11, 2.0/13
	linear := hi - lo
	quadratic := (hi*hi - lo*lo) / 2
	cubic := (hi*hi*hi - lo*lo*lo) / 3
	const count = 1024
	const dx, dy = 3.0 / count, 2.0 / count
	var volume float64
	for i := 0; i < count; i++ {
		x := -1 + dx*(float64(i)+.5)
		fx := ((4*x-6)*x + 2) * x / 512
		fxx := ((12*x-12)*x + 2) / 512
		for j := 0; j < count; j++ {
			y := -1 + dy*(float64(j)+.5)
			fy := y / 32
			g := 1 + fx*fx + fy*fy
			area := math.Sqrt(g)
			twoH := ((1+fy*fy)*fxx + (1+fx*fx)/32) / (g * area)
			k := fxx / (32 * g * g)
			volume += area * (linear - twoH*quadratic + k*cubic) * dx * dy
		}
	}
	return volume
}
