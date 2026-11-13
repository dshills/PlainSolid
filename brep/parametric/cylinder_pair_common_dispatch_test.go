package parametric

import (
	"context"
	"errors"
	"math"
	"testing"

	"plainsolid/geom"
)

func TestFiniteCylinderPairCommon(t *testing.T) {
	ctx := context.Background()
	for _, c := range []struct {
		x, z [2]float64
		want CylinderCommonConstruction
		l, u [2]bool
	}{
		{[2]float64{-6, 6}, [2]float64{-6, 6}, CylinderCommonIntersectingAxes, [2]bool{}, [2]bool{}},
		{[2]float64{-6, 2}, [2]float64{-6, 6}, CylinderCommonPerpendicularCap, [2]bool{}, [2]bool{true, false}},
		{[2]float64{-2, 6}, [2]float64{-6, 6}, CylinderCommonPerpendicularCap, [2]bool{true, false}, [2]bool{}},
		{[2]float64{-6, 6}, [2]float64{-6, 0}, CylinderCommonPerpendicularCap, [2]bool{}, [2]bool{false, true}},
		{[2]float64{-3, 2}, [2]float64{-6, 6}, CylinderCommonPerpendicularSlab, [2]bool{true, false}, [2]bool{true, false}},
		{[2]float64{0, 6}, [2]float64{-6, 0}, CylinderCommonPerpendicularQuadrant, [2]bool{true, false}, [2]bool{false, true}},
		{[2]float64{5, 6}, [2]float64{-6, 6}, CylinderCommonEmpty, [2]bool{}, [2]bool{}},
	} {
		a := geom.RawCylinder{Origin: geom.Vec3{Y: -1, Z: 2}, Axis: geom.Vec3{X: 1}, Radius: 5, Min: c.x[0], Max: c.x[1]}
		b := geom.RawCylinder{Origin: geom.Vec3{Y: -1, Z: 2}, Axis: geom.Vec3{Z: 1}, Radius: 5, Min: c.z[0], Max: c.z[1]}
		out, e := FiniteCylinderPairCommon(ctx, a, b, 1e-8)
		if e != nil || out.Construction != c.want {
			t.Fatal(c, out.Construction, e)
		}
		if c.want == CylinderCommonEmpty {
			if len(out.Solid.Faces()) != 0 {
				t.Fatal(c, "not empty")
			}
			continue
		}
		seen := map[int]bool{}
		for j := range 2 {
			if (len(out.LowerCapFaces[j]) > 0) != c.l[j] || (len(out.UpperCapFaces[j]) > 0) != c.u[j] {
				t.Fatal(c, out.LowerCapFaces, out.UpperCapFaces)
			}
			for _, g := range [][]int{out.SideFaces[j], out.LowerCapFaces[j], out.UpperCapFaces[j]} {
				for _, id := range g {
					if seen[id] || id < 0 || id >= len(out.Solid.Faces()) {
						t.Fatal(c, id)
					}
					seen[id] = true
				}
			}
		}
		if len(seen) != len(out.Solid.Faces()) {
			t.Fatal(c, "incomplete ancestry")
		}
	}
	a := geom.RawCylinder{Axis: geom.Vec3{X: 2}, Radius: 3, Min: -5, Max: 5}
	b := geom.RawCylinder{Origin: geom.Vec3{Z: 2}, Axis: geom.Vec3{X: 3, Y: 4}, Radius: 2, Min: -5, Max: 5}
	out, e := FiniteCylinderPairCommon(ctx, a, b, 1e-9)
	if e != nil || out.Construction != CylinderCommonSkewAxes || len(out.Solid.Faces()) != 4 {
		t.Fatal(out, e)
	}
	b.Min, b.Max = 0, 1
	out, e = FiniteCylinderPairCommon(ctx, a, b, 1e-9)
	if !errors.Is(e, geom.ErrConvergence) || len(out.Solid.Faces()) != 0 {
		t.Fatal(out, e)
	}
}

