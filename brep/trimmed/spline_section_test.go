package trimmed

import (
	"bytes"
	"context"
	"math"
	"strings"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

func TestSealSplinePlaneSection(t *testing.T) {
	k := []float64{0, 0, 0, 1, 1, 1}
	q := [3]float64{.25, -.25, .25}
	for _, sign := range []float64{1, -1} {
		p := make([]geom.Vec3, 0, 9)
		for i := range 3 {
			for j := range 3 {
				p = append(p, geom.Vec3{X: float64(i) / 2, Y: float64(j) / 2, Z: sign * (q[i] + q[j])})
			}
		}
		bowl, e := geom.NewBSplineSurface(2, 2, 3, 3, p, k, k, nil)
		if e != nil {
			t.Fatal(e)
		}
		r, h, e := SealSplinePlaneSection(bowl, sign*.12, 1e-7, intersect.SplinePlaneOptions{
			Evaluation: geom.ArcEvaluationOptions{ParameterTolerance: 1e-10, BoxTolerance: 1e-10},
		})
		if e != nil {
			t.Fatal(e)
		}
		if len(h.ContourEdges) < 3 || r.Faces()[h.SplineFace].Surface != bowl {
			t.Fatal(h)
		}
		var native bytes.Buffer
		if e = WriteJSON(&native, r); e != nil {
			t.Fatal(e)
		}
		if !strings.Contains(native.String(), `"version":2`) {
			t.Fatal("implicit recipes missing")
		}
		bad := strings.Replace(native.String(), `"Recipe":0`, `"Recipe":10000`, 1)
		if _, e = ReadJSON(strings.NewReader(bad), JSONLimits{}); e == nil {
			t.Fatal("accepted invalid implicit recipe reference")
		}
		r, e = ReadJSON(&native, JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		ctx := context.Background()
		m, e := r.Tessellate(ctx, parametric.TessellationOptions{Deflection: .025})
		if e != nil {
			t.Fatal(e)
		}
		volume := math.Pi * .12 * .12 / 2
		if m.ValidateSolid() != nil || math.Abs(m.SignedVolume()-volume) > .002 {
			t.Fatal(m.SignedVolume(), volume)
		}
		props, e := r.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 5e-5, MaxSubdivisions: 16})
		area := math.Pi*.12 + math.Pi/6*(math.Pow(1+4*.12, 1.5)-1)
		if e != nil || math.Abs(props.Volume-volume) > 2e-6 || math.Abs(props.Area-area) > 2e-6 ||
			props.Centroid.Distance(geom.Vec3{X: .5, Y: .5, Z: sign * .08}) > 2e-6 {
			t.Fatal(props, e)
		}
	}
}

func TestSealOpenSplinePlaneSection(t *testing.T) {
	k := []float64{0, 0, 0, 1, 1, 1}
	// z=x², cut at x=.4 and on a subdivision boundary at x=.5.
	var p []geom.Vec3
	for i := range 3 {
		for j := range 3 {
			z := 0.
			if i == 2 {
				z = 1
			}
			p = append(p, geom.Vec3{X: float64(i) / 2, Y: float64(j) / 2, Z: z})
		}
	}
	s, e := geom.NewBSplineSurface(2, 2, 3, 3, p, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		height float64
		above  bool
	}{{.16, true}, {.16, false}, {.25, true}, {.25, false}} {
		above := tc.above
		out, h, e := SealOpenSplinePlaneSection(s, tc.height, above, 1e-8, intersect.SplinePlaneOptions{})
		if e != nil {
			t.Fatal(above, e)
		}
		if len(h.WallFaces) != 3 || len(h.ContourEdges) == 0 {
			t.Fatal(h)
		}
		var data bytes.Buffer
		if e = WriteJSON(&data, out); e != nil {
			t.Fatal(e)
		}
		out, e = ReadJSON(&data, JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		props, e := out.MassProperties(context.Background(), parametric.IntegrationOptions{})
		v := 2 * tc.height * math.Sqrt(tc.height) / 3
		if above {
			v = 1.0/3 - tc.height + v
		}
		if e != nil || math.Abs(props.Volume-v) > 1e-8 {
			t.Fatal(above, props, v, e)
		}
		m, e := out.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .003})
		if e != nil || m.ValidateSolid() != nil || math.Abs(m.SignedVolume()-v) > .003 {
			t.Fatal(above, m.SignedVolume(), e)
		}
	}
}

