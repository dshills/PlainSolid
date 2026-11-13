package trimmed

import (
	"context"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

func TestClassifyTubeTrimByIntervalWinding(t *testing.T) {
	ctx := context.Background()
	k := []float64{0, 0, 1, 1}
	a, e := geom.NewBSplineSurface(1, 1, 2, 2,
		[]geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1}}, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	// The wall extends past both sides of A. On z=0 the transverse
	// branch is y=29/64+x*(1-x)/16 for 0<=x<=1.
	b, e := geom.NewBSplineSurface(2, 1, 3, 2,
		[]geom.Vec3{{X: -.5, Y: 13.0 / 32, Z: -1}, {X: -.5, Y: 13.0 / 32, Z: 1},
			{X: .5, Y: 17.0 / 32, Z: -1}, {X: .5, Y: 17.0 / 32, Z: 1},
			{X: 1.5, Y: 13.0 / 32, Z: -1}, {X: 1.5, Y: 13.0 / 32, Z: 1}},
		[]float64{-.5, -.5, -.5, 1.5, 1.5, 1.5}, []float64{-1, -1, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	da := geom.ParameterRectangle{UMax: 1, VMax: 1}
	db := geom.ParameterRectangle{UMin: -.5, UMax: 1.5, VMin: -1, VMax: 1}
	segments, e := intersect.RebuildTubeCurves(ctx, intersect.TubeRecipeData{
		A:     intersect.Patch{Surface: a, Max: geom.Vec2{X: 1, Y: 1}},
		B:     intersect.Patch{Surface: b, Min: geom.Vec2{X: -.5, Y: -1}, Max: geom.Vec2{X: 1.5, Y: 1}},
		Cells: []intersect.TubeRecipeCell{{DomainA: da, DomainB: db}},
	})
	if e != nil {
		t.Fatal(e)
	}
	c := segments[0].TrimA()
	x, e := c.EvalUV(0)
	y, f := c.EvalUV(1)
	if e != nil || f != nil {
		t.Fatal(e, f)
	}
	face := Face{Surface: a, Outer: []Use{
		{UVStart: geom.Vec2{}, UVEnd: geom.Vec2{X: 1}},
		{UVStart: geom.Vec2{X: 1}, UVEnd: y},
		{UVStart: y, UVEnd: x, Curve: reversePCurve(c)},
		{UVStart: x, UVEnd: geom.Vec2{}},
	}}
	for _, tc := range []struct {
		p    geom.Vec2
		want parametric.Location
	}{{geom.Vec2{X: .5, Y: 59.0 / 128}, parametric.Inside}, {geom.Vec2{X: .5, Y: .5}, parametric.Outside}} {
		q := geom.UVBox{Min: tc.p, Max: tc.p}
		loc, known, e := classifyTrimBox(ctx, face, q, 96)
		if e != nil || !known || loc != tc.want {
			t.Fatal(tc, loc, known, e)
		}
	}
	p := geom.Vec2{X: .5, Y: 15.0 / 32}
	if loc, known, e := classifyTrimBox(ctx, face, geom.UVBox{Min: p, Max: p}, 8); e != nil || known {
		t.Fatal(loc, known, e)
	}
}

func TestClassifyPlaneSectionTrimByIntervalWinding(t *testing.T) {
	ctx := context.Background()
	var poles []geom.Vec3
	for i, c := range []float64{7.0 / 16, 9.0 / 16, 7.0 / 16} {
		x := float64(i) / 2
		poles = append(poles, geom.Vec3{X: x, Z: -c}, geom.Vec3{X: x, Y: 1, Z: 1 - c})
	}
	s, e := geom.NewBSplineSurface(2, 1, 3, 2, poles, []float64{0, 0, 0, 1, 1, 1}, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	loops, _, e := intersect.TraceSplinePlaneContext(ctx, s, f, intersect.SplinePlaneOptions{})
	if e != nil || len(loops) != 1 || loops[0].Closed {
		t.Fatal(loops, e)
	}
	var upper []Use
	for _, segment := range loops[0].Segments {
		c := segment.SplineTrim()
		a, e := c.EvalUV(0)
		b, f := c.EvalUV(1)
		if e != nil || f != nil {
			t.Fatal(e, f)
		}
		upper = append(upper, Use{UVStart: a, UVEnd: b, Curve: c})
	}
	if upper[0].UVStart.X == 0 {
		for i, j := 0, len(upper)-1; i <= j; i, j = i+1, j-1 {
			upper[i], upper[j] = upper[j], upper[i]
		}
		for i, u := range upper {
			upper[i] = Use{UVStart: u.UVEnd, UVEnd: u.UVStart, Curve: reversePCurve(u.Curve)}
		}
	}
	right, left := upper[0].UVStart, upper[len(upper)-1].UVEnd
	face := Face{Surface: s, Outer: []Use{{UVEnd: geom.Vec2{X: 1}}, {UVStart: geom.Vec2{X: 1}, UVEnd: right}}}
	face.Outer = append(face.Outer, upper...)
	face.Outer = append(face.Outer, Use{UVStart: left})
	for _, tc := range []struct {
		p    geom.Vec2
		want parametric.Location
	}{{geom.Vec2{X: .5, Y: .46875}, parametric.Inside}, {geom.Vec2{X: .5, Y: .5625}, parametric.Outside}} {
		loc, known, e := classifyTrimBox(ctx, face, geom.UVBox{Min: tc.p, Max: tc.p}, 64)
		if e != nil || !known || loc != tc.want {
			t.Fatal(tc, loc, known, e)
		}
	}
}
