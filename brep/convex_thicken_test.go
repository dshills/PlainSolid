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

func TestConvexThickenOpenFace(t *testing.T) {
	ctx := context.Background()
	source := convexOffsetPrism(t, false)
	out, history, e := source.ThickenConvexOpenFace(ctx, 1, .5, brep.ConvexOffsetOptions{})
	if e != nil {
		t.Fatal(e)
	}
	faces := out.Faces()
	parts, e := out.Shells()
	if e != nil || len(parts) != 1 || len(faces) != 9 || len(faces[1].Holes) != 1 || len(faces[1].Holes[0]) != 3 ||
		history.LipFace != 1 || history.InteriorFaces[1] != -1 {
		t.Fatal("open cavity boundary and retained original cap", history, e)
	}
	for _, sourceFace := range []int{0, 2, 3, 4} {
		id := history.InteriorFaces[sourceFace]
		if id < len(source.Faces()) || id >= len(faces) {
			t.Fatal("original support must own its inward offset facet", history)
		}
	}
	m, e := out.MassProperties()
	if e != nil || math.Abs(m.Volume-93.0/4) > 1e-12 || math.Abs(m.SurfaceArea-99) > 1e-12 {
		t.Fatal("original volume minus independently integrated open triangular cavity", m, e)
	}
	for _, v := range out.Vertices()[len(source.Vertices()):] {
		if v.X < .5 || v.Y < .5 || 3*v.X+4*v.Y > 9.5 || v.Z != .5 && v.Z != 5 {
			t.Fatal("inner supports preserve normal thickness and the exact opening plane", v)
		}
	}
	for _, tc := range []struct {
		face int
		d    float64
		want error
	}{
		{1, 1, geom.ErrDomain},
		{-1, .5, geom.ErrInvalid},
		{1, 0, geom.ErrInvalid},
		{3, .5, geom.ErrUnrepresentable},
	} {
		out, h, e := source.ThickenConvexOpenFace(ctx, tc.face, tc.d, brep.ConvexOffsetOptions{})
		if !errors.Is(e, tc.want) || len(out.Faces()) != 0 || h.InteriorFaces != nil {
			t.Fatal(tc, h, e)
		}
	}
	if _, h, e := source.ThickenConvexOpenFace(ctx, 1, .5, brep.ConvexOffsetOptions{MaxPlaneTests: history.PlaneTests - 1}); !errors.Is(e, geom.ErrConvergence) || h.InteriorFaces != nil {
		t.Fatal("one short of complete work must fail atomically", h, e)
	}
}

func TestConvexThickenOpenFaceOCCT(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_CONVEX_OFFSET")
	if bin == "" {
		t.Skip("set PLAINSOLID_OCCT_CONVEX_OFFSET to the pinned OCCT comparator")
	}
	out, _, e := convexOffsetPrism(t, false).ThickenConvexOpenFace(context.Background(), 1, .5, brep.ConvexOffsetOptions{})
	if e != nil {
		t.Fatal(e)
	}
	compareConvexOffsetOCCT(t, bin, "triangle-open-top", -.5, out)
}
