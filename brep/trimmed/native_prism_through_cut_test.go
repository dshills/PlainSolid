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

func TestContainedNativePrismThroughExactClips(t *testing.T) {
	stock, _ := brep.Box(geom.Vec3{}, geom.Vec3{X: 2, Y: 2, Z: 3})
	for _, interval := range [][2]float64{{-1, 4}, {0, 3}, {0, 4}, {-1, 3}} {
		tool, _ := brep.Box(geom.Vec3{X: .25, Y: .5, Z: interval[0]}, geom.Vec3{X: 1.25, Y: 1.5, Z: interval[1]})
		q, e := trimmed.CutContainedNativePrismThrough(context.Background(), stock, tool, trimmed.NativePrismBooleanOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if q.ExactVolume().Cmp(big.NewRat(9, 1)) != 0 {
			t.Fatal(q.ExactVolume())
		}
		s := q.Solid()
		if e = s.Validate(); e != nil {
			t.Fatal(e)
		}
		if len(s.Faces()) != 10 || len(s.Faces()[0].Holes) != 1 || len(s.Faces()[1].Holes) != 1 {
			t.Fatal("through topology")
		}
		m, e := s.MassProperties()
		if e != nil || math.Abs(m.Volume-9) > 1e-12 || math.Abs(m.Centroid.X-13./12) > 1e-12 || math.Abs(m.Centroid.Y-1) > 1e-12 || math.Abs(m.Centroid.Z-1.5) > 1e-12 {
			t.Fatal(m, e)
		}
		history := q.Faces()
		if history[0].Operand != 0 || history[1].Operand != 0 || history[0].OriginalFace != 0 || history[1].OriginalFace != 1 {
			t.Fatal(history)
		}
		for _, use := range q.WallUses() {
			owner, _ := q.OriginalOwner(use.Use.Operand)
			poles, edges := owner.Vertices(), owner.Edges()
			lo, hi := 0., 3.
			if use.Use.Operand == 1 {
				lo, hi = interval[0], interval[1]
			}
			for i, z := range []int64{0, 3} {
				want := new(big.Rat).Quo(new(big.Rat).Sub(big.NewRat(z, 1), new(big.Rat).SetFloat64(lo)), new(big.Rat).SetFloat64(hi-lo))
				if use.HeightParameter[i].Cmp(want) != 0 {
					t.Fatal("height ownership", use)
				}
			}
			if history[use.Use.Face].OriginalFace != use.Use.Wall || history[use.Use.Face].Reversed != (use.Use.Operand == 1) {
				t.Fatal(history, use)
			}
			// Independently reconstruct each corner from original cap edges, authored
			// edge parameters and clipped height fractions, all in rational arithmetic.
			reconstructed := map[[3]float64]bool{}
			for end := 0; end < 2; end++ {
				edgePoint := func(edge int, t0 *big.Rat) [3]*big.Rat {
					e := edges[edge]
					a, b := poles[e.Start], poles[e.End]
					av, bv := [3]float64{a.X, a.Y, a.Z}, [3]float64{b.X, b.Y, b.Z}
					var p [3]*big.Rat
					for k := range p {
						p[k] = new(big.Rat).Add(new(big.Rat).SetFloat64(av[k]), new(big.Rat).Mul(t0, new(big.Rat).Sub(new(big.Rat).SetFloat64(bv[k]), new(big.Rat).SetFloat64(av[k]))))
					}
					return p
				}
				low, high := edgePoint(use.Use.LowerEdge, use.Use.LowerParameter[end]), edgePoint(use.Use.UpperEdge, use.Use.UpperParameter[end])
				for _, h := range use.HeightParameter {
					var p [3]float64
					for k := range p {
						v := new(big.Rat).Add(low[k], new(big.Rat).Mul(h, new(big.Rat).Sub(high[k], low[k])))
						var exact bool
						p[k], exact = v.Float64()
						if !exact {
							t.Fatal("rounded original corner")
						}
					}
					reconstructed[p] = true
				}
			}
			for _, u := range s.Faces()[use.Use.Face].Boundary {
				v := s.Vertices()[s.Edges()[u.Edge].Start]
				if !reconstructed[[3]float64{v.X, v.Y, v.Z}] {
					t.Fatal("foreign wall vertex", v)
				}
			}
		}
		for _, cap := range []int{q.NativeOperations(), q.NativeOperations() - 1} {
			out, e := trimmed.CutContainedNativePrismThrough(context.Background(), stock, tool, trimmed.NativePrismBooleanOptions{Native: trimmed.AffineArrangementPrismOptions{MaxOperations: cap}})
			if cap == q.NativeOperations() {
				if e != nil || out == nil {
					t.Fatal(e)
				}
			} else if e == nil || out != nil {
				t.Fatal("partial through")
			}
		}
		rebuilt, e := brep.NewSolid(s.Vertices(), s.Edges(), s.Faces())
		if e != nil {
			t.Fatal(e)
		}
		rm, _ := rebuilt.MassProperties()
		if rm.Volume != m.Volume {
			t.Fatal(rm)
		}
	}
}

func TestContainedNativePrismThroughNonconvexAndReject(t *testing.T) {
	stock, e := brep.Prism([]geom.Vec3{{}, {X: 3}, {X: 3, Y: 1}, {X: 1, Y: 1}, {X: 1, Y: 3}, {Y: 3}}, geom.Vec3{Z: 3})
	if e != nil {
		t.Fatal(e)
	}
	tool, e := brep.Prism([]geom.Vec3{{X: .125, Y: .125, Z: -1}, {X: .875, Y: .125, Z: -1}, {X: .125, Y: .875, Z: -1}}, geom.Vec3{Z: 5})
	if e != nil {
		t.Fatal(e)
	}
	pose := geom.Affine{{0, 0, -1, 5}, {1, .5, 0, -2}, {0, 1, 0, 1}}
	stock, e = stock.Transformed(pose)
	if e != nil {
		t.Fatal(e)
	}
	tool, e = tool.Transformed(pose)
	if e != nil {
		t.Fatal(e)
	}
	q, e := trimmed.CutContainedNativePrismThrough(context.Background(), stock, tool, trimmed.NativePrismBooleanOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if q.ExactVolume().Cmp(big.NewRat(453, 32)) != 0 {
		t.Fatal(q.ExactVolume())
	}
	m, e := q.Solid().MassProperties()
	if e != nil || math.Abs(m.Volume-453./32) > 1e-11 {
		t.Fatal(m, e)
	}
	box, _ := brep.Box(geom.Vec3{}, geom.Vec3{X: 2, Y: 2, Z: 3})
	for _, ends := range [][2]geom.Vec3{
		{{X: .5, Y: .5, Z: 1}, {X: 1.5, Y: 1.5, Z: 4}},
		{{X: .5, Y: .5, Z: -1}, {X: 1.5, Y: 1.5, Z: 2}},
		{{X: .5, Y: .5, Z: 1}, {X: 1.5, Y: 1.5, Z: 2}},
		{{X: 0, Y: .5, Z: -1}, {X: 1, Y: 1.5, Z: 4}},
		{{X: -.5, Y: .5, Z: -1}, {X: 1, Y: 1.5, Z: 4}},
	} {
		bad, _ := brep.Box(ends[0], ends[1])
		if out, e := trimmed.CutContainedNativePrismThrough(context.Background(), box, bad, trimmed.NativePrismBooleanOptions{}); e == nil || out != nil {
			t.Fatal("unsupported through", ends)
		}
	}
	if out, e := trimmed.CutContainedNativePrismThrough(context.Background(), stock, tool, trimmed.NativePrismBooleanOptions{Native: trimmed.AffineArrangementPrismOptions{MaxVertices: 6}}); e == nil || out != nil {
		t.Fatal("limit")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if out, e := trimmed.CutContainedNativePrismThrough(ctx, stock, tool, trimmed.NativePrismBooleanOptions{}); e != context.Canceled || out != nil {
		t.Fatal(out, e)
	}
}
