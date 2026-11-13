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

func TestOCCTFiniteCylinderAxialCut(t *testing.T) {
	refs := runOracle(t, "finite-cylinder-axial-cut")
	ctx := context.Background()
	a := geom.RawCylinder{Origin: geom.Vec3{X: 1, Y: -2, Z: 5}, Axis: geom.Vec3{Z: -3}, Radius: 2, Min: 0, Max: 3}
	b := geom.RawCylinder{Origin: geom.Vec3{X: 1.5, Y: -2}, Axis: geom.Vec3{Z: 2}, Radius: 3, Min: -.5, Max: .5}
	r, err := parametric.FiniteCylinderPairDifference(ctx, a, b, 1e-9)
	if err != nil || r.MinimumGapSquared == nil || r.MinimumGapSquared.RatString() != "4" {
		t.Fatal("axial split", err)
	}
	for i, name := range []string{"finite-axial-cut-first", "finite-axial-cut-second"} {
		s := r.Solids[i]
		if finiteOracleShellCount(s) != 1 {
			t.Fatal(name, "expected one shell")
		}
		p, err := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
		if err != nil {
			t.Fatal(name, err)
		}
		compare(t, refs[name], p.Area, p.Volume, p.Centroid)
	}
}

func TestOCCTFiniteObliqueParallelCylinderSets(t *testing.T) {
	refs := runOracle(t, "finite-oblique-parallel-sets")
	ctx := context.Background()
	a := geom.RawCylinder{Axis: geom.Vec3{X: 3, Y: 4}, Radius: 5, Min: 0, Max: 2}
	b := geom.RawCylinder{Origin: geom.Vec3{X: 6, Y: 8, Z: 5}, Axis: geom.Vec3{X: -6, Y: -8}, Radius: 5, Min: 0, Max: 1}
	u, err := parametric.FiniteCylinderPairUnion(ctx, a, b, 1e-9)
	if err != nil {
		t.Fatal(err)
	}
	d, err := parametric.FiniteCylinderPairDifference(ctx, a, b, 1e-9)
	if err != nil || len(d.Solids[1].Faces()) != 0 {
		t.Fatal("one difference component", err)
	}
	for i, name := range []string{"finite-oblique-parallel-union", "finite-oblique-parallel-cut"} {
		s := [2]parametric.Solid{u.Solid, d.Solids[0]}[i]
		if finiteOracleShellCount(s) != 1 {
			t.Fatal(name, "expected one shell")
		}
		p, err := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
		if err != nil {
			t.Fatal(name, err)
		}
		compare(t, refs[name], p.Area, p.Volume, p.Centroid)
	}
}

