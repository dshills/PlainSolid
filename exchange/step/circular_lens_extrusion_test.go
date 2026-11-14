package step

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"regexp"
	"strings"
	"testing"
)

func TestCircularLensSTEP(t *testing.T) {
	for _, normal := range []geom.Vec3{{Z: 1}, {Z: -1}, {X: 1}, {X: -1}, {Y: 1}, {Y: -1}} {
		for _, r := range []float64{10, 12} {
			f, _ := geom.FrameFromNormal(geom.Vec3{}, normal)
			x, y := r/2, r*math.Sqrt(3)/2
			theta := math.Atan2(y, x)
			a, b := geom.Vec2{X: x, Y: -y}, geom.Vec2{X: x, Y: y}
			m := CircularLensExtrusionModel{LengthUnit: .001, Body: CircularLensExtrusionBody{Name: "lens", Frame: f, Height: 5, Arcs: [2]trimmed.ExtrusionSegment{{Start: a, End: b, Radius: r, StartAngle: -theta, Sweep: 2 * theta}, {Start: b, End: a, Center: geom.Vec2{X: r}, Radius: r, StartAngle: math.Pi - theta, Sweep: (math.Pi + theta) - (math.Pi - theta)}}}}
			if _, e := BuildCircularLensExtrusion(m.Body); e != nil {
				t.Fatal("build", e)
			}
			var out bytes.Buffer
			if e := WriteCircularLensExtrusion(&out, m); e != nil {
				t.Fatal(r, e)
			}
			got, e := ReadCircularLensExtrusion(bytes.NewReader(out.Bytes()), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			var again bytes.Buffer
			if e = WriteCircularLensExtrusion(&again, got); e != nil || !bytes.Equal(out.Bytes(), again.Bytes()) {
				t.Fatal(e, "unstable")
			}
			t.Log(r, out.Len())
			if os.Getenv("PLAINSOLID_OCCT_STEP") != "" {
				dir := t.TempDir()
				src, dst := filepath.Join(dir, "lens.step"), filepath.Join(dir, "mass.json")
				if e = os.WriteFile(src, out.Bytes(), 0600); e != nil {
					t.Fatal(e)
				}
				stepOracle(t, dst, "step-mass-precise", src)
				raw, e := os.ReadFile(dst)
				if e != nil {
					t.Fatal(e)
				}
				var o struct {
					Volume, Area float64
					Centroid     [3]float64
				}
				if e = json.Unmarshal(raw, &o); e != nil {
					t.Fatal(e)
				}
				area := r * r * (2*math.Pi/3 - math.Sqrt(3)/2)
				v := 5 * area
				a := 2*area + 5*4*r*math.Pi/3
				c := f.ToWorld(geom.Vec3{X: r / 2, Z: 2.5})
				actual := geom.Vec3{X: o.Centroid[0], Y: o.Centroid[1], Z: o.Centroid[2]}
				if math.Abs(o.Volume-v) > 1e-7 || math.Abs(o.Area-a) > 1e-7 || actual.Distance(c) > 1e-7 {
					t.Fatal(o, v, a, c)
				}
			}
		}
	}

}

func TestUnequalCircularLensSTEP(t *testing.T) {
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	level := 8.75
	x := math.Sqrt(100 - level*level)
	a, b := geom.Vec2{X: x, Y: level}, geom.Vec2{X: -x, Y: level}
	startA, endA := math.Atan2(level, x), math.Atan2(level, -x)
	startB, endB := math.Atan2(level-10, -x), math.Atan2(level-10, x)
	p := CircularLensExtrusionBody{Name: "unequal lens", Frame: f, Height: 5, Arcs: [2]trimmed.ExtrusionSegment{{Start: a, End: b, Radius: 10, StartAngle: startA, Sweep: endA - startA}, {Start: b, End: a, Center: geom.Vec2{Y: 10}, Radius: 5, StartAngle: startB, Sweep: endB - startB}}}
	m := CircularLensExtrusionModel{Body: p, LengthUnit: .001}
	var out bytes.Buffer
	if e := WriteCircularLensExtrusion(&out, m); e != nil {
		t.Fatal(e)
	}
	t.Log(out.Len())
	got, e := ReadCircularLensExtrusion(bytes.NewReader(out.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var again bytes.Buffer
	if e = WriteCircularLensExtrusion(&again, got); e != nil || !bytes.Equal(out.Bytes(), again.Bytes()) {
		t.Fatal(e, "unstable")
	}
	if os.Getenv("PLAINSOLID_OCCT_STEP") == "" {
		return
	}
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "unequal.step"), filepath.Join(dir, "mass.json")
	if e = os.WriteFile(src, out.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	stepOracle(t, dst, "step-mass-precise", src)
	raw, e := os.ReadFile(dst)
	if e != nil {
		t.Fatal(e)
	}
	var o struct {
		Volume, Area float64
		Centroid     [3]float64
	}
	if e = json.Unmarshal(raw, &o); e != nil {
		t.Fatal(e)
	}
	sa, sb := p.Arcs[0].Sweep, p.Arcs[1].Sweep
	aa, ab := 50*(sa-math.Sin(sa)), 12.5*(sb-math.Sin(sb))
	ra := 40 * math.Pow(math.Sin(sa/2), 3) / (3 * (sa - math.Sin(sa)))
	rb := 20 * math.Pow(math.Sin(sb/2), 3) / (3 * (sb - math.Sin(sb)))
	v := 5 * (aa + ab)
	area := 2*(aa+ab) + 5*(10*sa+5*sb)
	c := geom.Vec3{Y: (aa*ra + ab*(10-rb)) / (aa + ab), Z: 2.5}
	actual := geom.Vec3{X: o.Centroid[0], Y: o.Centroid[1], Z: o.Centroid[2]}
	if math.Abs(o.Volume-v) > 1e-7 || math.Abs(o.Area-area) > 1e-7 || actual.Distance(c) > 1e-7 {
		t.Fatal(o, v, area, c)
	}
}

func TestCircularLensSTEPRejectsTampering(t *testing.T) {
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	x, y := 5., 5*math.Sqrt(3)
	theta := math.Atan2(y, x)
	a, b := geom.Vec2{X: x, Y: -y}, geom.Vec2{X: x, Y: y}
	p := CircularLensExtrusionBody{Frame: f, Height: 5, Arcs: [2]trimmed.ExtrusionSegment{{Start: a, End: b, Radius: 10, StartAngle: -theta, Sweep: 2 * theta}, {Start: b, End: a, Center: geom.Vec2{X: 10}, Radius: 10, StartAngle: math.Pi - theta, Sweep: (math.Pi + theta) - (math.Pi - theta)}}}
	m := CircularLensExtrusionModel{Body: p, LengthUnit: .001}
	var out bytes.Buffer
	if e := WriteCircularLensExtrusion(&out, m); e != nil {
		t.Fatal(e)
	}
	circle := regexp.MustCompile(`CIRCLE\('',#\d+,10\.\)`).FindString(out.String())
	if circle == "" {
		t.Fatal("circle")
	}
	changed := strings.Replace(out.String(), circle, strings.Replace(circle, ",10.)", ",9.)", 1), 1)
	if _, e := ReadCircularLensExtrusion(strings.NewReader(changed), Limits{}); e == nil {
		t.Fatal("altered circle accepted")
	}
	m.Body.Arcs[0].Sweep = -m.Body.Arcs[0].Sweep
	out.Reset()
	if e := WriteCircularLensExtrusion(&out, m); e == nil || out.Len() != 0 {
		t.Fatal("negative arc published")
	}
	m.Body = p
	m.Body.Arcs[1].Center.X = 20
	if _, e := BuildCircularLensExtrusion(m.Body); e == nil {
		t.Fatal("tangent circles accepted")
	}
}
