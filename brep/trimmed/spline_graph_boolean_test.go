package trimmed

import (
	"bytes"
	"context"
	"math"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
	"plainsolid/mesh"
)

func TestCombineSplineGraphSolids(t *testing.T) {
	ctx := context.Background()
	k := []float64{0, 0, 0, 1, 1, 1}
	build := func(kind int) *geom.BSplineSurface {
		t.Helper()
		var p []geom.Vec3
		for i := range 3 {
			for j := range 3 {
				// A common curved base, B at height base+1 and A at
				// height base+.75+x². Their crossing is exactly x=.5.
				z := 0.
				if j == 2 {
					z = .25
				}
				switch kind {
				case 1:
					z += .75
					if i == 2 {
						z++
					}
				case 2:
					z++
				case 3:
					z += 2
				}
				p = append(p, geom.Vec3{X: float64(i) / 2, Y: float64(j) / 2, Z: z})
			}
		}
		s, e := geom.NewBSplineSurface(2, 2, 3, 3, p, k, k, nil)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	base, a, b := build(0), build(1), build(2)
	for _, tc := range []struct {
		op    parametric.BooleanOp
		v, x  float64
		walls int
	}{
		{parametric.Union, 7. / 6, (1./2 + 9./64) / (7. / 6), 6},
		{parametric.Intersection, 11. / 12, (1./2 - 1./64) / (11. / 12), 6},
		{parametric.Difference, 1. / 6, 27. / 32, 3},
	} {
		s, h, e := CombineSplineGraphSolids(ctx, base, a, b, tc.op, 1e-8, intersect.SplinePlaneOptions{})
		if e != nil || len(h.WallFaces) != tc.walls || len(h.ContourEdges) == 0 || s.Topology().EulerCharacteristic != 2 ||
			s.faces[h.TopA].Surface != a || s.faces[h.TopB].Surface != b {
			t.Fatal(tc.op, h, s.Topology(), e)
		}
		var native bytes.Buffer
		if e = WriteJSON(&native, s); e != nil {
			t.Fatal(e)
		}
		s, e = ReadJSON(&native, JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		proof, e := s.ValidateEdgeAgreement(ctx, edgecheck.Options{})
		if e != nil {
			t.Fatal(e)
		}
		section := make(map[int]bool)
		for _, id := range h.ContourEdges {
			section[id] = true
		}
		uses := 0
		for _, u := range proof.Uses {
			if section[u.Edge] {
				if u.Report.Status != edgecheck.Certified || u.Report.SquaredUpperBound.Sign() != 0 {
					t.Fatal(u)
				}
				uses++
			}
		}
		if uses != 2*len(section) {
			t.Fatal(uses, h)
		}
		p, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-7})
		if e != nil || math.Abs(p.Volume-tc.v) > 1e-8 || math.Abs(p.Centroid.X-tc.x) > 1e-8 ||
			math.Abs(p.Centroid.Y-.5) > 1e-8 {
			t.Fatal(tc, p, e)
		}
		m, e := s.Tessellate(ctx, parametric.TessellationOptions{Deflection: .025})
		if e != nil || m.ValidateSolid() != nil || math.Abs(m.SignedVolume()-tc.v) > .015 {
			t.Fatal(tc, m.SignedVolume(), e)
		}
	}
	if s, _, e := CombineSplineGraphSolids(ctx, base, a, a, parametric.Difference, 1e-8, intersect.SplinePlaneOptions{}); e != nil || len(s.faces) != 0 {
		t.Fatal(s.Topology(), e)
	}
	if s, h, e := CombineSplineGraphSolids(ctx, base, b, build(3), parametric.Union, 1e-8, intersect.SplinePlaneOptions{}); e != nil ||
		h.TopA != -1 || h.TopB != 0 || s.Topology().EulerCharacteristic != 2 {
		t.Fatal(h, s.Topology(), e)
	}
	if _, _, e := CombineSplineGraphSolids(ctx, base, base, b, parametric.Union, 1e-8, intersect.SplinePlaneOptions{}); e == nil {
		t.Fatal("accepted a top touching the common base")
	}
}

