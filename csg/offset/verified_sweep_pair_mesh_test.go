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

func TestTessellateVerifiedNormalOffsetSweepPairPlanar(t *testing.T) {
	pair := verifiedSweepMeshPairFixture(t, false, false, 0)
	o := VerifiedSweepMeshOptions{Deflection: .36}
	got, err := TessellateVerifiedNormalOffsetSweepPair(context.Background(), pair, o)
	if err != nil {
		t.Fatal(err)
	}
	checkVerifiedSweepPairMesh(t, pair, got, o, false)
	z, _ := big.NewRat(1, 9).Float64()
	want := new(big.Rat).Mul(sweepRat(z), big.NewRat(8, 1))
	if got.SignedDisplayVolume.Cmp(want) != 0 {
		t.Fatal("stitched mesh differs from the exact represented planar volume", got.SignedDisplayVolume, want)
	}
	if got.SquaredFacetNormalTangent != nil {
		t.Fatal("distance request unexpectedly reported a normal angle")
	}
}

func TestTessellateVerifiedNormalOffsetSweepPairCurvedProjective(t *testing.T) {
	slowtest.Skip(t)
	pair := verifiedSweepMeshPairFixture(t, true, true, 0)
	o := VerifiedSweepMeshOptions{Deflection: .36, NormalAngle: .16}
	got, err := TessellateVerifiedNormalOffsetSweepPair(context.Background(), pair, o)
	if err != nil {
		t.Fatal(err)
	}
	checkVerifiedSweepPairMesh(t, pair, got, o, true)
	// The symmetric displacement removes the odd term of the cylinder's
	// exact offset Jacobian. The surface area is twice the arc length of
	// z=y²/64 over [-1,1], since the two original strips have unit width.
	want := 4.0 / 9.0 * (math.Hypot(1, 1.0/32.0) + 32*math.Asinh(1.0/32.0))
	v, _ := got.SignedDisplayVolume.Float64()
	if math.Abs(v-want) > want*.001 {
		t.Fatal("stitched display disagrees with the independently integrated curved sweep", v, want)
	}
	q, _ := got.SquaredFacetNormalTangent.Float64()
	angle := math.Atan(math.Sqrt(q))
	if got.SquaredFacetNormalTangent.Sign() <= 0 || angle > o.NormalAngle {
		t.Fatal("continuous facet angle was not certified", angle, o.NormalAngle)
	}
	t.Logf("independent analytic volume %.17g, displayed volume %.17g, certified facet angle %.10g rad", want, v, angle)
}

func TestTessellateVerifiedNormalOffsetSweepPairRefusesAtomically(t *testing.T) {
	ctx := context.Background()
	pair := verifiedSweepMeshPairFixture(t, true, true, 0)
	check := func(ctx context.Context, p *geom.VerifiedNormalOffsetSweepPair, o VerifiedSweepMeshOptions, want error) {
		t.Helper()
		got, err := TessellateVerifiedNormalOffsetSweepPair(ctx, p, o)
		if !errors.Is(err, want) || !reflect.DeepEqual(got, CertifiedVerifiedSweepPairMesh{}) {
			t.Fatal("unproved two-chart display published a result", o, got, err, want)
		}
	}
	check(ctx, &geom.VerifiedNormalOffsetSweepPair{}, VerifiedSweepMeshOptions{}, geom.ErrInvalid)
	check(ctx, pair, VerifiedSweepMeshOptions{MaxTriangles: 19}, ErrLimit)
	check(ctx, pair, VerifiedSweepMeshOptions{MaxWork: 1}, ErrLimit)
	check(ctx, pair, VerifiedSweepMeshOptions{Deflection: 8, MaxCollisionPairs: 1}, collision.ErrLimit)
	check(ctx, pair, VerifiedSweepMeshOptions{Deflection: .03, MaxDepth: 1}, geom.ErrConvergence)
	check(ctx, pair, VerifiedSweepMeshOptions{Deflection: 8, NormalAngle: 1e-7, MaxDepth: 1}, geom.ErrConvergence)
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	check(cancelled, pair, VerifiedSweepMeshOptions{}, context.Canceled)
	// The exact normal sweep remains nonempty. Its binary representation
	// has one z coordinate at this translation, so embedding must refuse.
	collapsed := verifiedSweepMeshPairFixture(t, false, false, 1<<60)
	check(ctx, collapsed, VerifiedSweepMeshOptions{Deflection: 8}, collision.ErrInvalid)
}

