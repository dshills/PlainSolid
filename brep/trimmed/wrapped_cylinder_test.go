package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func makeCanonicalWrappedCylinder(t *testing.T, circular bool, perturb float64) *geom.WrappedSurfacePatch {
	t.Helper()
	q := [][2]float64{{1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1}, {0, -1}, {1, -1}, {1, 0}}
	weights := []float64{1, .75, 1, .75, 1, .75, 1, .75, 1}
	if circular {
		weights = []float64{1, 1, 2, 2, 4, 2, 2, 1, 1}
	}
	var poles []geom.Vec3
	var w []float64
	for i, xy := range q {
		for _, z := range []float64{-2, 3} {
			x := 2 * xy[0]
			if i == 3 && z == 3 {
				x += perturb
			}
			poles = append(poles, geom.Vec3{X: x, Y: 2 * xy[1], Z: z})
			w = append(w, weights[i])
		}
	}
	u := []float64{0, 0, 0, 1, 1, 2, 2, 3, 3, 4, 4, 4}
	v := []float64{-1, -1, 1, 1}
	s, e := geom.NewBSplineSurface(2, 1, 9, 2, poles, u, v, w)
	if e != nil {
		t.Fatal(e)
	}
	p, e := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	o, _, e := geom.NewWrappedSurfacePatch(context.Background(), p, geom.WrappedSurfaceOptions{
		WrapU: true, ContinuityOrder: 1,
	})
	if e != nil {
		t.Fatal(e)
	}
	return o
}

func TestCanonicalWrappedCylinderMaterial(t *testing.T) {
	ctx := context.Background()
	w := makeCanonicalWrappedCylinder(t, true, 0)
	s, proof, e := NewWrappedCylinder(ctx, w, 1e-9, geom.SurfacePatchOptions{})
	if e != nil || !proof.Complete || !proof.Circular {
		t.Fatal(proof, e)
	}
	top := s.Topology()
	if top.Vertices != 8 || top.Edges != 9 || top.Faces != 3 || top.EulerCharacteristic != 2 || len(top.Shells) != 1 {
		t.Fatal(top)
	}
	agreement, e := s.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if e != nil || !agreement.Complete || len(agreement.Uses) != 18 {
		t.Fatal(agreement, e)
	}
	p, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 2e-6})
	if e != nil || math.Abs(p.Volume-20*math.Pi) > 1e-5 || math.Abs(p.Area-28*math.Pi) > 1e-5 ||
		p.Centroid.Distance(geom.Vec3{Z: .5}) > 1e-6 {
		t.Fatal(p, e)
	}
	var native bytes.Buffer
	if e = WriteJSON(&native, s); e != nil {
		t.Fatal(e)
	}
	q, e := ReadJSONContext(ctx, &native, JSONLimits{})
	if e != nil || q.Topology().EulerCharacteristic != 2 {
		t.Fatal(q.Topology(), e)
	}
	agreement, e = q.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if e != nil || !agreement.Complete {
		t.Fatal(agreement, e)
	}
	if _, proof, e := q.ValidateCanonicalWrappedCylinder(ctx, geom.SurfacePatchOptions{}); e != nil || !proof.Complete {
		t.Fatal("cannot revalidate native material", proof, e)
	}
	location, e := q.LocateCanonicalWrappedCylinder(ctx, geom.Vec3{X: 2}, geom.SurfacePatchOptions{})
	if e != nil || location != parametric.Boundary {
		t.Fatal(location, e)
	}
	ray, e := q.IntersectCanonicalWrappedCylinderLine(ctx, geom.Vec3{}, geom.Vec3{Z: 3}, -1, 2, geom.ToroidLineOptions{ParameterTolerance: 1e-10})
	if e != nil || len(ray.Hits) != 2 || ray.Hits[0].Parameter.Min.Cmp(big.NewRat(-2, 3)) != 0 ||
		ray.Hits[1].Parts != geom.CylinderUpperCap {
		t.Fatal(ray, e)
	}
	sec, e := q.SectionCanonicalWrappedFrustum(ctx, 0, geom.SurfacePatchOptions{})
	if e != nil || sec.ParameterV.Cmp(big.NewRat(-1, 5)) != 0 {
		t.Fatal(sec, e)
	}
	distance, e := q.BoundCanonicalWrappedCylinderDistance(ctx, geom.Vec3{Z: 4}, geom.CurveDistanceOptions{SquaredTolerance: 1e-3})
	if e != nil || distance.Lower.Cmp(big.NewRat(1, 1)) != 0 || distance.Upper.Cmp(big.NewRat(1, 1)) != 0 ||
		distance.Part != geom.CylinderUpperCap {
		t.Fatal(distance, e)
	}
	n, p2, e := NewWrappedCylinder(ctx, makeCanonicalWrappedCylinder(t, false, 0), 1e-9, geom.SurfacePatchOptions{})
	if e != nil || !p2.Complete || p2.Circular || len(n.faces) != 3 {
		t.Fatal(p2, e)
	}
	if _, _, e = NewWrappedCylinder(ctx, makeCanonicalWrappedCylinder(t, false, .125), 1e-9, geom.SurfacePatchOptions{}); !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("non-extruded support accepted", e)
	}
}