func TestCombineSplineGraphClosedContour(t *testing.T) {
	ctx := context.Background()
	const rr = .1171875
	k := []float64{0, 0, 0, 1, 1, 1}
	q := [3]float64{.25, -.25, .25}
	build := func(kind int) *geom.BSplineSurface {
		var p []geom.Vec3
		for i := range 3 {
			for j := range 3 {
				z := .25 * q[i]
				switch kind {
				case 1:
					z += 1 + q[i] + q[j] - rr
				case 2:
					z++
				}
				p = append(p, geom.Vec3{X: float64(i) / 2, Y: float64(j) / 2, Z: z})
			}
		}
		s, e := geom.NewBSplineSurface(2, 2, 3, 3, p, k, k, nil)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	base, a, b := build(0), build(1), build(2)
	bowl := math.Pi * rr * rr / 2
	for _, tc := range []struct {
		a, b  *geom.BSplineSurface
		op    parametric.BooleanOp
		v     float64
		euler int
	}{
		{a, b, parametric.Union, 1 + 1./6 - rr + bowl, 2},
		{a, b, parametric.Intersection, 1 - bowl, 2},
		{a, b, parametric.Difference, 1./6 - rr + bowl, 0},
		{b, a, parametric.Difference, bowl, 2},
	} {
		s, h, e := CombineSplineGraphSolids(ctx, base, tc.a, tc.b, tc.op, 1e-8, intersect.SplinePlaneOptions{})
		if e != nil || len(h.ContourEdges) == 0 || s.Topology().EulerCharacteristic != tc.euler {
			t.Fatal(tc.op, h, s.Topology(), e)
		}
		p, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-6})
		if e != nil || math.Abs(p.Volume-tc.v) > 1e-8 || math.Abs(p.Centroid.X-.5) > 1e-8 || math.Abs(p.Centroid.Y-.5) > 1e-8 {
			t.Fatal(tc, p, e)
		}
	}
}

func TestCombineSplineGraphNestedContours(t *testing.T) {
	ctx := context.Background()
	k := []float64{0, 0, 0, 0, 0, 1, 1, 1, 1, 1}
	lo, hi := .015625, .09375
	x2 := [5]float64{.25, 0, -1.0 / 12, 0, .25}
	x4 := [5]float64{.0625, -.0625, .0625, -.0625, .0625}
	build := func(kind int) *geom.BSplineSurface {
		var p []geom.Vec3
		for i := range 5 {
			for j := range 5 {
				z := 0.
				if kind == 1 {
					z = 1 + x4[i] + x4[j] + 2*x2[i]*x2[j] - (lo+hi)*(x2[i]+x2[j]) + lo*hi
				} else if kind == 2 {
					z = 1
				}
				p = append(p, geom.Vec3{X: float64(i) / 4, Y: float64(j) / 4, Z: z})
			}
		}
		s, e := geom.NewBSplineSurface(4, 4, 5, 5, p, k, k, nil)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	base, a, b := build(0), build(1), build(2)
	opts := intersect.SplinePlaneOptions{Trace: geom.SurfaceTraceOptions{Arcs: geom.SurfacePlaneArcOptions{
		MaxCellParameterWidth: .0625, MaxOperations: 8000000, ResolveSubdivisionCorners: true,
	}}}
	negative := math.Pi * math.Pow(hi-lo, 3) / 6
	squareIntegral := 7./180 - (lo+hi)/6 + lo*hi
	for _, tc := range []struct {
		a, b    *geom.BSplineSurface
		op      parametric.BooleanOp
		v       float64
		euler   int
		nShells int
	}{
		{a, b, parametric.Union, 1 + squareIntegral + negative, 2, 1},
		{a, b, parametric.Intersection, 1 - negative, 2, 1},
		{a, b, parametric.Difference, squareIntegral + negative, 2, 2},
		{b, a, parametric.Difference, negative, 0, 1},
	} {
		s, h, e := CombineSplineGraphSolids(ctx, base, tc.a, tc.b, tc.op, 1e-7, opts)
		if e != nil || len(h.ContourEdges) == 0 || s.Topology().EulerCharacteristic != tc.euler ||
			len(s.Topology().Shells) != tc.nShells {
			t.Fatal(tc.op, h, s.Topology(), e)
		}
		p, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-4})
		if e != nil || math.Abs(p.Volume-tc.v) > 1e-7 || math.Abs(p.Centroid.X-.5) > 1e-7 || math.Abs(p.Centroid.Y-.5) > 1e-7 {
			t.Fatal(tc, p, e)
		}
	}
}