func checkVerifiedSweepPairMesh(t *testing.T, pair *geom.VerifiedNormalOffsetSweepPair,
	got CertifiedVerifiedSweepPairMesh, o VerifiedSweepMeshOptions, curved bool) {
	t.Helper()
	if err := got.Mesh.ValidateSolid(); err != nil {
		t.Fatal(err)
	}
	n, nT := len(got.UVStations[0][0])-1, len(got.DisplacementStations)-1
	triangles := 8*n*n + 12*n*nT
	limit := sweepRat(o.Deflection)
	limit.Mul(limit, limit)
	distance, _ := got.DistanceUpperSquared.Float64()
	var normalAngle float64
	if got.SquaredFacetNormalTangent != nil {
		tangent, _ := got.SquaredFacetNormalTangent.Float64()
		normalAngle = math.Atan(math.Sqrt(tangent))
	}
	if n < 2 || nT < 2 || triangles != len(got.Mesh.Triangles) || len(got.Mesh.Vertices) != triangles/2+2 ||
		len(got.VertexStations) != len(got.Mesh.Vertices) || len(got.OriginalFace) != triangles ||
		!got.Collision.Complete || len(got.Collision.Contacts) != 0 ||
		got.SignedDisplayVolume.Sign() <= 0 || got.DistanceUpperSquared.Sign() <= 0 || got.DistanceUpperSquared.Cmp(limit) > 0 ||
		got.GeometryWork <= len(got.VertexStations)*4096 {
		t.Fatal("incomplete stitched boundary certificate", n, nT)
	}
	displacements := got.DisplacementStations
	if displacements[0].Cmp(big.NewRat(-1, 9)) != 0 || displacements[nT].Cmp(big.NewRat(1, 9)) != 0 {
		t.Fatal("lost original signed displacement interval", displacements)
	}
	join, _ := pair.Join()
	sources := pair.Sweeps()
	faces := pair.Boundary()
	var counts [2][6]int
	ownerUsage := make([][2]bool, len(got.Mesh.Vertices))
	ends := [3]int{n, n, nT}
	for id, tri := range got.Mesh.Triangles {
		f := got.OriginalFace[id]
		counts[f.Owner][f.Side]++
		side := int(f.Side)
		axis := side / 2
		free := [3][2]int{{1, 2}, {0, 2}, {0, 1}}[axis]
		var stations [3][3]int
		for i, vertex := range tri {
			ownerUsage[vertex][f.Owner] = true
			owner := got.VertexStations[vertex][f.Owner]
			if !owner.Present || owner.Index[axis] != side%2*ends[axis] {
				t.Fatal("triangle escaped its original owner chart or side", id, i, f, owner)
			}
			stations[i] = owner.Index
		}
		a, b, c := stations[0], stations[1], stations[2]
		u, v := free[0], free[1]
		det := (b[u]-a[u])*(c[v]-a[v]) - (b[v]-a[v])*(c[u]-a[u])
		face, _ := sources[f.Owner].Boundary(f.Side)
		if det == 0 || (det < 0) != face.ReverseWinding() {
			t.Fatal("triangle reversed retained original winding", id, f, det)
		}
		if curved {
			checkVerifiedSweepPairTriangle(t, got, f, tri, stations, math.Sqrt(distance), normalAngle)
		}
	}
	for _, face := range faces {
		if counts[face.Owner][face.Side] == 0 {
			t.Fatal("lost a retained analytic face", face)
		}
	}
	for owner, edge := range join.Edges {
		if counts[owner][edge] != 0 {
			t.Fatal("internal swept face survived", owner, edge)
		}
	}
	shared, nonbinary := 0, 0
	for id, source := range got.VertexStations {
		for owner, station := range source {
			if station.Present != ownerUsage[id][owner] {
				t.Fatal("stated vertex owner disagrees with actual incidence", id, source, ownerUsage[id])
			}
		}
		if !source[0].Present || !source[1].Present {
			continue
		}
		shared++
		a, b := source[0].Index, source[1].Index
		y := got.UVStations[0][1][a[1]]
		s := new(big.Rat).Add(y, big.NewRat(1, 1))
		s.Quo(s, big.NewRat(2, 1))
		rawB := got.UVStations[1][1][b[1]]
		expectedB := y
		if join.ReverseB {
			rawB = got.UVStations[1][0][b[0]]
			num := new(big.Rat).Sub(big.NewRat(1, 1), s)
			den := new(big.Rat).Add(big.NewRat(1, 1), new(big.Rat).Quo(s, big.NewRat(2, 1)))
			num.Quo(num, den).Mul(num, big.NewRat(2, 1))
			expectedB = new(big.Rat).Add(big.NewRat(2, 1), num)
			if _, exact := rawB.Float64(); !exact {
				nonbinary++
			}
			if b[1] != 0 || b[0] != n-a[1] {
				t.Fatal("reverse raw chart did not identify the matching shared vertex", a, b)
			}
		} else if b[0] != 0 || b[1] != a[1] {
			t.Fatal("linear raw chart did not identify the matching shared vertex", a, b)
		}
		if rawB.Cmp(expectedB) != 0 || a[0] != n || a[2] != b[2] ||
			(a[1] != 0 && a[1] != n && a[2] != 0 && a[2] != nT) {
			t.Fatal("shared vertex lost exact original station correspondence or stayed on the internal wall", a, b, rawB, expectedB)
		}
		yy, _ := y.Float64()
		d, _ := displacements[a[2]].Float64()
		want := geom.Vec3{X: 5.0 / 8, Y: yy, Z: d}
		if curved {
			length := math.Hypot(1, yy/32)
			want.Y = yy - d*yy/(32*length)
			want.Z = yy*yy/64 + d/length
		}
		if got.Mesh.Vertices[id].Sub(want).Length() > 5e-16 {
			t.Fatal("shared vertex differs from independent analytic wall", id, want, got.Mesh.Vertices[id])
		}
	}
	if shared != 2*n+2*nT || (join.ReverseB && nonbinary == 0) {
		t.Fatal("missing a shared wall-perimeter vertex or nonbinary raw station", shared, nonbinary)
	}
	bound, _ := got.DistanceUpperSquared.Float64()
	volume, _ := got.SignedDisplayVolume.Float64()
	t.Logf("%d triangles, %d shared vertices including %d nonbinary owner occurrences, distance %.10g, volume %.17g, geometry work %d",
		triangles, shared, nonbinary, math.Sqrt(bound), volume, got.GeometryWork)
}

