package step

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"os"
	"plainsolid/internal/slowtest"
	"reflect"
	"strings"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func TestRepairNestedMixedConicalAP242Assembly(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	original := nestedMixedConicalAP242Fixture(t, "rounded")
	l := Limits{MaxFaces: 14, MaxVertices: 56, MaxEdgeUses: 132}
	if _, err := ReadNestedMixedConicalAP242Assembly(ctx, bytes.NewReader(original), l, geom.SurfacePatchOptions{}); err == nil {
		t.Fatal("strict nested reader accepted the default rounded source")
	}
	p, err := RepairNestedMixedConicalAP242Assembly(ctx, bytes.NewReader(original), l, NestedMixedConicalAP242RepairOptions{3e-12})
	if err != nil {
		t.Fatal(err)
	}
	a := p.Assembly()
	if p.OriginalSTEP() != string(original) || a.Assembly.LengthUnit != .001 || a.Assembly.Roots[0] != 5 ||
		a.Group.DefinitionID != 31 || len(a.Parts) != 2 || len(a.Occurrences) != 5 || len(a.Instances) != 6 {
		t.Fatalf("lost original source, units or nested multiplicity: %+v", a.Group)
	}
	first, second := p.PartRepairs()[0], p.PartRepairs()[1]
	if first.DefinitionID != 62 || first.BodyID != 68 || first.RollingRim == nil || first.RollingRim.ReversedBoundID != 104 ||
		second.DefinitionID != 230 || second.BodyID != 236 || len(second.ChamferWalls) != 6 {
		t.Fatal("wrong isolated repaired definitions")
	}
	if !reflect.DeepEqual(first.GeneratedEntityIDs, map[int64]int64{223: 444, 224: 445}) ||
		!reflect.DeepEqual(second.GeneratedEntityIDs, map[int64]int64{439: 446, 440: 447, 441: 448, 442: 449, 443: 450, 444: 451}) {
		t.Fatalf("wrong generated ID relocation: %v %v", first.GeneratedEntityIDs, second.GeneratedEntityIDs)
	}
	for i, wall := range second.ChamferWalls {
		if wall.CorrectedGeneratorID != 446+int64(i) {
			t.Fatalf("wall #%d lost its final assembly generator", wall.FaceID)
		}
	}

	raw, err := Parse(bytes.NewReader(original), l)
	if err != nil {
		t.Fatal(err)
	}
	corrected := a.Assembly.Source
	owned := map[int64]bool{}
	for _, part := range p.PartRepairs() {
		local, err := nauoClosure(raw, []int64{part.BodyID}, 16)
		if err != nil {
			t.Fatal(err)
		}
		for id := range local {
			owned[id] = true
		}
	}
	if !reflect.DeepEqual(raw.Header, corrected.Header) {
		t.Fatal("assembly header changed")
	}
	for _, id := range raw.Order {
		en := raw.Entities[id]
		preserve := !owned[id]
		for _, rec := range en.Records {
			switch rec.Type {
			case "MANIFOLD_SOLID_BREP", "CLOSED_SHELL", "ADVANCED_FACE", "EDGE_LOOP", "ORIENTED_EDGE", "EDGE_CURVE", "VERTEX_POINT":
				preserve = true
			case "FACE_BOUND", "FACE_OUTER_BOUND":
				preserve = id != first.RollingRim.ReversedBoundID
			}
		}
		if preserve && !reflect.DeepEqual(en, corrected.Entities[id]) {
			t.Fatalf("changed the original assembly or topology entity #%d", id)
		}
	}
	localOrig := a.Parts[62].Rounded
	localChamfer := a.Parts[230].Chamfered
	if localOrig == nil || localChamfer == nil {
		t.Fatal("lost one local shape")
	}
	exact, err := ReadNestedMixedConicalAP242Assembly(ctx, bytes.NewReader(nestedMixedConicalAP242Fixture(t, "exact")), l, geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for i, solid := range []trimmed.Solid{exact.Parts[62].Rounded.Solid, exact.Parts[227].Chamfered.Solid} {
		native, err := prismNativeBytes(solid)
		if err != nil || rationalPosedRepairDigest(native) != p.PartRepairs()[i].NativeSHA256 {
			t.Fatalf("repaired local definition changed from the independent exact source: %v", err)
		}
	}
	for i, world := range a.Instances {
		want := [2]int64{[2]int64{58, 443}[i/3], [3]int64{221, 227, 437}[i%3]}
		if world.Path != want || world.ProductID != [3]int64{62, 62, 230}[i%3] ||
			!reflect.DeepEqual(world.Matrix(), exact.Instances[i].Matrix()) {
			t.Fatalf("changed the exact composed world or path %v", world.Path)
		}
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_MIXED_CONICAL_AP242"); oracle != "" {
		props := map[int64]parametric.Properties{}
		for id, solid := range map[int64]trimmed.Solid{62: localOrig.Solid, 230: localChamfer.Solid} {
			mass, err := solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
			if err != nil {
				t.Fatal(err)
			}
			props[id] = mass
		}
		var want []nestedMixedOccurrenceMeasure
		for i, world := range a.Instances {
			mass := props[world.ProductID]
			m := nestedMixedOccurrenceMeasure{Path: [2]int{i/3 + 1, i%3 + 1}}
			m.Volume, m.Area, m.SharedWithFirst = mass.Volume, mass.Area, i%3 < 2
			m.Topology = [4]int{1, 6, 10, 6}
			if world.ProductID == 230 {
				m.Topology = [4]int{1, 8, 14, 8}
			}
			for row, cells := range world.Matrix() {
				for col, value := range cells {
					m.Pose[4*row+col], _ = value.Float64()
				}
			}
			centroid, err := world.ApplyExact(mass.Centroid)
			if err != nil {
				t.Fatal(err)
			}
			for j, value := range centroid {
				m.Centroid[j], _ = value.Float64()
			}
			want = append(want, m)
		}
		for _, file := range [][]byte{original, []byte(p.CorrectedModelSTEP())} {
			checkNestedMixedConicalAP242Live(t, oracle, file, want)
		}
	}
}

func TestRepairNestedMixedConicalAP242AssemblyReject(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	raw := nestedMixedConicalAP242Fixture(t, "rounded")
	budget := NestedMixedConicalAP242RepairOptions{3e-12}
	reject := func(t *testing.T, data []byte, l Limits, o NestedMixedConicalAP242RepairOptions, detail string) {
		t.Helper()
		m, err := RepairNestedMixedConicalAP242Assembly(ctx, bytes.NewReader(data), l, o)
		if err == nil || m.OriginalSTEP() != "" || m.CorrectedModelSTEP() != "" || m.Assembly().Assembly.Source != nil {
			t.Fatalf("invalid source returned partial material: %v", err)
		}
		if detail != "" && !strings.Contains(err.Error(), detail) {
			t.Fatalf("unexpected failed proof: %v", err)
		}
	}
	reject(t, raw, Limits{}, NestedMixedConicalAP242RepairOptions{}, "")
	reject(t, raw, Limits{}, NestedMixedConicalAP242RepairOptions{2.8e-12}, "part #62")
	reject(t, raw, Limits{MaxBytes: int64(len(raw) - 1)}, budget, "")
	reject(t, raw, Limits{MaxVertices: 55}, budget, "verify corrected nested AP242 assembly")
	reject(t, nestedMixedConicalAP242Fixture(t, "exact"), Limits{}, budget, "")
	reject(t, mixedConicalAP242Fixture(t, "rounded"), Limits{}, budget, "authenticate rounded nested AP242 hierarchy")
	for _, tc := range []struct {
		name, before, after, detail string
	}{
		{"inexact original pose before repair", "#18 = DIRECTION('',(0.6,0.8,0.));", "#18 = DIRECTION('',(0.6,0.800000000000000001,0.));", "authenticate rounded nested AP242 hierarchy"},
		{"body closure borrowed across products", "#243 = VERTEX_POINT('',#244);", "#243 = VERTEX_POINT('',#76);", "authenticate rounded nested AP242 hierarchy"},
		{"third use misattached to root", "#443 = NEXT_ASSEMBLY_USAGE_OCCURRENCE('5','=>[0:1:1:4]','',#5,#31,$);", "#443 = NEXT_ASSEMBLY_USAGE_OCCURRENCE('5','=>[0:1:1:4]','',#5,#62,$);", "authenticate rounded nested AP242 hierarchy"},
		{"chamfer too far after valid rolling repair", "#267,5.,0.244978663127", "#267,5.,0.244978663227", "part #230"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if bytes.Count(raw, []byte(tc.before)) != 1 {
				t.Fatalf("mutation does not target one source record: %s", tc.before)
			}
			reject(t, bytes.Replace(raw, []byte(tc.before), []byte(tc.after), 1), Limits{}, budget, tc.detail)
		})
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if m, err := RepairNestedMixedConicalAP242Assembly(canceled, bytes.NewReader(raw), Limits{}, budget); !errors.Is(err, context.Canceled) || m.OriginalSTEP() != "" {
		t.Fatalf("canceled repair returned material: %v", err)
	}
}

func TestRepairedNestedMixedConicalAP242AssemblyReceipt(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	raw := nestedMixedConicalAP242Fixture(t, "rounded")
	m, err := RepairNestedMixedConicalAP242Assembly(ctx, bytes.NewReader(raw), Limits{}, NestedMixedConicalAP242RepairOptions{3e-12})
	if err != nil {
		t.Fatal(err)
	}
	var first, repeated bytes.Buffer
	if err := WriteRepairedNestedMixedConicalAP242JSON(ctx, &first, m); err != nil {
		t.Fatal(err)
	}
	got, err := ReadRepairedNestedMixedConicalAP242JSON(ctx, bytes.NewReader(first.Bytes()), IndependentMaterialJSONLimits{})
	if err != nil || got.OriginalSTEP() != string(raw) || got.CorrectedModelSTEP() != m.CorrectedModelSTEP() ||
		!reflect.DeepEqual(got.PartRepairs(), m.PartRepairs()) {
		t.Fatalf("receipt failed to reconstruct both local corrections: %v", err)
	}
	if err := WriteRepairedNestedMixedConicalAP242JSON(ctx, &repeated, got); err != nil || !bytes.Equal(first.Bytes(), repeated.Bytes()) {
		t.Fatalf("receipt bytes did not replay: %v", err)
	}
	reject := func(raw []byte, l IndependentMaterialJSONLimits) {
		t.Helper()
		x, err := ReadRepairedNestedMixedConicalAP242JSON(ctx, bytes.NewReader(raw), l)
		if err == nil || x.OriginalSTEP() != "" || x.Assembly().Assembly.Source != nil {
			t.Fatalf("bad receipt returned partial material: %v", err)
		}
	}
	reject(first.Bytes(), IndependentMaterialJSONLimits{MaxBytes: int64(first.Len() - 1)})
	reject(first.Bytes(), IndependentMaterialJSONLimits{Source: Limits{MaxEntities: 442}})
	for _, name := range []string{"source digest", "tight budget", "world pose", "remapped ID"} {
		t.Run(name, func(t *testing.T) {
			q, err := readRationalPosedPCurveRepairEnvelope(first.Bytes())
			if err != nil {
				t.Fatal(err)
			}
			var c nestedMixedConicalAP242RepairClaim
			if err := json.Unmarshal(q.Claims, &c); err != nil {
				t.Fatal(err)
			}
			switch name {
			case "source digest":
				q.Source = append(q.Source, '\n')
			case "tight budget":
				q.MaxDistance = 2.8e-12
			case "world pose":
				c.Instances[5].Pose[0][3] = big.NewRat(-101, 1)
			case "remapped ID":
				c.Parts[1].GeneratedEntityIDs[443] = 443
			}
			q.Claims, err = json.Marshal(c)
			if err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(q)
			if err != nil {
				t.Fatal(err)
			}
			reject(data, IndependentMaterialJSONLimits{})
		})
	}
}
