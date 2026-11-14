package trimmed

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
	"testing"
)

func TestWrappedCylinderUnionTopology(t *testing.T) {
	ctx := context.Background()
	a := wrappedOverlapSupport(t)
	b, _, e := a.InsertKnot(ctx, 0, .125, 1, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	ar := [2][2]*big.Rat{{big.NewRat(0, 1), big.NewRat(3, 1)}, {big.NewRat(-1, 1), big.NewRat(2, 1)}}
	for _, reverse := range []bool{false, true} {
		target := b
		br := [2][2]*big.Rat{{big.NewRat(2, 1), big.NewRat(5, 1)}, {big.NewRat(0, 1), big.NewRat(3, 1)}}
		o := WrappedCylinderIntersectionOptions{}
		if reverse {
			target, _, e = b.Reparameterize(ctx, geom.ParameterRectangle{UMin: 2, UMax: 10, VMin: -3, VMax: 7}, true, false, geom.SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
			o.NormalizeDomains = true
			o.ReverseBU = true
			br = [2][2]*big.Rat{{big.NewRat(0, 1), big.NewRat(6, 1)}, {big.NewRat(1, 1), big.NewRat(7, 1)}}
		}
		r, e := NewWrappedCylinderUnionTopology(ctx, a, target, ar, br, o)
		if e != nil {
			t.Fatal(e)
		}
		vertices, edges, faces := r.Vertices(), r.Edges(), r.Faces()
		if len(faces) != 10 || len(vertices)-len(edges)+len(faces) != 0 || len(r.BoundaryCycles()) != 2 {
			t.Fatal("not annulus")
		}
		open := Solid{vertices: vertices, edges: edges, faces: faces, tolerance: 1e-9}
		proof, e := open.ValidateEdgeAgreement(ctx, edgecheck.Options{})
		if e != nil || !proof.Complete {
			t.Fatal("continuous native proof", e)
		}
		seamUses := 0
		for _, face := range faces {
			for _, u := range face.Outer {
				edge := edges[u.Edge]
				s, end := edge.Start, edge.End
				if u.Reversed {
					s, end = end, s
				}
				for i, uv := range []geom.Vec2{u.UVStart, u.UVEnd} {
					id := s
					if i == 1 {
						id = end
					}
					j, e := a.EvaluateRational(ctx, new(big.Rat).SetFloat64(uv.X), new(big.Rat).SetFloat64(uv.Y), geom.SurfacePatchEvaluationOptions{})
					if e != nil {
						t.Fatal(e)
					}
					p, e := geom.RepresentedExactPoint(j.Point)
					if e != nil || p != vertices[id] {
						t.Fatal("lost exact seam vertex", e)
					}
				}
				if u.UVStart.X == 4 && u.UVEnd.X == 4 {
					seamUses++
					bad := u
					bad.UVStart.X += .125
					bad.UVEnd.X += .125
					same, err := compareWrappedEdgeUse(ctx, edge, a, bad, edgecheck.Options{})
					if err != nil || same {
						t.Fatal("accepted noninteger seam displacement", err)
					}

				}
			}
		}
		if seamUses == 0 {
			t.Fatal("missing original seam use")
		}
		for ci, cycle := range r.BoundaryCycles() {
			sum := 0.0
			first, last := -1, -1
			for _, u := range cycle {
				s, end := edges[u.Edge].Start, edges[u.Edge].End
				if u.Reversed {
					s, end = end, s
				}
				if first < 0 {
					first = s
				} else if last != s {
					t.Fatal("open cycle")
				}
				last = end
				sum += u.UVEnd.X - u.UVStart.X
			}
			want := 4.0
			if ci == 1 {
				want = -4
			}
			if first != last || sum != want {
				t.Fatal("wrong annular boundary")
			}
		}
		vertices[0] = geom.Vec3{X: 999}
		faces[0].Outer[0].UVStart.X = 999
		if r.Vertices()[0].X == 999 || r.Faces()[0].Outer[0].UVStart.X == 999 {
			t.Fatal("mutable native snapshot")
		}
		low := o
		low.MaxOperations = r.ExactOperations() - 1
		if q, e := NewWrappedCylinderUnionTopology(ctx, a, target, ar, br, low); e == nil || q != nil {
			t.Fatal("budget published partial")
		}
		cancel, stop := context.WithCancel(ctx)
		stop()
		if q, e := NewWrappedCylinderUnionTopology(cancel, a, target, ar, br, o); e == nil || q != nil {
			t.Fatal("cancel published partial")
		}
		nonbinary := ar
		nonbinary[0][1] = big.NewRat(10, 3)
		relation, _, e := geom.CompareWrappedSurfaceCharts(ctx, a, target, o.ExactSurfaceComparisonOptions)
		if e != nil {
			t.Fatal(e)
		}
		exact, e := relation.CertifyCylinderUnionComplex(ctx, nonbinary, br, geom.WrappedCylinderTwoOverlapOptions{})
		if e != nil || !exact.Complete() {
			t.Fatal("rational complex should remain valid", e)
		}
		if q, e := NewWrappedCylinderUnionTopology(ctx, a, target, nonbinary, br, o); !errors.Is(e, geom.ErrUnrepresentable) || q != nil {
			t.Fatal("rounded native conversion", e)
		}
	}
}
