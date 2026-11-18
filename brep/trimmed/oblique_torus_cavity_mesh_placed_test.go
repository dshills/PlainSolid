//go:build research

package trimmed

import (
	"context"
	"fmt"
	"math"
	"math/big"
	"os"
	"os/exec"
	"plainsolid/brep/placement"
	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"strings"
	"testing"
)

func TestPlacedNativeObliqueCavityDisplay(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	r := big.NewRat
	frame := [3][4]*big.Rat{{r(3, 5), r(0, 1), r(4, 5), r(1, 1)}, {r(0, 1), r(1, 1), r(0, 1), r(0, 1)}, {r(-4, 5), r(0, 1), r(3, 5), r(0, 1)}}
	body, _, e := ObliqueTorusCavity(ctx, 3, 1, 8, -6, 6, frame, 1e-9, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	s, e := PrepareObliqueTorusCavity(ctx, body, 3, 1, 8, -6, 6, frame, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	pose, e := placement.NewRationalRigid([3][4]*big.Rat{{r(9, 25), r(-4, 5), r(12, 25), r(1, 10)}, {r(12, 25), r(3, 5), r(16, 25), r(-2, 5)}, {r(-4, 5), r(0, 1), r(3, 5), r(3, 8)}}, 4096)
	if e != nil {
		t.Fatal(e)
	}
	options := PlacedObliqueTorusCavityMeshOptions{Mesh: ObliqueTorusCavityMeshOptions{Deflection: .5, Subdivisions: 8}}
	out, e := s.TessellatePlacedCertified(ctx, pose, options)
	if e != nil {
		t.Fatal(e)
	}
	if !out.BothDirections || !out.Collision.Complete || !out.LocalCollision.Complete || out.WorldRoundingUpper.Sign() <= 0 || out.DistanceUpper.Cmp(r(1, 2)) > 0 || len(out.TorusEdges) != 32 || out.ShellSignedVolumes[0].Sign() <= 0 || out.ShellSignedVolumes[1].Sign() >= 0 {
		t.Fatal("world certificate")
	}
	if out.LocalCollision.BroadPhasePairs+out.Collision.BroadPhasePairs > collision.DefaultMaxPairs {
		t.Fatal("shared collision quota")
	}
	volume, _ := out.SignedDisplayVolume.Float64()
	area := 0.
	for _, tri := range out.Mesh.Triangles {
		a, b, c := out.Mesh.Vertices[tri[0]], out.Mesh.Vertices[tri[1]], out.Mesh.Vertices[tri[2]]
		area += b.Sub(a).Cross(c.Sub(a)).Length() / 2
	}
	wantV, wantA := 768*math.Pi-6*math.Pi*math.Pi, 320*math.Pi+12*math.Pi*math.Pi
	if math.Abs(volume-wantV) > .02*wantV || math.Abs(area-wantA) > .02*wantA {
		t.Fatal("independent analytic", volume, area)
	}
	if path := os.Getenv("PLAINSOLID_OCCT_OBLIQUE_CAVITY_MESH"); path != "" {
		data, e := exec.Command(path).CombinedOutput()
		if e != nil {
			t.Fatal(e, string(data))
		}
		var v, a float64
		var lo, hi [3]float64
		_, e = fmt.Fscan(strings.NewReader(string(data)), &v, &a, &lo[0], &lo[1], &lo[2], &hi[0], &hi[1], &hi[2])
		if e != nil {
			t.Fatal(e, string(data))
		}
		if math.Abs(volume-v) > .02*v || math.Abs(area-a) > .02*a {
			t.Fatal("OCCT measures", v, a)
		}
		box, e := out.Mesh.Bounds()
		if e != nil {
			t.Fatal(e)
		}
		low := [3]float64{box.Min.X, box.Min.Y, box.Min.Z}
		high := [3]float64{box.Max.X, box.Max.Y, box.Max.Z}
		for i := 0; i < 3; i++ {
			if math.Abs(low[i]-lo[i]) > .5 || math.Abs(high[i]-hi[i]) > .5 {
				t.Fatal("world OCCT extrema", box, lo, hi)
			}
		}
	}
	limited := options
	limited.MaxWorldVertices = 1
	if bad, e := s.TessellatePlacedCertified(ctx, pose, limited); e == nil || len(bad.Mesh.Vertices) != 0 {
		t.Fatal("atomic transform quota")
	}
	// Even a loose distance bound cannot authorize a collapsed world display.
	huge := new(big.Rat).SetInt(new(big.Int).Lsh(big.NewInt(1), 60))
	collapsed, e := placement.NewRationalRigid([3][4]*big.Rat{{r(1, 1), r(0, 1), r(0, 1), huge}, {r(0, 1), r(1, 1), r(0, 1), huge}, {r(0, 1), r(0, 1), r(1, 1), huge}}, 4096)
	if e != nil {
		t.Fatal(e)
	}
	if bad, e := s.TessellatePlacedCertified(ctx, collapsed, PlacedObliqueTorusCavityMeshOptions{Mesh: ObliqueTorusCavityMeshOptions{Deflection: 100, Subdivisions: 1}}); e == nil || len(bad.Mesh.Vertices) != 0 {
		t.Fatal("collapsed world mesh accepted")
	}
	// A valid small torus near the circular stock rim lies entirely outside a
	// very coarse inscribed stock polygon. No-contact is insufficient: exact
	// represented shell nesting must reject, then accept a finer display.
	sideFrame := [3][4]*big.Rat{{r(1, 1), r(0, 1), r(0, 1), r(5, 1)}, {r(0, 1), r(1, 1), r(0, 1), r(5, 1)}, {r(0, 1), r(0, 1), r(1, 1), r(0, 1)}}
	sideBody, _, e := ObliqueTorusCavity(ctx, .25, .125, 8, -6, 6, sideFrame, 1e-9, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	side, e := PrepareObliqueTorusCavity(ctx, sideBody, .25, .125, 8, -6, 6, sideFrame, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	identity, e := placement.NewRationalRigid([3][4]*big.Rat{{r(1, 1), r(0, 1), r(0, 1), r(0, 1)}, {r(0, 1), r(1, 1), r(0, 1), r(0, 1)}, {r(0, 1), r(0, 1), r(1, 1), r(0, 1)}}, 4096)
	if e != nil {
		t.Fatal(e)
	}
	coarse := PlacedObliqueTorusCavityMeshOptions{Mesh: ObliqueTorusCavityMeshOptions{Deflection: 20, Subdivisions: 1}}
	if bad, e := side.TessellatePlacedCertified(ctx, identity, coarse); e == nil || len(bad.Mesh.Vertices) != 0 {
		t.Fatal("outside reversed shell admitted")
	}
	coarse.Mesh.Subdivisions = 2
	if good, e := side.TessellatePlacedCertified(ctx, identity, coarse); e != nil || good.Shells[1].Depth != 1 {
		t.Fatal("refined cavity nesting", e)
	}

}
