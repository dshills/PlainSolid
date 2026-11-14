package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"math/big"
	"os"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"reflect"
	"testing"
)

func rationalStationBoreSTEPFixture(t *testing.T, single ...bool) WrappedRationalStationBoreBody {
	t.Helper()
	n := 2
	if len(single) > 0 && single[0] {
		n = 1
	}
	outer := rationalStationSTEPFixture(t, n)
	xy := [][2]float64{{1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1}, {0, -1}, {1, -1}, {1, 0}}
	var poles []geom.Vec3
	for _, p := range xy {
		poles = append(poles, geom.Vec3{X: .25 + 5*p[0], Y: -.5 + 5*p[1]})
	}
	profile, e := geom.NewBSpline(2, poles, []float64{-2, -2, -2, -1, -1, 1, 1, 4, 4, 8, 8, 8}, []float64{1, .5, 1, .75, 1, .75, 1, .75, 1})
	if e != nil {
		t.Fatal(e)
	}
	r := big.NewRat
	stations := []trimmed.WrappedRationalProfileStation{{Height: r(0, 1), Scale: r(1, 10)}, {Height: r(2, 1), Scale: r(3, 20)}, {Height: r(3, 1), Scale: r(1, 10)}}
	if n == 1 {
		stations = stations[:2]
		stations[1].Height = r(1, 1)
	}
	inner, _, e := trimmed.LoftWrappedRationalProfileStations(context.Background(), profile, stations, 1e-10, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	inside, e := WrappedRationalStationLoftFromSolid(context.Background(), inner, "Tool", .001, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	solid, _, e := trimmed.CombineCoaxialWrappedRationalStationLofts(context.Background(), outer.Solid, inner, parametric.Difference, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	return WrappedRationalStationBoreBody{Name: "Weighted bore", Outer: outer, Inner: inside, CapNames: [2]string{"Lower annulus", "Upper annulus"}, Solid: solid, LengthUnit: .001}
}

func TestWrappedRationalStationBoreSTEP(t *testing.T) {
	m := rationalStationBoreSTEPFixture(t)
	var raw bytes.Buffer
	if e := WriteWrappedRationalStationBore(&raw, m); e != nil {
		t.Fatal(e)
	}
	got, e := ReadWrappedRationalStationBore(bytes.NewReader(raw.Bytes()), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if got.Name != m.Name || got.CapNames != m.CapNames {
		t.Fatal(got.Name, got.CapNames)
	}
	originalGraph, err := Parse(bytes.NewReader(raw.Bytes()), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	for _, side := range []WrappedRationalStationLoftBody{got.Outer, got.Inner} {
		for i, w := range side.Walls {
			for _, id := range w.SideEdges {
				if _, err := simple(originalGraph, id, "EDGE_CURVE", 5); err != nil {
					t.Fatal("lost original edge ID", id, err)
				}
			}
			for _, id := range w.CornerVertices {
				if _, err := simple(originalGraph, id, "VERTEX_POINT", 2); err != nil {
					t.Fatal("lost original vertex ID", id, err)
				}
			}
			if i > 0 && side.Walls[i-1].SideEdges[2] != w.SideEdges[0] {
				t.Fatal("unshared source station")
			}
		}
	}
	for i, pair := range [][2]WrappedRationalStationLoftBody{{m.Outer, got.Outer}, {m.Inner, got.Inner}} {
		if len(pair[0].Walls) != len(pair[1].Walls) {
			t.Fatal("lost original stations", i)
		}
		for j, w := range pair[0].Walls {
			a, _ := w.Geometry.ExportRecipe()
			b, _ := pair[1].Walls[j].Geometry.ExportRecipe()
			if !reflect.DeepEqual(a.Surface.Data(), b.Surface.Data()) {
				t.Fatal("original wall changed", i, j)
			}
		}
	}
	p, e := got.Solid.ValidateWrappedRationalStationBore(context.Background(), geom.SurfacePatchOptions{})
	if e != nil || !p.Complete {
		t.Fatal(p, e)
	}
	faces := got.Solid.Faces()
	for _, id := range p.InnerWalls {
		if !faces[id].Reversed {
			t.Fatal("outward hole")
		}
	}
	var again bytes.Buffer
	if e = WriteWrappedRationalStationBore(&again, got); e != nil || !bytes.Equal(raw.Bytes(), again.Bytes()) {
		t.Fatal("nondeterministic", e)
	}
	if _, e = ReadWrappedStationBore(bytes.NewReader(raw.Bytes()), Limits{}, WrappedSurfaceOptions{}); e == nil {
		t.Fatal("circular reader widened")
	}
	var native bytes.Buffer
	if e = trimmed.WriteJSON(&native, got.Solid); e != nil {
		t.Fatal(e)
	}
	loaded, e := trimmed.ReadJSONContext(context.Background(), &native, trimmed.JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	recovered, e := WrappedRationalStationBoreFromSolid(context.Background(), loaded, "Restored", .001, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	again.Reset()
	if e = WriteWrappedRationalStationBore(&again, recovered); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadWrappedRationalStationBore(&again, Limits{}, WrappedSurfaceOptions{}); e != nil {
		t.Fatal(e)
	}
	for _, mutation := range []string{"inner orientation", "annulus hole", "pcurve owner"} {
		t.Run(mutation, func(t *testing.T) {
			f, e := Parse(bytes.NewReader(raw.Bytes()), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			changed := false
			for _, id := range f.Order {
				if mutation == "inner orientation" {
					if rec, e := simple(f, id, "ADVANCED_FACE", 4); e == nil {
						sid, _ := ref(rec.Args[2])
						sense, _ := boolean(rec.Args[3])
						if _, ok := component(f.Entities[sid], "B_SPLINE_SURFACE_WITH_KNOTS"); ok && !sense {
							rec.Args[3] = Value{Kind: Enumeration, Text: "T"}
							ovalSet(f, id, rec)
							changed = true
							break
						}
					}
				}
				if mutation == "annulus hole" {
					if rec, e := simple(f, id, "ADVANCED_FACE", 4); e == nil && len(rec.Args[1].Items) == 2 {
						rec.Args[1].Items = rec.Args[1].Items[:1]
						ovalSet(f, id, rec)
						changed = true
						break
					}
				}
				if mutation == "pcurve owner" {
					if rec, e := simple(f, id, "PCURVE", 3); e == nil {
						rec.Args[1] = Value{Kind: Reference, Ref: got.Outer.Walls[0].SideEdges[0]}
						ovalSet(f, id, rec)
						changed = true
						break
					}
				}
			}
			if !changed {
				t.Fatal("no mutation")
			}
			if _, e = ReadWrappedRationalStationBore(bytes.NewReader(serializeWrappedProofFile(f)), Limits{}, WrappedSurfaceOptions{}); e == nil {
				t.Fatal("mutation accepted")
			}
		})
	}
	bad := m
	bad.Inner = m.Outer
	again.Reset()
	if e = WriteWrappedRationalStationBore(&again, bad); e == nil || again.Len() != 0 {
		t.Fatal("zero clearance or partial write", e)
	}
	if _, e = ReadWrappedRationalStationBore(bytes.NewReader(raw.Bytes()), Limits{MaxFaces: 4}, WrappedSurfaceOptions{}); e == nil {
		t.Fatal("ignored face quota")
	}
}

func TestWrappedRationalStationBoreSTEPOCCT(t *testing.T) {
	if os.Getenv("PLAINSOLID_OCCT_ORACLE") == "" {
		t.Skip("set PLAINSOLID_OCCT_ORACLE and PLAINSOLID_OCCT_STEP=1")
	}
	m := rationalStationBoreSTEPFixture(t)
	var raw bytes.Buffer
	if e := WriteWrappedRationalStationBore(&raw, m); e != nil {
		t.Fatal(e)
	}
	path := t.TempDir() + "/weighted-bore.step"
	if e := os.WriteFile(path, raw.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	out := t.TempDir() + "/mass.json"
	stepOracle(t, out, "step-mass-precise", path)
	b, e := os.ReadFile(out)
	if e != nil {
		t.Fatal(e)
	}
	var p struct {
		Volume   float64
		Centroid [3]float64
	}
	if e = json.Unmarshal(b, &p); e != nil {
		t.Fatal(e)
	}
	if math.Abs(p.Volume-125./16*rationalStationSTEPArea()) > 1e-6 || math.Abs(p.Centroid[2]-383./300) > 1e-7 {
		t.Fatal(p)
	}
	t.Logf("independent OCCT volume %.12g centroid %v", p.Volume, p.Centroid)
}

func TestWrappedRationalStationBoreSTEPSingle(t *testing.T) {
	m := rationalStationBoreSTEPFixture(t, true)
	var raw bytes.Buffer
	if e := WriteWrappedRationalStationBore(&raw, m); e != nil {
		t.Fatal(e)
	}
	got, e := ReadWrappedRationalStationBore(bytes.NewReader(raw.Bytes()), Limits{}, WrappedSurfaceOptions{})
	if e != nil || len(got.Outer.Walls) != 1 || len(got.Inner.Walls) != 1 {
		t.Fatal(got, e)
	}
	if _, e = ReadWrappedRationalStationBore(bytes.NewReader(raw.Bytes()), Limits{}, WrappedSurfaceOptions{MaxOperations: 1}); e == nil {
		t.Fatal("work quota ignored")
	}
}
