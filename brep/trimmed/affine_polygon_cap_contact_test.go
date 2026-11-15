package trimmed

import (
	"context"
	"encoding/json"
	"math"
	"math/big"
	"os"
	"os/exec"
	"plainsolid/geom"
	"testing"
)

func affinePolygonCapFixture(t *testing.T, shift float64, sameSide bool) (Solid, Solid) {
	a := convexPolygonShellMappedFixture(t, []geom.Vec2{{}, {X: 2}, {X: 3, Y: 1}, {X: 2, Y: 2}, {Y: 2}}, func(x, y, z float64) geom.Vec3 { z -= 2; return geom.Vec3{X: x + z/4, Y: y + x/8, Z: z + y/4} })
	c := convexPolygonShellMappedFixture(t, []geom.Vec2{{X: .5, Y: .5}, {X: 2.75, Y: .5}, {X: .5, Y: 2.75}}, func(x, y, z float64) geom.Vec3 {
		x += shift
		if sameSide {
			z -= 2
		}
		return geom.Vec3{X: x - z/8, Y: y + x/8 + z/4, Z: z + y/4}
	})
	return a, c
}
func TestConvexAffinePolygonCapContact(t *testing.T) {
	ctx := context.Background()
	a, b := affinePolygonCapFixture(t, 0, false)
	for order, sources := range [][2]Solid{{a, b}, {b, a}} {
		faces := [2]int{1, 0}
		if order == 1 {
			faces = [2]int{0, 1}
		}
		r, e := CertifyConvexAffinePolygonCapContact(ctx, sources[0], sources[1], faces, ConvexAffinePolygonCapOptions{})
		if e != nil {
			t.Fatal(e)
		}
		area, squared, _, e := r.Contact().Contact().CoplanarArea()
		if e != nil || area.Cmp(big.NewRat(143, 1024)) != 0 || squared.Cmp(big.NewRat(22268961, 4194304)) != 0 {
			t.Fatal(area, squared, e)
		}
		points, e := r.Contact().Contact().Vertices()
		if e != nil || len(points) != 5 {
			t.Fatal("complete cap polygon", len(points), e)
		}
		if r.ExactCommonVolume().Sign() != 0 || r.OriginalFaces() != faces {
			t.Fatal("ownership")
		}
		x, _ := r.OriginalShell(0)
		y, _ := r.OriginalShell(1)
		if new(big.Rat).Add(x.ExactVolume(), y.ExactVolume()).Cmp(big.NewRat(60639, 4096)) != 0 {
			t.Fatal("source volume")
		}
		pl := r.SupportingPlane()
		pl[0].SetInt64(999)
		if r.SupportingPlane()[0].Cmp(pl[0]) == 0 {
			t.Fatal("mutable")
		}
		if q, e := CertifyConvexAffinePolygonCapContact(ctx, sources[0], sources[1], faces, ConvexAffinePolygonCapOptions{MaxOperations: r.ExactOperations()}); e != nil || q == nil {
			t.Fatal("exact budget", e)
		}
		if q, e := CertifyConvexAffinePolygonCapContact(ctx, sources[0], sources[1], faces, ConvexAffinePolygonCapOptions{MaxOperations: r.ExactOperations() - 1}); e == nil || q != nil {
			t.Fatal("short budget")
		}
	}
}
func TestConvexAffinePolygonCapContactRejects(t *testing.T) {
	for _, tc := range []struct {
		shift float64
		same  bool
		faces [2]int
	}{{0, true, [2]int{1, 1}}, {0, false, [2]int{0, 0}}, {4, false, [2]int{1, 0}}, {2.5, false, [2]int{1, 0}}} {
		a, b := affinePolygonCapFixture(t, tc.shift, tc.same)
		if r, e := CertifyConvexAffinePolygonCapContact(context.Background(), a, b, tc.faces, ConvexAffinePolygonCapOptions{}); e == nil || r != nil {
			t.Fatal("unproved interface", tc)
		}
	}
	a, b := affinePolygonCapFixture(t, 0, false)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if r, e := CertifyConvexAffinePolygonCapContact(ctx, a, b, [2]int{1, 0}, ConvexAffinePolygonCapOptions{}); e == nil || r != nil {
		t.Fatal("cancel")
	}
}

func TestConvexAffinePolygonCapOCCT(t *testing.T) {
	executable := os.Getenv("PLAINSOLID_OCCT_AFFINE_POLYGON_CAP")
	if executable == "" {
		t.Skip("set PLAINSOLID_OCCT_AFFINE_POLYGON_CAP to independent tools/occtref/affine_polygon_caps.cxx executable")
	}
	data, e := exec.Command(executable).Output()
	if e != nil {
		t.Fatal(e)
	}
	var oracle struct {
		Valid                                bool
		Solids                               int
		Volume, CommonVolume, CapAreaSquared float64
	}
	if e = json.Unmarshal(data, &oracle); e != nil {
		t.Fatal(e)
	}
	a, b := affinePolygonCapFixture(t, 0, false)
	r, e := CertifyConvexAffinePolygonCapContact(context.Background(), a, b, [2]int{1, 0}, ConvexAffinePolygonCapOptions{})
	if e != nil {
		t.Fatal(e)
	}
	_, squared, _, e := r.Contact().Contact().CoplanarArea()
	if e != nil {
		t.Fatal(e)
	}
	sq, _ := squared.Float64()
	x, _ := r.OriginalShell(0)
	y, _ := r.OriginalShell(1)
	volume, _ := new(big.Rat).Add(x.ExactVolume(), y.ExactVolume()).Float64()
	if !oracle.Valid || oracle.Solids != 1 || math.Abs(oracle.Volume-volume) > 1e-10 || math.Abs(oracle.CommonVolume) > 1e-12 || math.Abs(oracle.CapAreaSquared-sq) > 1e-10 {
		t.Fatal(oracle, volume, sq)
	}
}
