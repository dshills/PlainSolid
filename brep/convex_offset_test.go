package brep_test

import (
	"context"
	"errors"
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	"plainsolid/brep"
	"plainsolid/geom"
)

func convexOffsetPrism(t *testing.T, clipped bool) brep.Solid {
	t.Helper()
	profile := []geom.Vec3{{}, {X: 4}, {Y: 3}}
	if clipped {
		profile = []geom.Vec3{{}, {X: 4}, {X: 4, Y: 3.25}, {X: 3, Y: 4}, {Y: 4}}
	}
	s, e := brep.Prism(profile, geom.Vec3{Z: 5})
	if e != nil {
		t.Fatal(e)
	}
	return s
}

func TestConvexMiterOffset(t *testing.T) {
	ctx := context.Background()
	stock := convexOffsetPrism(t, false)
	for _, tc := range []struct {
		distance, volume, area float64
	}{
		{.5, 81, 135},
		{-.5, 6, 27},
	} {
		out, history, e := stock.OffsetConvex(ctx, tc.distance, brep.ConvexOffsetOptions{})
		if e != nil {
			t.Fatal(tc, e)
		}
		m, e := out.MassProperties()
		if e != nil || math.Abs(m.Volume-tc.volume) > 1e-12 || math.Abs(m.SurfaceArea-tc.area) > 1e-12 || len(out.Faces()) != 5 || len(out.Edges()) != 9 {
			t.Fatal(tc, m, history, e)
		}
		for i, face := range history.Faces {
			if face != i {
				t.Fatal("original supporting face order and ownership", history)
			}
		}
		d := tc.distance
		for _, p := range out.Vertices() {
			// The three side supports have outward unit normals (-1,0),
			// (0,-1), and (3/5,4/5), so their raw lifts are d,d,5d.
			x := p.X == -d
			y := p.Y == -d
			h := 3*p.X+4*p.Y == 12+5*d
			if !(x && y || x && h || y && h) || p.Z != -d && p.Z != 5+d {
				t.Fatal("vertex must lie on three independent translated supports", p, tc)
			}
		}
		if _, h, e := stock.OffsetConvex(ctx, d, brep.ConvexOffsetOptions{MaxPlaneTests: history.PlaneTests}); e != nil || h.PlaneTests != history.PlaneTests {
			t.Fatal("deterministic exact work", history, h, e)
		}
		if out, h, e := stock.OffsetConvex(ctx, d, brep.ConvexOffsetOptions{MaxPlaneTests: history.PlaneTests - 1}); !errors.Is(e, geom.ErrConvergence) || len(out.Faces()) != 0 || h.Faces != nil {
			t.Fatal("quota must fail atomically", h, e)
		}
	}
}

func TestConvexMiterOffsetFacetExtinction(t *testing.T) {
	ctx := context.Background()
	stock := convexOffsetPrism(t, true)
	out, hist, e := stock.OffsetConvex(ctx, -1.5, brep.ConvexOffsetOptions{})
	if e != nil {
		t.Fatal(e)
	}
	m, e := out.MassProperties()
	if e != nil || math.Abs(m.Volume-2) > 1e-12 || len(out.Faces()) != 6 || len(out.Vertices()) != 8 || len(hist.Faces) != 7 || hist.Faces[4] != -1 {
		t.Fatal("the oblique support has only a shared edge at extinction", m, hist, e)
	}
	out, hist, e = stock.OffsetConvex(ctx, -2, brep.ConvexOffsetOptions{})
	if e != nil || len(out.Faces()) != 0 {
		t.Fatal("collapse to a segment must regularize to empty", hist, e)
	}
	for _, id := range hist.Faces {
		if id != -1 {
			t.Fatal("no remaining positive-volume material owns a face", hist)
		}
	}
}

func TestConvexMiterOffsetReject(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		profile []geom.Vec3
		d       float64
		want    error
	}{
		{[]geom.Vec3{{}, {X: 3}, {Y: 3}}, .5, geom.ErrUnrepresentable},
		{[]geom.Vec3{{}, {X: 4}, {Y: 3}}, .1, geom.ErrUnrepresentable},
		{[]geom.Vec3{{}, {X: 4}, {X: 2, Y: 1}, {X: 4, Y: 3}, {Y: 3}}, .5, geom.ErrDomain},
	} {
		s, e := brep.Prism(tc.profile, geom.Vec3{Z: 5})
		if e != nil {
			t.Fatal(e)
		}
		out, h, e := s.OffsetConvex(ctx, tc.d, brep.ConvexOffsetOptions{})
		if !errors.Is(e, tc.want) || len(out.Faces()) != 0 || h.Faces != nil {
			t.Fatal(tc, h, e)
		}
	}
	stock := convexOffsetPrism(t, false)
	ctx, cancel := context.WithCancel(ctx)
	cancel()
	if _, h, e := stock.OffsetConvex(ctx, .5, brep.ConvexOffsetOptions{}); !errors.Is(e, context.Canceled) || h.Faces != nil {
		t.Fatal("cancellation must publish no result", h, e)
	}
}

func TestConvexMiterOffsetOCCT(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_CONVEX_OFFSET")
	if bin == "" {
		t.Skip("set PLAINSOLID_OCCT_CONVEX_OFFSET to the pinned OCCT comparator")
	}
	ctx := context.Background()
	for _, tc := range []struct {
		shape   string
		clipped bool
		d       float64
	}{
		{"triangle", false, .5},
		{"triangle", false, -.5},
	} {
		s := convexOffsetPrism(t, tc.clipped)
		out, _, e := s.OffsetConvex(ctx, tc.d, brep.ConvexOffsetOptions{})
		if e != nil {
			t.Fatal(e)
		}
		compareConvexOffsetOCCT(t, bin, tc.shape, tc.d, out)
	}
}

func compareConvexOffsetOCCT(t *testing.T, bin, shape string, d float64, out brep.Solid) {
	t.Helper()
	m, e := out.MassProperties()
	if e != nil {
		t.Fatal(e)
	}
	b, e := exec.Command(bin, shape, strconv.FormatFloat(d, 'g', -1, 64)).CombinedOutput()
	if e != nil {
		t.Fatal(e, string(b))
	}
	var values []float64
	for _, x := range strings.Fields(string(b)) {
		v, e := strconv.ParseFloat(x, 64)
		if e != nil {
			t.Fatal(e, string(b))
		}
		values = append(values, v)
	}
	if len(values) != 5 || values[0] != 1 || values[1] != 1 || values[2] != float64(len(out.Faces())) ||
		math.Abs(values[3]-m.Volume) > 1e-10 || math.Abs(values[4]-m.SurfaceArea) > 1e-10 {
		t.Fatal("OCCT miter offset geometry", string(b), m)
	}
}
