// The OCCT executable is an optional development oracle, not a Go dependency.
package occtcheck

import (
	"bufio"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/csg"
	"plainsolid/csg/fillet"
	"plainsolid/csg/poly"
	"plainsolid/exchange/step"
	"plainsolid/geom"
	"plainsolid/solid"
)

type record struct {
	Name         string
	Area, Volume float64
	Centroid     [3]float64
}

func runOracle(t *testing.T, args ...string) map[string]record {
	t.Helper()
	program := os.Getenv("PLAINSOLID_OCCT_ORACLE")
	if program == "" {
		t.Skip("set PLAINSOLID_OCCT_ORACLE to the optional OCCT executable")
	}
	path := filepath.Join(t.TempDir(), "result.jsonl")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if b, e := exec.CommandContext(ctx, program, append([]string{path}, args...)...).CombinedOutput(); e != nil {
		t.Fatalf("OCCT executable: %v\n%s", e, b)
	}
	f, e := os.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	results := make(map[string]record)
	for s.Scan() {
		var r record
		if e = json.Unmarshal(s.Bytes(), &r); e != nil {
			t.Fatal(e)
		}
		if _, ok := results[r.Name]; ok || r.Name == "" || !(r.Volume > 0) || !(r.Area > 0) {
			t.Fatalf("invalid reference record %+v", r)
		}
		results[r.Name] = r
	}
	if e = s.Err(); e != nil {
		t.Fatal(e)
	}
	return results
}

func compare(t *testing.T, w record, area, volume float64, centroid geom.Vec3) {
	t.Helper()
	if w.Name == "" {
		t.Fatal("reference result missing")
	}
	if math.Abs(area-w.Area) > 3e-7*w.Area || math.Abs(volume-w.Volume) > 3e-7*w.Volume || centroid.Sub(geom.Vec3{X: w.Centroid[0], Y: w.Centroid[1], Z: w.Centroid[2]}).Length() > 3e-7 {
		t.Fatalf("%s: Go area %.15g volume %.15g centroid %v; OCCT area %.15g volume %.15g centroid %v", w.Name, area, volume, centroid, w.Area, w.Volume, w.Centroid)
	}
}

func TestOCCTMassProperties(t *testing.T) {
	ref := runOracle(t)
	box, e := brep.Box(geom.Vec3{X: -1, Y: 2, Z: 3}, geom.Vec3{X: 1, Y: 5, Z: 7})
	if e != nil {
		t.Fatal(e)
	}
	m, e := box.MassProperties()
	if e != nil {
		t.Fatal(e)
	}
	compare(t, ref["box"], m.SurfaceArea, m.Volume, m.Centroid)
	f, e := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	a := parametric.Ball{Radius: 2}
	b := parametric.Ball{Center: geom.Vec3{X: 2.25}, Radius: 1.75}
	for _, tc := range []struct {
		name string
		make func() (parametric.Solid, error)
	}{
		{"sphere", func() (parametric.Solid, error) { return parametric.Sphere(a.Center, a.Radius) }},
		{"cylinder", func() (parametric.Solid, error) { return parametric.Cylinder(f, 1.5, 2.25) }},
		{"cone", func() (parametric.Solid, error) { return parametric.Cone(f, math.Atan2(1.5, 2.25), 2.25) }},
		{"torus", func() (parametric.Solid, error) { return parametric.Torus(f, 3, .75) }},
		{"union", func() (parametric.Solid, error) { return parametric.CombineSpheres(a, b, parametric.Union, 1e-10) }},
		{"intersection", func() (parametric.Solid, error) {
			return parametric.CombineSpheres(a, b, parametric.Intersection, 1e-10)
		}},
		{"difference", func() (parametric.Solid, error) { return parametric.CombineSpheres(a, b, parametric.Difference, 1e-10) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, e := tc.make()
			if e != nil {
				t.Fatal(e)
			}
			p, e := s.MassProperties(context.Background(), parametric.IntegrationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			compare(t, ref[tc.name], p.Area, p.Volume, p.Centroid)
		})
	}
}

func TestOCCTSTEPImport(t *testing.T) {
	if os.Getenv("PLAINSOLID_OCCT_STEP") == "" {
		t.Skip("set PLAINSOLID_OCCT_STEP=1 for a reference executable with STEP enabled")
	}
	for _, hole := range []bool{false, true} {
		name := "box"
		if hole {
			name = "holed prism"
		}
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "test.step")
			outer := []geom.Vec3{{}, {X: 6}, {X: 6, Y: 5}, {Y: 5}}
			var holes [][]geom.Vec3
			if hole {
				holes = [][]geom.Vec3{{{X: 1, Y: 1}, {X: 2, Y: 1}, {X: 2, Y: 3}, {X: 1, Y: 3}}}
			}
			s, e := brep.PrismWithHoles(outer, holes, geom.Vec3{Z: 4})
			if e != nil {
				t.Fatal(e)
			}
			f, e := os.Create(path)
			if e != nil {
				t.Fatal(e)
			}
			err := step.Write(f, step.Model{LengthUnit: .001, Bodies: []step.Body{{Name: name, Solid: s}}})
			closeErr := f.Close()
			if err != nil || closeErr != nil {
				t.Fatalf("STEP write: %v %v", err, closeErr)
			}
			r := runOracle(t, path)
			m, e := s.MassProperties()
			if e != nil {
				t.Fatal(e)
			}
			compare(t, r["step"], m.SurfaceArea, m.Volume, m.Centroid)
		})
	}
}