func TestFiniteParallelCylinderPairCommon(t *testing.T) {
	ctx := context.Background()
	a := geom.RawCylinder{Origin: geom.Vec3{X: 1, Y: 2}, Axis: geom.Vec3{Y: 2}, Radius: 2, Min: -1, Max: 3}
	b := geom.RawCylinder{Origin: geom.Vec3{X: 2, Y: 10}, Axis: geom.Vec3{Y: -3}, Radius: 2, Min: 1, Max: 2}
	out, e := FiniteCylinderPairCommon(ctx, a, b, 1e-9)
	if e != nil || out.Construction != CylinderCommonParallelAxes || len(out.SideFaces[0]) != 1 || len(out.SideFaces[1]) != 1 ||
		len(out.LowerCapFaces[1]) != 2 || len(out.UpperCapFaces[1]) != 2 || len(out.LowerCapFaces[0])+len(out.UpperCapFaces[0]) != 0 {
		t.Fatal(out, e)
	}
	p, e := out.Solid.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-7})
	if e != nil || math.Abs(p.Volume-3*(8*math.Acos(.25)-math.Sqrt(15)/2)) > 1e-8 || math.Abs(p.Centroid.Y-5.5) > 1e-9 {
		t.Fatal(p, e)
	}
	b.Origin.X = 5
	out, e = FiniteCylinderPairCommon(ctx, a, b, 1e-9)
	if e != nil || out.Construction != CylinderCommonEmpty || len(out.Solid.Faces()) != 0 {
		t.Fatal("regularized tangent parallel common", out, e)
	}
}

func TestFiniteCylinderPairCommonSignedCoordinateCaps(t *testing.T) {
	ctx := context.Background()
	center := geom.Vec3{X: 1, Y: 2, Z: -3}
	// The two raw origins do not coincide. The signed directions and raw
	// norms put the shared axes at center and the physical caps at [-2, 3]
	// and [-6, 6]. Cap ancestry must follow raw Min/Max after placement.
	a := geom.RawCylinder{Origin: center.Add(geom.Vec3{Z: 2}), Axis: geom.Vec3{Z: -2}, Radius: 5, Min: 0, Max: 2.5}
	b := geom.RawCylinder{Origin: center.Add(geom.Vec3{Y: 4}), Axis: geom.Vec3{Y: 4}, Radius: 5, Min: -2.5, Max: .5}
	want, e := PerpendicularCylinderSlab(ctx, geom.Vec3{}, 5, -2, 3, 6, 1e-8, geom.ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	props, e := want.Solid.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-6})
	if e != nil {
		t.Fatal(e)
	}
	for _, pair := range [][2]geom.RawCylinder{{a, b}, {b, a}} {
		out, e := FiniteCylinderPairCommon(ctx, pair[0], pair[1], 1e-8)
		if e != nil || out.Construction != CylinderCommonPerpendicularSlab {
			t.Fatal(out.Construction, e)
		}
		owner := 0
		if pair[1] == a {
			owner = 1
		}
		faces := out.Solid.Faces()
		for cap, ids := range [2][]int{out.LowerCapFaces[owner], out.UpperCapFaces[owner]} {
			if len(ids) == 0 {
				t.Fatal("missing cap ownership", out.LowerCapFaces, out.UpperCapFaces)
			}
			z := [2]float64{-1, -6}[cap]
			for _, id := range ids {
				f := faces[id]
				p, e := f.Surface.Eval((f.U.Min+f.U.Max)/2, (f.V.Min+f.V.Max)/2)
				if e != nil || p.Z != z {
					t.Fatal("cap moved", p, z, e)
				}
			}
		}
		if len(out.LowerCapFaces[1-owner])+len(out.UpperCapFaces[1-owner]) != 0 {
			t.Fatal("unclipped owner acquired a cap")
		}
		got, e := out.Solid.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-6})
		centroid := center.Add(geom.Vec3{X: -props.Centroid.Y, Y: props.Centroid.Z, Z: -props.Centroid.X})
		if e != nil || math.Abs(got.Volume-props.Volume) > 1e-6 || got.Centroid.Sub(centroid).Length() > 1e-6 {
			t.Fatal(got, props, centroid, e)
		}
	}
}

