package step

import (
	"bytes"
	"encoding/json"
	"io"
	"math"
	"os"
	"path/filepath"
	"plainsolid/geom"
	"strings"
	"testing"
)

func TestWriteSpheres(t *testing.T) {
	m := SphereModel{LengthUnit: .001, Bodies: []SphereBody{{Name: "sphere α", Center: geom.Vec3{X: 1, Y: 2, Z: 3}, Radius: 2}}}
	var b, again bytes.Buffer
	if e := WriteSpheres(&b, m); e != nil {
		t.Fatal(e)
	}
	if e := WriteSpheres(&again, m); e != nil || !bytes.Equal(b.Bytes(), again.Bytes()) {
		t.Fatal("determinism", e)
	}
	if strings.Contains(b.String(), "POLY_LOOP") {
		t.Fatal("faceting")
	}
	got, e := ReadAnalyticBodies(&b, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	checkSphere(t, got)
	if got.Bodies[0].Name != m.Bodies[0].Name {
		t.Fatal("name")
	}
	for _, w := range []io.Writer{nil, failWriter{}, shortWriter{}} {
		if e := WriteSpheres(w, m); e == nil {
			t.Fatal("writer error")
		}
	}
	for _, radius := range []float64{0, -1, math.NaN(), math.Inf(1)} {
		m.Bodies[0].Radius = radius
		if e := WriteSpheres(io.Discard, m); e == nil {
			t.Fatal("invalid radius")
		}
	}
}
func TestWriteMultipleSpheres(t *testing.T) {
	m := SphereModel{LengthUnit: 1, Bodies: []SphereBody{{Radius: 1}, {Center: geom.Vec3{X: 5}, Radius: 2}}}
	var b bytes.Buffer
	if e := WriteSpheres(&b, m); e != nil {
		t.Fatal(e)
	}
	got, e := ReadAnalyticBodies(&b, Limits{})
	if e != nil || len(got.Bodies) != 2 {
		t.Fatal(got, e)
	}
}
func TestOCCTSphericalSTEPExport(t *testing.T) {
	m := SphereModel{LengthUnit: .001, Bodies: []SphereBody{{Center: geom.Vec3{X: 1, Y: 2, Z: 3}, Radius: 2}}}
	p := filepath.Join(t.TempDir(), "sphere.step")
	f, e := os.Create(p)
	if e != nil {
		t.Fatal(e)
	}
	e = WriteSpheres(f, m)
	ce := f.Close()
	if e != nil || ce != nil {
		t.Fatal(e, ce)
	}
	out := filepath.Join(t.TempDir(), "mass.json")
	stepOracle(t, out, p)
	data, e := os.ReadFile(out)
	if e != nil {
		t.Fatal(e)
	}
	var result struct {
		Area, Volume float64
		Centroid     [3]float64
	}
	if e = json.Unmarshal(data, &result); e != nil {
		t.Fatal(e)
	}
	if math.Abs(result.Area-16*math.Pi) > 1e-6 || math.Abs(result.Volume-32*math.Pi/3) > 1e-6 || math.Abs(result.Centroid[0]-1) > 1e-9 || math.Abs(result.Centroid[1]-2) > 1e-9 || math.Abs(result.Centroid[2]-3) > 1e-9 {
		t.Fatal(result)
	}
}
