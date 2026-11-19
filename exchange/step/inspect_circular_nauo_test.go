package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"math/big"
	"os"
	"os/exec"
	"testing"
	"time"
)

func circularNAUOSource(t *testing.T) ([]byte, Assembly, int64) {
	t.Helper()
	raw, e := os.ReadFile("testdata/occt-circular-nauo.step")
	if e != nil {
		t.Fatal(e)
	}
	a, e := ReadAssembly(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	for _, p := range a.Products {
		if p.Name == "Circular cylinder part" {
			return raw, a, p.DefinitionID
		}
	}
	t.Fatal("missing product")
	return nil, Assembly{}, 0
}
func TestInspectCircularNAUO(t *testing.T) {
	t.Parallel()
	raw, _, id := circularNAUOSource(t)
	a, e := InspectCircularNAUOProduct(bytes.NewReader(raw), Limits{}, id)
	if e != nil {
		t.Fatal(e)
	}
	if a.Product.Name != "Circular cylinder part" || a.LengthUnit != .001 || len(a.NAUOIDs) != 1 || len(a.FailedConditions) != 4 {
		t.Fatal(a)
	}
	y, _ := new(big.Rat).SetString("-4.898587196589E-16")
	residual := new(big.Rat).Mul(y, y)
	for i, r := range a.Rims {
		if r.Height.Cmp(big.NewRat(int64(i*5), 1)) != 0 || r.SquaredRadiusResidual.Cmp(residual) != 0 || r.PlaneResidual.Sign() != 0 {
			t.Fatal(r)
		}
	}
	if a.SeamSquaredRadiusResidual.Cmp(residual) != 0 || a.TurnDefectLower.Sign() <= 0 || a.TurnDefectUpper.Cmp(a.TurnDefectLower) <= 0 {
		t.Fatal("source exact failure not established", a)
	}
	angle, _ := new(big.Rat).SetString("6.28318530718")
	if a.SeamAngleDifference.Cmp(angle) != 0 {
		t.Fatal(a.SeamAngleDifference)
	}
	if _, e = ReadExactPlanarNAUOProducts(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("strict planar reader widened")
	}
}
func TestInspectCircularNAUOMutations(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"cap-owner", "rim-radius", "pcurve-radius", "coedge", "bad-loop", "seam-owner", "seam-order", "angle-unit", "product-owner", "limit", "unknown"} {
		t.Run(kind, func(t *testing.T) {
			raw, assembly, id := circularNAUOSource(t)
			a, e := InspectCircularNAUOProduct(bytes.NewReader(raw), Limits{}, id)
			if e != nil {
				t.Fatal(e)
			}
			f := assembly.Source
			l := Limits{}
			switch kind {
			case "cap-owner":
				pc := f.Entities[a.Rims[0].PlanePCurveID]
				other := f.Entities[a.Rims[1].CapFaceID].Records[0].Args[2].Ref
				pc.Records[0].Args[1].Ref = other
				f.Entities[pc.ID] = pc
			case "rim-radius":
				c := f.Entities[a.Rims[0].CircleID]
				c.Records[0].Args[2].Text = "2.0000000000000001"
				f.Entities[c.ID] = c
			case "pcurve-radius":
				pc := f.Entities[a.Rims[0].PlanePCurveID].Records[0]
				rep := f.Entities[pc.Args[2].Ref].Records[0]
				c := f.Entities[rep.Args[1].Items[0].Ref]
				c.Records[0].Args[2].Text = "2.0000000000000001"
				f.Entities[c.ID] = c
			case "coedge":
				face := f.Entities[a.Rims[0].CapFaceID].Records[0]
				bound := f.Entities[face.Args[1].Items[0].Ref].Records[0]
				loop := f.Entities[bound.Args[1].Ref].Records[0]
				en := f.Entities[loop.Args[1].Items[0].Ref]
				en.Records[0].Args[4].Text = "T"
				f.Entities[en.ID] = en
			case "bad-loop":
				face := f.Entities[a.SideFaceID].Records[0]
				bound := f.Entities[face.Args[1].Items[0].Ref].Records[0]
				loop := f.Entities[bound.Args[1].Ref]
				loop.Records[0].Args[1].Items = loop.Records[0].Args[1].Items[:3]
				f.Entities[loop.ID] = loop
			case "seam-owner":
				pc := f.Entities[a.SeamBranches[0].PCurveID]
				pc.Records[0].Args[1].Ref = f.Entities[a.Rims[0].CapFaceID].Records[0].Args[2].Ref
				f.Entities[pc.ID] = pc
			case "seam-order":
				edge := f.Entities[a.SeamEdgeID].Records[0]
				sc := f.Entities[edge.Args[3].Ref]
				sc.Records[0].Args[2].Items[0], sc.Records[0].Args[2].Items[1] = sc.Records[0].Args[2].Items[1], sc.Records[0].Args[2].Items[0]
				f.Entities[sc.ID] = sc
			case "angle-unit":
				for eid, en := range f.Entities {
					for i, r := range en.Records {
						if r.Type == "SI_UNIT" && len(r.Args) == 2 && r.Args[1].Text == "RADIAN" {
							en.Records[i].Args[0] = Value{Kind: Enumeration, Text: "MILLI"}
							f.Entities[eid] = en
						}
					}
				}
			case "product-owner":
				for _, p := range assembly.Products {
					if p.DefinitionID != id && p.DefinitionID != assembly.Roots[0] {
						rep := f.Entities[p.Representations[0]]
						for i, v := range rep.Records[0].Args[1].Items {
							if ent := f.Entities[v.Ref]; len(ent.Records) == 1 && ent.Records[0].Type == "MANIFOLD_SOLID_BREP" {
								rep.Records[0].Args[1].Items[i].Ref = a.SourceBodyID
							}
						}
						f.Entities[rep.ID] = rep
					}
				}
			case "limit":
				l.MaxFaces = 2
			case "unknown":
				id = -1
			}
			b, e := mappedPlanarSourceBytes(f)
			if e != nil {
				t.Fatal(e)
			}
			got, e := InspectCircularNAUOProduct(bytes.NewReader(b), l, id)
			if e == nil || got.SourceBodyID != 0 || got.FailedConditions != nil {
				t.Fatal("non-atomic invalid ownership", kind, e)
			}
		})
	}
}
func TestInspectCircularNAUOExactVertexDoesNotHidePeriod(t *testing.T) {
	t.Parallel()
	raw, a, id := circularNAUOSource(t)
	report, e := InspectCircularNAUOProduct(bytes.NewReader(raw), Limits{}, id)
	if e != nil {
		t.Fatal(e)
	}
	f := a.Source
	for _, rim := range report.Rims {
		vp := f.Entities[rim.VertexID].Records[0]
		p := f.Entities[vp.Args[1].Ref]
		p.Records[0].Args[1].Items[1].Text = "0"
		f.Entities[p.ID] = p
	}
	edge := f.Entities[report.SeamEdgeID].Records[0]
	sc := f.Entities[edge.Args[3].Ref].Records[0]
	line := f.Entities[sc.Args[1].Ref].Records[0]
	point := f.Entities[line.Args[1].Ref]
	point.Records[0].Args[1].Items[1].Text = "0"
	f.Entities[point.ID] = point
	b, e := mappedPlanarSourceBytes(f)
	if e != nil {
		t.Fatal(e)
	}
	got, e := InspectCircularNAUOProduct(bytes.NewReader(b), Limits{}, id)
	if e != nil {
		t.Fatal(e)
	}
	if len(got.FailedConditions) != 1 || got.FailedConditions[0].Kind != CircularNAUONonperiodicSeam {
		t.Fatal("period failure hidden", got)
	}
}
func TestOCCTCircularNAUOInput(t *testing.T) {
	t.Parallel()
	bin := os.Getenv("PLAINSOLID_OCCT_DISK_BOUND_MASS")
	if bin == "" {
		t.Skip("optional pinned OCCT")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	data, e := exec.CommandContext(ctx, bin, "testdata/occt-circular-nauo.step").CombinedOutput()
	if e != nil {
		t.Fatal(e, string(data))
	}
	at := bytes.LastIndex(data, []byte(`{"Faces"`))
	if at < 0 {
		t.Fatal(string(data))
	}
	var m struct {
		Faces, Solids int
		Volume, Area  float64
	}
	if e = json.Unmarshal(data[at:], &m); e != nil {
		t.Fatal(e)
	}
	if m.Faces != 15 || m.Solids != 3 || math.Abs(m.Volume-(48+20*math.Pi)) > 1e-9 || math.Abs(m.Area-(104+28*math.Pi)) > 1e-9 {
		t.Fatal(m)
	}
}
func FuzzInspectCircularNAUO(f *testing.F) {
	raw, e := os.ReadFile("testdata/occt-circular-nauo.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(raw)
	f.Add([]byte("ISO-10303-21;"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<20 {
			t.Skip()
		}
		_, _ = InspectCircularNAUOProduct(bytes.NewReader(b), Limits{MaxBytes: 1 << 20, MaxEntities: 1000, MaxFaces: 16, MaxVertices: 32, MaxEdgeUses: 128}, 389)
	})
}
