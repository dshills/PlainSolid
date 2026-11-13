package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"plainsolid/brep/trimmed"
	"plainsolid/csg/fillet"
	"plainsolid/geom"
	"testing"
)

func longEdgeChainFixture() LongEdgeChainModel {
	return LongEdgeChainModel{LengthUnit: .001, Bodies: []LongEdgeChainBody{{LongEdgeTreatmentBody: LongEdgeTreatmentBody{Name: "PC5 actual chain", Length: 20, Width: 10, Height: 10, Radius: 1, TopSetback: 1, SideSetback: 1}, Bore: trimmed.ExtrusionCircle{Center: geom.Vec2{X: 5, Y: 5}, Radius: 1}, Pocket: trimmed.ExtrusionRectangle{Min: geom.Vec2{X: 10, Y: 5}, Max: geom.Vec2{X: 13, Y: 8}}, Boss: trimmed.ExtrusionBoss{Footprint: trimmed.ExtrusionRectangle{Min: geom.Vec2{X: 15, Y: 5}, Max: geom.Vec2{X: 17, Y: 7}}, Height: 3, Upper: true}}}}
}
func TestLongEdgeChainTopology(t *testing.T) {
	m := longEdgeChainFixture()
	b := m.Bodies[0]
	for _, full := range []bool{false, true} {
		var raw bytes.Buffer
		var source trimmed.Solid
		if full {
			r, e := fillet.TreatLongCapEdgesWithFeatures(context.Background(), b.Length, b.Width, b.Height, b.Radius, b.TopSetback, b.SideSetback, b.Bore, b.Pocket, b.Boss, fillet.Options{})
			if e != nil {
				t.Fatal(e)
			}
			source = r.Solid
			if e = WriteLongEdgeChains(&raw, m); e != nil {
				t.Fatal(full, e)
			}
		} else {
			r, e := fillet.TreatOppositeLongCapEdges(context.Background(), b.Length, b.Width, b.Height, b.Radius, b.TopSetback, b.SideSetback, fillet.Options{})
			if e != nil {
				t.Fatal(e)
			}
			source = r.Solid
			if e = WriteLongEdgeTreatments(&raw, LongEdgeTreatmentModel{LengthUnit: m.LengthUnit, Bodies: []LongEdgeTreatmentBody{b.LongEdgeTreatmentBody}}); e != nil {
				t.Fatal(full, e)
			}
		}
		f, e := Parse(&raw, Limits{})
		if e != nil {
			t.Fatal(e)
		}
		counts := map[string]int{}
		id := int64(0)
		for _, v := range f.Order {
			for _, r := range f.Entities[v].Records {
				counts[r.Type]++
				if r.Type == "MANIFOLD_SOLID_BREP" {
					id = v
				}
			}
		}
		if counts["ADVANCED_FACE"] != len(source.Faces()) || counts["EDGE_CURVE"] != len(source.Edges()) || counts["VERTEX_POINT"] != len(source.Vertices()) || counts["PCURVE"] != 2*len(source.Edges()) {
			t.Fatal(counts)
		}

		vertexOrder := []int64{}
		edgeOrder := []int64{}
		for _, sid := range f.Order {
			en := f.Entities[sid]
			if len(en.Records) != 1 {
				continue
			}
			switch en.Records[0].Type {
			case "VERTEX_POINT":
				vertexOrder = append(vertexOrder, sid)
			case "EDGE_CURVE":
				edgeOrder = append(edgeOrder, sid)
			}
		}
		for i, p := range source.Vertices() {
			q, e := analyticVertex(f, vertexOrder[i])
			if e != nil || p != q {
				t.Fatal("changed native vertex", i, p, q, e)
			}
		}
		for i, edge := range source.Edges() {
			r, e := simple(f, edgeOrder[i], "EDGE_CURVE", 5)
			if e != nil {
				t.Fatal(e)
			}
			a, _ := ref(r.Args[1])
			z, _ := ref(r.Args[2])
			if a != vertexOrder[edge.Start] || z != vertexOrder[edge.End] {
				t.Fatal("changed native edge identity", i)
			}
		}
		// Bounds carry native face reversal; verify the complete emitted
		// directed graph without assuming EDGE_CURVE.same_sense is true.
		ownership := map[int64][]bool{}
		negative := 0
		for _, eid := range f.Order {
			en := f.Entities[eid]
			if len(en.Records) != 1 {
				continue
			}
			rec := en.Records[0]
			if rec.Type == "EDGE_CURVE" {
				same, _ := boolean(rec.Args[4])
				if !same {
					negative++
				}
			}
			if rec.Type != "ADVANCED_FACE" {
				continue
			}
			face, e := readCircularSourceFace(f, eid)
			if e != nil {
				t.Fatal(e)
			}
			for _, loop := range face.loops {
				for k, use := range loop {
					ownership[use.id] = append(ownership[use.id], use.sense)
					er, e := simple(f, use.id, "EDGE_CURVE", 5)
					if e != nil {
						t.Fatal(e)
					}
					a, _ := ref(er.Args[1])
					z, _ := ref(er.Args[2])
					end := z
					if !use.sense {
						end = a
					}
					next := loop[(k+1)%len(loop)]
					nr, e := simple(f, next.id, "EDGE_CURVE", 5)
					if e != nil {
						t.Fatal(e)
					}
					start, _ := ref(nr.Args[1])
					if !next.sense {
						start, _ = ref(nr.Args[2])
					}
					if start != end {
						t.Fatal("broken source identity")
					}
				}
			}
		}
		if full && negative == 0 {
			t.Fatal("lost clockwise bore curve direction")
		}
		if len(ownership) != len(source.Edges()) {
			t.Fatal("unowned native edge")
		}
		for _, uses := range ownership {
			if len(uses) != 2 || uses[0] == uses[1] {
				t.Fatal("nonmanifold emitted edge")
			}
		}
		if !full {
			if _, e = readCircularSourceGraph(f, id); e != nil {
				t.Fatal(full, e)
			}
		}
	}
}
func TestLongEdgeChainOCCT(t *testing.T) {
	m := longEdgeChainFixture()
	b := m.Bodies[0]
	for _, full := range []bool{false, true} {
		var raw bytes.Buffer
		area, volume := 0., 0.
		if full {
			r, e := fillet.TreatLongCapEdgesWithFeatures(context.Background(), b.Length, b.Width, b.Height, b.Radius, b.TopSetback, b.SideSetback, b.Bore, b.Pocket, b.Boss, fillet.Options{})
			if e != nil {
				t.Fatal(e)
			}
			area, volume = r.NominalSurfaceArea, r.NominalVolume
			if e = WriteLongEdgeChains(&raw, m); e != nil {
				t.Fatal(full, e)
			}
		} else {
			r, e := fillet.TreatOppositeLongCapEdges(context.Background(), b.Length, b.Width, b.Height, b.Radius, b.TopSetback, b.SideSetback, fillet.Options{})
			if e != nil {
				t.Fatal(e)
			}
			area, volume = r.NominalSurfaceArea, r.NominalVolume
			if e = WriteLongEdgeTreatments(&raw, LongEdgeTreatmentModel{LengthUnit: m.LengthUnit, Bodies: []LongEdgeTreatmentBody{b.LongEdgeTreatmentBody}}); e != nil {
				t.Fatal(full, e)
			}
		}
		path := filepath.Join(t.TempDir(), "body.step")
		if e := os.WriteFile(path, raw.Bytes(), 0600); e != nil {
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
		if math.Abs(got.Area-area) > 1e-7 || math.Abs(got.Volume-volume) > 1e-7 {
			t.Fatal(full, got, area, volume)
		}
	}
}

func TestLongEdgeChainIndependentOCCT(t *testing.T) {
	m := longEdgeChainFixture()
	for _, full := range []bool{false, true} {
		source := filepath.Join(t.TempDir(), "independent.step")
		mode := "write-step-long-edge-treatment"
		if full {
			mode = "write-step-long-edge-chain"
		}
		stepOracle(t, source, mode)
		var raw bytes.Buffer
		var e error
		if full {
			e = WriteLongEdgeChains(&raw, m)
		} else {
			e = WriteLongEdgeTreatments(&raw, LongEdgeTreatmentModel{LengthUnit: m.LengthUnit, Bodies: []LongEdgeTreatmentBody{m.Bodies[0].LongEdgeTreatmentBody}})
		}
		if e != nil {
			t.Fatal(e)
		}
		path := filepath.Join(t.TempDir(), "native.step")
		if e = os.WriteFile(path, raw.Bytes(), 0600); e != nil {
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
			data, e := os.ReadFile(dest)
			if e != nil {
				t.Fatal(e)
			}
			if e = json.Unmarshal(data, &got[i]); e != nil {
				t.Fatal(e)
			}
		}
		if math.Abs(got[0].Area-got[1].Area) > 1e-7 || math.Abs(got[0].Volume-got[1].Volume) > 1e-7 {
			t.Fatal(full, got)
		}
		for i := 0; i < 3; i++ {
			if math.Abs(got[0].Centroid[i]-got[1].Centroid[i]) > 1e-8 {
				t.Fatal(full, got)
			}
		}
	}
}
func TestLongEdgeChainRejects(t *testing.T) {
	for _, kind := range []string{"length", "radius", "chamfer", "bore", "pocket", "boss", "unit", "name"} {
		m := longEdgeChainFixture()
		b := &m.Bodies[0]
		switch kind {
		case "length":
			b.Length = math.Inf(1)
		case "radius":
			b.Radius = 20
		case "chamfer":
			b.TopSetback = 20
		case "bore":
			b.Bore.Center.Y = .5
		case "pocket":
			b.Pocket.Min = geom.Vec2{X: 4, Y: 4}
			b.Pocket.Max = geom.Vec2{X: 6, Y: 6}
		case "boss":
			b.Boss.Upper = false
		case "unit":
			m.LengthUnit = math.NaN()
		case "name":
			b.Name = string([]byte{255})
		}
		var out bytes.Buffer
		if e := WriteLongEdgeChains(&out, m); e == nil || out.Len() != 0 {
			t.Fatal(kind, "partial or invalid output")
		}
	}
	if e := WriteLongEdgeChains(failWriter{}, longEdgeChainFixture()); e == nil {
		t.Fatal("write error lost")
	}
}
func FuzzWriteLongEdgeTreatment(f *testing.F) {
	f.Add(20., 10., 10., 1., 1., 1.)
	f.Add(0., 10., 10., 1., 1., 1.)
	f.Fuzz(func(t *testing.T, l, w, h, r, top, side float64) {
		for _, v := range []float64{l, w, h, r, top, side} {
			if math.Abs(v) > 1e4 {
				return
			}
		}
		var out bytes.Buffer
		_ = WriteLongEdgeTreatments(&out, LongEdgeTreatmentModel{LengthUnit: .001, Bodies: []LongEdgeTreatmentBody{{Length: l, Width: w, Height: h, Radius: r, TopSetback: top, SideSetback: side}}})
	})
}

func TestLongEdgeTreatmentUnequalOCCT(t *testing.T) {
	for _, setback := range [][2]float64{{.5, 1.5}, {2, 1}, {1, 2}} {
		b := longEdgeChainFixture().Bodies[0].LongEdgeTreatmentBody
		b.TopSetback, b.SideSetback = setback[0], setback[1]
		r, e := fillet.TreatOppositeLongCapEdges(context.Background(), b.Length, b.Width, b.Height, b.Radius, b.TopSetback, b.SideSetback, fillet.Options{})
		if e != nil {
			t.Fatal(e)
		}
		var raw bytes.Buffer
		if e = WriteLongEdgeTreatments(&raw, LongEdgeTreatmentModel{LengthUnit: .001, Bodies: []LongEdgeTreatmentBody{b}}); e != nil {
			t.Fatal(e)
		}
		path := filepath.Join(t.TempDir(), "unequal.step")
		if e = os.WriteFile(path, raw.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
		dest := filepath.Join(t.TempDir(), "mass.json")
		stepOracle(t, dest, "step-mass-precise", path)
		data, e := os.ReadFile(dest)
		if e != nil {
			t.Fatal(e)
		}
		var got struct{ Area, Volume float64 }
		if e = json.Unmarshal(data, &got); e != nil {
			t.Fatal(e)
		}
		if math.Abs(got.Area-r.NominalSurfaceArea) > 1e-7 || math.Abs(got.Volume-r.NominalVolume) > 1e-7 {
			t.Fatal(setback, got, r.NominalSurfaceArea, r.NominalVolume)
		}
	}
}