func TestFiniteCylinderPairCommonOutsideAxialSupport(t *testing.T) {
	ctx := context.Background()
	a := geom.RawCylinder{Axis: geom.Vec3{X: 2}, Radius: 3, Min: 1, Max: 3}
	b := geom.RawCylinder{Axis: geom.Vec3{Z: 1}, Radius: 2, Min: -6, Max: 6}
	// A's physical start is x=2, exactly on B's outermost generator.
	out, e := FiniteCylinderPairCommon(ctx, a, b, 1e-8)
	if e != nil || out.Construction != CylinderCommonEmpty || len(out.Solid.Faces()) != 0 {
		t.Fatal(out, e)
	}
	a.Min = math.Nextafter(1, 0)
	out, e = FiniteCylinderPairCommon(ctx, a, b, 1e-8)
	if !errors.Is(e, geom.ErrConvergence) || len(out.Solid.Faces()) != 0 {
		t.Fatal("an unsupported but positive interval was misclassified", out, e)
	}
	// The feet are not dyadic raw parameters here. Cap exclusion still
	// proves empty without constructing or rounding either placement.
	a = geom.RawCylinder{Axis: geom.Vec3{X: 3, Y: 1}, Radius: 3, Min: -15, Max: -10}
	b = geom.RawCylinder{Origin: geom.Vec3{X: 1, Y: -1, Z: .5}, Axis: geom.Vec3{Z: 2}, Radius: 2, Min: -10, Max: 10}
	for _, p := range [][2]geom.RawCylinder{{a, b}, {b, a}} {
		out, e := FiniteCylinderPairCommon(ctx, p[0], p[1], 1e-8)
		if e != nil || out.Construction != CylinderCommonEmpty || len(out.Solid.Faces()) != 0 {
			t.Fatal(out, e)
		}
	}
}

func TestFiniteCylinderPairCommonNonDyadicFootParameters(t *testing.T) {
	ctx := context.Background()
	// Both axes cross at the origin, but the first foot has raw parameter
	// -1/3. Recentring the original cap bounds must keep that exact fraction.
	a := geom.RawCylinder{Origin: geom.Vec3{X: 1}, Axis: geom.Vec3{X: 3}, Radius: 5, Min: -3, Max: 3}
	b := geom.RawCylinder{Axis: geom.Vec3{Z: 1}, Radius: 5, Min: -6, Max: 6}
	out, e := FiniteCylinderPairCommon(ctx, a, b, 1e-8)
	if e != nil || out.Construction != CylinderCommonIntersectingAxes {
		t.Fatal(out.Construction, e)
	}
	props, e := out.Solid.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-6})
	if e != nil || math.Abs(props.Volume-2000.0/3) > 1e-6 {
		t.Fatal(props, e)
	}
	a.Max = 0
	out, e = FiniteCylinderPairCommon(ctx, a, b, 1e-8)
	if e != nil || out.Construction != CylinderCommonPerpendicularCap || len(out.UpperCapFaces[0]) == 0 {
		t.Fatal(out.Construction, out.UpperCapFaces, e)
	}
	faces := out.Solid.Faces()
	for _, id := range out.UpperCapFaces[0] {
		f := faces[id]
		p, e := f.Surface.Eval((f.U.Min+f.U.Max)/2, (f.V.Min+f.V.Max)/2)
		if e != nil || p.X != 1 {
			t.Fatal("cap must stay on the original plane", p, e)
		}
	}
}

func TestFiniteCylinderPairCommonTangentCaps(t *testing.T) {
	ctx := context.Background()
	a := geom.RawCylinder{Axis: geom.Vec3{X: 2}, Radius: 5, Min: -2.5, Max: 2.5}
	b := geom.RawCylinder{Axis: geom.Vec3{Z: -1}, Radius: 5, Min: -5, Max: 5}
	out, e := FiniteCylinderPairCommon(ctx, a, b, 1e-8)
	if e != nil || out.Construction != CylinderCommonIntersectingAxes {
		t.Fatal(out.Construction, e)
	}
	for i := range 2 {
		if len(out.SideFaces[i]) == 0 || len(out.LowerCapFaces[i])+len(out.UpperCapFaces[i]) != 0 {
			t.Fatal("tangent caps do not own planar area", out.SideFaces, out.LowerCapFaces, out.UpperCapFaces)
		}
	}
	p, e := out.Solid.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-6})
	if e != nil || math.Abs(p.Volume-2000.0/3) > 1e-6 || p.Centroid.Length() > 1e-9 {
		t.Fatal(p, e)
	}
	query, e := geom.NewCylinderPairCommon(a, b)
	if e != nil {
		t.Fatal(e)
	}
	loc, mask, e := query.Classify(geom.Vec3{X: 5})
	if e != nil || loc != geom.BoreBoundary || mask != geom.RawCylinderFirstMax|geom.RawCylinderSecondSide {
		t.Fatal("contact still owns the original raw cap", loc, mask, e)
	}
}

