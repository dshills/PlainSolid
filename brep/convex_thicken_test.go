package brep_test

import (
	"context"
	"errors"
	"math"
	"os"
	"slices"
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

func TestConvexThickenMultipleLipsAndClosedCavity(t *testing.T) {
	ctx := context.Background()
	source := convexOffsetPrism(t, false)
	for _, tc := range []struct {
		open        []int
		volume      float64
		shells      int
		faces       int
		oracleShape string
	}{
		{nil, 24, 2, 10, "triangle-closed"},
		{[]int{1, 0}, 22.5, 1, 8, "triangle-open-both"},
	} {
		t.Run(tc.oracleShape, func(t *testing.T) {
			out, h, e := source.ThickenConvex(ctx, tc.open, .5, brep.ConvexOffsetOptions{})
			if e != nil {
				t.Fatal(e)
			}
			wantLips := append([]int(nil), tc.open...)
			slices.Sort(wantLips)
			faces := out.Faces()
			if h.LipFace != -1 || !slices.Equal(wantLips, h.LipFaces) || len(faces) != tc.faces {
				t.Fatal("source lip identities and complete exact wall collection", h)
			}
			for _, i := range wantLips {
				if h.InteriorFaces[i] != -1 || len(faces[i].Holes) != 1 || len(faces[i].Holes[0]) != 3 {
					t.Fatal("an original opening retains an oriented hole", i, h)
				}
			}
			m, e := out.MassProperties()
			if e != nil || math.Abs(m.Volume-tc.volume) > 1e-12 || math.Abs(m.SurfaceArea-99) > 1e-12 {
				t.Fatal("exact signed material volume and original surface area", m, e)
			}
			shells, e := out.Shells()
			if e != nil || len(shells) != tc.shells {
				t.Fatal("complete regularized shell topology", len(shells), e)
			}
			if len(shells) == 2 {
				inner, e := shells[1].MassProperties()
				if e != nil || inner.Volume != -6 {
					t.Fatal("the closed cavity shell must face into material", inner, e)
				}
			}
			if bin := os.Getenv("PLAINSOLID_OCCT_CONVEX_OFFSET"); bin != "" {
				compareConvexOffsetOCCT(t, bin, tc.oracleShape, -.5, out)
			}
		})
	}
	for _, tc := range []struct {
		open []int
		want error
	}{
		{[]int{0, 0}, geom.ErrInvalid},
		{[]int{1, 2}, geom.ErrDomain},
		{[]int{0, 1, 2, 3, 4}, geom.ErrDomain},
	} {
		out, h, e := source.ThickenConvex(ctx, tc.open, .5, brep.ConvexOffsetOptions{})
		if !errors.Is(e, tc.want) || len(out.Faces()) != 0 || h.InteriorFaces != nil {
			t.Fatal("invalid or touching openings must fail atomically", tc, h, e)
		}
	}
}
