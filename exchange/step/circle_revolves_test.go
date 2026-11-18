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

func circleRevolveFixture(t *testing.T, angle float64) CircleRevolveModel {
	t.Helper()
	f, e := geom.NewFrame(geom.Vec3{X: 2, Y: -3, Z: 4}, geom.Vec3{Y: 1}, geom.Vec3{X: -1})
	if e != nil {
		t.Fatal(e)
	}
	return CircleRevolveModel{LengthUnit: .001, Bodies: []CircleRevolveBody{{Name: "signed disk", Frame: f, MajorRadius: 3, AxialCenter: 1.25, MinorRadius: .5, Angle: angle}}}
}
func TestCircleRevolveSignedRoundtrip(t *testing.T) {
	t.Parallel()
	for _, angle := range []float64{math.Pi / 2, -math.Pi / 2, 1.3 * math.Pi, -1.3 * math.Pi, 2 * math.Pi, -2 * math.Pi} {
		t.Run(real(angle), func(t *testing.T) {
			m := circleRevolveFixture(t, angle)
			b := m.Bodies[0]
			var raw bytes.Buffer
			if e := WriteCircleRevolves(&raw, m); e != nil {
				t.Fatal(e)
			}
			normal, e := ReadCircleRevolves(bytes.NewReader(raw.Bytes()), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			if normal.Bodies[0].Angle != math.Abs(angle) || normal.Bodies[0].AxialCenter != 0 {
				t.Fatal(normal)
			}
			got, e := ReadCircleRevolvesWithResolution(bytes.NewReader(raw.Bytes()), Limits{}, CircleRevolveResolution{Frame: b.Frame, Negative: angle < 0})
			if e != nil {
				t.Fatal(e)
			}
			gb := got.Bodies[0]
			if gb.Angle != angle || math.Abs(gb.AxialCenter-b.AxialCenter) > 1e-12 || gb.Frame != b.Frame || gb.Name != b.Name || got.LengthUnit != m.LengthUnit {
				t.Fatal(got)
			}
			native, e := trimmed.RevolveCircle(context.Background(), b.Frame, b.MajorRadius, b.AxialCenter, b.MinorRadius, b.Angle)
			if e != nil {
				t.Fatal(e)
			}
			mass, e := native.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-10})
			if e != nil {
				t.Fatal(e)
			}
			decoded, e := ReadAnalyticBodies(bytes.NewReader(raw.Bytes()), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			actual, e := decoded.Bodies[0].Solid.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-10})
			if e != nil {
				t.Fatal(e)
			}
			if math.Abs(actual.Volume-mass.Volume) > 1e-8 || math.Abs(actual.Area-mass.Area) > 1e-8 || actual.Centroid.Distance(mass.Centroid) > 1e-8 {
				t.Fatal(actual, mass)
			}
		})
	}
}
func TestCircleRevolveRejects(t *testing.T) {
	t.Parallel()
	m := circleRevolveFixture(t, -1)
	var raw bytes.Buffer
	if e := WriteCircleRevolves(&raw, m); e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"sign", "origin", "axis", "start"} {
		t.Run(kind, func(t *testing.T) {
			h := CircleRevolveResolution{Frame: m.Bodies[0].Frame, Negative: true}
			f := h.Frame
			switch kind {
			case "sign":
				h.Negative = false
			case "origin":
				h.Frame, _ = geom.NewFrame(f.Origin().Add(f.XAxis()), f.XAxis(), f.ZAxis())
			case "axis":
				h.Frame, _ = geom.NewFrame(f.Origin(), f.XAxis(), f.ZAxis().Scale(-1))
			case "start":
				h.Frame, _ = geom.NewFrame(f.Origin(), f.YAxis(), f.ZAxis())
			}
			if _, e := ReadCircleRevolvesWithResolution(bytes.NewReader(raw.Bytes()), Limits{}, h); e == nil {
				t.Fatal("invalid resolution accepted")
			}
		})
	}
	for _, kind := range []string{"pcurve", "sense", "extra"} {
		t.Run(kind, func(t *testing.T) {
			f, e := Parse(bytes.NewReader(raw.Bytes()), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			changed := false
			for _, id := range f.Order {
				en := f.Entities[id]
				if len(en.Records) != 1 {
					continue
				}
				r := en.Records[0]
				switch kind {
				case "pcurve":
					if r.Type == "PCURVE" {
						r.Args[1] = Value{Kind: Reference, Ref: id}
						changed = true
					}
				case "sense":
					if r.Type == "ADVANCED_FACE" {
						r.Args[3] = Value{Kind: Enumeration, Text: "F"}
						changed = true
					}
				case "extra":
					if r.Type == "TOROIDAL_SURFACE" {
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
			if !changed {
				t.Fatal("mutation absent")
			}
			if _, e = ReadCircleRevolvesWithResolution(bytes.NewReader(serializeWrappedProofFile(f)), Limits{}, CircleRevolveResolution{Frame: m.Bodies[0].Frame, Negative: true}); e == nil {
				t.Fatal("altered source accepted")
			}
		})
	}
	for _, a := range []float64{0, 7, -7, math.NaN()} {
		bad := circleRevolveFixture(t, a)
		var out bytes.Buffer
		if e := WriteCircleRevolves(&out, bad); e == nil || out.Len() != 0 {
			t.Fatal("invalid angle accepted")
		}
	}
	if e := WriteCircleRevolves(failWriter{}, m); e == nil {
		t.Fatal("lost write error")
	}
	for _, l := range []Limits{{MaxFaces: 1}, {MaxVertices: 1}, {MaxEdgeUses: 1}, {MaxBytes: 32}} {
		if _, e := ReadCircleRevolves(bytes.NewReader(raw.Bytes()), l); e == nil {
			t.Fatal("ignored limit")
		}
	}
}
func TestCircleRevolveSignedOCCT(t *testing.T) {
	t.Parallel()
	for _, angle := range []float64{-math.Pi / 2, -1.3 * math.Pi, -2 * math.Pi} {
		t.Run(real(angle), func(t *testing.T) {
			m := circleRevolveFixture(t, angle)
			var raw bytes.Buffer
			if e := WriteCircleRevolves(&raw, m); e != nil {
				t.Fatal(e)
			}
			got, e := ReadCircleRevolvesWithResolution(bytes.NewReader(raw.Bytes()), Limits{}, CircleRevolveResolution{Frame: m.Bodies[0].Frame, Negative: true})
			if e != nil {
				t.Fatal(e)
			}
			raw.Reset()
			if e = WriteCircleRevolves(&raw, got); e != nil {
				t.Fatal(e)
			}
			path := filepath.Join(t.TempDir(), "body.step")
			if e = os.WriteFile(path, raw.Bytes(), 0600); e != nil {
				t.Fatal(e)
			}
			dest := filepath.Join(t.TempDir(), "mass.json")
			stepOracle(t, dest, "step-mass-precise", path)
			data, e := os.ReadFile(dest)
			if e != nil {
				t.Fatal(e)
			}
			var actual struct {
				Area, Volume float64
				Centroid     [3]float64
			}
			if e = json.Unmarshal(data, &actual); e != nil {
				t.Fatal(e)
			}
			b := m.Bodies[0]
			s, e := trimmed.RevolveCircle(context.Background(), b.Frame, b.MajorRadius, b.AxialCenter, b.MinorRadius, b.Angle)
			if e != nil {
				t.Fatal(e)
			}
			mass, e := s.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-10})
			if e != nil {
				t.Fatal(e)
			}
			c := geom.Vec3{X: actual.Centroid[0], Y: actual.Centroid[1], Z: actual.Centroid[2]}
			if math.Abs(actual.Volume-mass.Volume) > 1e-8 || math.Abs(actual.Area-mass.Area) > 1e-8 || c.Distance(mass.Centroid) > 1e-8 {
				t.Fatal(actual, mass)
			}
		})
	}
}

func TestCircleRevolveIndependentResolution(t *testing.T) {
	t.Parallel()
	// This independently generated positive-chart elbow also has a valid negative
	// feature description starting at its other cap. The hint explicitly supplies
	// that history; it is never guessed from the STEP material graph.
	source := filepath.Join(t.TempDir(), "source.step")
	stepOracle(t, source, "write-step-torus-elbow")
	data, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	frame, e := geom.NewFrame(geom.Vec3{X: 1, Y: 2, Z: 1.75}, geom.Vec3{Y: 1}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadCircleRevolvesWithResolution(bytes.NewReader(data), Limits{}, CircleRevolveResolution{Frame: frame, Negative: true})
	if e != nil {
		t.Fatal(e)
	}
	b := m.Bodies[0]
	if math.Abs(b.Angle+math.Pi/2) > 1e-10 || b.AxialCenter != 1.25 || b.MajorRadius != 3 || b.MinorRadius != 1 {
		t.Fatal(b)
	}
	var out bytes.Buffer
	if e = WriteCircleRevolves(&out, m); e != nil {
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
func TestCircleRevolveUnrepresentableResolution(t *testing.T) {
	t.Parallel()
	m := circleRevolveFixture(t, 1)
	var raw bytes.Buffer
	if e := WriteCircleRevolves(&raw, m); e != nil {
		t.Fatal(e)
	}
	b := m.Bodies[0]
	frame, e := geom.NewFrame(b.Frame.Origin().Add(b.Frame.ZAxis().Scale(1e100)), b.Frame.XAxis(), b.Frame.ZAxis())
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ReadCircleRevolvesWithResolution(&raw, Limits{}, CircleRevolveResolution{Frame: frame}); e == nil {
		t.Fatal("rounded axial cancellation accepted")
	}
}
func FuzzReadCircleRevolves(f *testing.F) {
	frame, e := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if e != nil {
		f.Fatal(e)
	}
	var raw bytes.Buffer
	if e = WriteCircleRevolves(&raw, CircleRevolveModel{LengthUnit: .001, Bodies: []CircleRevolveBody{{Frame: frame, MajorRadius: 3, MinorRadius: .5, AxialCenter: .25, Angle: -1}}}); e != nil {
		f.Fatal(e)
	}
	f.Add(raw.Bytes())
	f.Add([]byte("ISO-10303-21;"))
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = ReadCircleRevolvesWithResolution(bytes.NewReader(data), Limits{MaxBytes: 128 << 10, MaxEntities: 4000, MaxValues: 30000, MaxVertices: 256, MaxFaces: 128, MaxSolids: 4, MaxEdgeUses: 1024}, CircleRevolveResolution{Frame: frame, Negative: true})
	})
}