func TestOCCTFiniteEccentricCylinderShoulder(t *testing.T) {
	refs := runOracle(t, "finite-eccentric-shoulder-union")
	ctx := context.Background()
	a := geom.RawCylinder{Origin: geom.Vec3{X: 2, Y: 1, Z: 3}, Axis: geom.Vec3{Z: -2}, Radius: 4, Min: 0, Max: 2}
	for i, name := range []string{"finite-eccentric-shoulder-overlap", "finite-eccentric-shoulder-touch", "finite-eccentric-shoulder-both"} {
		b := geom.RawCylinder{Origin: geom.Vec3{X: 3, Y: 1}, Axis: geom.Vec3{Z: 2}, Radius: 2, Min: -1.5, Max: [3]float64{.5, -.5, 2.5}[i]}
		r, err := parametric.FiniteCylinderPairUnion(ctx, a, b, 1e-9)
		if err != nil || finiteOracleShellCount(r.Solid) != 1 {
			t.Fatal(name, "expected one connected shell", err)
		}
		p, err := r.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
		if err != nil {
			t.Fatal(name, err)
		}
		compare(t, refs[name], p.Area, p.Volume, p.Centroid)
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
	cutter.Origin.X++
	cutter.Origin.Y += .5
	out, err = parametric.FiniteCylinderPairDifference(ctx, stock, cutter, 1e-8)
	if err != nil || len(out.Solids[1].Faces()) != 0 || finiteOracleShellCount(out.Solids[0]) != 1 {
		t.Fatal("expected one closed eccentric bore", err)
	}
	mass, err = out.Solids[0].MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
	if err != nil {
		t.Fatal(err)
	}
	compare(t, refs["finite-eccentric-cut"], mass.Area, mass.Volume, mass.Centroid)
}

func TestOCCTUnequalFiniteCylinderAsymmetricCaps(t *testing.T) {
	refs := runOracle(t, "finite-cylinder-sets")
	ctx := context.Background()
	stock := geom.RawCylinder{Origin: geom.Vec3{X: 1, Y: 2, Z: 5}, Axis: geom.Vec3{Z: -2}, Radius: 5, Min: -1, Max: 4.5}
	cutter := geom.RawCylinder{Origin: geom.Vec3{X: 1, Y: 4, Z: 3}, Axis: geom.Vec3{Y: 2}, Radius: 3, Min: -5, Max: 2}
	check := func(name string, s parametric.Solid) {
		t.Helper()
		if finiteOracleShellCount(s) != 1 {
			t.Fatal(name, "expected one shell")
		}
		mass, err := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
		if err != nil {
			t.Fatal(name, err)
		}
		compare(t, refs[name], mass.Area, mass.Volume, mass.Centroid)
	}
	for _, pair := range [][2]geom.RawCylinder{{stock, cutter}, {cutter, stock}} {
		u, err := parametric.FiniteCylinderPairUnion(ctx, pair[0], pair[1], 1e-8)
		if err != nil {
			t.Fatal(err)
		}
		check("finite-unequal-offset-union", u.Solid)
	}
	d, err := parametric.FiniteCylinderPairDifference(ctx, stock, cutter, 1e-8)
	if err != nil || len(d.Solids[1].Faces()) != 0 {
		t.Fatal("expected one retained stock", err)
	}
	check("finite-unequal-offset-cut", d.Solids[0])
}

func TestOCCTFiniteParallelCylinderCavity(t *testing.T) {
	refs := runOracle(t, "finite-cylinder-sets")
	ctx := context.Background()
	stock := geom.RawCylinder{Origin: geom.Vec3{X: 1, Y: 2, Z: 3}, Axis: geom.Vec3{Z: -2}, Radius: 5, Min: -2, Max: 2}
	cutter := geom.RawCylinder{Origin: geom.Vec3{X: 2, Y: 2.5, Z: 2}, Axis: geom.Vec3{Z: -1}, Radius: 2, Min: 0, Max: 2}
	d, err := parametric.FiniteCylinderPairDifference(ctx, stock, cutter, 1e-9)
	if err != nil || len(d.Solids[1].Faces()) != 0 || finiteOracleShellCount(d.Solids[0]) != 2 {
		t.Fatal("expected one stock with two closed shells", err)
	}
	m, err := d.Solids[0].MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
	if err != nil {
		t.Fatal(err)
	}
	compare(t, refs["finite-parallel-cavity"], m.Area, m.Volume, m.Centroid)
}

func TestOCCTFiniteCoaxialCylinderUnion(t *testing.T) {
	refs := runOracle(t, "finite-cylinder-sets")
	ctx := context.Background()
	a := geom.RawCylinder{Origin: geom.Vec3{X: 1, Y: 2, Z: 3}, Axis: geom.Vec3{Z: -2}, Radius: 2, Min: -1, Max: 2}
	for i, name := range []string{"finite-coaxial-overlap-union", "finite-coaxial-touch-union"} {
		b := geom.RawCylinder{Origin: geom.Vec3{X: 1, Y: 2, Z: 5}, Axis: geom.Vec3{Z: 3}, Radius: 2, Min: float64(i - 1), Max: 2}
		u, err := parametric.FiniteCylinderPairUnion(ctx, a, b, 1e-9)
		if err != nil || finiteOracleShellCount(u.Solid) != 1 {
			t.Fatal(name, err)
		}
		m, err := u.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
		if err != nil {
			t.Fatal(name, err)
		}
		compare(t, refs[name], m.Area, m.Volume, m.Centroid)
	}
}

func TestOCCTFiniteParallelCylinderBlindBore(t *testing.T) {
	refs := runOracle(t, "finite-cylinder-sets")
	ctx := context.Background()
	stock := geom.RawCylinder{Origin: geom.Vec3{X: 1, Y: 2, Z: 3}, Axis: geom.Vec3{Z: -2}, Radius: 5, Min: -2, Max: 2}
	for _, tc := range []struct {
		name string
		tool geom.RawCylinder
	}{
		{"finite-blind-lower", geom.RawCylinder{Origin: geom.Vec3{X: 2, Y: 2, Z: 3}, Axis: geom.Vec3{Z: 2}, Radius: 2, Min: 0, Max: 3}},
		{"finite-blind-upper", geom.RawCylinder{Origin: geom.Vec3{X: 2, Y: 2, Z: 3}, Axis: geom.Vec3{Z: 2}, Radius: 2, Min: -2, Max: 0}},
	} {
		d, err := parametric.FiniteCylinderPairDifference(ctx, stock, tc.tool, 1e-9)
		if err != nil || len(d.Solids[1].Faces()) != 0 || finiteOracleShellCount(d.Solids[0]) != 1 {
			t.Fatal(tc.name, err)
		}
		m, err := d.Solids[0].MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
		if err != nil {
			t.Fatal(tc.name, err)
		}
		compare(t, refs[tc.name], m.Area, m.Volume, m.Centroid)
	}
}

func TestOCCTFiniteCoaxialSteppedCylinderUnion(t *testing.T) {
	refs := runOracle(t, "finite-cylinder-sets")
	ctx := context.Background()
	a := geom.RawCylinder{Origin: geom.Vec3{X: 1, Y: 2, Z: 3}, Axis: geom.Vec3{Z: -2}, Radius: 5, Min: -2, Max: 2}
	for i, name := range []string{"finite-coaxial-stepped-overlap", "finite-coaxial-stepped-touch"} {
		b := geom.RawCylinder{Origin: geom.Vec3{X: 1, Y: 2, Z: float64(5 + 2*i)}, Axis: geom.Vec3{Z: 2}, Radius: 2, Min: 0, Max: float64(3 - i)}
		u, err := parametric.FiniteCylinderPairUnion(ctx, a, b, 1e-9)
		if err != nil || finiteOracleShellCount(u.Solid) != 1 {
			t.Fatal(name, err)
		}
		m, err := u.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
		if err != nil {
			t.Fatal(name, err)
		}
		compare(t, refs[name], m.Area, m.Volume, m.Centroid)
	}
}

func TestOCCTFiniteParallelTransverseCylinderSets(t *testing.T) {
	refs := runOracle(t, "finite-cylinder-sets")
	ctx := context.Background()
	a := geom.RawCylinder{Origin: geom.Vec3{X: 1, Y: 2, Z: 3}, Axis: geom.Vec3{Z: -2}, Radius: 5, Min: -2, Max: 2}
	b := geom.RawCylinder{Origin: geom.Vec3{X: 5, Y: 2, Z: 3}, Axis: geom.Vec3{Z: 4}, Radius: 3, Min: -1, Max: 1}
	check := func(name string, s parametric.Solid) {
		t.Helper()
		if finiteOracleShellCount(s) != 1 {
			t.Fatal(name, "expected one shell")
		}
		m, err := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
		if err != nil {
			t.Fatal(name, err)
		}
		compare(t, refs[name], m.Area, m.Volume, m.Centroid)
	}
	for _, pair := range [][2]geom.RawCylinder{{a, b}, {b, a}} {
		u, err := parametric.FiniteCylinderPairUnion(ctx, pair[0], pair[1], 1e-9)
		if err != nil {
			t.Fatal(err)
		}
		check("finite-parallel-transverse-union", u.Solid)
	}
	d, err := parametric.FiniteCylinderPairDifference(ctx, a, b, 1e-9)
	if err != nil || len(d.Solids[1].Faces()) != 0 {
		t.Fatal("expected one retained stock", err)
	}
	check("finite-parallel-transverse-cut", d.Solids[0])
	b.Axis, b.Min, b.Max = geom.Vec3{Z: -4}, -2, 2
	d, err = parametric.FiniteCylinderPairDifference(ctx, a, b, 1e-9)
	refName := "finite-parallel-transverse-covering-cut"
	if _, ok := refs[refName]; !ok || err != nil || len(d.Solids[1].Faces()) != 0 ||
		len(d.StockCapFaces[0]) != 2 || len(d.CutterCapFaces[0]) != 0 || len(d.CutterWallFaces[0]) != 1 {
		t.Fatal("expected one retained stock with original stock caps and a cutter wall", err)
	}
	check(refName, d.Solids[0])
}

func TestOCCTFiniteTransverseBlindCylinderCut(t *testing.T) {
	refs := runOracle(t, "finite-cylinder-sets")
	ctx := context.Background()
	a := geom.RawCylinder{Axis: geom.Vec3{Z: 2}, Min: 0, Max: 4, Radius: 5}
	for _, tc := range []struct {
		name     string
		min, max float64
		faces    int
		caps     int
	}{
		{"finite-transverse-upper-notch", -5, -1.5, 10, 2},
		{"finite-transverse-lower-notch", -2.5, 1, 10, 2},
		{"finite-transverse-interior-notch", -3, -1, 16, 4},
	} {
		b := geom.RawCylinder{Origin: geom.Vec3{X: 4}, Axis: geom.Vec3{Z: -2}, Min: tc.min, Max: tc.max, Radius: 3}
		r, err := parametric.FiniteCylinderPairDifference(ctx, a, b, 1e-8)
		if err != nil || finiteOracleShellCount(r.Solids[0]) != 1 || len(r.Solids[0].Faces()) != tc.faces ||
			len(r.CutterWallFaces[0]) != 1 || len(r.CutterCapFaces[0]) != tc.caps {
			t.Fatal(tc.name, "unexpected analytic shell or original floor ownership", err)
		}
		m, err := r.Solids[0].MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
		if err != nil {
			t.Fatal(tc.name, err)
		}
		compare(t, refs[tc.name], m.Area, m.Volume, m.Centroid)
	}
}

func TestOCCTFiniteTransverseShoulderCylinderUnions(t *testing.T) {
	refs := runOracle(t, "finite-cylinder-sets")
	ctx := context.Background()
	a := geom.RawCylinder{Axis: geom.Vec3{Z: 2}, Min: 0, Max: 4, Radius: 5}
	for _, tc := range []struct {
		name     string
		min, max float64
		faces    int
	}{
		{"finite-transverse-common-lower-union", -2.5, 0, 12},
		{"finite-transverse-common-upper-union", -4, -1.5, 12},
		{"finite-transverse-staggered-union", -5, -1, 14},
		{"finite-transverse-nested-union", -2.5, -.5, 14},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := geom.RawCylinder{Origin: geom.Vec3{X: 4}, Axis: geom.Vec3{Z: -2}, Min: tc.min, Max: tc.max, Radius: 3}
			r, err := parametric.FiniteCylinderPairUnion(ctx, a, b, 1e-8)
			if err != nil || len(r.Solid.Faces()) != tc.faces || finiteOracleShellCount(r.Solid) != 1 {
				t.Fatal("unexpected analytic shell", err)
			}
			m, err := r.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
			if err != nil {
				t.Fatal(err)
			}
			compare(t, refs[tc.name], m.Area, m.Volume, m.Centroid)
		})
	}
}
