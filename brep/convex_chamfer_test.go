package brep_test

import (
	"context"
	"errors"
	"math"
	"os"
	"testing"

	"plainsolid/brep"
	"plainsolid/geom"
)

func convexChamferEdge(t *testing.T, s brep.Solid) int {
	t.Helper()
	verts := s.Vertices()
	for i, e := range s.Edges() {
		a, b := verts[e.Start], verts[e.End]
		if a.X == 4 && b.X == 4 && a.Y == 0 && b.Y == 0 {
			return i
		}
	}
	t.Fatal("original vertical oblique edge is absent")
	return -1
}

func TestChamferConvexObliqueEdge(t *testing.T) {
	ctx := context.Background()
	s := convexOffsetPrism(t, false)
	edge := convexChamferEdge(t, s)
	for _, c := range []struct {
		face  int
		other int
		a, b  float64
	}{{2, 3, .5, .625}, {3, 2, .625, .5}} {
		out, history, e := s.ChamferConvexEdge(ctx, edge, c.face, c.a, c.b, brep.ConvexOffsetOptions{})
		if e != nil {
			t.Fatal(c, history, e)
		}
		faces, verts, edges := out.Faces(), out.Vertices(), out.Edges()
		if history.SourceFaces != ([2]int{c.face, c.other}) || history.ChamferFace != len(s.Faces()) ||
			len(faces) != 6 || len(verts) != 8 || len(faces[history.ChamferFace].Boundary) != 4 {
			t.Fatal("original face identity and one new four-edge chamfer", history)
		}
		m, e := out.MassProperties()
		if e != nil || math.Abs(m.Volume-945.0/32) > 1e-12 || math.Abs(m.SurfaceArea-1089.0/16) > 1e-12 {
			t.Fatal("oblique projected setbacks", m, e)
		}
		for _, use := range faces[history.ChamferFace].Boundary {
			v := verts[edges[use.Edge].Start]
			if use.Reversed {
				v = verts[edges[use.Edge].End]
			}
			if v.X != 3.5 || v.Y != 0 && v.Y != .375 || v.Z != 0 && v.Z != 5 {
				t.Fatal("exact and unequal setbacks on original supports", v)
			}
		}
		if _, h, e := s.ChamferConvexEdge(ctx, edge, c.face, c.a, c.b, brep.ConvexOffsetOptions{MaxPlaneTests: history.PlaneTests - 1}); !errors.Is(e, geom.ErrConvergence) || h != (brep.ConvexChamferHistory{}) {
			t.Fatal("one short of complete work must fail atomically", h, e)
		}
	}
	for _, tc := range []struct {
		face int
		a, b float64
		want error
	}{
		{1, .5, .625, geom.ErrInvalid},
		{2, 4, .625, geom.ErrDomain},
		{2, .5, 5, geom.ErrDomain},
		{2, .5, .5, geom.ErrUnrepresentable},
	} {
		if out, h, e := s.ChamferConvexEdge(ctx, edge, tc.face, tc.a, tc.b, brep.ConvexOffsetOptions{}); !errors.Is(e, tc.want) || len(out.Faces()) != 0 || h != (brep.ConvexChamferHistory{}) {
			t.Fatal(tc, h, e)
		}
	}
}

func TestChamferConvexObliqueEdgeOCCT(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_CONVEX_OFFSET")
	if bin == "" {
		t.Skip("set PLAINSOLID_OCCT_CONVEX_OFFSET to the pinned OCCT comparator")
	}
	s := convexOffsetPrism(t, false)
	out, _, e := s.ChamferConvexEdge(context.Background(), convexChamferEdge(t, s), 2, .5, .625, brep.ConvexOffsetOptions{})
	if e != nil {
		t.Fatal(e)
	}
	compareConvexOffsetOCCT(t, bin, "triangle-chamfer", .5, out)
}