func TestOCCTPlanarBooleans(t *testing.T) {
	ref := runOracle(t)
	box := func(min, max geom.Vec3) brep.Solid {
		t.Helper()
		s, e := brep.Box(min, max)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	a := box(geom.Vec3{X: -1, Y: -1, Z: -1}, geom.Vec3{X: 1, Y: 1, Z: 1})
	b := box(geom.Vec3{X: -.6, Y: -1.1, Z: -.7}, geom.Vec3{X: 1.5, Y: .7, Z: 1.3})
	rot, e := geom.Rotation(geom.Vec3{X: 1, Y: 2, Z: 3}, .25)
	if e != nil {
		t.Fatal(e)
	}
	b, e = b.Transformed(rot)
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		name string
		op   func(brep.Solid, brep.Solid, float64) (brep.Solid, error)
	}{
		{"convex-union", csg.Union}, {"convex-intersection", csg.Intersect}, {"convex-difference", csg.Difference},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, e := tc.op(a, b, 1e-9)
			if e != nil {
				t.Fatal(e)
			}
			m, e := s.MassProperties()
			if e != nil {
				t.Fatal(e)
			}
			compare(t, ref[tc.name], m.SurfaceArea, m.Volume, m.Centroid)
		})
	}
	aligned := box(geom.Vec3{X: 0, Y: -1, Z: -1}, geom.Vec3{X: 2, Y: 1, Z: 1})
	for _, tc := range []struct {
		name string
		op   func(brep.Solid, brep.Solid, float64) (brep.Solid, error)
	}{
		{"coplanar-union", csg.Union}, {"coplanar-difference", csg.Difference},
	} {
		s, e := tc.op(a, aligned, 1e-9)
		if e != nil {
			t.Fatal(e)
		}
		m, e := s.MassProperties()
		if e != nil {
			t.Fatal(e)
		}
		compare(t, ref[tc.name], m.SurfaceArea, m.Volume, m.Centroid)
	}

	tool := box(geom.Vec3{X: -.3, Y: -.4, Z: -2}, geom.Vec3{X: .3, Y: .4, Z: 2})
	p, e := solid.FromOrthogonal(a, solid.Options{})
	if e != nil {
		t.Fatal(e)
	}
	q, e := solid.FromOrthogonal(tool, solid.Options{})
	if e != nil {
		t.Fatal(e)
	}
	s, e := p.Difference(q)
	if e != nil {
		t.Fatal(e)
	}
	out, e := s.BRep(solid.Options{})
	if e != nil {
		t.Fatal(e)
	}
	props, e := out.MassProperties()
	if e != nil {
		t.Fatal(e)
	}
	compare(t, ref["orthogonal-tunnel"], props.SurfaceArea, props.Volume, props.Centroid)
}

