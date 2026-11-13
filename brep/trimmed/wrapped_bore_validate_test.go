package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestCanonicalWrappedBoreRecovery(t *testing.T) {
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	outer := makeCanonicalWrappedCylinder(t, true, 0)
	inner, _, e := outer.Transformed(ctx, geom.Affine{{.5, 0, 0, 0}, {0, .5, 0, 0}, {0, 0, 1, 0}}, geom.WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"tube", "blind-bore", "cavity"} {
		t.Run(kind, func(t *testing.T) {
			w := inner
			if kind != "tube" {
				hi := int64(3)
				if kind == "cavity" {
					hi = 1
				}
				var err error
				w, err = geom.ReheightWrappedCylinder(ctx, w, big.NewRat(-1, 1), big.NewRat(hi, 1), o)
				if err != nil {
					t.Fatal(err)
				}
			}
			s, _, e := NewWrappedCylinderBore(ctx, outer, w, 1e-9, o)
			if e != nil {
				t.Fatal(e)
			}
			p, e := s.ValidateCanonicalWrappedBore(ctx, o)
			if e != nil || !p.Complete || p.Kind != kind {
				t.Fatal(p, e)
			}
			faces := s.Faces()
			faces[0], faces[len(faces)-1] = faces[len(faces)-1], faces[0]
			q, e := NewSolid(s.Vertices(), s.Edges(), faces, s.Tolerance())
			if e != nil {
				t.Fatal(e)
			}
			if p, e = q.ValidateCanonicalWrappedBore(ctx, o); e != nil || !p.Complete {
				t.Fatal(p, e)
			}
			for i := range faces {
				faces[i].Reversed = !faces[i].Reversed
			}
			q, e = NewSolid(s.Vertices(), s.Edges(), faces, s.Tolerance())
			if e != nil {
				t.Fatal(e)
			}
			if p, e = q.ValidateCanonicalWrappedBore(ctx, o); e == nil || p.Complete {
				t.Fatal("inward accepted")
			}
		})
	}
}
