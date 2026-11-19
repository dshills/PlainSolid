//go:build research

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

func TestNativeSideAbuttingPrismFuse(t *testing.T) {
	for _, zs := range [][2]float64{{1, 5}, {1, 3}, {0, 4}} {
		for _, swap := range []bool{false, true} {
			a := transitionPrism(t, geom.Vec3{}, geom.Vec3{X: 2, Y: 2, Z: 4})
			b := transitionPrism(t, geom.Vec3{X: 2, Y: .5, Z: zs[0]}, geom.Vec3{X: 3, Y: 1.5, Z: zs[1]})
			owners := [2]brep.Solid{a, b}
			if swap {
				owners = [2]brep.Solid{b, a}
			}
			q, e := trimmed.FuseSideAbuttingNativePrisms(context.Background(), owners[0], owners[1], trimmed.NativePrismBooleanOptions{})
			if e != nil {
				t.Fatal(zs, swap, e)
			}
			h := zs[1] - zs[0]
			want := 16 + h
			m, e := q.Solid().MassProperties()
			if e != nil || math.Abs(m.Volume-want) > 1e-10 || math.Abs(m.Centroid.X-(20+2.75*h)/want) > 1e-10 || math.Abs(m.Centroid.Z-(32+h*(zs[0]+zs[1])/2)/want) > 1e-10 {
				t.Fatal(m, e)
			}
			if len(q.Solid().Vertices())-len(q.Solid().Edges())+len(q.Solid().Faces()) != 2 {
				t.Fatal("Euler")
			}
			// Independent projected side area: both original wall areas minus
			// the twice-counted internal overlap, with unnormalized plane x-y/4=2.
			sharedArea := 0.
			verts, eds := q.Solid().Vertices(), q.Solid().Edges()
			for _, f := range q.Solid().Faces() {
				pts := []geom.Vec3{}
				on := true
				for _, u := range f.Boundary {
					ed := eds[u.Edge]
					i := ed.Start
					if u.Reversed {
						i = ed.End
					}
					v := verts[i]
					on = on && v.X-.25*v.Y == 2
					pts = append(pts, v)
				}
				if on {
					for i := 1; i+1 < len(pts); i++ {
						sharedArea += math.Abs(pts[i].Sub(pts[0]).Cross(pts[i+1].Sub(pts[0])).X) / 2
					}
				}
			}
			overlap := math.Min(4, zs[1]) - math.Max(0, zs[0])
			if math.Abs(sharedArea-(8+h-2*overlap)) > 1e-10 {
				t.Fatal("wrong internal wall removal", sharedArea)
			}
			for _, f := range q.Faces() {
				if f.Operand < 0 || f.OriginalFace < 0 || f.Reversed {
					t.Fatal("ancestry")
				}
				original := owners[f.Operand]
				points := func(s brep.Solid, face brep.Face) []geom.Vec3 {
					out := []geom.Vec3{}
					v, e := s.Vertices(), s.Edges()
					for _, u := range face.Boundary {
						id := e[u.Edge].Start
						if u.Reversed {
							id = e[u.Edge].End
						}
						out = append(out, v[id])
					}
					return out
				}
				a, b := points(original, original.Faces()[f.OriginalFace]), points(q.Solid(), q.Solid().Faces()[f.Face])
				n := a[1].Sub(a[0]).Cross(a[2].Sub(a[0]))
				nn := b[1].Sub(b[0]).Cross(b[2].Sub(b[0]))
				if n.Dot(nn) <= 0 {
					t.Fatal("source normal")
				}
				for _, p := range b {
					if n.Dot(p.Sub(a[0])) != 0 {
						t.Fatal("source plane")
					}
				}

			}
			r, e := trimmed.CombineNativePrisms(context.Background(), owners[0], owners[1], intersect.AffineRingUnion, trimmed.NativePrismDispatchOptions{})
			if e != nil {
				t.Fatal("dispatcher", zs, e)
			}
			if len(r.Components()) != 1 {
				t.Fatal("components")
			}
		}
	}
}

func TestNativeSideAbuttingPrismFuseLimitsAndContact(t *testing.T) {
	ctx := context.Background()
	a := transitionPrism(t, geom.Vec3{}, geom.Vec3{X: 2, Y: 2, Z: 4})
	b := transitionPrism(t, geom.Vec3{X: 2, Y: .5, Z: 1}, geom.Vec3{X: 3, Y: 1.5, Z: 5})
	q, e := trimmed.FuseSideAbuttingNativePrisms(ctx, a, b, trimmed.NativePrismBooleanOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for _, n := range []int{q.NativeOperations(), q.NativeOperations() - 1} {
		o := trimmed.NativePrismBooleanOptions{}
		o.Native.MaxOperations = n
		out, e := trimmed.FuseSideAbuttingNativePrisms(ctx, a, b, o)
		if n == q.NativeOperations() {
			if e != nil {
				t.Fatal(e)
			}
		} else if e == nil || out != nil {
			t.Fatal("partial publication")
		}
	}
	for _, ends := range [][2]geom.Vec3{
		{{X: 2, Y: 2, Z: 1}, {X: 3, Y: 3, Z: 5}},
		{{X: 2, Y: .5, Z: 4}, {X: 3, Y: 1.5, Z: 5}},
		{{X: 1, Y: .5, Z: 1}, {X: 3, Y: 1.5, Z: 5}},
		{{X: math.Nextafter(2, 3), Y: .5, Z: 1}, {X: 3, Y: 1.5, Z: 5}},
	} {
		bad := transitionPrism(t, ends[0], ends[1])
		if out, e := trimmed.FuseSideAbuttingNativePrisms(ctx, a, bad, trimmed.NativePrismBooleanOptions{}); e == nil || out != nil {
			t.Fatal("unqualified contact", ends)
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if out, e := trimmed.FuseSideAbuttingNativePrisms(canceled, a, b, trimmed.NativePrismBooleanOptions{}); e != context.Canceled || out != nil {
		t.Fatal(out, e)
	}
}
