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

func annularFrustumFixture() WrappedAnnularFrustumBody {
	return WrappedAnnularFrustumBody{Name: "annular frustum", LengthUnit: .001, CapNames: [2]string{"lower annulus", "upper annulus"}, Outer: WrappedStationLoftBody{Stations: []trimmed.WrappedLoftStation{{Z: 0, Radius: 3}, {Z: 5, Radius: 4}}}, Inner: WrappedStationLoftBody{Stations: []trimmed.WrappedLoftStation{{Z: 0, Radius: 1}, {Z: 5, Radius: 1.5}}}}
}
func TestWrappedAnnularFrustum(t *testing.T) {
	slowtest.Skip(t)
	for _, kind := range []string{"both-taper", "outer-cylinder", "inner-cylinder", "narrowing"} {
		t.Run(kind, func(t *testing.T) {
			m := annularFrustumFixture()
			switch kind {
			case "outer-cylinder":
				m.Outer.Stations[1].Radius = 3
			case "inner-cylinder":
				m.Inner.Stations[1].Radius = 1
			case "narrowing":
				m.Outer.Stations[0].Radius = 4
				m.Outer.Stations[1].Radius = 3
				m.Inner.Stations[0].Radius = 1.5
				m.Inner.Stations[1].Radius = 1
			}
			var b bytes.Buffer
			if e := WriteWrappedAnnularFrustum(&b, m); e != nil {
				t.Fatal(e)
			}
			q, e := ReadWrappedAnnularFrustum(bytes.NewReader(b.Bytes()), Limits{}, WrappedSurfaceOptions{})
			if e != nil {
				t.Fatal(e)
			}
			if len(q.Outer.Walls) != 1 || len(q.Inner.Walls) != 1 || q.CapNames != m.CapNames {
				t.Fatal(q)
			}
			if _, e = ReadWrappedStationBore(bytes.NewReader(b.Bytes()), Limits{}, WrappedSurfaceOptions{}); e == nil {
				t.Fatal("station API broadened")
			}
			if _, e = ReadWrappedTube(bytes.NewReader(b.Bytes()), Limits{}, WrappedSurfaceOptions{}); e == nil {
				t.Fatal("tube API broadened")
			}
			native, e := WrappedAnnularFrustumFromSolid(context.Background(), q.Solid, "native", .001, geom.SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
			b.Reset()
			if e = WriteWrappedAnnularFrustum(&b, native); e != nil {
				t.Fatal(e)
			}
		})
	}
	for _, kind := range []string{"both-cylinder", "crossing", "height", "axis", "multi"} {
		t.Run(kind, func(t *testing.T) {
			m := annularFrustumFixture()
			switch kind {
			case "both-cylinder":
				m.Outer.Stations[1].Radius = 3
				m.Inner.Stations[1].Radius = 1
			case "crossing":
				m.Inner.Stations[1].Radius = 5
			case "height":
				m.Inner.Stations[1].Z = 6
			case "axis":
				m.Inner.Center = geom.Vec2{X: 1}
			case "multi":
				m.Inner.Stations = append(m.Inner.Stations, trimmed.WrappedLoftStation{Z: 6, Radius: 1})
			}
			var b bytes.Buffer
			if e := WriteWrappedAnnularFrustum(&b, m); e == nil || b.Len() != 0 {
				t.Fatal("invalid annular frustum emitted")
			}
		})
	}
	raw, e := os.ReadFile("testdata/wrapped-tube.step")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ReadWrappedAnnularFrustum(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{}); e == nil {
		t.Fatal("cylindrical tube reclassified")
	}
}
func TestWrappedAnnularFrustumOCCT(t *testing.T) {
	t.Parallel()
	source := t.TempDir() + "/source.step"
	stepOracle(t, source, "write-step-wrapped-annular-frustum")
	raw, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	q, e := ReadWrappedAnnularFrustum(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteWrappedAnnularFrustum(&b, q); e != nil {
		t.Fatal(e)
	}
	path := t.TempDir() + "/native.step"
	if e = os.WriteFile(path, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	volume := 5 * math.Pi * (9 + 12 + 16 - 1 - 1.5 - 2.25) / 3
	area := math.Pi * (7*math.Hypot(5, 1) + 2.5*math.Hypot(5, .5) + 9 - 1 + 16 - 2.25)
	for _, path := range []string{source, path} {
		out := t.TempDir() + "/mass.json"
		stepOracle(t, out, "step-mass-precise", path)
		data, e := os.ReadFile(out)
		if e != nil {
			t.Fatal(e)
		}
		var p struct{ Area, Volume float64 }
		if e = json.Unmarshal(data, &p); e != nil || math.Abs(p.Area-area) > 1e-7 || math.Abs(p.Volume-volume) > 1e-7 {
			t.Fatal(p, area, volume, e)
		}
	}
}

func TestIndependentWrappedAnnularFrustum(t *testing.T) {
	t.Parallel()
	for _, path := range []string{"testdata/wrapped-annular-frustum.step", "testdata/wrapped-tube.step"} {
		raw, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		m, e := ReadWrappedMaterials(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
		if e != nil || len(m.Bodies) != 1 {
			t.Fatal(m, e)
		}
		if path == "testdata/wrapped-tube.step" {
			if m.Bodies[0].Tube == nil || m.Bodies[0].AnnularFrustum != nil {
				t.Fatal("tube family changed")
			}
			continue
		}
		q := m.Bodies[0].AnnularFrustum
		if q == nil || q.Outer.Stations[1].Radius != 4 || q.Inner.Stations[1].Radius != 1.5 {
			t.Fatal(q)
		}
		f, e := Parse(bytes.NewReader(raw), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		for _, wall := range []WrappedSplineFace{q.Outer.Walls[0], q.Inner.Walls[0]} {
			for _, id := range wall.SideEdges {
				if _, e = simple(f, id, "EDGE_CURVE", 5); e != nil {
					t.Fatal("source edge", e)
				}
			}
		}
		// Even an unchanged geometric rim cannot declare the annular hole outer.
		for _, id := range f.Order {
			r, e := simple(f, id, "ADVANCED_FACE", 4)
			if e != nil {
				continue
			}
			sid, _ := ref(r.Args[2])
			if _, e = simple(f, sid, "PLANE", 2); e != nil {
				continue
			}
			bounds, _ := listRefs(r.Args[1], 2)
			for _, bid := range bounds {
				bound := f.Entities[bid].Records[0]
				lid, _ := ref(bound.Args[1])
				loop, _ := simple(f, lid, "EDGE_LOOP", 2)
				uses, _ := listRefs(loop.Args[1], 1)
				use, _ := simple(f, uses[0], "ORIENTED_EDGE", 5)
				sense, _ := boolean(use.Args[4])
				if !sense {
					bound.Type = "FACE_OUTER_BOUND"
					ovalSet(f, bid, bound)
				}
			}
		}
		if _, e = ReadWrappedAnnularFrustum(bytes.NewReader(serializeWrappedProofFile(f)), Limits{}, WrappedSurfaceOptions{}); e == nil {
			t.Fatal("invalid cap role")
		}
	}
}