func TestWrappedCylinderExactHorizontalSplit(t *testing.T) {
	ctx := context.Background()
	w := makeCanonicalWrappedCylinder(t, true, 0)
	a, b, h, e := SplitWrappedCylinderAtZ(ctx, w, .25, 1e-9, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(h, e)
	}
	for i, s := range []Solid{a, b} {
		p, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 2e-6})
		want := [2]float64{9 * math.Pi, 11 * math.Pi}[i]
		if e != nil || math.Abs(p.Volume-want) > 1e-5 || s.Topology().EulerCharacteristic != 2 {
			t.Fatal(p, want, e)
		}
	}
	for i := range 4 {
		x, _, e := exactEdgeSpline(a.edges[h.Lower.CutRim[i]])
		if e != nil {
			t.Fatal(e)
		}
		y, _, e := exactEdgeSpline(b.edges[h.Upper.CutRim[i]])
		if e != nil {
			t.Fatal(e)
		}
		p, e := geom.CompareExactCurvesContext(ctx, x, y, geom.ExactCurveComparisonOptions{})
		if e != nil || !p.Complete || !p.Equal {
			t.Fatal(p, e)
		}
	}
	if !b.faces[h.Upper.CutCap].Reversed || a.faces[h.Lower.CutCap].Reversed {
		t.Fatal("cut caps have inconsistent orientations")
	}
	if _, _, _, e = SplitWrappedCylinderAtZ(ctx, w, -2, 1e-9, geom.SurfacePatchOptions{}); !errors.Is(e, geom.ErrDomain) {
		t.Fatal("tangent cut returned two solids", e)
	}
}

func TestAxialWrappedCylinderBoolean(t *testing.T) {
	ctx := context.Background()
	w := makeCanonicalWrappedCylinder(t, true, 0)
	o := geom.SurfacePatchOptions{}
	a, _, e := NewWrappedCylinder(ctx, w, 1e-9, o)
	if e != nil {
		t.Fatal(e)
	}
	cylinder := func(lo, hi int64) Solid {
		t.Helper()
		x, e := geom.ReheightWrappedCylinder(ctx, w, big.NewRat(lo, 1), big.NewRat(hi, 1), o)
		if e != nil {
			t.Fatal(e)
		}
		s, _, e := NewWrappedCylinder(ctx, x, 1e-9, o)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	b := cylinder(0, 2)
	for _, c := range []struct {
		op     parametric.BooleanOp
		height float64
		shells int
	}{{parametric.Union, 5, 1}, {parametric.Intersection, 2, 1}, {parametric.Difference, 3, 2}} {
		out, h, e := CombineSplineShells(ctx, a, b, c.op, SplineShellCombineOptions{})
		if e != nil || h.Method != "axial-wrapped-cylinder" || len(out.Topology().Shells) != c.shells {
			t.Fatal(c, h, e)
		}
		p, e := out.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 2e-6})
		if e != nil || math.Abs(p.Volume-4*math.Pi*c.height) > 1e-5 {
			t.Fatal(c, p, e)
		}
		if c.op == parametric.Difference && (len(h.FacesA[0]) != 2 || len(h.EdgesA[8]) != 2 || len(h.FacesB[1]) != 1 || len(h.FacesB[2]) != 1) {
			t.Fatal("missing exact overlap ancestry", h)
		}
	}
	touch := cylinder(3, 4)
	p, _, e := CombineAxialWrappedCylinders(ctx, a, touch, parametric.Intersection, o)
	if e != nil || len(p.faces) != 0 {
		t.Fatal("face-only contact is not material", e)
	}
}

