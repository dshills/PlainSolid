package trimmed_test

import (
	"context"
	"math"
	"plainsolid/brep"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"plainsolid/intersect"
	"testing"
)

func TestNativeTiedCrossingPrismFuse(t *testing.T) {
	for _, high := range []bool{false, true} {
		for _, swap := range []bool{false, true} {
			a := transitionPrism(t, geom.Vec3{}, geom.Vec3{X: 2, Y: 2, Z: 4})
			lo, hi := 0., 2.
			if high {
				lo, hi = 2, 4
			}
			b := transitionPrism(t, geom.Vec3{X: 1, Y: .5, Z: lo}, geom.Vec3{X: 3, Y: 1.5, Z: hi})
			owners := [2]brep.Solid{a, b}
			if swap {
				owners = [2]brep.Solid{b, a}
			}
			q, e := trimmed.FuseTiedCrossingNativePrisms(context.Background(), owners[0], owners[1], trimmed.NativePrismBooleanOptions{})
			if e != nil {
				t.Fatal(high, swap, e)
			}
			m, e := q.Solid().MassProperties()
			z := 17. / 9
			if high {
				z = 19. / 9
			}
			if e != nil || math.Abs(m.Volume-18) > 1e-10 || math.Abs(m.Centroid.Z-z) > 1e-10 || math.Abs(m.Centroid.X-17./12) > 1e-10 {
				t.Fatal(m, e)
			}
			if len(q.Solid().Vertices())-len(q.Solid().Edges())+len(q.Solid().Faces()) != 2 {
				t.Fatal("Euler")
			}
			area := [3]float64{}
			vertices := q.Solid().Vertices()
			faces := q.Solid().Faces()
			edges := q.Solid().Edges()
			for _, c := range q.TiedCapContributions() {
				f := faces[c.Face]
				pts := []geom.Vec3{}
				for _, u := range f.Boundary {
					ed := edges[u.Edge]
					i := ed.Start
					if u.Reversed {
						i = ed.End
					}
					pts = append(pts, vertices[i])
				}
				ar := math.Abs(pts[1].Sub(pts[0]).Cross(pts[2].Sub(pts[0])).Z) / 2
				if len(c.Parents) == 2 {
					area[2] += ar
				} else {
					area[c.Parents[0].Operand] += ar
				}
				for _, p := range c.Parents {
					original := owners[p.Operand].Faces()[p.OriginalFace]
					ov, oe := owners[p.Operand].Vertices(), owners[p.Operand].Edges()
					op := []geom.Vec3{}
					for _, u := range original.Boundary {
						ed := oe[u.Edge]
						i := ed.Start
						if u.Reversed {
							i = ed.End
						}
						op = append(op, ov[i])
					}
					if op[1].Sub(op[0]).Cross(op[2].Sub(op[0])).Dot(pts[1].Sub(pts[0]).Cross(pts[2].Sub(pts[0]))) <= 0 {
						t.Fatal("cap normal")
					}
				}
			}
			want := [3]float64{3, 1, 1}
			if swap {
				want = [3]float64{1, 3, 1}
			}
			for i := range want {
				if math.Abs(area[i]-want[i]) > 1e-10 {
					t.Fatal("parent partitions", area)
				}
			}
			r, e := trimmed.CombineNativePrisms(context.Background(), owners[0], owners[1], intersect.AffineRingUnion, trimmed.NativePrismDispatchOptions{})
			if e != nil || r.Family() != trimmed.NativePrismFamilyTiedFuse {
				t.Fatal("route", e)
			}
		}
	}
}

