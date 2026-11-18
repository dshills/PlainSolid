//go:build research

package trimmed_test

import (
	"context"
	"math"
	"math/big"
	"plainsolid/brep"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"testing"
)

func TestContainedNativePrismBossFuseBothEntries(t *testing.T) {
	stock, _ := brep.Box(geom.Vec3{}, geom.Vec3{X: 2, Y: 2, Z: 3})
	for _, tc := range []struct {
		lo, hi                 float64
		opened, sourceTerminal int
		fractions              [2]*big.Rat
	}{{1, 4, 1, 1, [2]*big.Rat{big.NewRat(2, 3), big.NewRat(1, 1)}}, {-1, 2, 0, 0, [2]*big.Rat{big.NewRat(0, 1), big.NewRat(1, 3)}}} {
		tool, _ := brep.Box(geom.Vec3{X: .5, Y: .5, Z: tc.lo}, geom.Vec3{X: 1.5, Y: 1.5, Z: tc.hi})
		q, e := trimmed.FuseContainedNativePrismBoss(context.Background(), stock, tool, trimmed.NativePrismBooleanOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if q.ExactVolume().Cmp(big.NewRat(13, 1)) != 0 || q.ShoulderCap() != tc.opened || q.TerminalCap() != 2 {
			t.Fatal(q.ExactVolume(), q.ShoulderCap())
		}
		solid := q.Solid()
		if e := solid.Validate(); e != nil {
			t.Fatal(e)
		}
		if len(solid.Faces()) != 11 || len(solid.Faces()[tc.opened].Holes) != 1 {
			t.Fatal(solid.Faces())
		}
		mass, e := solid.MassProperties()
		if e != nil || math.Abs(mass.Volume-13) > 1e-11 {
			t.Fatal(mass, e)
		}
		wantZ := 43. / 26
		if tc.opened == 0 {
			wantZ = 35. / 26
		}
		if math.Abs(mass.Centroid.X-1) > 1e-12 || math.Abs(mass.Centroid.Y-1) > 1e-12 || math.Abs(mass.Centroid.Z-wantZ) > 1e-12 {
			t.Fatal("independent stock plus boss centroid", mass.Centroid)
		}
		volume := q.ExactVolume()
		volume.SetInt64(0)
		if q.ExactVolume().Cmp(big.NewRat(13, 1)) != 0 {
			t.Fatal("mutable volume")
		}
		uses := q.WallUses()
		uses[0].HeightParameter[0].SetInt64(99)
		if q.WallUses()[0].HeightParameter[0].Sign() != 0 {
			t.Fatal("mutable wall provenance")
		}
		history := q.Faces()
		if len(history) != len(solid.Faces()) || history[2].Operand != 1 || history[2].OriginalFace != tc.sourceTerminal || history[2].Reversed {
			t.Fatal(history)
		}
		for _, use := range q.WallUses() {
			if use.Use.Operand == 1 {
				for i := 0; i < 2; i++ {
					if use.HeightParameter[i].Cmp(tc.fractions[i]) != 0 {
						t.Fatal(use)
					}
				}
				if history[use.Use.Face].Reversed {
					t.Fatal("tool wall reversed")
				}
			} else if history[use.Use.Face].Reversed {
				t.Fatal("stock wall reversed")
			}
		}
		for _, cap := range []int{q.NativeOperations(), q.NativeOperations() - 1} {
			out, e := trimmed.FuseContainedNativePrismBoss(context.Background(), stock, tool, trimmed.NativePrismBooleanOptions{Native: trimmed.AffineArrangementPrismOptions{MaxOperations: cap}})
			if cap == q.NativeOperations() {
				if e != nil || out == nil {
					t.Fatal(e)
				}
			} else if e == nil || out != nil {
				t.Fatal("partial boss shell")
			}
		}
		rebuilt, e := brep.NewSolid(solid.Vertices(), solid.Edges(), solid.Faces())
		if e != nil {
			t.Fatal(e)
		}
		m, _ := rebuilt.MassProperties()
		if m.Volume != mass.Volume {
			t.Fatal(m)
		}
	}
}
func TestContainedNativePrismBossFuseSlantedAndReject(t *testing.T) {
	stock, _ := brep.Box(geom.Vec3{}, geom.Vec3{X: 2, Y: 2, Z: 3})
	tool, _ := brep.Box(geom.Vec3{X: .5, Y: .5, Z: 1}, geom.Vec3{X: 1.5, Y: 1.5, Z: 4})
	pose := geom.Affine{{0, 0, -1, 5}, {1, .5, 0, -2}, {0, 1, 0, 1}}
	a, e := stock.Transformed(pose)
	if e != nil {
		t.Fatal(e)
	}
	b, e := tool.Transformed(pose)
	if e != nil {
		t.Fatal(e)
	}
	q, e := trimmed.FuseContainedNativePrismBoss(context.Background(), a, b, trimmed.NativePrismBooleanOptions{})
	if e != nil {
		t.Fatal(e)
	}
	mass, e := q.Solid().MassProperties()
	if e != nil || math.Abs(mass.Volume-13) > 1e-11 {
		t.Fatal(mass, e)
	}
	for _, box := range [][2]geom.Vec3{{{X: .5, Y: .5, Z: 1}, {X: 1.5, Y: 1.5, Z: 3}}, {{X: 0, Y: .5, Z: 1}, {X: 1, Y: 1.5, Z: 4}}, {{X: -.5, Y: .5, Z: 1}, {X: 1, Y: 1.5, Z: 4}}, {{X: .5, Y: .5, Z: -1}, {X: 1.5, Y: 1.5, Z: 4}}, {{X: .5, Y: .5, Z: 1}, {X: 1.5, Y: 1.5, Z: 2}}, {{X: .5, Y: .5, Z: 3}, {X: 1.5, Y: 1.5, Z: 4}}} {
		bad, e := brep.Box(box[0], box[1])
		if e != nil {
			t.Fatal(e)
		}
		if out, e := trimmed.FuseContainedNativePrismBoss(context.Background(), stock, bad, trimmed.NativePrismBooleanOptions{}); e == nil || out != nil {
			t.Fatal("unsupported boss regime accepted", box)
		}
	}
	if out, e := trimmed.FuseContainedNativePrismBoss(context.Background(), stock, tool, trimmed.NativePrismBooleanOptions{Native: trimmed.AffineArrangementPrismOptions{MaxVertices: 6}}); e == nil || out != nil {
		t.Fatal("vertex limit")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if out, e := trimmed.FuseContainedNativePrismBoss(ctx, stock, tool, trimmed.NativePrismBooleanOptions{}); e != context.Canceled || out != nil {
		t.Fatal(out, e)
	}
}

func TestContainedNativeBossNonconvexStockTriangleTool(t *testing.T) {
	stock, e := brep.Prism([]geom.Vec3{{}, {X: 3}, {X: 3, Y: 1}, {X: 1, Y: 1}, {X: 1, Y: 3}, {Y: 3}}, geom.Vec3{Z: 3})
	if e != nil {
		t.Fatal(e)
	}
	tool, e := brep.Prism([]geom.Vec3{{X: .125, Y: .125, Z: 1}, {X: .875, Y: .125, Z: 1}, {X: .125, Y: .875, Z: 1}}, geom.Vec3{Z: 3})
	if e != nil {
		t.Fatal(e)
	}
	q, e := trimmed.FuseContainedNativePrismBoss(context.Background(), stock, tool, trimmed.NativePrismBooleanOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if q.ExactVolume().Cmp(big.NewRat(489, 32)) != 0 || len(q.Solid().Faces()) != 12 {
		t.Fatal(q.ExactVolume(), len(q.Solid().Faces()))
	}
	mass, e := q.Solid().MassProperties()
	if e != nil || math.Abs(mass.Volume-489./32) > 1e-10 {
		t.Fatal(mass, e)
	}
	if len(q.Solid().Faces()[q.TerminalCap()].Boundary) != 3 {
		t.Fatal("triangle terminal not preserved")
	}
}
