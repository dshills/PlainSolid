package offset

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"reflect"
	"testing"

	"plainsolid/collision"
	"plainsolid/geom"
)

func TestTessellateVerifiedNormalOffsetSweepPlanarRationalSlab(t *testing.T) {
	ctx := context.Background()
	s := verifiedSweepSlab(t, new(big.Rat))
	o := VerifiedSweepMeshOptions{Deflection: .2}
	got, err := TessellateVerifiedNormalOffsetSweep(ctx, s, o)
	if err != nil {
		t.Fatal(err)
	}
	checkVerifiedSweepMesh(t, s, got, o.Deflection)
	t0, t1 := big.NewRat(-1, 7), big.NewRat(2, 7)
	ts := got.DisplacementStations
	if len(ts) < 3 || ts[0].Cmp(t0) != 0 || ts[len(ts)-1].Cmp(t1) != 0 ||
		ts[(len(ts)-1)/2].Cmp(big.NewRat(1, 14)) != 0 {
		t.Fatal("lost exact nonbinary displacement stations", ts)
	}
	z0, _ := t0.Float64()
	z1, _ := t1.Float64()
	want := new(big.Rat).Sub(new(big.Rat).SetFloat64(z1), new(big.Rat).SetFloat64(z0))
	if got.SignedDisplayVolume.Cmp(want) != 0 {
		t.Fatal("wrong exact represented slab volume", got.SignedDisplayVolume, want)
	}
	for id, reference := range got.VertexStations {
		z, _ := ts[reference[2]].Float64()
		point := geom.Vec3{X: got.UStations[reference[0]], Y: got.VStations[reference[1]], Z: z}
		if got.Mesh.Vertices[id] != point {
			t.Fatal("lost original shared slab corner", id, reference, point, got.Mesh.Vertices[id])
		}
	}
	v, _ := got.SignedDisplayVolume.Float64()
	if math.Abs(v-3.0/7.0) > 1e-15 {
		t.Fatal("display volume differs from the analytic slab", v)
	}
	// A whole layer has exact XY box diameter squared 2. The rounded
	// upper displacement lies outside its true point-height interval.
	coarse, err := TessellateVerifiedNormalOffsetSweep(ctx, s, VerifiedSweepMeshOptions{Deflection: 2, MaxTriangles: 12})
	if err != nil {
		t.Fatal(err)
	}
	gap := new(big.Rat).Sub(t1, new(big.Rat).SetFloat64(z1))
	wantDiameter := new(big.Rat).Add(big.NewRat(2, 1), gap.Mul(gap, gap))
	if gap.Sign() == 0 || coarse.DistanceUpperSquared.Cmp(wantDiameter) != 0 {
		t.Fatal("physical box certificate omitted actual coordinate rounding", coarse.DistanceUpperSquared, wantDiameter)
	}
	logVerifiedSweepMesh(t, got)
}

