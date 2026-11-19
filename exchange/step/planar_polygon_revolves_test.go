package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"testing"
)

func planarRevolveFixture(axis geom.Vec3, angle float64) PlanarPolygonRevolveModel {
	return PlanarPolygonRevolveModel{LengthUnit: .001, Bodies: []PlanarPolygonRevolveBody{{Name: "PlainCAD axis edit", Points: []geom.Vec3{{}, {X: 20}, {X: 20, Y: 10}, {Y: 10}}, AxisDirection: axis, Angle: angle}}}
}
func TestPlanarPolygonRevolvePlainCAD(t *testing.T) {
	t.Parallel()
	for _, axis := range []geom.Vec3{{X: 2}, {Y: 2}, {X: -2}, {Y: -2}} {
		for _, angle := range []float64{math.Pi / 2, -math.Pi / 2, -1.3 * math.Pi} {
			m := planarRevolveFixture(axis, angle)
			b := m.Bodies[0]
			p, e := trimmed.PreparePlanarPolygonRevolution(context.Background(), b.Points, b.AxisPoint, b.AxisDirection)
			if e != nil {
				t.Fatal(e)
			}
			var data bytes.Buffer
			if e = WritePlanarPolygonRevolves(&data, m); e != nil {
				t.Fatal(axis, angle, e)
			}
			normal, e := ReadRectangleRevolves(bytes.NewReader(data.Bytes()), Limits{})
			if e != nil {
				t.Fatal(axis, angle, e)
			}
			if math.Abs(normal.Bodies[0].Angle-math.Abs(angle)) > 1e-12 {
				t.Fatal(normal)
			}
			got, e := ReadPlanarRectangleRevolvesWithResolution(bytes.NewReader(data.Bytes()), Limits{}, RectangleRevolveResolution{Frame: p.Frame, Negative: angle < 0})
			if e != nil {
				t.Fatal(axis, angle, e)
			}
			var out bytes.Buffer
			if e = WritePlanarPolygonRevolves(&out, got); e != nil {
				t.Fatal(e)
			}
			native, e := trimmed.RevolvePlanarPolygon(context.Background(), b.Points, b.AxisPoint, b.AxisDirection, b.Angle)
			if e != nil {
				t.Fatal(e)
			}
			want, e := native.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-10})
			if e != nil {
				t.Fatal(e)
			}
			decoded, e := ReadAnalyticBodies(bytes.NewReader(out.Bytes()), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			actual, e := decoded.Bodies[0].Solid.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-10})
			if e != nil {
				t.Fatal(e)
			}
			if math.Abs(actual.Volume-want.Volume) > 1e-7 || math.Abs(actual.Area-want.Area) > 1e-7 || actual.Centroid.Distance(want.Centroid) > 1e-8 {
				t.Fatal(axis, angle, actual, want)
			}
		}
	}
}
func TestPlanarPolygonRevolveOCCT(t *testing.T) {
	t.Parallel()
	for _, axis := range []geom.Vec3{{X: 2}, {Y: 2}, {X: -2}} {
		for _, angle := range []float64{math.Pi / 2, -math.Pi / 2} {
			m := planarRevolveFixture(axis, angle)
			var out bytes.Buffer
			if e := WritePlanarPolygonRevolves(&out, m); e != nil {
				t.Fatal(e)
			}
			path := filepath.Join(t.TempDir(), "body.step")
			if e := os.WriteFile(path, out.Bytes(), 0600); e != nil {
				t.Fatal(e)
			}
			dest := filepath.Join(t.TempDir(), "mass.json")
			stepOracle(t, dest, "step-mass-precise", path)
			data, e := os.ReadFile(dest)
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
			b := m.Bodies[0]
			native, e := trimmed.RevolvePlanarPolygon(context.Background(), b.Points, b.AxisPoint, b.AxisDirection, b.Angle)
			if e != nil {
				t.Fatal(e)
			}
			want, e := native.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-10})
			if e != nil {
				t.Fatal(e)
			}
			r, h := 10., 20.
			if axis.Y != 0 {
				r, h = 20, 10
			}
			closed := math.Pi * r * r * h / 4
			if math.Abs(got.Volume-closed) > 1e-7 || math.Abs(got.Volume-want.Volume) > 1e-7 || math.Abs(got.Area-want.Area) > 1e-7 {
				t.Fatal(axis, angle, got, want)
			}
			c := geom.Vec3{X: got.Centroid[0], Y: got.Centroid[1], Z: got.Centroid[2]}
			if c.Distance(want.Centroid) > 1e-8 {
				t.Fatal(c, want.Centroid)
			}
		}
	}
}
func TestPlanarRectangleRejects(t *testing.T) {
	t.Parallel()
	m := planarRevolveFixture(geom.Vec3{Y: 1}, -1)
	var raw bytes.Buffer
	if e := WritePlanarPolygonRevolves(&raw, m); e != nil {
		t.Fatal(e)
	}
	p, e := trimmed.PreparePlanarPolygonRevolution(context.Background(), m.Bodies[0].Points, geom.Vec3{}, geom.Vec3{Y: 1})
	if e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"sign", "axis", "origin", "start"} {
		h := RectangleRevolveResolution{Frame: p.Frame, Negative: true}
		switch kind {
		case "sign":
			h.Negative = false
		case "axis":
			h.Frame, _ = geom.NewFrame(p.Frame.Origin(), p.Frame.XAxis(), p.Frame.ZAxis().Scale(-1))
		case "origin":
			h.Frame, _ = geom.NewFrame(p.Frame.Origin().Add(p.Frame.XAxis()), p.Frame.XAxis(), p.Frame.ZAxis())
		case "start":
			h.Frame, _ = geom.NewFrame(p.Frame.Origin(), p.Frame.YAxis(), p.Frame.ZAxis())
		}
		if _, e = ReadPlanarRectangleRevolvesWithResolution(bytes.NewReader(raw.Bytes()), Limits{}, h); e == nil {
			t.Fatal(kind)
		}
	}
	for _, kind := range []string{"tilt", "transverse", "cross", "full"} {
		bad := planarRevolveFixture(geom.Vec3{Y: 1}, 1)
		switch kind {
		case "tilt":
			bad.Bodies[0].AxisDirection.X = 1
		case "transverse":
			bad.Bodies[0].AxisDirection = geom.Vec3{Z: 1}
		case "cross":
			bad.Bodies[0].AxisPoint.X = 10
		case "full":
			bad.Bodies[0].Angle = 2 * math.Pi
		}
		var out bytes.Buffer
		if e = WritePlanarPolygonRevolves(&out, bad); e == nil || out.Len() != 0 {
			t.Fatal(kind)
		}
	}
	if e = WritePlanarPolygonRevolves(failWriter{}, m); e == nil {
		t.Fatal("write error lost")
	}
	for _, l := range []Limits{{MaxFaces: 1}, {MaxVertices: 1}, {MaxEdgeUses: 1}, {MaxBytes: 32}} {
		if _, e = ReadRectangleRevolves(bytes.NewReader(raw.Bytes()), l); e == nil {
			t.Fatal("limit ignored")
		}
	}
	for _, kind := range []string{"pcurve", "sense", "extra"} {
		f, e := Parse(bytes.NewReader(raw.Bytes()), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		for _, id := range f.Order {
			en := f.Entities[id]
			if len(en.Records) != 1 {
				continue
			}
			r := en.Records[0]
			changed := false
			switch kind {
			case "pcurve":
				if r.Type == "PCURVE" {
					r.Args[1] = Value{Kind: Reference, Ref: id}
					changed = true
				}
			case "sense":
				if r.Type == "ADVANCED_FACE" {
					sense, _ := boolean(r.Args[3])
					v := "T"
					if sense {
						v = "F"
					}
					r.Args[3] = Value{Kind: Enumeration, Text: v}
					changed = true
				}
			case "extra":
				if r.Type == "CYLINDRICAL_SURFACE" {
					newID := int64(10000)
					f.Entities[newID] = Entity{ID: newID, Records: []Record{r}}
					f.Order = append(f.Order, newID)
					changed = true
				}
			}
			if changed {
				break
			}
		}
		if _, e = ReadRectangleRevolves(bytes.NewReader(serializeWrappedProofFile(f)), Limits{}); e == nil {
			t.Fatal(kind)
		}
	}
}

