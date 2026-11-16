package iges

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"os"
	"reflect"
	"strings"
	"testing"

	"plainsolid/geom"
)

func TestRepairConicalRimFilletBoreIGES(t *testing.T) {
	ctx := context.Background()
	raw := conicalRimFilletBoreIGESFixture(t, "rounded")
	p, err := RepairConicalRimFilletBore(ctx, bytes.NewReader(raw), Limits{}, ConicalRimFilletBoreRepairOptions{3e-11})
	if err != nil {
		t.Fatal(err)
	}
	source := p.OriginalIGES()
	if !bytes.Equal(source, raw) {
		t.Fatal("original IGES bytes changed")
	}
	source[0] ^= 1
	if !bytes.Equal(raw, p.OriginalIGES()) || !bytes.Equal(raw, p.Material().SourceIGES()) {
		t.Fatal("original source storage aliases the returned copy")
	}
	e := p.Evidence()
	if e.ConeFaceID != 7 || e.ConeSupportID != 9 || e.ConeSeamCurveID != 21 || e.ConeSeamEdgeIndex != 1 ||
		e.ConeRootPointID != 11 || e.ConeAxisID != 13 || e.ConePhaseID != 15 ||
		e.ConeVertexIndices != [2]int{1, 2} || e.ConeRimEdgeIndices != [2]int{3, 2} ||
		e.OriginalRimStations != [2]string{"3/16", "21/8"} || e.ExactSlope != "4/3" || e.OriginalAngleDegrees != "53.130102354" {
		t.Fatalf("wrong original cone evidence: %+v", e)
	}
	bound, ok := new(big.Rat).SetString(e.ConeDisplacementBound)
	if !ok || bound.Cmp(igesBoreRat(1.9e-11)) <= 0 || bound.Cmp(igesBoreRat(2e-11)) >= 0 {
		t.Fatalf("unexpected finite cone displacement: %v", bound)
	}
	wantArcs := []ConicalRimFilletBoreArcRepair{
		{25, 27, 2, [2]int{7, 71}, false, "127675647829/100000000000000000000000000"},
		{29, 31, 3, [2]int{7, 59}, false, "183186799059/200000000000000000000000000"},
		{33, 35, 4, [2]int{59, 59}, true, "16653345369/320000000000000000000000000"},
		{37, 39, 5, [2]int{59, 83}, false, "682787160129/800000000000000000000000000"},
		{41, 43, 6, [2]int{71, 71}, true, "16653345369/160000000000000000000000000"},
		{45, 47, 7, [2]int{71, 97}, false, "460742555209/400000000000000000000000000"},
	}
	wantTori := []ConicalRimFilletBoreTorusRepair{
		{59, 61, 67, "383026943487/200000000000000000000000000"},
		{71, 73, 79, "272004641027/100000000000000000000000000"},
	}
	if !reflect.DeepEqual(e.ArcOccurrences, wantArcs) || !reflect.DeepEqual(e.TorusOccurrences, wantTori) {
		t.Fatalf("wrong original chart occurrence evidence: %v %v", e.ArcOccurrences, e.TorusOccurrences)
	}
	m := p.Material()
	if m.LengthUnit != .001 || m.SourceGroupID != 1 || m.SourceSolidID != 3 || m.SourceShellID != 5 ||
		m.SourceEdgeListID != 19 || m.SourceVertexListID != 23 || !reflect.DeepEqual(m.NativeFaceSourceIDs, []int{7, 59, 83, 71, 97, 111}) {
		t.Fatalf("lost original units or owners: %g %v", m.LengthUnit, m.NativeFaceSourceIDs)
	}
	wantCurves := []int{21, 25, 29, 33, 37, 41, 45, 49, 53, 57}
	uses := map[int]int{}
	for i, edge := range m.NativeEdgeSourceIndices {
		if m.NativeCurveSourceIDs[i] != wantCurves[edge-1] {
			t.Fatalf("incorrect original curve for edge index %d", edge)
		}
		uses[edge]++
	}
	if !reflect.DeepEqual(uses, map[int]int{1: 1, 2: 4, 3: 4, 4: 1, 5: 4, 6: 1, 7: 4, 8: 4, 9: 4, 10: 1}) {
		t.Fatalf("original edge partition changed: %v", uses)
	}
	exact, err := ReadConicalRimFilletBore(ctx, bytes.NewReader(conicalRimFilletBoreIGESFixture(t, "exact")), Limits{}, geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(exact.NativeVertexSourceIndices, m.NativeVertexSourceIndices) ||
		!reflect.DeepEqual(exact.NativeEdgeSourceIndices, m.NativeEdgeSourceIndices) {
		t.Fatal("source edge or vertex indexing changed")
	}
	a, err := igesPrismBytes(m.Solid)
	if err != nil {
		t.Fatal(err)
	}
	b, err := igesPrismBytes(exact.Solid)
	if err != nil || !bytes.Equal(a, b) {
		t.Fatalf("repaired native bytes differ from the independent exact body: %v", err)
	}
}

func TestRepairConicalRimFilletBoreIGESReject(t *testing.T) {
	ctx := context.Background()
	raw := conicalRimFilletBoreIGESFixture(t, "rounded")
	reject := func(t *testing.T, p []byte, distance float64, limits Limits, detail string) {
		t.Helper()
		m, err := RepairConicalRimFilletBore(ctx, bytes.NewReader(p), limits, ConicalRimFilletBoreRepairOptions{distance})
		if err == nil || len(m.OriginalIGES()) != 0 || m.Material().Solid.Topology().Faces != 0 {
			t.Fatalf("unsupported original returned partial material: %v", err)
		}
		if detail != "" && !strings.Contains(err.Error(), detail) {
			t.Fatalf("unexpected failed proof: %v", err)
		}
	}
	reject(t, raw, 1e-11, Limits{}, "finite type 194 angle displacement")
	reject(t, raw, 0, Limits{}, "")
	reject(t, raw, math.NaN(), Limits{}, "")
	reject(t, raw, 3e-11, Limits{MaxBytes: int64(len(raw) - 1)}, "")
	reject(t, raw, 3e-11, Limits{MaxEntities: 60}, "")
	reject(t, raw, 3e-11, Limits{MaxControlPoints: 17}, "")
	reject(t, conicalRimFilletBoreIGESFixture(t, "exact"), 3e-11, Limits{}, "")
	for _, tc := range []struct {
		name, from, to, detail string
	}{
		{"angle outside", "53.130102354", "53.130103354", "finite type 194 angle displacement"},
		{"changed root radius", "194,11,13,8.", "194,11,13,9.", ""},
		{"moved original line seam", "110,8.25,2.,0.1875", "110,8.26,2.,0.1875", ""},
		{"lower meridian phase outside", "124,1.,0.,5.551115123E-17,7.6875,-5.551115123E-17", "124,1.,0.,5.551115123E-07,7.6875,-5.551115123E-07", "type 124 arc occurrence #35"},
		{"upper meridian hidden scale", "124,1.,0.,-5.551115123E-17,10.375", "124,2.,0.,-5.551115123E-17,10.375", ""},
		{"upper complementary sweep", "1.125,-1.5,0.,1.875; ", "1.125,-1.5,0.,-1.875;", ""},
		{"changed original torus", "198,63,65,7.6875", "198,63,65,8.6875", ""},
		{"same-sense source lower rim", "19,4,0,0,0,19,3,0,0", "19,4,0,0,0,19,3,1,0", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if len(tc.from) != len(tc.to) || bytes.Count(raw, []byte(tc.from)) != 1 {
				t.Fatalf("mutation does not preserve the original record shape: %q", tc.from)
			}
			reject(t, bytes.Replace(raw, []byte(tc.from), []byte(tc.to), 1), 3e-11, Limits{}, tc.detail)
		})
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if m, err := RepairConicalRimFilletBore(canceled, bytes.NewReader(raw), Limits{}, ConicalRimFilletBoreRepairOptions{3e-11}); !errors.Is(err, context.Canceled) || len(m.OriginalIGES()) != 0 {
		t.Fatalf("canceled repair returned material: %v", err)
	}
}

func TestRepairedConicalRimFilletBoreIGESReceipt(t *testing.T) {
	ctx := context.Background()
	raw := conicalRimFilletBoreIGESFixture(t, "rounded")
	m, err := RepairConicalRimFilletBore(ctx, bytes.NewReader(raw), Limits{}, ConicalRimFilletBoreRepairOptions{3e-11})
	if err != nil {
		t.Fatal(err)
	}
	var first, second bytes.Buffer
	if err := WriteRepairedConicalRimFilletBoreJSON(ctx, &first, m); err != nil {
		t.Fatal(err)
	}
	got, err := ReadRepairedConicalRimFilletBoreJSON(ctx, bytes.NewReader(first.Bytes()), ConicalRimFilletBoreRepairJSONLimits{})
	if err != nil || !bytes.Equal(got.OriginalIGES(), raw) || !reflect.DeepEqual(got.Evidence(), m.Evidence()) {
		t.Fatalf("receipt did not reconstruct the original source proof: %v", err)
	}
	if err := WriteRepairedConicalRimFilletBoreJSON(ctx, &second, got); err != nil || !bytes.Equal(first.Bytes(), second.Bytes()) {
		t.Fatalf("receipt did not replay byte-identically: %v", err)
	}
	reject := func(raw []byte, l ConicalRimFilletBoreRepairJSONLimits) {
		t.Helper()
		x, err := ReadRepairedConicalRimFilletBoreJSON(ctx, bytes.NewReader(raw), l)
		if err == nil || len(x.OriginalIGES()) != 0 {
			t.Fatalf("invalid replay returned source material: %v", err)
		}
	}
	reject(first.Bytes(), ConicalRimFilletBoreRepairJSONLimits{MaxBytes: int64(first.Len() - 1)})
	reject(first.Bytes(), ConicalRimFilletBoreRepairJSONLimits{Source: Limits{MaxEntities: 60}})
	for _, name := range []string{"bytes", "budget", "tight budget", "original arc owner", "duplicate source field"} {
		t.Run(name, func(t *testing.T) {
			q := new(conicalRimFilletBoreRepairEnvelope)
			if err := json.Unmarshal(first.Bytes(), q); err != nil {
				t.Fatal(err)
			}
			c := new(conicalRimFilletBoreRepairClaim)
			if err := json.Unmarshal(q.Claims, c); err != nil {
				t.Fatal(err)
			}
			switch name {
			case "bytes":
				q.Source[0] ^= 1
			case "budget":
				q.MaxDistance *= 2
			case "tight budget":
				q.MaxDistance /= 3
			case "original arc owner":
				c.Evidence.ArcOccurrences[2].FaceIDs[0]++
			}
			q.Claims, err = json.Marshal(c)
			if err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(q)
			if err != nil {
				t.Fatal(err)
			}
			if name == "duplicate source field" {
				data = append([]byte(`{"Source":"",`), data[1:]...)
			}
			reject(data, ConicalRimFilletBoreRepairJSONLimits{})
		})
	}
}

func FuzzRepairConicalRimFilletBoreIGES(f *testing.F) {
	for _, kind := range []string{"rounded", "exact"} {
		p, err := os.ReadFile("testdata/occt-conical-double-rim-eccentric-bore-" + kind + ".igs")
		if err != nil {
			f.Fatal(err)
		}
		f.Add(p)
	}
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, source []byte) {
		m, err := RepairConicalRimFilletBore(context.Background(), bytes.NewReader(source), Limits{}, ConicalRimFilletBoreRepairOptions{3e-11})
		if err != nil {
			if len(m.OriginalIGES()) != 0 {
				t.Fatal("invalid source returned partial material")
			}
			return
		}
		if !bytes.Equal(m.OriginalIGES(), source) || m.Material().Solid.Topology().Faces != 6 {
			t.Fatal("successful repair lost the original source or topology")
		}
	})
}