func TestFiniteCylinderPairCommonClippedWithSupportingCaps(t *testing.T) {
	ctx := context.Background()
	a := geom.RawCylinder{Axis: geom.Vec3{X: 1}, Radius: 5, Min: -5, Max: 2}
	b := geom.RawCylinder{Axis: geom.Vec3{Z: 1}, Radius: 5, Min: -5, Max: 5}
	for _, c := range []struct {
		x, z [2]float64
		kind CylinderCommonConstruction
	}{
		{[2]float64{-5, 2}, [2]float64{-5, 5}, CylinderCommonPerpendicularCap},
		{[2]float64{-3, 2}, [2]float64{-5, 5}, CylinderCommonPerpendicularSlab},
		{[2]float64{0, 5}, [2]float64{-5, 0}, CylinderCommonPerpendicularQuadrant},
	} {
		a.Min, a.Max, b.Min, b.Max = c.x[0], c.x[1], c.z[0], c.z[1]
		tight, e := FiniteCylinderPairCommon(ctx, a, b, 1e-8)
		if e != nil || tight.Construction != c.kind {
			t.Fatal(c, tight.Construction, e)
		}
		for _, v := range []*float64{&a.Min, &a.Max, &b.Min, &b.Max} {
			if math.Abs(*v) == 5 {
				*v = math.Copysign(6, *v)
			}
		}
		wide, e := FiniteCylinderPairCommon(ctx, a, b, 1e-8)
		if e != nil || tight.Construction != wide.Construction {
			t.Fatal(c, wide.Construction, e)
		}
		pt, e := tight.Solid.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-6})
		if e != nil {
			t.Fatal(e)
		}
		pw, e := wide.Solid.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-6})
		if e != nil || pt != pw {
			t.Fatal("touching the support changed material", c, pt, pw, e)
		}
	}
}

func TestFiniteObliqueParallelCylinderPairEmpty(t *testing.T) {
	ctx := context.Background()
	a := geom.RawCylinder{Axis: geom.Vec3{X: 3, Y: 4}, Radius: 2, Min: 0, Max: 2}
	b := geom.RawCylinder{Origin: geom.Vec3{Z: 4}, Axis: geom.Vec3{X: -6, Y: -8}, Radius: 2, Min: -1, Max: 0}
	assertEmpty := func(a, b geom.RawCylinder) {
		t.Helper()
		out, e := FiniteCylinderPairCommon(ctx, a, b, 1e-9)
		if e != nil || out.Construction != CylinderCommonEmpty || len(out.Solid.Faces()) != 0 {
			t.Fatal(out, e)
		}
	}
	assertEmpty(a, b) // Axial overlap, exactly tangent radial disks.
	b.Origin.Z = math.Nextafter(4, 0)
	out, e := FiniteCylinderPairCommon(ctx, a, b, 1e-9)
	if !errors.Is(e, geom.ErrDomain) || len(out.Solid.Faces()) != 0 {
		t.Fatal("positive but unsupported radial overlap", out, e)
	}
	b.Origin = geom.Vec3{X: 6, Y: 8}
	b.Min, b.Max = -2, 0
	assertEmpty(a, b) // Same axis, cap-only contact with reversed raw direction.
	assertEmpty(b, a)
}