func checkVerifiedSweepPairTriangle(t *testing.T, got CertifiedVerifiedSweepPairMesh, face VerifiedSweepPairMeshFace,
	tri [3]int, stations [3][3]int, distance, angle float64) {
	t.Helper()
	var u, v, d float64
	for _, index := range stations {
		a, _ := got.UVStations[face.Owner][0][index[0]].Float64()
		b, _ := got.UVStations[face.Owner][1][index[1]].Float64()
		displacement, _ := got.DisplacementStations[index[2]].Float64()
		u, v, d = u+a/3, v+b/3, d+displacement/3
	}

	x, y := u-19.0/8, v
	if face.Owner == 1 {
		projective := (u - 2) / 2
		x = 13.0/8 + v
		y = 1 - 3*projective/(1+projective/2)
	}
	length := math.Hypot(1, y/32)
	original := geom.Vec3{X: x, Y: y - d*y/(32*length), Z: y*y/64 + d/length}
	a, b, c := got.Mesh.Vertices[tri[0]], got.Mesh.Vertices[tri[1]], got.Mesh.Vertices[tri[2]]
	if deviation := a.Add(b).Add(c).Scale(1.0 / 3).Sub(original).Length(); deviation > distance+1e-14 {
		t.Fatal("facet and independently evaluated analytic point exceed the certified distance", face, deviation, distance)
	}
	var outward geom.Vec3
	switch face.Side {
	case geom.NormalOffsetSweepLower:
		outward = geom.Vec3{Y: y / 32, Z: -1}
	case geom.NormalOffsetSweepUpper:
		outward = geom.Vec3{Y: -y / 32, Z: 1}
	case geom.NormalOffsetSweepUMin:
		outward = geom.Vec3{X: -1}
		if face.Owner == 1 {
			outward = geom.Vec3{Y: 1, Z: 1.0 / 32}
		}
	case geom.NormalOffsetSweepUMax:
		outward = geom.Vec3{Y: -1, Z: 1.0 / 32}
	case geom.NormalOffsetSweepVMin:
		outward = geom.Vec3{Y: -1, Z: 1.0 / 32}
	case geom.NormalOffsetSweepVMax:
		outward = geom.Vec3{Y: 1, Z: 1.0 / 32}
		if face.Owner == 1 {
			outward = geom.Vec3{X: 1}
		}
	}
	facet := b.Sub(a).Cross(c.Sub(a))
	deviation := math.Atan2(facet.Cross(outward).Length(), facet.Dot(outward))
	if deviation > angle+1e-12 {
		t.Fatal("actual facet exceeds the certified normal angle against the analytic cylinder", face, deviation, angle)
	}
}

