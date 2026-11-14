package step

import (
	"bytes"
	"testing"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func TestCircularLensAuthoredUnequalSweep(t *testing.T) {
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	p := CircularLensExtrusionBody{Name: "actual", Frame: f, Height: 10}
	p.Arcs = [2]trimmed.ExtrusionSegment{
		{Start: geom.Vec2{X: -5.809475019311125, Y: 10.5}, End: geom.Vec2{X: 5.809475019311125, Y: 10.5}, Center: geom.Vec2{Y: 12}, Radius: 6, StartAngle: 3.3942729087318715, Sweep: 2.6362321433056355},
		{Start: geom.Vec2{X: 5.809475019311125, Y: 10.5}, End: geom.Vec2{X: -5.809475019311125, Y: 10.5}, Radius: 12, StartAngle: 1.065435816510739, Sweep: 1.0107210205683153},
	}
	m := CircularLensExtrusionModel{Body: p, LengthUnit: .001}
	var b bytes.Buffer
	if e := WriteCircularLensExtrusion(&b, m); e != nil {
		t.Fatal("raw writer", e)
	}
	got, e := ReadCircularLensExtrusion(bytes.NewReader(b.Bytes()), Limits{})
	if e != nil {
		t.Fatal("reader", e)
	}
	var again bytes.Buffer
	if e = WriteCircularLensExtrusion(&again, got); e != nil || !bytes.Equal(b.Bytes(), again.Bytes()) {
		t.Fatal("graph", e)
	}
}