func TestOCCTParallelCylinderBooleans(t *testing.T) {
	r := runOracle(t)
	f, e := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	a, b := parametric.DiskProfile{Radius: 2}, parametric.DiskProfile{Radius: 1.25, Center: geom.Vec2{X: 1.5}}
	for _, tc := range []struct {
		name string
		op   parametric.BooleanOp
	}{
		{"cylinder-union", parametric.Union}, {"cylinder-intersection", parametric.Intersection}, {"cylinder-difference", parametric.Difference},
	} {
		s, e := parametric.CombineParallelCylinders(f, a, b, 3, tc.op, 1e-9)
		if e != nil {
			t.Fatal(e)
		}
		p, e := s.MassProperties(context.Background(), parametric.IntegrationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		compare(t, r[tc.name], p.Area, p.Volume, p.Centroid)
	}
}

func TestOCCTNonconvexBooleans(t *testing.T) {
	r := runOracle(t)
	branch := func(cells [][3]int, xf geom.Affine) brep.Solid {
		t.Helper()
		boxes := make([]geom.Box, len(cells))
		for i, c := range cells {
			v := geom.Vec3{X: float64(c[0]), Y: float64(c[1]), Z: float64(c[2])}
			boxes[i] = geom.Box{Min: v, Max: v.Add(geom.Vec3{X: 1, Y: 1, Z: 1})}
		}
		p, e := solid.New(boxes...)
		if e != nil {
			t.Fatal(e)
		}
		b, e := p.BRep(solid.Options{})
		if e != nil {
			t.Fatal(e)
		}
		b, e = b.Transformed(xf)
		if e != nil {
			t.Fatal(e)
		}
		return b
	}
	a := branch([][3]int{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}, {0, 0, 1}}, geom.Affine{{1, .25, 0, 0}, {0, 1, .25, 0}, {.25, 0, 1, 0}})
	b := branch([][3]int{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}, {0, 1, 1}}, geom.Affine{{1, 0, .25, .5}, {.25, 1, 0, .25}, {0, .25, 1, .125}})
	for _, tc := range []struct {
		name string
		op   poly.Operation
	}{
		{"nonconvex-union", poly.Union}, {"nonconvex-intersection", poly.Intersection}, {"nonconvex-difference", poly.Difference},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, e := poly.Combine(context.Background(), a, b, tc.op, poly.Options{})
			if e != nil {
				t.Fatal(e)
			}
			p, e := s.MassProperties()
			if e != nil {
				t.Fatal(e)
			}
			compare(t, r[tc.name], p.SurfaceArea, p.Volume, p.Centroid)
		})
	}
}

func TestOCCTSphereCylinder(t *testing.T) {
	r := runOracle(t)
	f, e := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		name string
		op   parametric.BooleanOp
	}{{"bored-sphere", parametric.Difference}, {"spherical-core", parametric.Intersection}} {
		s, e := parametric.SphereCylinder(f, 3, math.Sqrt(5), tc.op, 1e-10)
		if e != nil {
			t.Fatal(e)
		}
		p, e := s.MassProperties(context.Background(), parametric.IntegrationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		compare(t, r[tc.name], p.Area, p.Volume, p.Centroid)
	}
}

