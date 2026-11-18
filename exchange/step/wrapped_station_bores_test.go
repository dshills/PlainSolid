package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func stationBoreFixture() WrappedStationBoreBody {
	return WrappedStationBoreBody{Name: "station bore", LengthUnit: .001, CapNames: [2]string{"lower annulus", "upper annulus"}, Outer: WrappedStationLoftBody{Center: geom.Vec2{}, Stations: []trimmed.WrappedLoftStation{{Z: 0, Radius: 3}, {Z: 2, Radius: 4}, {Z: 5, Radius: 3}}}, Inner: WrappedStationLoftBody{Center: geom.Vec2{}, Stations: []trimmed.WrappedLoftStation{{Z: 0, Radius: 1}, {Z: 2, Radius: 1.5}, {Z: 5, Radius: 1}}}}
}
func TestWrappedStationBore(t *testing.T) {
	slowtest.Skip(t)
	var b bytes.Buffer
	if e := WriteWrappedStationBore(&b, stationBoreFixture()); e != nil {
		t.Fatal(e)
	}
	q, e := ReadWrappedStationBore(bytes.NewReader(b.Bytes()), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if q.Name != "station bore" || len(q.Outer.Walls) != 2 || len(q.Inner.Walls) != 2 || q.CapNames[1] != "upper annulus" {
		t.Fatal(q)
	}
	native, e := WrappedStationBoreFromSolid(context.Background(), q.Solid, "native", .001, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	b.Reset()
	if e = WriteWrappedStationBore(&b, native); e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"crossing", "height", "axis"} {
		t.Run(kind, func(t *testing.T) {
			m := stationBoreFixture()
			switch kind {
			case "crossing":
				m.Inner.Stations[1].Radius = 5
			case "height":
				m.Inner.Stations[2].Z = 6
			case "axis":
				m.Inner.Center.X = 1
			}
			var out bytes.Buffer
			if e := WriteWrappedStationBore(&out, m); e == nil || out.Len() != 0 {
				t.Fatal("invalid bore emitted")
			}
		})
	}
}

func TestWrappedStationBoreSourceRejections(t *testing.T) {
	slowtest.Skip(t)
	var b bytes.Buffer
	if e := WriteWrappedStationBore(&b, stationBoreFixture()); e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"inner-sense", "cap-use", "cap-role", "extra-edge", "extra-shell", "duplicate-face"} {
		t.Run(kind, func(t *testing.T) {
			f, e := Parse(bytes.NewReader(b.Bytes()), Limits{})
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
				case "inner-sense":
					if r.Type == "ADVANCED_FACE" && r.Args[3].Text == "F" {
						sid, _ := ref(r.Args[2])
						if _, ok := component(f.Entities[sid], "B_SPLINE_SURFACE_WITH_KNOTS"); ok {
							r.Args[3].Text = "T"
							ovalSet(f, id, r)
							changed = true
						}
					}
				case "cap-use", "cap-role":
					if r.Type == "ADVANCED_FACE" {
						sid, _ := ref(r.Args[2])
						if _, e := simple(f, sid, "PLANE", 2); e == nil {
							bounds, _ := listRefs(r.Args[1], 2)
							br := f.Entities[bounds[1]].Records[0]
							if kind == "cap-role" {
								br.Type = "FACE_OUTER_BOUND"
								ovalSet(f, bounds[1], br)
								changed = true
								break
							}
							lid, _ := ref(br.Args[1])
							loop, _ := simple(f, lid, "EDGE_LOOP", 2)
							uses, _ := listRefs(loop.Args[1], 1)
							use := f.Entities[uses[0]].Records[0]
							use.Args[4].Text = "T"
							ovalSet(f, uses[0], use)
							changed = true
						}
					}
				case "extra-edge", "extra-shell":
					typ := "EDGE_CURVE"
					if kind == "extra-shell" {
						typ = "CLOSED_SHELL"
					}
					if r.Type == typ {
						nid := f.Order[len(f.Order)-1] + 1
						en.ID = nid
						f.Entities[nid] = en
						f.Order = append(f.Order, nid)
						changed = true
					}
				case "duplicate-face":
					if r.Type == "CLOSED_SHELL" {
						r.Args[1].Items[1] = r.Args[1].Items[0]
						ovalSet(f, id, r)
						changed = true
					}
				}
				if changed {
					break
				}
			}
			if !changed {
				t.Fatal("mutation missed")
			}
			if _, e = ReadWrappedStationBore(bytes.NewReader(serializeWrappedProofFile(f)), Limits{}, WrappedSurfaceOptions{}); e == nil {
				t.Fatal("malformed source accepted")
			}
		})
	}
	if _, e := ReadWrappedStationBore(bytes.NewReader(b.Bytes()), Limits{MaxFaces: 5}, WrappedSurfaceOptions{}); e == nil {
		t.Fatal("face limit")
	}
	if _, e := ReadWrappedStationBore(bytes.NewReader(b.Bytes()), Limits{}, WrappedSurfaceOptions{MaxSurfaces: 3}); e == nil {
		t.Fatal("surface limit")
	}
}