func TestNativeTiedCrossingPrismFuseHoleAndLimits(t *testing.T) {
	ctx := context.Background()
	a := transitionPrism(t, geom.Vec3{}, geom.Vec3{X: 4, Y: 3, Z: 4})
	b, e := brep.PrismWithHoles([]geom.Vec3{{X: 2, Y: .5}, {X: 5, Y: .5}, {X: 5, Y: 2.5}, {X: 2, Y: 2.5}}, [][]geom.Vec3{{{X: 2.5, Y: 1}, {X: 3, Y: 1}, {X: 3, Y: 2}, {X: 2.5, Y: 2}}}, geom.Vec3{Z: 2})
	if e != nil {
		t.Fatal(e)
	}
	b, e = b.Transformed(geom.Affine{{1, .25, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}})
	if e != nil {
		t.Fatal(e)
	}
	q, e := trimmed.FuseTiedCrossingNativePrisms(ctx, a, b, trimmed.NativePrismBooleanOptions{})
	if e != nil {
		t.Fatal(e)
	}
	m, e := q.Solid().MassProperties()
	if e != nil || math.Abs(m.Volume-52) > 1e-10 {
		t.Fatal(m, e)
	}
	// The tool hole is filled by stock throughout the common layer; its original
	// wall cannot survive as an exposed wall on the final material.
	holeEdges := map[int]bool{}
	for _, f := range b.Faces() {
		for _, h := range f.Holes {
			for _, u := range h {
				holeEdges[u.Edge] = true
			}
		}
	}
	for _, u := range q.WallUses() {
		if u.Use.Operand == 1 && holeEdges[u.Use.LowerEdge] {
			t.Fatal("filled hole wall exposed")
		}
	}
	w, aw, nw := q.Operations()
	for _, lim := range [][2]int{{w, aw}, {w - 1, aw}, {w, aw - 1}} {
		o := trimmed.NativePrismDispatchOptions{}
		o.Construct.Native.MaxOperations = lim[0]
		o.Construct.Arrangement.MaxOperations = lim[1]
		o.Publish.MaxOperations = nw
		out, e := trimmed.CombineNativePrisms(ctx, a, b, intersect.AffineRingUnion, o)
		if lim[0] == w && lim[1] == aw {
			if e != nil {
				t.Fatal(e)
			}
		} else if e == nil || out != nil {
			t.Fatal("partial proof")
		}
	}
	o := trimmed.NativePrismDispatchOptions{}
	o.Publish.MaxOperations = nw - 1
	if out, e := trimmed.CombineNativePrisms(ctx, a, b, intersect.AffineRingUnion, o); e == nil || out != nil {
		t.Fatal("partial native publication")
	}
	// Nested and equal-height cases remain separate APIs.
	for _, z := range [][2]float64{{1, 3}, {0, 4}, {4, 5}} {
		bad := transitionPrism(t, geom.Vec3{X: 2, Y: .5, Z: z[0]}, geom.Vec3{X: 5, Y: 2.5, Z: z[1]})
		if out, e := trimmed.FuseTiedCrossingNativePrisms(ctx, a, bad, trimmed.NativePrismBooleanOptions{}); e == nil || out != nil {
			t.Fatal("unrelated axial family")
		}
	}
}

func TestNativeTiedCrossingPrismFuseUnsheared(t *testing.T) {
	a, _ := brep.Box(geom.Vec3{}, geom.Vec3{X: 2, Y: 2, Z: 4})
	b, _ := brep.Box(geom.Vec3{X: 1, Y: .5}, geom.Vec3{X: 3, Y: 1.5, Z: 2})
	q, e := trimmed.CombineNativePrisms(context.Background(), a, b, intersect.AffineRingUnion, trimmed.NativePrismDispatchOptions{})
	if e != nil || q.Family() != trimmed.NativePrismFamilyTiedFuse {
		t.Fatal(q, e)
	}
}

func TestNativeTiedCrossingPrismFuseAtomic(t *testing.T) {
	a := transitionPrism(t, geom.Vec3{}, geom.Vec3{X: 2, Y: 2, Z: 4})
	b, e := brep.Prism([]geom.Vec3{{X: 1, Y: .5}, {X: 4, Y: 1}, {X: 1, Y: 1.5}}, geom.Vec3{Z: 2})
	if e != nil {
		t.Fatal(e)
	}
	b, e = b.Transformed(geom.Affine{{1, .25, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}})
	if e != nil {
		t.Fatal(e)
	}
	if out, e := trimmed.FuseTiedCrossingNativePrisms(context.Background(), a, b, trimmed.NativePrismBooleanOptions{}); e == nil || out != nil {
		t.Fatal("nonbinary native endpoint rounded")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if out, e := trimmed.FuseTiedCrossingNativePrisms(ctx, a, b, trimmed.NativePrismBooleanOptions{}); e != context.Canceled || out != nil {
		t.Fatal(out, e)
	}
}