func TestPlanarRectangleIndependentOCCT(t *testing.T) {
	t.Parallel()
	source := filepath.Join(t.TempDir(), "source.step")
	stepOracle(t, source, "write-step-cylinder-sector")
	data, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	h, e := geom.NewFrame(geom.Vec3{X: 1, Y: 2, Z: 1}, geom.Vec3{Y: 1}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadPlanarRectangleRevolvesWithResolution(bytes.NewReader(data), Limits{}, RectangleRevolveResolution{Frame: h, Negative: true})
	if e != nil {
		t.Fatal(e)
	}
	if math.Abs(m.Bodies[0].Angle+math.Pi/2) > 1e-10 {
		t.Fatal(m)
	}
	var out bytes.Buffer
	if e = WritePlanarPolygonRevolves(&out, m); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "resolved.step")
	if e = os.WriteFile(path, out.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	type measure struct {
		Area, Volume float64
		Centroid     [3]float64
	}
	var got [2]measure
	for i, p := range []string{source, path} {
		dest := filepath.Join(t.TempDir(), "mass.json")
		stepOracle(t, dest, "step-mass-precise", p)
		raw, e := os.ReadFile(dest)
		if e != nil {
			t.Fatal(e)
		}
		if e = json.Unmarshal(raw, &got[i]); e != nil {
			t.Fatal(e)
		}
	}
	if math.Abs(got[0].Area-got[1].Area) > 1e-8 || math.Abs(got[0].Volume-got[1].Volume) > 1e-8 {
		t.Fatal(got)
	}
	for i := 0; i < 3; i++ {
		if math.Abs(got[0].Centroid[i]-got[1].Centroid[i]) > 1e-8 {
			t.Fatal(got)
		}
	}
}
func TestPlanarPolygonNonrectangularFallback(t *testing.T) {
	t.Parallel()
	m := PlanarPolygonRevolveModel{LengthUnit: .001, Bodies: []PlanarPolygonRevolveBody{{Points: []geom.Vec3{{}, {X: 4}, {X: 4, Y: 3}, {X: 2, Y: 3}, {X: 2, Y: 1}, {Y: 1}}, AxisDirection: geom.Vec3{Y: 1}, Angle: -1}}}
	var out bytes.Buffer
	if e := WritePlanarPolygonRevolves(&out, m); e != nil {
		t.Fatal(e)
	}
	if _, e := ReadRectangleRevolves(bytes.NewReader(out.Bytes()), Limits{}); e == nil {
		t.Fatal("nonrectangular inverse accepted")
	}
	path := filepath.Join(t.TempDir(), "concave.step")
	if e := os.WriteFile(path, out.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	dest := filepath.Join(t.TempDir(), "mass.json")
	stepOracle(t, dest, "step-mass-precise", path)
	raw, e := os.ReadFile(dest)
	if e != nil {
		t.Fatal(e)
	}
	var got struct{ Volume float64 }
	if e = json.Unmarshal(raw, &got); e != nil {
		t.Fatal(e)
	}
	if math.Abs(got.Volume-20) > 1e-8 {
		t.Fatal(got)
	}
}
func FuzzReadRectangleRevolves(f *testing.F) {
	var raw bytes.Buffer
	if e := WritePlanarPolygonRevolves(&raw, planarRevolveFixture(geom.Vec3{Y: 1}, -1)); e != nil {
		f.Fatal(e)
	}
	f.Add(raw.Bytes())
	f.Add([]byte("ISO-10303-21;"))
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = ReadRectangleRevolves(bytes.NewReader(data), Limits{MaxBytes: 128 << 10, MaxEntities: 4000, MaxValues: 30000, MaxVertices: 256, MaxFaces: 128, MaxSolids: 4, MaxEdgeUses: 1024})
	})
}