func TestWrappedStationBoreOCCT(t *testing.T) {
	t.Parallel()
	source := t.TempDir() + "/source.step"
	stepOracle(t, source, "write-step-wrapped-station-bore")
	raw, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	imported, e := ReadWrappedStationBore(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(imported.Outer.Walls) != 2 || len(imported.Inner.Walls) != 2 {
		t.Fatal("band counts")
	}
	native, e := WrappedStationBoreFromSolid(context.Background(), imported.Solid, "native", .001, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteWrappedStationBore(&b, native); e != nil {
		t.Fatal(e)
	}
	path := t.TempDir() + "/native.step"
	if e = os.WriteFile(path, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	volume, area := 0., 0.
	for _, side := range []struct {
		r    []float64
		sign float64
	}{{[]float64{3, 4, 3}, 1}, {[]float64{1, 1.5, 1}, -1}} {
		for i, h := range []float64{2, 3} {
			a, c := side.r[i], side.r[i+1]
			volume += side.sign * math.Pi * h * (a*a + a*c + c*c) / 3
			area += math.Pi * (a + c) * math.Hypot(h, c-a)
		}
	}
	area += 2 * math.Pi * (9 - 1)
	for _, p := range []string{source, path} {
		out := t.TempDir() + "/mass.json"
		stepOracle(t, out, "step-mass-precise", p)
		data, e := os.ReadFile(out)
		if e != nil {
			t.Fatal(e)
		}
		var got struct{ Area, Volume float64 }
		if e = json.Unmarshal(data, &got); e != nil {
			t.Fatal(e)
		}
		if math.Abs(got.Area-area) > 1e-7 || math.Abs(got.Volume-volume) > 1e-7 {
			t.Fatal(got, area, volume)
		}
	}
}

func TestIndependentWrappedStationBore(t *testing.T) {
	slowtest.Skip(t)
	raw, e := os.ReadFile("testdata/wrapped-station-bore.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadWrappedStationBore(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	f, e := Parse(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	for _, body := range []WrappedStationLoftBody{m.Outer, m.Inner} {
		if len(body.Walls) != 2 {
			t.Fatal("bands")
		}
		if body.Walls[0].SideEdges[2] != body.Walls[1].SideEdges[0] {
			t.Fatal("lost shared rim")
		}
		for _, w := range body.Walls {
			for _, id := range w.SideEdges {
				if _, e = simple(f, id, "EDGE_CURVE", 5); e != nil {
					t.Fatal("lost source edge", e)
				}
			}
			for _, id := range w.CornerVertices {
				if _, e = simple(f, id, "VERTEX_POINT", 2); e != nil {
					t.Fatal("lost source vertex", e)
				}
			}
		}
	}
}

func TestWrappedStationBoreIndependentPartitions(t *testing.T) {
	slowtest.Skip(t)
	m := stationBoreFixture()
	m.Inner.Stations = []trimmed.WrappedLoftStation{{Z: 0, Radius: 1}, {Z: 1, Radius: 1.25}, {Z: 2, Radius: 1.5}, {Z: 5, Radius: 1}}
	var b bytes.Buffer
	if e := WriteWrappedStationBore(&b, m); e != nil {
		t.Fatal(e)
	}
	got, e := ReadWrappedStationBore(&b, Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(got.Outer.Walls) != 2 || len(got.Inner.Walls) != 3 {
		t.Fatal("source partition changed")
	}
}

func FuzzWrappedStationBore(f *testing.F) {
	raw, e := os.ReadFile("testdata/wrapped-station-bore.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(raw)
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = ReadWrappedStationBore(bytes.NewReader(data), Limits{MaxBytes: 256 << 10, MaxEntities: 1024, MaxFaces: 16, MaxVertices: 512, MaxEdgeUses: 128}, WrappedSurfaceOptions{MaxSurfaces: 8, MaxPoles: 256, MaxOperations: 2000000})
	})
}

func TestWrappedStationBoreSingleWall(t *testing.T) {
	slowtest.Skip(t)
	for _, innerSingle := range []bool{true, false} {
		m := stationBoreFixture()
		if innerSingle {
			m.Inner.Stations = []trimmed.WrappedLoftStation{{Z: 0, Radius: 1}, {Z: 5, Radius: 1.5}}
		} else {
			m.Outer.Stations = []trimmed.WrappedLoftStation{{Z: 0, Radius: 3}, {Z: 5, Radius: 4}}
		}
		var b bytes.Buffer
		if e := WriteWrappedStationBore(&b, m); e != nil {
			t.Fatal(innerSingle, e)
		}
		got, e := ReadWrappedStationBore(bytes.NewReader(b.Bytes()), Limits{}, WrappedSurfaceOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if len(got.Outer.Walls)+len(got.Inner.Walls) != 3 {
			t.Fatal("bands")
		}
		mixed, e := ReadWrappedMaterials(bytes.NewReader(b.Bytes()), Limits{}, WrappedSurfaceOptions{})
		if e != nil || len(mixed.Bodies) != 1 || mixed.Bodies[0].StationBore == nil {
			t.Fatal(mixed, e)
		}
		native, e := WrappedStationBoreFromSolid(context.Background(), got.Solid, "native", .001, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if len(native.Outer.Walls) != len(got.Outer.Walls) || len(native.Inner.Walls) != len(got.Inner.Walls) {
			t.Fatal("native partition changed", len(native.Outer.Walls), len(native.Inner.Walls))
		}
		b.Reset()
		if e = WriteWrappedStationBore(&b, native); e != nil {
			t.Fatal(e)
		}
	}
	m := stationBoreFixture()
	m.Inner.Stations = m.Inner.Stations[:2]
	m.Outer.Stations = m.Outer.Stations[:2]
	var b bytes.Buffer
	if e := WriteWrappedStationBore(&b, m); e == nil || b.Len() != 0 {
		t.Fatal("both single broadened")
	}
}

func TestWrappedStationBoreSingleOCCT(t *testing.T) {
	t.Parallel()
	source := t.TempDir() + "/source.step"
	stepOracle(t, source, "write-step-wrapped-station-bore-single")
	raw, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	got, e := ReadWrappedStationBore(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(got.Outer.Walls) != 2 || len(got.Inner.Walls) != 1 || got.Inner.Stations[1].Radius != 1.5 {
		t.Fatal("single-band source")
	}
	var b bytes.Buffer
	if e = WriteWrappedStationBore(&b, got); e != nil {
		t.Fatal(e)
	}
	path := t.TempDir() + "/native.step"
	if e = os.WriteFile(path, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	volume := math.Pi * (5*(9+12+16)/3. - 5*(1+1.5+2.25)/3.)
	area := math.Pi*7*(math.Sqrt(5)+math.Sqrt(10)) + math.Pi*2.5*math.Hypot(5, .5) + math.Pi*(9-1+9-2.25)
	for _, p := range []string{source, path} {
		out := t.TempDir() + "/mass.json"
		stepOracle(t, out, "step-mass-precise", p)
		raw, e := os.ReadFile(out)
		if e != nil {
			t.Fatal(e)
		}
		var mass struct{ Area, Volume float64 }
		if e = json.Unmarshal(raw, &mass); e != nil || math.Abs(mass.Volume-volume) > 1e-7 || math.Abs(mass.Area-area) > 1e-7 {
			t.Fatal(mass, area, volume, e)
		}
	}
}

func TestIndependentWrappedStationBoreSingle(t *testing.T) {
	t.Parallel()
	raw, e := os.ReadFile("testdata/wrapped-station-bore-single.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadWrappedStationBore(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Outer.Walls) != 2 || len(m.Inner.Walls) != 1 || m.Inner.Stations[1].Radius != 1.5 {
		t.Fatal("independent source")
	}
	f, e := Parse(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	for _, id := range m.Inner.Walls[0].SideEdges {
		if _, e = simple(f, id, "EDGE_CURVE", 5); e != nil {
			t.Fatal("source ancestry", e)
		}
	}
}