func TestGraphBooleanIsolatedTangency(t *testing.T) {
	ctx := context.Background()
	k := []float64{0, 0, 0, 1, 1, 1}
	q := [3]float64{.25, -.25, .25}
	makeSpline := func(height float64, bowl bool) *geom.BSplineSurface {
		var p []geom.Vec3
		for i := range 3 {
			for j := range 3 {
				z := height
				if bowl {
					z += q[i] + q[j]
				}
				p = append(p, geom.Vec3{X: float64(i) / 2, Y: float64(j) / 2, Z: z})
			}
		}
		s, e := geom.NewBSplineSurface(2, 2, 3, 3, p, k, k, nil)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	base, upper, lower := makeSpline(0, false), makeSpline(1, true), makeSpline(1, false)
	o := intersect.SplinePlaneOptions{Trace: geom.SurfaceTraceOptions{Arcs: geom.SurfacePlaneArcOptions{IsolateQuadraticTangencies: true}}}
	s, h, e := CombineSplineGraphSolids(ctx, base, upper, lower, parametric.Union, 1e-8, o)
	if e != nil || h.TopA == -1 || h.TopB != -1 || s.Topology().EulerCharacteristic != 2 {
		t.Fatal(h, s.Topology(), e)
	}
	s, h, e = CombineSplineGraphSolids(ctx, base, upper, lower, parametric.Intersection, 1e-8, o)
	if e != nil || h.TopA != -1 || h.TopB == -1 || s.Topology().EulerCharacteristic != 2 {
		t.Fatal(h, s.Topology(), e)
	}
	s, _, e = CombineSplineGraphSolids(ctx, base, lower, upper, parametric.Difference, 1e-8, o)
	if e != nil || len(s.faces) != 0 {
		t.Fatal(s.Topology(), e)
	}
	if _, _, e = CombineSplineGraphSolids(ctx, base, upper, lower, parametric.Difference, 1e-8, o); e == nil {
		t.Fatal("accepted a nonmanifold tangent difference")
	}
}

func TestCombineSplineGraphDisjointOpenContours(t *testing.T) {
	ctx := context.Background()
	k := []float64{0, 0, 0, 1, 1, 1}
	// A-B = (x-.25)(x-.75). The two transverse open sections
	// cut off two disjoint positive regions and one negative strip.
	build := func(kind int) *geom.BSplineSurface {
		var p []geom.Vec3
		for i, q := range [3]float64{.1875, -.3125, .1875} {
			for j := range 3 {
				z := 0.
				if kind > 0 {
					z = 1
				}
				if kind == 1 {
					z += q
				}
				p = append(p, geom.Vec3{X: float64(i) / 2, Y: float64(j) / 2, Z: z})
			}
		}
		s, e := geom.NewBSplineSurface(2, 2, 3, 3, p, k, k, nil)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	o := intersect.SplinePlaneOptions{Trace: geom.SurfaceTraceOptions{Arcs: geom.SurfacePlaneArcOptions{MaxCellParameterWidth: .125, ResolveSubdivisionCorners: true}}}
	base, a, b := build(0), build(1), build(2)
	for _, x := range []struct {
		op    parametric.BooleanOp
		v     float64
		shell int
		fa    int
		fb    int
	}{{parametric.Union, 1 + 1./24, 1, 2, 1}, {parametric.Intersection, 1 - 1./48, 1, 1, 2},
		{parametric.Difference, 1. / 24, 2, 2, 2}} {
		s, h, e := CombineSplineGraphSolids(ctx, base, a, b, x.op, 1e-8, o)
		if e != nil || len(h.FacesA) != x.fa || len(h.FacesB) != x.fb || len(s.Topology().Shells) != x.shell {
			t.Fatal(x, h, s.Topology(), e)
		}
		p, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-6})
		if e != nil || math.Abs(p.Volume-x.v) > 1e-8 || math.Abs(p.Centroid.X-.5) > 1e-8 {
			t.Fatal(x, p, e)
		}
	}
}

