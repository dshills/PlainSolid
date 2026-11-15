package trimmed

import (
	"bytes"
	"context"
	"math/big"
	"testing"
)

func TestConvexAffinePolygonCapFuse(t *testing.T) {
	ctx := context.Background()
	a, b := affinePolygonCapFixture(t, 0, false)
	for order, pair := range [][2]Solid{{a, b}, {b, a}} {
		faces := [2]int{1, 0}
		if order == 1 {
			faces = [2]int{0, 1}
		}
		contact, e := CertifyConvexAffinePolygonCapContact(ctx, pair[0], pair[1], faces, ConvexAffinePolygonCapOptions{})
		if e != nil {
			t.Fatal(e)
		}
		r, e := contact.Fuse(ctx, ConvexAffinePolygonCapFuseOptions{})
		if e != nil {
			t.Fatal(e)
		}
		s := r.Solid()
		if r.ExactVolume().Cmp(big.NewRat(60639, 4096)) != 0 || len(s.vertices)-len(s.edges)+len(s.faces) != 2 {
			t.Fatal("mass/topology", r.ExactVolume(), len(s.vertices), len(s.edges), len(s.faces))
		}
		for _, area := range r.RemovedCapUVArea() {
			if area.Cmp(big.NewRat(143, 1024)) != 0 {
				t.Fatal("removed original cap area", area)
			}
		}
		if len(r.FaceSources()) != len(s.faces) {
			t.Fatal("source count")
		}
		for fi, source := range r.FaceSources() {
			if s.faces[fi].Surface != pair[source.Owner].faces[source.Face].Surface {
				t.Fatal("replaced support")
			}
			fragment := r.SourceFaceFragments(source.Owner, source.Face)
			if fragment == nil {
				t.Fatal("missing original fragment")
			}
		}
		for owner, source := range pair {
			for fi := range source.faces {
				f := r.SourceFaceFragments(owner, fi)
				if f == nil {
					t.Fatal("missing face partition")
				}
				if len(f.Cycles()) == 0 {
					t.Fatal("missing cycles")
				}
				vertices := f.Vertices()
				for _, cycle := range f.Cycles() {
					for _, use := range cycle.Uses {
						if use.OriginalEdge < 0 {
							continue
						}
						original := source.faces[fi].Outer[use.OriginalLocalEdge]
						if original.Edge != use.OriginalEdge {
							t.Fatal("original edge id")
						}
						for k, id := range []int{use.Start, use.End} {
							parameter := new(big.Rat).Set(use.Parameter[k])
							if original.Reversed {
								parameter.Sub(big.NewRat(1, 1), parameter)
							}
							a, c := [2]*big.Rat{ratInset(original.UVStart.X), ratInset(original.UVStart.Y)}, [2]*big.Rat{ratInset(original.UVEnd.X), ratInset(original.UVEnd.Y)}
							for axis := 0; axis < 2; axis++ {
								expected := new(big.Rat).Add(a[axis], new(big.Rat).Mul(parameter, new(big.Rat).Sub(c[axis], a[axis])))
								if expected.Cmp(vertices[id][axis]) != 0 {
									t.Fatal("original normalized edge range")
								}
							}
						}
					}
				}

			}
		}
		// Persistence revalidates the actual sewn original spline supports and uses.
		var data bytes.Buffer
		if e = WriteJSON(&data, s); e != nil {
			t.Fatal(e)
		}
		decoded, e := ReadJSONContext(ctx, &data, JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		if len(decoded.faces) != len(s.faces) {
			t.Fatal("reload")
		}
		t.Log("native faces", len(s.faces), "vertices", len(s.vertices), "edges", len(s.edges), "work", r.ExactOperations())
		if q, e := contact.Fuse(ctx, ConvexAffinePolygonCapFuseOptions{MaxFaces: len(s.faces) - 1}); e == nil || q != nil {
			t.Fatal("face cap")
		}
		if q, e := contact.Fuse(ctx, ConvexAffinePolygonCapFuseOptions{MaxOperations: r.ExactOperations()}); e != nil || q == nil {
			t.Fatal("exact work", e)
		}
		if q, e := contact.Fuse(ctx, ConvexAffinePolygonCapFuseOptions{MaxOperations: r.ExactOperations() - 1}); e == nil || q != nil {
			t.Fatal("short work")
		}
	}
}

func TestConvexAffinePolygonCapFuseRejectsOriginalNonbinaryUV(t *testing.T) {
	a, b := affinePolygonCapFixtureUnits(t, 0, false, false)
	r, e := CertifyConvexAffinePolygonCapContact(context.Background(), a, b, [2]int{1, 0}, ConvexAffinePolygonCapOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if output, e := r.Fuse(context.Background(), ConvexAffinePolygonCapFuseOptions{}); e == nil || output != nil {
		t.Fatal("original normalized side needs 8/9; no rounded UV")
	}
}
