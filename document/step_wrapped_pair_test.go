package document

import (
	"bytes"
	"context"
	"os"
	"plainsolid/brep/trimmed"
	"plainsolid/exchange/step"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestWrappedPairRecoveryTopology(t *testing.T) {
	slowtest.Skip(t)
	for _, name := range []string{"wrapped-shoulder", "wrapped-toroidal-void"} {
		t.Run(name, func(t *testing.T) {
			raw, e := os.ReadFile("../exchange/step/testdata/" + name + ".step")
			if e != nil {
				t.Fatal(e)
			}
			m, e := step.ReadWrappedMaterials(bytes.NewReader(raw), step.Limits{}, step.WrappedSurfaceOptions{})
			if e != nil {
				t.Fatal(e)
			}
			var s trimmed.Solid
			if name == "wrapped-shoulder" {
				s = m.Bodies[0].Shoulder.Solid
			} else {
				s = m.Bodies[0].ToroidalVoid.Solid
			}
			check := func(q trimmed.Solid) (trimmed.WrappedPairProof, error) {
				if name == "wrapped-shoulder" {
					return q.ValidateCanonicalWrappedShoulder(context.Background(), geom.SurfacePatchOptions{})
				}
				return q.ValidateCanonicalWrappedToroidalVoid(context.Background(), geom.SurfacePatchOptions{})
			}
			faces := s.Faces()
			faces[0], faces[len(faces)-1] = faces[len(faces)-1], faces[0]
			q, e := trimmed.NewSolid(s.Vertices(), s.Edges(), faces, s.Tolerance())
			if e != nil {
				t.Fatal(e)
			}
			p, e := check(q)
			if e != nil || !p.Complete {
				t.Fatal("reindexing", e)
			}
			for i := range faces {
				faces[i].Reversed = !faces[i].Reversed
			}
			q, e = trimmed.NewSolid(s.Vertices(), s.Edges(), faces, s.Tolerance())
			if e != nil {
				t.Fatal(e)
			}
			if p, e = check(q); e == nil || p.Complete {
				t.Fatal("inward accepted")
			}
		})
	}
}
