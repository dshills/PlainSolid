package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestMixedDiskGridDisplay(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	frame, e := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	disks := [2]ExtrusionCircle{{Center: geom.Vec2{X: 3.5, Y: 4}, Radius: 2.5}, {Center: geom.Vec2{X: 6.5, Y: 4}, Radius: 2.5}}
	for _, depth := range []float64{3, 6} {
		stock, e := CutSideOpenDualDisks(ctx, frame, geom.Vec2{Y: 4}, geom.Vec2{X: 10, Y: 10}, disks, 6, depth, 1e-10, trimvalidate.Options{})
		if e != nil {
			t.Fatal(e)
		}
		options := make([]SolidMixedFaceMeshOptions, len(stock.Solid.faces))
		for fi, f := range stock.Solid.faces {
			cuts := make([][][]*big.Rat, 1+len(f.Holes))
			for li, loop := range append([][]Use{f.Outer}, f.Holes...) {
				cuts[li] = make([][]*big.Rat, len(loop))
				for ui := range loop {
					for k := 0; k <= 16; k++ {
						cuts[li][ui] = append(cuts[li][ui], big.NewRat(int64(k), 16))
					}
				}
			}
			options[fi] = SolidMixedFaceMeshOptions{Face: MixedExactMultiSpanMeshOptions{Face: MixedExactCurvedMeshOptions{RectangularGrid: true, Cuts: cuts, Interior: CurvedQuadraticTrimMeshOptions{Deflection: .1}}}}
		}
		r, e := stock.Solid.TessellateMixedClosedCertified(ctx, StoredMixedClosedMeshOptions{Faces: options})
		if e != nil {
			t.Fatal(depth, e)
		}
		if !r.DisplayCollision.Complete || len(r.DisplayCollision.Contacts) != 0 || r.SquaredDistance.Cmp(big.NewRat(1, 100)) > 0 {
			t.Fatal("missing full certificate")
		}
		if e = r.Mesh.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		grids := 0
		for fi, f := range r.Network.Faces {
			if f.RectangularGrid {
				grids++
			}
			for local, id := range r.Network.FaceVertices[fi] {
				if r.Mesh.Vertices[id] != f.Mesh.Vertices[local] {
					t.Fatal("shared boundary changed")
				}
			}
		}
		if grids != 2 {
			t.Fatal("expected two exact curved rectangle charts", grids)
		}
		pairs := map[[2]int]bool{}
		for _, tri := range r.Mesh.Triangles {
			for j, a := range tri {
				b := tri[(j+1)%3]
				if a > b {
					a, b = b, a
				}
				pairs[[2]int{a, b}] = true
			}
		}
		if len(r.Mesh.Vertices)-len(pairs)+len(r.Mesh.Triangles) != 2 {
			t.Fatal("Euler invariant")
		}
		// Independent area of the two overlapping side-opening circular notches.
		cutArea := 6.25*(math.Pi-math.Acos(.6)) + 3
		want := 360 - depth*cutArea
		volume, _ := r.DisplayVolume.Float64()
		if math.Abs(volume-want) > .1 {
			t.Fatal("analytic volume", depth, volume, want)
		}
		t.Logf("depth=%g vertices=%d triangles=%d volume=%.12g ideal=%.12g BVH=%d", depth, len(r.Mesh.Vertices), len(r.Mesh.Triangles), volume, want, r.DisplayCollision.BroadPhasePairs)
	}
}

func TestExactRectangleSeedProofAndLimits(t *testing.T) {
	coords := [][2]*big.Rat{{big.NewRat(0, 1), big.NewRat(0, 1)}, {big.NewRat(1, 3), big.NewRat(0, 1)}, {big.NewRat(1, 1), big.NewRat(0, 1)}, {big.NewRat(1, 1), big.NewRat(2, 3)}, {big.NewRat(1, 1), big.NewRat(1, 1)}, {big.NewRat(1, 3), big.NewRat(1, 1)}, {big.NewRat(0, 1), big.NewRat(1, 1)}, {big.NewRat(0, 1), big.NewRat(2, 3)}}
	seed := CertifiedAffineQuadraticTrimMesh{ExactUV: coords, Region: AffineQuadraticTrimRegionArea{PolygonUVArea: big.NewRat(1, 1), FillSquaredDistance: new(big.Rat)}, BoundaryLoops: [][]int{{0, 1, 2, 3, 4, 5, 6, 7}}}
	for _, uv := range coords {
		u, _ := uv[0].Float64()
		v, _ := uv[1].Float64()
		seed.Mesh.Vertices = append(seed.Mesh.Vertices, geom.Vec3{X: u, Y: v})
		seed.ExactPoints = append(seed.ExactPoints, [3]*big.Rat{new(big.Rat).Set(uv[0]), new(big.Rat).Set(uv[1]), new(big.Rat)})
	}
	grid, used, e := mixedRectangleGridSeed(context.Background(), seed, 8)
	if e != nil || !used || len(grid.Mesh.Triangles) != 8 || len(grid.Mesh.Vertices) != 9 {
		t.Fatal("exact rectangle", e)
	}
	for i, p := range seed.Mesh.Vertices {
		if grid.Mesh.Vertices[i] != p {
			t.Fatal("boundary replaced")
		}
	}
	if grid.DisplaySquaredDistance.Sign() <= 0 || grid.FillSquaredDistance.Sign() <= 0 {
		t.Fatal("nonbinary interior rounding omitted")
	}
	if _, _, e := mixedRectangleGridSeed(context.Background(), seed, 7); e == nil {
		t.Fatal("triangle budget")
	}
	mismatch := seed
	mismatch.ExactUV = append([][2]*big.Rat(nil), seed.ExactUV...)
	mismatch.ExactPoints = append([][3]*big.Rat(nil), seed.ExactPoints...)
	mismatch.ExactUV[5] = [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 1)}
	mismatch.ExactPoints[5] = [3]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 1), new(big.Rat)}
	if _, used, e := mixedRectangleGridSeed(context.Background(), mismatch, 8); e != nil || used {
		t.Fatal("mismatched opposite cuts must not be invented", e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, e := mixedRectangleGridSeed(ctx, seed, 8); e != context.Canceled {
		t.Fatal("cancel", e)
	}
}