func TestVerifiedSweepPairStationBracketUsesOutwardAdjacentValues(t *testing.T) {
	for _, station := range []*big.Rat{big.NewRat(-1, 10), big.NewRat(1, 10), big.NewRat(-1, 3), big.NewRat(1, 3), big.NewRat(5, 8)} {
		got := verifiedSweepPairStationBracket(station, [2]float64{-1, 1})
		if sweepRat(got[0]).Cmp(station) > 0 || sweepRat(got[1]).Cmp(station) < 0 ||
			(got[0] != got[1] && math.Nextafter(got[0], got[1]) != got[1]) {
			t.Fatal("binary bound rounded a rational source station inward", station, got)
		}
	}
}

func verifiedSweepMeshPairFixture(t *testing.T, curved, projective bool, elevation int64) *geom.VerifiedNormalOffsetSweepPair {
	t.Helper()
	ctx := context.Background()
	var sweeps [2]*geom.VerifiedNormalOffsetSweep
	for owner := range sweeps {
		// Author both charts independently. The second can use order
		// (reversed y,x) and the exact projective weight ratio 3/2.
		recipe := geom.ExactBezierSurfaceRecipe{
			Version: 1, DegreeU: 2, DegreeV: 2, Domain: geom.ParameterRectangle{UMin: 2, UMax: 3, VMin: -1, VMax: 1},
		}
		view := geom.NormalOffsetSweepProjection{Axes: [3]int{0, 1, 2}}
		backward := owner == 1 && projective
		if backward {
			recipe.Domain = geom.ParameterRectangle{UMin: 2, UMax: 4, VMin: -1, VMax: 0}
			view = geom.NormalOffsetSweepProjection{Axes: [3]int{1, 0, 2}, Reversed: [3]bool{true}}
		}
		for i := int64(0); i <= 2; i++ {
			for j := int64(0); j <= 2; j++ {
				xi, yi, weight := i, j, big.NewRat(1, 1)
				if backward {
					xi, yi, weight = j, 2-i, []*big.Rat{big.NewRat(1, 1), big.NewRat(3, 2), big.NewRat(9, 4)}[i]
				}
				x := big.NewRat(5+4*(xi+int64(owner)*2-2), 8)
				y := big.NewRat(yi-1, 1)
				z := big.NewRat(elevation, 1)
				if curved {
					z.Add(z, big.NewRat([3]int64{1, -1, 1}[yi], 64))
				}
				recipe.Controls = append(recipe.Controls, [4]string{
					x.Mul(x, weight).RatString(), y.Mul(y, weight).RatString(), z.Mul(z, weight).RatString(), weight.RatString(),
				})
			}
		}
		source, err := geom.ImportExactBezierSurfaceRecipe(ctx, recipe, geom.SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		sweeps[owner] = startVerifiedSweep(t, source, big.NewRat(-1, 9), big.NewRat(1, 9), view)
	}
	join := geom.VerifiedNormalOffsetSweepJoin{
		Edges: [2]geom.SurfaceEdge{geom.SurfaceEdgeUMax, geom.SurfaceEdgeUMin}, Axis: 0, AInteriorSign: -1,
	}
	if projective {
		join.Edges[1], join.ReverseB, join.BStationRatio = geom.SurfaceEdgeVMin, true, big.NewRat(3, 2)
	}
	pair, proof, err := geom.NewVerifiedNormalOffsetSweepPair(ctx, sweeps[0], sweeps[1], join, geom.NormalOffsetCoverageOptions{})
	if err != nil || !proof.Complete {
		t.Fatal("independent exact sweep pair is required", proof, err)
	}
	return pair
}