func TestConcentricWrappedCylinderTube(t *testing.T) {
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	w := makeCanonicalWrappedCylinder(t, true, 0)
	x, _, e := w.Transformed(ctx, geom.Affine{{.5, 0, 0, 0}, {0, .5, 0, 0}, {0, 0, 1, 0}}, geom.WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	a, _, e := NewWrappedCylinder(ctx, w, 1e-9, o)
	if e != nil {
		t.Fatal(e)
	}
	b, _, e := NewWrappedCylinder(ctx, x, 1e-9, o)
	if e != nil {
		t.Fatal(e)
	}
	s, h, e := CombineSplineShells(ctx, a, b, parametric.Difference, SplineShellCombineOptions{})
	if e != nil || h.Method != "concentric-wrapped-cylinder" || len(s.faces) != 4 || len(h.EdgesB[8]) != 1 {
		t.Fatal(h, e)
	}
	p, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 2e-6})
	if e != nil || math.Abs(p.Volume-15*math.Pi) > 1e-5 || math.Abs(p.Area-36*math.Pi) > 1e-5 {
		t.Fatal(p, e)
	}
	for _, c := range []struct {
		p    geom.Vec3
		want parametric.Location
	}{{geom.Vec3{X: .5, Z: 3}, parametric.Outside},
		{geom.Vec3{X: 1, Z: 3}, parametric.Boundary},
		{geom.Vec3{X: 1.5, Z: 3}, parametric.Boundary},
		{geom.Vec3{X: 1.5}, parametric.Inside},
		{geom.Vec3{}, parametric.Outside}} {
		loc, e := LocateWrappedCylinderTube(ctx, w, x, c.p, o)
		if e != nil || loc != c.want {
			t.Fatal(c, loc, e)
		}
	}
}

func TestWrappedCylinderBlindBoreAndCavity(t *testing.T) {
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	w := makeCanonicalWrappedCylinder(t, true, 0)
	x, _, e := w.Transformed(ctx, geom.Affine{{.5, 0, 0, 0}, {0, .5, 0, 0}, {0, 0, 1, 0}}, geom.WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	a, _, e := NewWrappedCylinder(ctx, w, 1e-9, o)
	if e != nil {
		t.Fatal(e)
	}
	for _, c := range []struct {
		lo, hi               int64
		shells, holes, euler int
		volume, area         float64
	}{{1, 3, 1, 1, 2, 18 * math.Pi, 32 * math.Pi},
		{-1, 1, 2, 0, 4, 18 * math.Pi, 34 * math.Pi}} {
		bs, e := geom.ReheightWrappedCylinder(ctx, x, big.NewRat(c.lo, 1), big.NewRat(c.hi, 1), o)
		if e != nil {
			t.Fatal(e)
		}
		b, _, e := NewWrappedCylinder(ctx, bs, 1e-9, o)
		if e != nil {
			t.Fatal(e)
		}
		s, h, e := CombineSplineShells(ctx, a, b, parametric.Difference, SplineShellCombineOptions{})
		topo := s.Topology()
		if e != nil || len(topo.Shells) != c.shells || topo.InnerWires != c.holes || topo.EulerCharacteristic != c.euler ||
			h.Method != "concentric-wrapped-cylinder" {
			t.Fatal(c, topo, h, e)
		}
		p, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 2e-6})
		if e != nil || math.Abs(p.Volume-c.volume) > 1e-5 || math.Abs(p.Area-c.area) > 1e-5 {
			t.Fatal(c, p, e)
		}
		if c.holes == 0 {
			check, e := s.ValidateNestedSplineComponents(ctx, SplineShellBooleanOptions{})
			if e != nil || len(check) != 2 || check[1].Parent != 0 || check[1].Depth != 1 {
				t.Fatal(check, e)
			}
		}
		lower, upper := h.FacesB[1][0], h.FacesB[2][0]
		if s.faces[lower].Reversed || s.faces[upper].Reversed != (c.holes == 0) {
			t.Fatal("floor or cavity orientation is wrong", lower, upper)
		}
		for _, q := range []struct {
			z    float64
			want parametric.Location
		}{{float64(c.lo), parametric.Boundary},
			{float64(c.lo+c.hi) / 2, parametric.Outside},
			{float64(c.lo) - .25, parametric.Inside}} {
			r, e := LocateWrappedCylinderBore(ctx, w, bs, geom.Vec3{Z: q.z}, o)
			if e != nil || r != q.want {
				t.Fatal(c, q, r, e)
			}
		}
	}
}

