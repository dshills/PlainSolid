package trimmed

import (
	"bytes"
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"reflect"
	"testing"
)

func arcLoopMixedCornerFixture() []ExtrusionSegment {
	line := func(a, b geom.Vec2) ExtrusionSegment { return ExtrusionSegment{Start: a, End: b} }
	return []ExtrusionSegment{
		line(geom.Vec2{}, geom.Vec2{X: 3}),
		{Start: geom.Vec2{X: 3}, End: geom.Vec2{X: 4, Y: 1}, Center: geom.Vec2{X: 3, Y: 1}, Radius: 1, StartAngle: -math.Pi / 2, Sweep: math.Pi / 2},
		line(geom.Vec2{X: 4, Y: 1}, geom.Vec2{X: 2, Y: 1}),
		{Start: geom.Vec2{X: 2, Y: 1}, End: geom.Vec2{X: 1, Y: 2}, Center: geom.Vec2{X: 2, Y: 2}, Radius: 1, StartAngle: -math.Pi / 2, Sweep: -math.Pi / 2},
		line(geom.Vec2{X: 1, Y: 2}, geom.Vec2{X: 1, Y: 4}), line(geom.Vec2{X: 1, Y: 4}, geom.Vec2{Y: 4}), line(geom.Vec2{Y: 4}, geom.Vec2{}),
	}
}
func TestExtrudeArcLoopSignedCorners(t *testing.T) {
	ctx := context.Background()
	segments := arcLoopMixedCornerFixture()
	copyInput := append([]ExtrusionSegment(nil), segments...)
	for _, axis := range []geom.Vec3{{X: 1}, {X: -1}, {Y: 1}, {Y: -1}, {Z: 1}, {Z: -1}} {
		for _, h := range []float64{2, -2} {
			frame, _ := geom.FrameFromNormal(geom.Vec3{X: 4, Y: 8, Z: 16}, axis)
			result, e := ExtrudeArcLoop(ctx, frame, segments, h, 1e-9)
			if e != nil {
				t.Fatal(axis, h, e)
			}
			if math.Abs(result.Area-7) > 1e-12 || math.Abs(result.Perimeter-(12+math.Pi)) > 1e-12 {
				t.Fatal(result.Area, result.Perimeter)
			}
			mass, e := result.Solid.MassProperties(ctx, parametric.IntegrationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			if math.Abs(mass.Volume-14) > 1e-8 || math.Abs(mass.Area-(14+2*(12+math.Pi))) > 1e-8 {
				t.Fatal(mass)
			}
			m, e := result.Solid.Tessellate(ctx, parametric.TessellationOptions{Deflection: .05, Angle: .1, MaxTriangles: 100000, MaxSubdivisions: 128})
			if e != nil {
				t.Fatal(e)
			}
			if e = m.ValidateSolid(); e != nil {
				t.Fatal(e)
			}
			var data bytes.Buffer
			if e = WriteJSON(&data, result.Solid); e != nil {
				t.Fatal(e)
			}
			restored, e := ReadJSON(&data, JSONLimits{})
			if e != nil {
				t.Fatal(e)
			}
			p, e := restored.MassProperties(ctx, parametric.IntegrationOptions{})
			if e != nil || math.Abs(p.Volume-mass.Volume) > 1e-8 {
				t.Fatal(p, e)
			}
		}
	}
	if !reflect.DeepEqual(copyInput, segments) {
		t.Fatal("mutated source")
	}
}
func TestExtrudeArcLoopConvexCircularSegment(t *testing.T) {
	ctx := context.Background()
	frame, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	for _, sweep := range []float64{1.7, -1.7, math.Pi / 2} {
		arc := ExtrusionSegment{Center: geom.Vec2{X: 5, Y: 7}, Radius: 3, StartAngle: .37, Sweep: sweep}
		arc.Start = arcLoopPoint(arc, arc.StartAngle)
		arc.End = arcLoopPoint(arc, arc.StartAngle+sweep)
		result, e := ExtrudeArcLoop(ctx, frame, []ExtrusionSegment{arc, {Start: arc.End, End: arc.Start}}, 4, 1e-9)
		if e != nil {
			t.Fatal(sweep, e)
		}
		want := 9 * (math.Abs(sweep) - math.Sin(math.Abs(sweep))) / 2
		mass, e := result.Solid.MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil || math.Abs(mass.Volume-4*want) > 1e-8 {
			t.Fatal(mass, e, want)
		}
		m, e := result.Solid.Tessellate(ctx, parametric.TessellationOptions{Deflection: .05, Angle: .1, MaxTriangles: 100000, MaxSubdivisions: 128})
		if e != nil {
			t.Fatal(e)
		}
		if e = m.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		var data bytes.Buffer
		if e = WriteJSON(&data, result.Solid); e != nil {
			t.Fatal(e)
		}
		restored, e := ReadJSON(&data, JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		restoredMass, e := restored.MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil || math.Abs(restoredMass.Volume-mass.Volume) > 1e-8 {
			t.Fatal(restoredMass, e)
		}

	}
}
func TestExtrudeArcLoopRejects(t *testing.T) {
	ctx := context.Background()
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	for _, edit := range []func([]ExtrusionSegment){
		func(s []ExtrusionSegment) { s[0].End.X += .01 },
		func(s []ExtrusionSegment) { s[1].Radius += .001 },
		func(s []ExtrusionSegment) { s[3].Sweep = math.Pi / 2 },
		func(s []ExtrusionSegment) { s[3].StartAngle = 1e100 },
	} {
		s := arcLoopMixedCornerFixture()
		edit(s)
		if r, e := ExtrudeArcLoop(ctx, f, s, 2, 1e-9); e == nil || len(r.Solid.Faces()) != 0 {
			t.Fatal("accepted invalid loop", e)
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := ExtrudeArcLoop(canceled, f, arcLoopMixedCornerFixture(), 2, 1e-9); e != context.Canceled {
		t.Fatal(e)
	}
}

func TestExtrudeArcLoopAuthoredMajorFragment(t *testing.T) {
	p := geom.Vec2{X: 7.071067811865, Y: 7.071067811865}
	segments := []ExtrusionSegment{{Start: geom.Vec2{Y: -10}, End: geom.Vec2{X: 10}}, {Start: geom.Vec2{X: 10}, End: p}, {Start: p, End: geom.Vec2{Y: -10}, Radius: 10, StartAngle: math.Pi / 4, Sweep: 5 * math.Pi / 4}}
	frame, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	r, e := ExtrudeArcLoop(context.Background(), frame, segments, 10, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	mass, e := r.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	want := 10 * (75*math.Pi + 50 - 50*(math.Pi/4-math.Sqrt(.5)))
	if math.Abs(mass.Volume-want) > 1e-8 {
		t.Fatal(mass.Volume, want)
	}
	var data bytes.Buffer
	if e = WriteJSON(&data, r.Solid); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadJSON(&data, JSONLimits{}); e != nil {
		t.Fatal(e)
	}
}

func TestExtrudeArcLoopTwoCircleRegions(t *testing.T) {
	ctx := context.Background()
	frame, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	for _, r := range []float64{10, 12} {
		y := math.Sqrt(3) * r / 2
		p, q := geom.Vec2{X: r / 2, Y: -y}, geom.Vec2{X: r / 2, Y: y}
		lens := r * r * (2*math.Pi/3 - math.Sqrt(3)/2)
		cases := []struct {
			segments []ExtrusionSegment
			area     float64
		}{
			{[]ExtrusionSegment{{Start: p, End: q, Center: geom.Vec2{X: r}, Radius: r, StartAngle: 4 * math.Pi / 3, Sweep: 4 * math.Pi / 3}, {Start: q, End: p, Radius: r, StartAngle: math.Pi / 3, Sweep: -2 * math.Pi / 3}}, math.Pi*r*r - lens},
			{[]ExtrusionSegment{{Start: q, End: p, Center: geom.Vec2{X: r}, Radius: r, StartAngle: 2 * math.Pi / 3, Sweep: 2 * math.Pi / 3}, {Start: p, End: q, Radius: r, StartAngle: 5 * math.Pi / 3, Sweep: 2 * math.Pi / 3}}, lens},
		}
		for _, c := range cases {
			result, e := ExtrudeArcLoop(ctx, frame, c.segments, 10, 1e-9)
			if e != nil {
				t.Fatal(r, c.area, e)
			}
			mass, e := result.Solid.MassProperties(ctx, parametric.IntegrationOptions{})
			if e != nil || math.Abs(mass.Volume-10*c.area) > 1e-7 {
				t.Fatal(mass, c.area, e)
			}
			m, e := result.Solid.Tessellate(ctx, parametric.TessellationOptions{Deflection: .05, Angle: .1, MaxTriangles: 100000, MaxSubdivisions: 128})
			if e != nil {
				t.Fatal(e)
			}
			if e = m.ValidateSolid(); e != nil {
				t.Fatal(e)
			}
			var data bytes.Buffer
			if e = WriteJSON(&data, result.Solid); e != nil {
				t.Fatal(e)
			}
			if _, e = ReadJSON(&data, JSONLimits{}); e != nil {
				t.Fatal(e)
			}
		}
	}
}

func TestExtrudeArcLoopAuthoredCircularNotch(t *testing.T) {
	for _, scale := range []float64{1, 1.2} {
		for _, reverse := range []bool{false, true} {
			r := 10 * scale
			x := 4.841229182759271 * scale
			y := 8.75 * scale
			p, q := geom.Vec2{X: x, Y: y}, geom.Vec2{X: -x, Y: y}
			s := []ExtrusionSegment{{Start: geom.Vec2{X: -r}, End: geom.Vec2{X: r}}, {Start: geom.Vec2{X: r}, End: p, Radius: r, StartAngle: 0, Sweep: 1.065435816510739}, {Start: p, End: q, Center: geom.Vec2{Y: r}, Radius: r / 2, StartAngle: 6.0305050520375065, Sweep: -2.6362321433056355}, {Start: q, End: geom.Vec2{X: -r}, Radius: r, StartAngle: 2.076156837079054, Sweep: 1.065435816510739}}
			if reverse {
				for i := range s {
					s[i].Start, s[i].End = s[i].End, s[i].Start
					if s[i].Radius > 0 {
						s[i].StartAngle += s[i].Sweep
						s[i].Sweep = -s[i].Sweep
					}
				}
				for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
					s[i], s[j] = s[j], s[i]
				}
			}
			frame, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
			result, e := ExtrudeArcLoop(context.Background(), frame, s, 10, 1e-9)
			if e != nil {
				t.Fatal(scale, reverse, e)
			}
			overlap := r*r*math.Acos(.875) + r*r/4*math.Acos(.25) - math.Sqrt(9375)*scale*scale/2
			want := 10 * (math.Pi*r*r/2 - overlap)
			mass, e := result.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{})
			if e != nil || math.Abs(mass.Volume-want) > 1e-7 {
				t.Fatal(mass, want, e)
			}
		}
	}
}