func TestTessellateVerifiedNormalOffsetSweepCurved(t *testing.T) {
	ctx := context.Background()
	for _, rotated := range []bool{false, true} {
		t.Run(fmt.Sprint(rotated), func(t *testing.T) {
			var net [][4]*big.Rat
			for i, x := range []int64{-1, 0, 1} {
				for _, y := range []int64{0, 1} {
					a, b := x, y
					if rotated {
						a, b = -y, x
					}
					net = append(net, [4]*big.Rat{big.NewRat(a, 1), big.NewRat(b, 1), big.NewRat([]int64{1, -1, 1}[i], 1), big.NewRat(1, 1)})
				}
			}
			p, err := geom.NewExactHomogeneousBezierSurface(ctx, 2, 1, net, geom.SurfacePatchOptions{})
			if err != nil {
				t.Fatal(err)
			}
			p, err = p.Trim(geom.ParameterRectangle{UMin: .375, UMax: .625, VMax: 1})
			if err != nil {
				t.Fatal(err)
			}
			view := geom.NormalOffsetSweepProjection{Axes: [3]int{0, 1, 2}}
			if rotated {
				view = geom.NormalOffsetSweepProjection{Axes: [3]int{1, 0, 2}, Reversed: [3]bool{false, true}}
			}
			s := startVerifiedSweep(t, p, big.NewRat(-1, 10), big.NewRat(1, 10), view)
			o := VerifiedSweepMeshOptions{Deflection: .16}
			got, err := TessellateVerifiedNormalOffsetSweep(ctx, s, o)
			if err != nil {
				t.Fatal(err)
			}
			checkVerifiedSweepMesh(t, s, got, o.Deflection)
			if got.UStations[0] != .375 || got.UStations[len(got.UStations)-1] != .625 {
				t.Fatal("lost the original restricted source domain", got.UStations)
			}
			// The odd displacement part of the analytic Jacobian
			// integrates to zero over [-.1,.1]. What remains is the
			// original parabolic strip area times its thickness.
			want := .2 * (.25*math.Sqrt(1.25) + .5*math.Asinh(.5))
			v, _ := got.SignedDisplayVolume.Float64()
			if math.Abs(v-want) > want*.02 {
				t.Fatal("display volume differs from the independently integrated swept strip", v, want)
			}
			bound, _ := got.DistanceUpperSquared.Float64()
			observed := 0.0
			for _, tri := range got.Mesh.Triangles {
				var u, v, d float64
				var poly geom.Vec3
				for _, id := range tri {
					station := got.VertexStations[id]
					u += got.UStations[station[0]]
					v += got.VStations[station[1]]
					tm, _ := got.DisplacementStations[station[2]].Float64()
					d += tm
					poly = poly.Add(got.Mesh.Vertices[id])
				}
				x, y := 2*u/3-1, v/3
				normal := geom.Vec3{X: -2 * x, Z: 1}
				analytic := geom.Vec3{X: x, Y: y, Z: x * x}.Add(normal.Scale(d / (3 * normal.Length())))
				if rotated {
					analytic.X, analytic.Y = -analytic.Y, analytic.X
				}
				delta := analytic.Sub(poly.Scale(1.0 / 3)).Length()
				if delta*delta > bound {
					t.Fatal("independent continuous sweep escaped the cell certificate", delta, bound)
				}
				observed = max(observed, delta)
			}
			if observed <= 0 {
				t.Fatal("curved control must exercise nonzero approximation error")
			}
			t.Logf("independent analytic volume %.16g, sampled mesh deviation %.12g", want, observed)
			logVerifiedSweepMesh(t, got)
		})
	}
}

func TestTessellateVerifiedNormalOffsetSweepAtomicRefusals(t *testing.T) {
	ctx := context.Background()
	s := verifiedSweepSlab(t, new(big.Rat))
	requireEmpty := func(o VerifiedSweepMeshOptions, expected error) {
		t.Helper()
		out, err := TessellateVerifiedNormalOffsetSweep(ctx, s, o)
		if !errors.Is(err, expected) || !reflect.DeepEqual(out, CertifiedVerifiedSweepMesh{}) {
			t.Fatal("unproved sweep published a partial boundary", o, err)
		}
	}
	requireEmpty(VerifiedSweepMeshOptions{MaxTriangles: 11}, ErrLimit)
	requireEmpty(VerifiedSweepMeshOptions{MaxWork: 1}, ErrLimit)
	requireEmpty(VerifiedSweepMeshOptions{Deflection: 2, MaxCollisionPairs: 1}, collision.ErrLimit)
	requireEmpty(VerifiedSweepMeshOptions{Deflection: .01, MaxDepth: 1}, geom.ErrConvergence)
	stopped, cancel := context.WithCancel(ctx)
	cancel()
	if out, err := TessellateVerifiedNormalOffsetSweep(stopped, s, VerifiedSweepMeshOptions{}); !errors.Is(err, context.Canceled) ||
		!reflect.DeepEqual(out, CertifiedVerifiedSweepMesh{}) {
		t.Fatal("canceled sweep published a boundary", err)
	}
	// The exact positive slab still exists at this source elevation.
	// All represented displacement layers collapse to one scalar.
	s = verifiedSweepSlab(t, big.NewRat(1<<60, 1))
	requireEmpty(VerifiedSweepMeshOptions{Deflection: 2}, collision.ErrInvalid)
}