func TestCombineSplineGraphMixedContours(t *testing.T) {
	ctx := context.Background()
	k := []float64{0, 0, 0, 0, 0, 1, 1, 1, 1, 1}
	// Piecewise quadratic graph: a linear crossing in the left span
	// and a circular depression well inside the right span. The shared
	// knot has an exactly equal boundary row but no section contact.
	k = []float64{0, 0, 0, .5, .5, 1, 1, 1}
	ky := []float64{0, 0, 0, 1, 1, 1}
	const radius2 = .0087890625
	build := func(kind int) *geom.BSplineSurface {
		var poles []geom.Vec3
		for i, x := range []float64{0, .25, .5, .75, 1} {
			for j, y := range []float64{0, .5, 1} {
				z := 0.
				if kind > 0 {
					z = 2
				}
				if kind == 1 {
					// On the right: f=(x-.75)^2+(y-.5)^2-r².
					// On the left, match its boundary trace and subtract
					// a large linear factor toward x=0.
					q := [3]float64{.25, -.25, .25}[j]
					x2 := [5]float64{.0625 - .5, .0625 - .25, .0625, -.0625, .0625}[i]
					z += x2 + q - radius2
				}
				poles = append(poles, geom.Vec3{X: x, Y: y, Z: z})
			}
		}
		s, e := geom.NewBSplineSurface(2, 2, 5, 3, poles, k, ky, nil)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	base, a, b := build(0), build(1), build(2)
	o := intersect.SplinePlaneOptions{Trace: geom.SurfaceTraceOptions{Arcs: geom.SurfacePlaneArcOptions{
		MaxCellParameterWidth: .0625, MaxOperations: 8000000, ResolveSubdivisionCorners: true,
	}}}
	// Left half: f=x-c(y), c=.5-(.0625+(y-.5)^2-r²).
	// Right half: integrate the paraboloid and remove its circular dip.
	c0 := .5 - .0625 + radius2
	negativeLeft := -(c0*c0 - c0/6 + 1./80) / 2
	positiveLeft := ((.5-c0)*(.5-c0) + (.5-c0)/6 + 1./80) / 2
	fullRight := 1./96 + 1./24 - radius2/2
	dip := math.Pi * radius2 * radius2 / 2
	positive := positiveLeft + fullRight + dip
	negative := negativeLeft - dip
	for _, tc := range []struct {
		op    parametric.BooleanOp
		v     float64
		euler int
	}{
		{parametric.Union, 2 + positive, 2},
		{parametric.Intersection, 2 + negative, 2},
		{parametric.Difference, positive, 0},
	} {
		s, h, e := CombineSplineGraphSolids(ctx, base, a, b, tc.op, 1e-7, o)
		if e != nil || s.Topology().EulerCharacteristic != tc.euler || len(h.ContourEdges) == 0 {
			t.Fatal(tc, h, s.Topology(), e)
		}
		var m mesh.Mesh
		if tc.op == parametric.Difference {
			// Adaptive deflection for this pinched and knotted face can
			// exceed the current subdivision budget. The seed is closed.
			m, e = s.refine(ctx, s.minimumLevel, 0, 0)
		} else {
			m, e = s.Tessellate(ctx, parametric.TessellationOptions{Deflection: .025})
		}
		if e != nil || m.ValidateSolid() != nil || math.Abs(m.SignedVolume()-tc.v) > .02 {
			t.Fatal(tc, m.SignedVolume(), e)
		}
	}
}