func TestOCCTFiniteSphereCylinder(t *testing.T) {
	r := runOracle(t)
	ctx := context.Background()
	f, e := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	check := func(name string, s parametric.Solid, e error) {
		t.Helper()
		if e != nil {
			t.Fatal(name, e)
		}
		p, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil {
			t.Fatal(name, e)
		}
		compare(t, r[name], p.Area, p.Volume, p.Centroid)
	}
	d, e := parametric.SphereFiniteCylinderDifference(ctx, f, 5, 3, 2, 6, 1e-9)
	if e != nil || len(d.Solids) != 1 {
		t.Fatal("blind cut", e, len(d.Solids))
	}
	check("finite-sphere-cylinder-blind-cut", d.Solids[0], nil)
	u, e := parametric.SphereFiniteCylinderUnion(ctx, f, 5, 5, -4, 4, 1e-9)
	if e != nil || len(u.Solids) != 1 {
		t.Fatal("union", e, len(u.Solids))
	}
	check("finite-sphere-cylinder-union", u.Solids[0], nil)
	c, e := parametric.SphereFiniteCylinderCommon(ctx, f, 5, 3, 2, 6, 1e-9)
	check("finite-sphere-cylinder-common", c.Solid, e)
	a, e := parametric.SphereFiniteCylinderCommonAlgebraic(ctx, f, 5, 2, -4.75, 4.75, 1e-8, geom.ExactCurvePlaneEventOptions{})
	check("finite-sphere-cylinder-algebraic-common", a.Solid, e)
	b, e := parametric.SphereOffAxisBlindBore(ctx, f, 5, 2, 1, -.75, 1e-9)
	check("off-axis-sphere-blind-bore", b.Solid, e)
	w, e := parametric.SphereOffAxisBlindBore(ctx, f, 5, 1, 2, .25, 1e-9)
	check("wide-offset-sphere-blind-bore", w.Solid, e)
	tb, e := parametric.SphereOffAxisThroughBore(ctx, f, 5, 1, 2, 1e-9)
	check("offset-sphere-through-bore", tb.Solid, e)
	oc, e := parametric.SphereOffAxisCylinderCommon(ctx, f, 5, 1, 2, -2, 6, 1e-9)
	check("offset-sphere-cylinder-common", oc.Solid, e)
	pc, e := parametric.SphereOffAxisCylinderPartialCommon(ctx, f, 5, 1, 2, -2, 4.25, 1e-9)
	check("offset-sphere-cylinder-partial-common", pc.Solid, e)
	cc, e := parametric.SphereOffAxisCylinderChordCommon(ctx, f, 5, 1, 2, -2, 4.875, 1e-9)
	check("offset-sphere-cylinder-chord-common", cc.Solid, e)
	dc, e := parametric.SphereOffAxisCylinderDoubleCommon(ctx, f, 5, 1, 2, -4.25, 4.75, 1e-9)
	check("offset-sphere-cylinder-double-common", dc.Solid, e)
	ou, e := parametric.SphereOffAxisCylinderUnion(ctx, f, 5, 1, 2, -1, 5.5, 1e-9)
	check("offset-sphere-cylinder-union", ou.Solid, e)

	sc, e := parametric.SphereOffAxisCylinderSameSideCommon(ctx, f, 5, 1, 2, 4.125, 4.75, 1e-9)
	check("offset-sphere-cylinder-same-side-common", sc.Solid, e)

	cs, e := parametric.SphereFiniteOffsetCylinderCommon(ctx, f, 5, 6, 1, -2, 7, 1e-8)
	check("offset-sphere-cylinder-covered-slab", cs.Solid, e)

	ccr, e := parametric.PerpendicularCylinderCommonRadii(ctx, geom.Vec3{}, 3, 2, 4, 5, 1e-9)
	check("perpendicular-cylinder-common", ccr.Solid, e)

	sk, e := parametric.SkewOrthogonalCylinderCommon(ctx, geom.Vec3{}, 3, 2, .5, 4, 5, 1e-9)
	check("skew-orthogonal-cylinder-common", sk.Solid, e)

	ov, e := parametric.SkewOrthogonalOverlapCommon(ctx, geom.Vec3{}, 3, 2, 2, 4, 5, 1e-9)
	check("skew-orthogonal-overlap-common", ov.Solid, e)
	capRange := [2]float64{-3, 3}
	ob, e := parametric.ObliqueEqualCylinderCommon(ctx, geom.Vec3{}, geom.Vec3{X: 3, Y: 4}, geom.Vec3{X: 4, Y: 3}, 2, capRange, capRange, 1e-9)
	check("oblique-equal-cylinder-common", ob.Solid, e)
	gen, e := parametric.ObliqueCylinderCommon(ctx, geom.Vec3{}, geom.Vec3{X: 2}, geom.Vec3{X: 3, Y: 4}, 1, 2, capRange, capRange, 1e-9)
	check("oblique-unequal-cylinder-common", gen.Solid, e)
}

