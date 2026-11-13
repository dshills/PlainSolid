package step

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"plainsolid/geom"
	"testing"
)

func obliqueModel(t *testing.T) ObliqueCylinderModel {
	t.Helper()
	f, e := geom.NewFrame(geom.Vec3{X: 1, Y: 2, Z: 3}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	p, e := geom.NewPlane(geom.Vec3{X: 1, Y: 2, Z: 7}, geom.Vec3{X: -.25, Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	return ObliqueCylinderModel{LengthUnit: .001, Bodies: []ObliqueCylinderBody{{Name: "slanted", Frame: f, Radius: 2, Plane: p}}}
}
func TestObliqueExportRoundtrip(t *testing.T) {
	var b bytes.Buffer
	if e := WriteObliqueCylinders(&b, obliqueModel(t)); e != nil {
		t.Fatal(e)
	}
	checkOblique(t, b.String())
	if e := WriteObliqueCylinders(failWriter{}, obliqueModel(t)); e == nil {
		t.Fatal("writer error lost")
	}
	if e := WriteObliqueCylinders(nil, obliqueModel(t)); e == nil {
		t.Fatal("nil writer")
	}
	for _, kind := range []string{"radius", "frame", "plane", "cross", "units"} {
		m := obliqueModel(t)
		switch kind {
		case "radius":
			m.Bodies[0].Radius = -1
		case "frame":
			m.Bodies[0].Frame = geom.Frame{}
		case "plane":
			m.Bodies[0].Plane = geom.Plane{}
		case "cross":
			m.Bodies[0].Plane, _ = geom.NewPlane(geom.Vec3{X: 1, Y: 2, Z: 3}, geom.Vec3{X: 1, Z: 1})
		case "units":
			m.LengthUnit = 0
		}
		var output bytes.Buffer
		if e := WriteObliqueCylinders(&output, m); e == nil || output.Len() != 0 {
			t.Fatal(kind, e, output.String())
		}
	}
}
func TestOCCTObliqueExport(t *testing.T) {
	var b bytes.Buffer
	if e := WriteObliqueCylinders(&b, obliqueModel(t)); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "oblique.step")
	if e := os.WriteFile(path, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	out := filepath.Join(t.TempDir(), "mass.json")
	stepOracle(t, out, "step-mass-precise", path)
	data, e := os.ReadFile(out)
	if e != nil {
		t.Fatal(e)
	}
	var got struct {
		Area, Volume float64
		Centroid     [3]float64
	}
	if e = json.Unmarshal(data, &got); e != nil {
		t.Fatal(e)
	}
	if math.Abs(got.Volume-16*math.Pi) > 1e-4 || math.Abs(got.Area-(20+math.Sqrt(17))*math.Pi) > 2e-4 || math.Abs(got.Centroid[0]-1.0625) > 2e-6 || math.Abs(got.Centroid[1]-2) > 2e-6 || math.Abs(got.Centroid[2]-5.0078125) > 2e-6 {
		t.Fatal(got)
	}
	// OCCT reconstructs the omitted cylinder p-curve approximately. Compare
	// against its independently exported identical seam graph as well as the
	// closed-form error envelope above; adaptive mass integration alone does
	// not remove that geometric approximation.
	fixture := filepath.Join(t.TempDir(), "reference.step")
	stepOracle(t, fixture, "write-step-oblique-cylinder")
	reference := filepath.Join(t.TempDir(), "reference.json")
	stepOracle(t, reference, "step-mass-precise", fixture)
	raw, e := os.ReadFile(reference)
	if e != nil {
		t.Fatal(e)
	}
	var ref struct {
		Area, Volume float64
		Centroid     [3]float64
	}
	if e = json.Unmarshal(raw, &ref); e != nil {
		t.Fatal(e)
	}
	if math.Abs(got.Area-ref.Area) > 1e-7 || math.Abs(got.Volume-ref.Volume) > 1e-7 {
		t.Fatal("independent OCCT roundtrip differs", got, ref)
	}
	for i := range got.Centroid {
		if math.Abs(got.Centroid[i]-ref.Centroid[i]) > 1e-8 {
			t.Fatal(got, ref)
		}
	}

}