func TestSealProjectedSplinePlaneSection(t *testing.T) {
	k := []float64{0, 0, 0, 1, 1, 1}
	q := [3]float64{.25, -.25, .25}
	var p []geom.Vec3
	for i := range 3 {
		for j := range 3 {
			// Negative projected determinant exercises reversed chart orientation.
			p = append(p, geom.Vec3{X: 3 - float64(i), Y: -2 + 1.5*float64(j), Z: q[i] + q[j]})
		}
	}
	s, e := geom.NewBSplineSurface(2, 2, 3, 3, p, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	out, h, e := SealProjectedSplinePlaneSection(s, .12, 1e-7, intersect.SplinePlaneOptions{}, geom.SurfaceInjectivityOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(h.ContourEdges) < 3 || out.Faces()[0].Surface != s {
		t.Fatal(h)
	}
	x, e := out.MassProperties(ctx, parametric.IntegrationOptions{})
	want := 6 * math.Pi * .12 * .12 / 2
	if e != nil || math.Abs(x.Volume-want) > 1e-8 || x.Centroid.Distance(geom.Vec3{X: 2, Y: -.5, Z: .08}) > 1e-8 {
		t.Fatal(x, want, e)
	}
	proof, e := out.ValidateSplineRegularity(ctx, geom.SurfacePatchOptions{}, geom.SurfaceRegularityOptions{})
	if e != nil || len(proof) != 2 || !proof[0].Supported || !proof[0].Proof.Complete || proof[1].Supported {
		t.Fatal(proof, e)
	}
}

func TestSealDisjointSplinePlaneSections(t *testing.T) {
	ku := []float64{0, 0, 0, .5, .5, 1, 1, 1}
	kv := []float64{0, 0, 0, 1, 1, 1}
	// Two exact piecewise-quadratic valleys. Each yields one circle
	// of radius sqrt(height) in the actual XY projection.
	u := [5]float64{0, .25, .5, .75, 1}
	z := [5]float64{.0625, -.0625, .0625, -.0625, .0625}
	y := [3]float64{.25, -.25, .25}
	var p []geom.Vec3
	for i := range u {
		for j := range y {
			p = append(p, geom.Vec3{X: u[i], Y: float64(j) / 2, Z: z[i] + y[j]})
		}
	}
	s, e := geom.NewBSplineSurface(2, 2, 5, 3, p, ku, kv, nil)
	if e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	out, h, e := SealDisjointSplinePlaneSections(ctx, s, .0225, 1e-7, intersect.SplinePlaneOptions{}, geom.SurfaceInjectivityOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(h) != 2 || len(out.Topology().Shells) != 2 || h[1].SplineFace != 2 {
		t.Fatal(h)
	}
	a, e := out.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-6})
	want := math.Pi * .0225 * .0225
	if e != nil || math.Abs(a.Volume-want) > 1e-8 {
		t.Fatal(a, want, e)
	}
}

func TestSealNestedSplinePlaneSections(t *testing.T) {
	// z=(rho²-a)(rho²-b) about the center, with exact dyadic a,b.
	// Use the Bernstein expansion of (u-.5)^2 and its square.
	a, b := .015625, .09375
	x2 := [5]float64{.25, 0, -1.0 / 12, 0, .25}
	x4 := [5]float64{.0625, -.0625, .0625, -.0625, .0625}
	k := []float64{0, 0, 0, 0, 0, 1, 1, 1, 1, 1}
	var p []geom.Vec3
	for i := range 5 {
		for j := range 5 {
			z := x4[i] + x4[j] + 2*x2[i]*x2[j] - (a+b)*(x2[i]+x2[j]) + a*b
			p = append(p, geom.Vec3{X: float64(i) / 4, Y: float64(j) / 4, Z: z})
		}
	}
	s, e := geom.NewBSplineSurface(4, 4, 5, 5, p, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	opts := intersect.SplinePlaneOptions{Trace: geom.SurfaceTraceOptions{Arcs: geom.SurfacePlaneArcOptions{MaxCellParameterWidth: .0625, MaxOperations: 8000000}}}
	out, h, e := SealNestedSplinePlaneSections(ctx, s, 0, 1e-7, opts, geom.SurfaceInjectivityOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(h) != 1 || len(h[0].InnerContours) != 1 || out.Topology().EulerCharacteristic != 0 {
		t.Fatal(h, out.Topology())
	}
	props, e := out.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-6})
	want := math.Pi * math.Pow(b-a, 3) / 6
	if e != nil || math.Abs(props.Volume-want) > 5e-9 || math.Abs(props.Centroid.X-.5) > 1e-8 || math.Abs(props.Centroid.Y-.5) > 1e-8 {
		t.Fatal(props, want, e)
	}
}