func verifiedSweepSlab(t *testing.T, elevation *big.Rat) *geom.VerifiedNormalOffsetSweep {
	t.Helper()
	var net [][4]*big.Rat
	for u := range 2 {
		for v := range 2 {
			net = append(net, [4]*big.Rat{big.NewRat(int64(u), 1), big.NewRat(int64(v), 1), elevation, big.NewRat(1, 1)})
		}
	}
	p, err := geom.NewExactHomogeneousBezierSurface(context.Background(), 1, 1, net, geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return startVerifiedSweep(t, p, big.NewRat(-1, 7), big.NewRat(2, 7),
		geom.NormalOffsetSweepProjection{Axes: [3]int{0, 1, 2}})
}

func startVerifiedSweep(t *testing.T, p *geom.SurfacePatch, lower, upper *big.Rat, view geom.NormalOffsetSweepProjection) *geom.VerifiedNormalOffsetSweep {
	t.Helper()
	s, proof, err := geom.NewVerifiedNormalOffsetSweep(context.Background(), p, lower, upper, view, geom.NormalOffsetCoverageOptions{})
	if err != nil || !proof.Complete {
		t.Fatal("expected an independently verified input sweep", proof, err)
	}
	return s
}

func checkVerifiedSweepMesh(t *testing.T, s *geom.VerifiedNormalOffsetSweep, m CertifiedVerifiedSweepMesh, deflection float64) {
	t.Helper()
	if err := m.Mesh.ValidateSolid(); err != nil {
		t.Fatal(err)
	}
	n, k := len(m.UStations)-1, len(m.DisplacementStations)-1
	count := 4*n*n + 8*n*k
	d := new(big.Rat).SetFloat64(deflection)
	d.Mul(d, d)
	if len(m.VStations) != n+1 || len(m.Mesh.Triangles) != count || len(m.OriginalFace) != count ||
		len(m.Mesh.Vertices) != count/2+2 || len(m.VertexStations) != len(m.Mesh.Vertices) ||
		m.DistanceUpperSquared.Sign() <= 0 || m.DistanceUpperSquared.Cmp(d) > 0 ||
		m.SignedDisplayVolume.Sign() <= 0 || !m.Collision.Complete || len(m.Collision.Contacts) != 0 ||
		m.GeometryWork <= len(m.Mesh.Vertices)*4096 {
		t.Fatal("incomplete original sweep certificate", n, k)
	}
	ends := [3]int{n, n, k}
	seen := make(map[[3]int]bool)
	for _, station := range m.VertexStations {
		if seen[station] || !(station[0] == 0 || station[0] == ends[0] ||
			station[1] == 0 || station[1] == ends[1] || station[2] == 0 || station[2] == ends[2]) {
			t.Fatal("lost unique boundary parameter ownership", station)
		}
		seen[station] = true
	}
	for i, tri := range m.Mesh.Triangles {
		side := m.OriginalFace[i]
		f, err := s.Boundary(side)
		if err != nil {
			t.Fatal(err)
		}
		fixed := int(side) / 2
		fixedStation := 0
		if int(side)%2 != 0 {
			fixedStation = ends[fixed]
		}
		a, b, c := m.VertexStations[tri[0]], m.VertexStations[tri[1]], m.VertexStations[tri[2]]
		if a[fixed] != fixedStation || b[fixed] != fixedStation || c[fixed] != fixedStation {
			t.Fatal("triangle escaped its original analytic face", i, side)
		}
		directions := [3][2]int{{1, 2}, {0, 2}, {0, 1}}[fixed]
		x, y := directions[0], directions[1]
		det := (b[x]-a[x])*(c[y]-a[y]) - (b[y]-a[y])*(c[x]-a[x])
		if det == 0 || (det < 0) != f.ReverseWinding() {
			t.Fatal("triangle changed analytic face winding", i, side)
		}
	}
}

func logVerifiedSweepMesh(t *testing.T, m CertifiedVerifiedSweepMesh) {
	t.Helper()
	distance, _ := m.DistanceUpperSquared.Float64()
	t.Logf("grid=%dx%dx%d, vertices=%d, triangles=%d, certified distance=%.12g, signed volume=%s, work=%d, BVH pairs=%d",
		len(m.UStations)-1, len(m.VStations)-1, len(m.DisplacementStations)-1, len(m.Mesh.Vertices), len(m.Mesh.Triangles),
		math.Sqrt(distance), m.SignedDisplayVolume.FloatString(16), m.GeometryWork, m.Collision.BroadPhasePairs)
}
