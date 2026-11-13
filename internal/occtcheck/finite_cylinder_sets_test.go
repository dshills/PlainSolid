package occtcheck

import (
	"context"
	"errors"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestOCCTFiniteRawCylinderSets(t *testing.T) {
	refs := runOracle(t, "finite-cylinder-sets")
	ctx := context.Background()
	a := geom.RawCylinder{Origin: geom.Vec3{X: 7, Y: 1, Z: 2}, Axis: geom.Vec3{X: -3, Y: -4}, Radius: 2, Min: -2, Max: 4}
	b := geom.RawCylinder{Origin: geom.Vec3{Z: 2}, Axis: geom.Vec3{X: 4, Y: -3}, Radius: 2, Min: -2, Max: 4}
	union, e := parametric.FiniteCylinderPairUnion(ctx, a, b, 1e-8)
	if e != nil {
		t.Fatal(e)
	}
	p, e := union.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
	if e != nil {
		t.Fatal(e)
	}
	compare(t, refs["finite-raw-union"], p.Area, p.Volume, p.Centroid)
	for _, wide := range []bool{false, true} {
		prefix := "finite-raw-"
		if wide {
			b.Radius = 3
			prefix = "finite-raw-wide-"
		}
		difference, e := parametric.FiniteCylinderPairDifference(ctx, a, b, 1e-8)
		if e != nil {
			t.Fatal(e)
		}
		if wide == (difference.MinimumGapSquared == nil) || wide == (difference.ContactNodes != nil) {
			t.Fatal("component contact semantics")
		}
		var area, volume float64
		var moment geom.Vec3
		for i, s := range difference.Solids {
			mass, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
			if e != nil {
				t.Fatal(e)
			}
			suffix := "minus"
			if i == 1 {
				suffix = "plus"
			}
			compare(t, refs[prefix+suffix], mass.Area, mass.Volume, mass.Centroid)
			area += mass.Area
			volume += mass.Volume
			moment = moment.Add(mass.Centroid.Scale(mass.Volume))
		}
		compare(t, refs[prefix+"cut"], area, volume, moment.Scale(1/volume))
	}
}

// Native rectangular-patch topology has no public shell count. Count connected
// face components through shared edge identity, independently of mesh sampling.
func finiteOracleShellCount(s parametric.Solid) int {
	faces := s.Faces()
	parent := make([]int, len(faces))
	for i := range parent {
		parent[i] = i
	}
	var root func(int) int
	root = func(i int) int {
		if parent[i] != i {
			parent[i] = root(parent[i])
		}
		return parent[i]
	}
	owners := map[int]int{}
	for i, f := range faces {
		for _, use := range f.Boundary {
			if j, ok := owners[use.Edge]; ok {
				parent[root(i)] = root(j)
			} else {
				owners[use.Edge] = i
			}
		}
	}
	count := 0
	for i := range faces {
		if root(i) == i {
			count++
		}
	}
	return count
}
func TestOCCTSeparatedFiniteCylinderUnion(t *testing.T) {
	refs := runOracle(t, "finite-cylinder-sets")
	ctx := context.Background()
	a := geom.RawCylinder{Axis: geom.Vec3{Z: 2}, Radius: 2, Min: -1, Max: 1}
	for _, tc := range []struct {
		name   string
		origin geom.Vec3
	}{{"finite-separated-radial", geom.Vec3{X: 4}}, {"finite-separated-axial", geom.Vec3{Z: 8}}} {
		b := geom.RawCylinder{Origin: tc.origin, Axis: geom.Vec3{Z: -3}, Radius: 1, Min: -1, Max: 1}
		u, e := parametric.FiniteCylinderPairUnion(ctx, a, b, 1e-9)
		if e != nil || finiteOracleShellCount(u.Solid) != 2 {
			t.Fatal("two independent native solids", e)
		}
		p, e := u.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
		if e != nil {
			t.Fatal(e)
		}
		compare(t, refs[tc.name], p.Area, p.Volume, p.Centroid)
	}
	// This is the native manifold policy, deliberately not an OCCT equivalence
	// assertion: a radial tangency has no strictly separated closed components.
	b := geom.RawCylinder{Origin: geom.Vec3{X: 3}, Axis: geom.Vec3{Z: -3}, Radius: 1, Min: -1, Max: 1}
	bad, e := parametric.FiniteCylinderPairUnion(ctx, a, b, 1e-9)
	if !errors.Is(e, geom.ErrDomain) || len(bad.Solid.Faces()) != 0 {
		t.Fatal("tangent policy", e)
	}
}

func TestOCCTUnequalFiniteCylinderUnion(t *testing.T) {
	refs := runOracle(t, "finite-cylinder-sets")
	ctx := context.Background()
	center := geom.Vec3{X: 1, Y: 2, Z: 3}
	a := geom.RawCylinder{Origin: center, Axis: geom.Vec3{Z: -2}, Radius: 5, Min: -2, Max: 2}
	b := geom.RawCylinder{Origin: center, Axis: geom.Vec3{Y: 3}, Radius: 3, Min: -2, Max: 2}
	for _, pair := range [][2]geom.RawCylinder{{a, b}, {b, a}} {
		u, e := parametric.FiniteCylinderPairUnion(ctx, pair[0], pair[1], 1e-8)
		if e != nil || finiteOracleShellCount(u.Solid) != 1 {
			t.Fatal("single closed union", e)
		}
		mass, e := u.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
		if e != nil {
			t.Fatal(e)
		}
		compare(t, refs["finite-unequal-union"], mass.Area, mass.Volume, mass.Centroid)
	}
}

func TestOCCTUnequalFiniteCylinderDifference(t *testing.T) {
	refs := runOracle(t, "finite-cylinder-sets")
	ctx := context.Background()
	center := geom.Vec3{X: 1, Y: 2, Z: 3}
	stock := geom.RawCylinder{Origin: center, Axis: geom.Vec3{Z: -2}, Radius: 5, Min: -2, Max: 2}
	cutter := geom.RawCylinder{Origin: center, Axis: geom.Vec3{Y: 3}, Radius: 3, Min: -2, Max: 2}
	out, err := parametric.FiniteCylinderPairDifference(ctx, stock, cutter, 1e-8)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Solids[1].Faces()) != 0 || finiteOracleShellCount(out.Solids[0]) != 1 {
		t.Fatal("expected one closed bored stock")
	}
	s := out.Solids[0]
	mass, err := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
	if err != nil {
		t.Fatal(err)
	}
	compare(t, refs["finite-unequal-cut"], mass.Area, mass.Volume, mass.Centroid)
	m, err := s.Tessellate(ctx, parametric.TessellationOptions{Deflection: 1, AnalyticBound: true})
	if err != nil {
		t.Fatal(err)
	}
	if err = m.ValidateSolid(); err != nil {
		t.Fatal(err)
	}
	edges := map[[2]int]bool{}
	for _, triangle := range m.Triangles {
		for j, a := range triangle {
			b := triangle[(j+1)%3]
			if a > b {
				a, b = b, a
			}
			edges[[2]int{a, b}] = true
		}
	}
	if len(m.Vertices)-len(edges)+len(m.Triangles) != 0 {
		t.Fatal("expected genus-one boundary")
	}
}

func TestOCCTCoaxialFiniteCylinderThroughDifference(t *testing.T) {
	refs := runOracle(t, "finite-cylinder-sets")
	ctx := context.Background()
	stock := geom.RawCylinder{Origin: geom.Vec3{X: 1, Y: 2, Z: 3}, Axis: geom.Vec3{Z: -2}, Radius: 5, Min: -2, Max: 2}
	cutter := geom.RawCylinder{Origin: geom.Vec3{X: 1, Y: 2, Z: 5}, Axis: geom.Vec3{Z: 3}, Radius: 3, Min: -3, Max: 2}
	out, err := parametric.FiniteCylinderPairDifference(ctx, stock, cutter, 1e-8)
	if err != nil || len(out.Solids[1].Faces()) != 0 || finiteOracleShellCount(out.Solids[0]) != 1 {
		t.Fatal("expected one closed coaxial bore", err)
	}
	mass, err := out.Solids[0].MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
	if err != nil {
		t.Fatal(err)
	}
	compare(t, refs["finite-coaxial-cut"], mass.Area, mass.Volume, mass.Centroid)
}
