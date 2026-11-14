package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/brep/parametric"
	"testing"
	"time"
)

func TestRepairCircularNAUO(t *testing.T) {
	raw, _, id := circularNAUOSource(t)
	a, e := RepairCircularNAUOProduct(bytes.NewReader(raw), Limits{}, id, CircularNAUORepairOptions{MaxDistance: 1e-9})
	if e != nil {
		t.Fatal(e)
	}
	if a.Body.Name != "Circular cylinder part" || a.Body.Radius != 2 || a.Body.Height != 5 || a.LengthUnit != .001 || len(a.Source.FailedConditions) != 4 {
		t.Fatal(a)
	}
	if len(a.Solid.Faces()) != 3 || a.NativeFaceSourceIDs[0] != a.Source.SideFaceID || a.NativePrimaryEdgeSourceIDs[1] != a.Source.SeamEdgeID || a.NativeRimVertexSourceIDs[0] != a.Source.Rims[0].VertexID {
		t.Fatal("native/source role mapping")
	}
	squared := new(big.Rat).Mul(a.Bounds.Tolerance, a.Bounds.Tolerance)
	for _, v := range a.Bounds.VertexDisplacementSquared {
		if v.Sign() <= 0 || v.Cmp(squared) > 0 {
			t.Fatal(v)
		}
	}
	for _, v := range a.Bounds.SeamModelOwnerDisplacementSquaredUpper {
		if v.Sign() <= 0 || v.Cmp(squared) > 0 {
			t.Fatal(v)
		}
	}
	for i, v := range a.Solid.Vertices()[:2] {
		if v.X != 2 || v.Y != 0 || v.Z != float64(i*5) {
			t.Fatal("repair did not own projected seam", v)
		}
	}
	m, e := a.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if math.Abs(m.Volume-20*math.Pi) > 1e-8 || math.Abs(m.Area-28*math.Pi) > 1e-8 || math.Abs(m.Centroid.Z-2.5) > 1e-9 {
		t.Fatal(m)
	}
	if _, e = ReadExactPlanarNAUOProducts(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("exact source gate widened")
	}
}
func TestRepairCircularNAUOReject(t *testing.T) {
	for _, kind := range []string{"zero", "negative", "nan", "large-tolerance", "small-tolerance", "vertex-gap", "wrong-ray", "nonbinary-radius", "nonbinary-corner", "bad-owner", "limit"} {
		t.Run(kind, func(t *testing.T) {
			raw, a, id := circularNAUOSource(t)
			report, e := InspectCircularNAUOProduct(bytes.NewReader(raw), Limits{}, id)
			if e != nil {
				t.Fatal(e)
			}
			f := a.Source
			o := CircularNAUORepairOptions{MaxDistance: 1e-9}
			l := Limits{}
			switch kind {
			case "zero":
				o.MaxDistance = 0
			case "negative":
				o.MaxDistance = -1
			case "nan":
				o.MaxDistance = math.NaN()
			case "large-tolerance":
				o.MaxDistance = 1e-5
			case "small-tolerance":
				o.MaxDistance = 1e-14
			case "limit":
				l.MaxEntities = 100
			case "bad-owner":
				en := f.Entities[report.Rims[0].PlanePCurveID]
				en.Records[0].Args[1].Ref = report.CylinderID
				f.Entities[en.ID] = en
			case "vertex-gap", "wrong-ray", "nonbinary-radius":
				points := []int64{}
				for _, r := range report.Rims {
					points = append(points, f.Entities[r.VertexID].Records[0].Args[1].Ref)
				}
				edge := f.Entities[report.SeamEdgeID].Records[0]
				sc := f.Entities[edge.Args[3].Ref].Records[0]
				line := f.Entities[sc.Args[1].Ref].Records[0]
				points = append(points, line.Args[1].Ref)
				for _, pid := range points {
					en := f.Entities[pid]
					switch kind {
					case "vertex-gap":
						en.Records[0].Args[1].Items[1].Text = "-0.00000001"
					case "wrong-ray":
						en.Records[0].Args[1].Items[0].Text = "-2"
						en.Records[0].Args[1].Items[1].Text = "0"
					case "nonbinary-radius":
						en.Records[0].Args[1].Items[0].Text = "2.1"
					}
					f.Entities[pid] = en
				}
				if kind == "nonbinary-radius" {
					entities := []int64{report.CylinderID, report.Rims[0].CircleID, report.Rims[1].CircleID}
					for _, r := range report.Rims {
						pc := f.Entities[r.PlanePCurveID].Records[0]
						rep := f.Entities[pc.Args[2].Ref].Records[0]
						entities = append(entities, rep.Args[1].Items[0].Ref)
					}
					for _, eid := range entities {
						en := f.Entities[eid]
						en.Records[0].Args[2].Text = "2.1"
						f.Entities[eid] = en
					}
				}
			case "nonbinary-corner":
				closure, e := nauoClosure(f, []int64{report.SourceBodyID}, 64)
				if e != nil {
					t.Fatal(e)
				}
				for eid := range closure {
					en := f.Entities[eid]
					if len(en.Records) == 1 && en.Records[0].Type == "CARTESIAN_POINT" && len(en.Records[0].Args[1].Items) == 3 {
						v := en.Records[0].Args[1].Items[0]
						q, _ := new(big.Rat).SetString(v.Text)
						q.Add(q, new(big.Rat).SetInt(new(big.Int).Lsh(big.NewInt(1), 54)))
						en.Records[0].Args[1].Items[0].Text = q.FloatString(0)
						f.Entities[eid] = en
					}
				}
			}
			b, e := mappedPlanarSourceBytes(f)
			if e != nil {
				t.Fatal(e)
			}
			if kind == "vertex-gap" || kind == "wrong-ray" || kind == "nonbinary-radius" || kind == "nonbinary-corner" {
				if _, e = InspectCircularNAUOProduct(bytes.NewReader(b), Limits{}, id); e != nil {
					t.Fatal("positive inspection mutant invalid", e)
				}
			}
			got, e := RepairCircularNAUOProduct(bytes.NewReader(b), l, id, o)
			if e == nil || got.Source.SourceBodyID != 0 || len(got.Solid.Faces()) != 0 {
				t.Fatal("accepted unsupported repair", e)
			}
		})
	}
}
func TestOCCTRepairCircularNAUO(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_DISK_BOUND_MASS")
	if bin == "" {
		t.Skip("optional pinned OCCT")
	}
	raw, _, id := circularNAUOSource(t)
	a, e := RepairCircularNAUOProduct(bytes.NewReader(raw), Limits{}, id, CircularNAUORepairOptions{MaxDistance: 1e-9})
	if e != nil {
		t.Fatal(e)
	}
	var output bytes.Buffer
	if e = WriteCylinders(&output, CylinderModel{Bodies: []CylinderBody{a.Body}, LengthUnit: a.LengthUnit}); e != nil {
		t.Fatal(e)
	}
	file := filepath.Join(t.TempDir(), "explicitly-repaired-cylinder.step")
	if e = os.WriteFile(file, output.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	type measure struct {
		Faces, Solids int
		Volume, Area  float64
		Center        [3]float64
	}
	run := func(path string) measure {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		data, e := exec.CommandContext(ctx, bin, path).CombinedOutput()
		if e != nil {
			t.Fatal(e, string(data))
		}
		at := bytes.LastIndex(data, []byte(`{"Faces"`))
		if at < 0 {
			t.Fatal(string(data))
		}
		var m measure
		if e = json.Unmarshal(data[at:], &m); e != nil {
			t.Fatal(e)
		}
		return m
	}
	source, repaired := run("testdata/occt-circular-nauo.step"), run(file)
	if repaired.Faces != 3 || repaired.Solids != 1 || math.Abs(repaired.Volume-(source.Volume-48)) > 1e-9 || math.Abs(repaired.Area-(source.Area-104)) > 1e-9 || math.Abs(repaired.Center[2]-2.5) > 1e-9 {
		t.Fatal(source, repaired)
	}
}
func FuzzRepairCircularNAUO(f *testing.F) {
	raw, e := os.ReadFile("testdata/occt-circular-nauo.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(raw, 1e-9)
	f.Add(raw, 1e-14)
	f.Add([]byte("ISO-10303-21;"), 1e-9)
	f.Fuzz(func(t *testing.T, b []byte, tolerance float64) {
		if len(b) > 1<<20 {
			t.Skip()
		}
		got, e := RepairCircularNAUOProduct(bytes.NewReader(b), Limits{MaxBytes: 1 << 20, MaxEntities: 1000, MaxFaces: 16, MaxVertices: 32, MaxEdgeUses: 128}, 389, CircularNAUORepairOptions{MaxDistance: tolerance})
		if e != nil && len(got.Solid.Faces()) != 0 {
			t.Fatal("partial repair")
		}
	})
}