func TestOCCTBoredBox(t *testing.T) {
	r := runOracle(t)
	box, e := brep.Box(geom.Vec3{X: -2, Y: -2, Z: -1}, geom.Vec3{X: 4, Y: 3, Z: 2})
	if e != nil {
		t.Fatal(e)
	}
	s, _, e := trimmed.BoreCylinder(box, 0, 1, geom.Vec3{X: .5, Y: .25}, .75, 1e-10)
	if e != nil {
		t.Fatal(e)
	}
	p, e := s.MassProperties(context.Background(), parametric.IntegrationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	compare(t, r["bored-box"], p.Area, p.Volume, p.Centroid)
	box, e = brep.Box(geom.Vec3{}, geom.Vec3{X: 4, Y: 5, Z: 6})
	if e != nil {
		t.Fatal(e)
	}
	s, _, e = trimmed.BoreCylinderAlong(box, 0, 3, geom.Vec3{X: 1, Y: 2}, geom.Vec3{X: 2, Z: 1}, .35, 1e-10)
	if e != nil {
		t.Fatal(e)
	}
	p, e = s.MassProperties(context.Background(), parametric.IntegrationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	compare(t, r["oblique-bored-box"], p.Area, p.Volume, p.Centroid)
	box, e = brep.Box(geom.Vec3{}, geom.Vec3{X: 8, Y: 6, Z: 4})
	if e != nil {
		t.Fatal(e)
	}
	s, _, e = trimmed.BoreCylinders(box, []trimmed.CylinderCut{
		{Entry: 0, Exit: 1, AxisPoint: geom.Vec3{X: 2, Y: 3}, Direction: geom.Vec3{Z: 1}, Radius: .5},
		{Entry: 0, Exit: 1, AxisPoint: geom.Vec3{X: 6, Y: 3}, Direction: geom.Vec3{Z: 1}, Radius: .75},
	}, 1e-10)
	if e != nil {
		t.Fatal(e)
	}
	p, e = s.MassProperties(context.Background(), parametric.IntegrationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	compare(t, r["double-bored-box"], p.Area, p.Volume, p.Centroid)
}

func TestOCCTCornerRoundedPrism(t *testing.T) {
	r := runOracle(t)
	ctx := context.Background()
	s, e := fillet.Prism(ctx, []geom.Vec2{{}, {X: 6}, {X: 6, Y: 4}, {Y: 4}}, 0, 3, .6, fillet.Options{})
	if e != nil {
		t.Fatal(e)
	}
	p, e := s.Solid.MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	compare(t, r["corner-rounded-prism"], p.Area, p.Volume, p.Centroid)
}

func TestOCCTSphericalPocket(t *testing.T) {
	r := runOracle(t)
	ctx := context.Background()
	s, _, e := trimmed.CutSphereFromBox(geom.Vec3{}, geom.Vec3{X: 8, Y: 6, Z: 4}, parametric.Ball{Center: geom.Vec3{X: 4, Y: 3, Z: 4.5}, Radius: 1}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	p, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	compare(t, r["spherical-pocket"], p.Area, p.Volume, p.Centroid)
}
