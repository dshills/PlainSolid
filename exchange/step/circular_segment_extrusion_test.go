package step

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"plainsolid/geom"
	"strings"
	"testing"
)

func TestCircularSegmentSTEP(t *testing.T) {
	t.Parallel()
	for _, n := range []geom.Vec3{{Z: 1}, {Z: -1}, {X: 1}, {X: -1}, {Y: 1}, {Y: -1}} {
		for _, sweep := range []float64{math.Pi / 2, 1.5 * math.Pi, 2.5} {
			f, _ := geom.FrameFromNormal(geom.Vec3{}, n)
			p := CircularSegmentExtrusionBody{Name: "circular segment", Frame: f, Radius: 3, Height: 2, StartAngle: -sweep / 2, Sweep: sweep}
			m := CircularSegmentExtrusionModel{Body: p, LengthUnit: .001}
			var b bytes.Buffer
			if e := WriteCircularSegmentExtrusion(&b, m); e != nil {
				t.Fatal(n, sweep, e)
			}
			got, e := ReadCircularSegmentExtrusion(bytes.NewReader(b.Bytes()), Limits{})
			if e != nil {
				t.Fatal(n, sweep, e)
			}
			var again bytes.Buffer
			if e = WriteCircularSegmentExtrusion(&again, got); e != nil || !bytes.Equal(b.Bytes(), again.Bytes()) {
				t.Fatal(e, "unstable")
			}
			if os.Getenv("PLAINSOLID_OCCT_STEP") == "" {
				continue
			}
			dir := t.TempDir()
			src, dst := filepath.Join(dir, "segment.step"), filepath.Join(dir, "mass.json")
			if e = os.WriteFile(src, b.Bytes(), 0600); e != nil {
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
			area := 4.5 * (sweep - math.Sin(sweep))
			v := 2 * area
			a := 2*area + 2*(3*sweep+6*math.Sin(sweep/2))
			rho := 4 * math.Pow(math.Sin(sweep/2), 3) / (sweep - math.Sin(sweep))
			mid := p.StartAngle + sweep/2
			c := f.ToWorld(geom.Vec3{X: rho * math.Cos(mid), Y: rho * math.Sin(mid), Z: 1})
			actual := geom.Vec3{X: o.Centroid[0], Y: o.Centroid[1], Z: o.Centroid[2]}
			if math.Abs(o.Volume-v) > 1e-8 || math.Abs(o.Area-a) > 1e-8 || actual.Distance(c) > 1e-8 {
				t.Fatal(o, v, a, c)
			}
		}
	}
}

func TestCircularSegmentActualSolverSTEP(t *testing.T) {
	t.Parallel()
	f, _ := geom.FrameFromNormal(geom.Vec3{Z: 13}, geom.Vec3{Z: 1})
	p := CircularSegmentExtrusionBody{Frame: f, Radius: 20, Height: 5, StartAngle: 2.5002222596456e-7, Sweep: 3.141592153545341}
	m := CircularSegmentExtrusionModel{Body: p, LengthUnit: .001}
	var b bytes.Buffer
	if e := WriteCircularSegmentExtrusion(&b, m); e != nil {
		t.Fatal(e)
	}
	got, e := ReadCircularSegmentExtrusion(bytes.NewReader(b.Bytes()), Limits{})
	if e != nil || got != m {
		t.Fatal(got, e)
	}
	t.Log("solver STEP bytes", b.Len())
}

func TestCircularSegmentSTEPRejectsAngularTamper(t *testing.T) {
	t.Parallel()
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	m := CircularSegmentExtrusionModel{LengthUnit: .001, Body: CircularSegmentExtrusionBody{Frame: f, Radius: 3, Height: 2, StartAngle: -.5, Sweep: 1}}
	var b bytes.Buffer
	if e := WriteCircularSegmentExtrusion(&b, m); e != nil {
		t.Fatal(e)
	}
	old := "CARTESIAN_POINT('',(-0.5,2.))"
	if !strings.Contains(b.String(), old) {
		t.Fatal("missing axial angular pcurve")
	}
	changed := strings.Replace(b.String(), old, "CARTESIAN_POINT('',(-0.49,2.))", 1)
	if _, e := ReadCircularSegmentExtrusion(strings.NewReader(changed), Limits{}); e == nil {
		t.Fatal("tampered angular pcurve accepted")
	}
	m.Body.StartAngle = 0
	m.Body.Sweep = 4
	b.Reset()
	if e := WriteCircularSegmentExtrusion(&b, m); e == nil || b.Len() != 0 {
		t.Fatal("branch crossing accepted")
	}
}