func TestFinitePerpendicularCylinderPairContainment(t *testing.T) {
	ctx := context.Background()
	// A smaller cylinder of radius 3 fits at its x=±4 caps exactly because
	// the perpendicular radius is 5. The larger body's Y caps also touch.
	a := geom.RawCylinder{Origin: geom.Vec3{X: 1, Y: 2, Z: 3}, Axis: geom.Vec3{X: -2}, Radius: 3, Min: -2, Max: 2}
	b := geom.RawCylinder{Origin: geom.Vec3{X: 1, Y: 2, Z: 3}, Axis: geom.Vec3{Y: 1}, Radius: 5, Min: -3, Max: 3}
	for owner, pair := range [][2]geom.RawCylinder{{a, b}, {b, a}} {
		out, e := FiniteCylinderPairCommon(ctx, pair[0], pair[1], 1e-9)
		if e != nil || out.Construction != CylinderCommonContainedCylinder || len(out.Solid.Faces()) != 3 {
			t.Fatal(out.Construction, e)
		}
		for _, group := range [][2][]int{out.SideFaces, out.LowerCapFaces, out.UpperCapFaces} {
			if len(group[owner]) != 1 || len(group[1-owner]) != 0 {
				t.Fatal("lost original owner", group)
			}
		}
		p, e := out.Solid.MassProperties(ctx, IntegrationOptions{})
		if e != nil || math.Abs(p.Volume-72*math.Pi) > 1e-8 || p.Centroid.Sub(a.Origin).Length() > 1e-9 {
			t.Fatal(p, e)
		}
		for i, id := range []int{out.LowerCapFaces[owner][0], out.UpperCapFaces[owner][0]} {
			f := out.Solid.Faces()[id]
			point, e := f.Surface.Eval(0, 0)
			if e != nil || point.X != [2]float64{5, -3}[i] {
				t.Fatal("raw cap direction lost", point, e)
			}
		}
	}
	a.Max = math.Nextafter(a.Max, math.Inf(1))
	out, e := FiniteCylinderPairCommon(ctx, a, b, 1e-9)
	if !errors.Is(e, geom.ErrConvergence) || len(out.Solid.Faces()) != 0 {
		t.Fatal("noncontained sliver was silently removed", out, e)
	}
}

func TestFiniteSkewPerpendicularCylinderPairContainment(t *testing.T) {
	ctx := context.Background()
	a := geom.RawCylinder{Origin: geom.Vec3{X: 2, Y: 3, Z: 4}, Axis: geom.Vec3{X: 2}, Radius: 2, Min: -2, Max: 2}
	b := geom.RawCylinder{Origin: geom.Vec3{X: 2, Y: 4, Z: 4}, Axis: geom.Vec3{Z: -4}, Radius: 5, Min: -.5, Max: .5}
	for owner, pair := range [][2]geom.RawCylinder{{a, b}, {b, a}} {
		out, e := FiniteCylinderPairCommon(ctx, pair[0], pair[1], 1e-9)
		if e != nil || out.Construction != CylinderCommonContainedCylinder || len(out.SideFaces[owner]) != 1 || len(out.SideFaces[1-owner]) != 0 {
			t.Fatal(out.Construction, out.SideFaces, e)
		}
		p, e := out.Solid.MassProperties(ctx, IntegrationOptions{})
		if e != nil || math.Abs(p.Volume-32*math.Pi) > 1e-8 || p.Centroid.Sub(a.Origin).Length() > 1e-9 {
			t.Fatal(p, e)
		}
	}
	b.Origin.Y = math.Nextafter(4, math.Inf(1))
	out, e := FiniteCylinderPairCommon(ctx, a, b, 1e-9)
	if !errors.Is(e, geom.ErrConvergence) || len(out.Solid.Faces()) != 0 {
		t.Fatal("radial sliver incorrectly ignored", out, e)
	}
}

func TestFinitePerpendicularCylinderPairRejectsRoundedCapPlacement(t *testing.T) {
	// The raw cap offset is a binary64 value, but the physical cap plane at
	// 1 + 0.1 cannot be represented exactly. Returning 1.1 would move it.
	a := geom.RawCylinder{Origin: geom.Vec3{X: 1}, Axis: geom.Vec3{X: 1}, Radius: 5, Min: -6, Max: .1}
	b := geom.RawCylinder{Origin: geom.Vec3{X: 1}, Axis: geom.Vec3{Z: 1}, Radius: 5, Min: -6, Max: 3}
	out, e := FiniteCylinderPairCommon(context.Background(), a, b, 1e-9)
	if !errors.Is(e, geom.ErrUnrepresentable) || len(out.Solid.Faces()) != 0 {
		t.Fatal("cap placement silently rounded", out, e)
	}
}