func TestWrappedRationalDraftFrustum(t *testing.T) {
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	w := makeCanonicalWrappedCylinder(t, true, 0)
	d, proof, e := geom.DraftWrappedFrustum(ctx, w, 0, -1, o)
	if e != nil || !proof.Complete || proof.LowerRadius.Cmp(big.NewRat(2, 1)) != 0 ||
		proof.UpperRadius.Cmp(big.NewRat(1, 1)) != 0 || d.Domain() != w.Domain() {
		t.Fatal(proof, e)
	}
	s, proof, e := NewWrappedFrustum(ctx, d, 1e-9, o)
	if e != nil || !proof.Circular || s.Topology().EulerCharacteristic != 2 {
		t.Fatal(proof, e)
	}
	p, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 2e-6})
	if e != nil || math.Abs(p.Volume-35*math.Pi/3) > 1e-5 ||
		math.Abs(p.Area-(5+3*math.Sqrt(26))*math.Pi) > 1e-5 ||
		math.Abs(p.Centroid.Z-(5.*11./28.-2)) > 1e-6 {
		t.Fatal(p, e)
	}
	if _, _, e = NewWrappedCylinder(ctx, d, 1e-9, o); !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("drafted support accepted as constant-radius cylinder", e)
	}
	if r, _, e := geom.DraftWrappedFrustum(ctx, d, 0, -1, o); r != nil || !errors.Is(e, geom.ErrDegenerate) {
		t.Fatal("collapsed endpoint accepted", e)
	}
	a, b, split, e := SplitWrappedFrustumAtZ(ctx, d, .5, 1e-9, o)
	if e != nil || split.Lower.CutCap != 2 || split.Upper.CutCap != 1 {
		t.Fatal(split, e)
	}
	for i, q := range []Solid{a, b} {
		m, e := q.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 2e-6})
		want := [2]float64{185 * math.Pi / 24, 95 * math.Pi / 24}[i]
		if e != nil || math.Abs(m.Volume-want) > 1e-5 {
			t.Fatal(i, m, want, e)
		}
	}
	a, b, _, e = SplitWrappedFrustumAtZ(ctx, d, 0, 1e-9, o) // radius 8/5
	if !errors.Is(e, geom.ErrUnrepresentable) || len(a.faces) != 0 || len(b.faces) != 0 {
		t.Fatal("nonrepresentable cut geometry was returned", e)
	}
}

func TestSteppedCoaxialWrappedCylinders(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	aSupport, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, 2, 4, o)
	if e != nil {
		t.Fatal(e)
	}
	bSupport, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{Z: 2}, 1, 4, o)
	if e != nil {
		t.Fatal(e)
	}
	a, _, e := NewWrappedCylinder(ctx, aSupport, 1e-9, o)
	if e != nil {
		t.Fatal(e)
	}
	b, _, e := NewWrappedCylinder(ctx, bSupport, 1e-9, o)
	if e != nil {
		t.Fatal(e)
	}
	for _, c := range []struct {
		op    parametric.BooleanOp
		v, a  float64
		faces int
	}{{parametric.Union, 18, 28, 5}, {parametric.Intersection, 2, 6, 3}, {parametric.Difference, 14, 28, 5}} {
		x, h, e := CombineSplineShells(ctx, a, b, c.op, SplineShellCombineOptions{})
		if e != nil || h.Method != "stepped-wrapped-cylinder" || len(x.faces) != c.faces || x.Topology().EulerCharacteristic != 2 {
			t.Fatal(c, h, x.Topology(), e)
		}
		p, e := x.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 2e-6})
		if e != nil || math.Abs(p.Volume-c.v*math.Pi) > 1e-5 || math.Abs(p.Area-c.a*math.Pi) > 1e-5 {
			t.Fatal(c, p, e)
		}
	}
}
