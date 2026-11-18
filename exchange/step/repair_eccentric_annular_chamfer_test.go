package step

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"os"
	"plainsolid/internal/slowtest"
	"reflect"
	"strings"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestRepairEccentricAnnularChamferSTEP(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	raw := eccentricAnnularChamferSTEPFixture(t, "rounded")
	if _, err := ReadEccentricAnnularChamfer(ctx, bytes.NewReader(raw), Limits{}, geom.SurfacePatchOptions{}); err == nil {
		t.Fatal("strict reader accepted the unchanged rounded source")
	}
	repaired, err := RepairEccentricAnnularChamfer(ctx, bytes.NewReader(raw), Limits{}, EccentricAnnularChamferRepairOptions{2e-12})
	if err != nil {
		t.Fatal(err)
	}
	if repaired.OriginalSTEP() != string(raw) {
		t.Fatal("original source bytes changed")
	}
	want := []struct {
		ids        [8]int64
		axis       int
		slope      string
		z, axial   [2]string
		cone, seam [2]float64
	}{
		{[8]int64{17, 45, 46, 48, 21, 26, 39, 31}, 1, "1/4", [2]string{"4", "9"}, [2]string{"2", "7"}, [2]float64{1.01e-12, 1.02e-12}, [2]float64{1.80e-12, 1.81e-12}},
		{[8]int64{50, 70, 71, 73, 54, 57, 62, 39}, 1, "3/8", [2]string{"2", "4"}, [2]string{"0", "2"}, [2]float64{9.7e-13, 9.8e-13}, [2]float64{8e-13, 8.1e-13}},
		{[8]int64{75, 95, 96, 98, 79, 82, 87, 31}, -1, "1/4", [2]string{"10", "9"}, [2]string{"0", "1"}, [2]float64{1.4e-13, 1.5e-13}, [2]float64{2.5e-13, 2.6e-13}},
		{[8]int64{140, 160, 161, 163, 144, 147, 107, 154}, -1, "1/16", [2]string{"2", "3"}, [2]string{"0", "-1"}, [2]float64{2.6e-15, 2.7e-15}, [2]float64{7.5e-15, 7.6e-15}},
		{[8]int64{165, 185, 186, 188, 169, 172, 127, 179}, 1, "5/16", [2]string{"10", "8"}, [2]string{"0", "-2"}, [2]float64{6.2e-14, 6.3e-14}, [2]float64{1.3e-13, 1.4e-13}},
		{[8]int64{190, 202, 203, 205, 194, 195, 154, 179}, 1, "1/8", [2]string{"3", "8"}, [2]string{"1", "6"}, [2]float64{1.45e-12, 1.46e-12}, [2]float64{1.51e-12, 1.52e-12}},
	}
	checkBound := func(bound string, interval [2]float64) {
		t.Helper()
		q, ok := new(big.Rat).SetString(bound)
		if !ok || q.Cmp(sideRat(interval[0])) <= 0 || q.Cmp(sideRat(interval[1])) >= 0 {
			t.Fatalf("exact displacement enclosure outside expected source interval %g", interval)
		}
	}
	for i, wall := range repaired.Walls() {
		ids := [8]int64{wall.FaceID, wall.ConeID, wall.PlacementID, wall.AxisDirectionID, wall.SeamEdgeID, wall.SeamCurveID, wall.RimEdgeIDs[0], wall.RimEdgeIDs[1]}
		w := want[i]
		if ids != w.ids || wall.AxisSign != w.axis || wall.ExactSlope != w.slope ||
			wall.OriginalZStations != w.z || wall.OriginalAxialStations != w.axial || wall.CorrectedGeneratorID != 213+int64(i) {
			t.Fatalf("wrong directed wall record %d: %+v", i, wall)
		}
		checkBound(wall.ConeDisplacementBound, w.cone)
		checkBound(wall.SeamDisplacementBound, w.seam)
	}
	m := repaired.Material()
	if m.LengthUnit != .001 || m.SourceBodyID != 15 || m.SourceShellID != 16 ||
		!reflect.DeepEqual(m.NativeFaceSourceIDs, []int64{50, 17, 75, 100, 120, 140, 190, 165}) {
		t.Fatalf("wrong original unit or face owners: %v", m.NativeFaceSourceIDs)
	}
	counts := map[int64]int{}
	for _, id := range m.NativeEdgeSourceIDs {
		counts[id]++
	}
	if !reflect.DeepEqual(counts, map[int64]int{21: 1, 31: 4, 39: 4, 54: 1, 62: 4, 79: 1, 87: 4, 107: 4, 127: 4, 144: 1, 154: 4, 169: 1, 179: 4, 194: 1}) {
		t.Fatalf("wrong original edge partition: %v", counts)
	}
	f, err := Parse(bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range f.Order {
		en := f.Entities[id]
		if len(en.Records) != 1 {
			continue
		}
		switch en.Records[0].Type {
		case "MANIFOLD_SOLID_BREP", "CLOSED_SHELL", "ADVANCED_FACE", "FACE_BOUND", "EDGE_LOOP",
			"ORIENTED_EDGE", "EDGE_CURVE", "VERTEX_POINT", "CIRCLE", "PLANE":
			if !reflect.DeepEqual(en, m.Source.Entities[id]) {
				t.Fatalf("changed original topology or exact edge geometry #%d", id)
			}
		}
	}
	var points []geom.Vec3
	for i, id := range m.NativeVertexSourceIDs {
		if id == 0 {
			continue
		}
		vertex, err := simple(f, id, "VERTEX_POINT", 2)
		if err != nil {
			t.Fatal(err)
		}
		q, err := mappedExactCoordinate(f, vertex.Args[1].Ref, "CARTESIAN_POINT", 3)
		if err != nil || q != m.Solid.Vertices()[i] {
			t.Fatalf("changed original vertex #%d: %v", id, err)
		}
		points = append(points, q)
	}
	if len(points) != 8 {
		t.Fatalf("lost source vertices: %v", points)
	}
	exact, err := ReadEccentricAnnularChamfer(ctx, bytes.NewReader(eccentricAnnularChamferSTEPFixture(t, "exact")), Limits{}, geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := ReadEccentricAnnularChamfer(ctx, strings.NewReader(repaired.CorrectedModelSTEP()), Limits{}, geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	a, err := prismNativeBytes(m.Solid)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []EccentricAnnularChamferSTEP{exact, reopened} {
		b, err := prismNativeBytes(p.Solid)
		if err != nil || !bytes.Equal(a, b) {
			t.Fatalf("exact independent or reopened native body changed: %v", err)
		}
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_ECCENTRIC_ANNULAR_IMPORT_READER"); oracle != "" {
		mass, err := m.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
		if err != nil {
			t.Fatal(err)
		}
		for _, source := range [][]byte{raw, []byte(repaired.CorrectedModelSTEP())} {
			verifyEccentricAnnularChamferSTEPConsumer(t, oracle, source, points, 14, mass, "--full-rims")
		}
	}
}

func TestRepairEccentricAnnularChamferSTEPReject(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	raw := eccentricAnnularChamferSTEPFixture(t, "rounded")
	reject := func(t *testing.T, data []byte, options EccentricAnnularChamferRepairOptions, limits Limits, detail string) {
		t.Helper()
		m, err := RepairEccentricAnnularChamfer(ctx, bytes.NewReader(data), limits, options)
		if err == nil || m.OriginalSTEP() != "" || m.CorrectedModelSTEP() != "" || m.Material().Source != nil {
			t.Fatalf("unsupported source returned partial material: %v", err)
		}
		if detail != "" && !strings.Contains(err.Error(), detail) {
			t.Fatalf("wrong failed proof: %v", err)
		}
	}
	reject(t, raw, EccentricAnnularChamferRepairOptions{}, Limits{}, "")
	reject(t, raw, EccentricAnnularChamferRepairOptions{math.NaN()}, Limits{}, "")
	reject(t, raw, EccentricAnnularChamferRepairOptions{1e-12}, Limits{}, "finite cone angle displacement")
	reject(t, raw, EccentricAnnularChamferRepairOptions{1.6e-12}, Limits{}, "finite line seam displacement")
	budget := EccentricAnnularChamferRepairOptions{2e-12}
	reject(t, raw, budget, Limits{MaxVertices: 7}, "")
	reject(t, raw, budget, Limits{MaxEdgeUses: 27}, "")
	reject(t, raw, budget, Limits{MaxBytes: int64(len(raw) - 1)}, "")
	reject(t, eccentricAnnularChamferSTEPFixture(t, "exact"), budget, Limits{}, "")
	for _, tc := range []struct {
		name, before, after, detail string
	}{
		{"angle beyond the source bound", "#46,5.,0.244978663127", "#46,5.,0.244978663227", "finite cone angle displacement"},
		{"source rooted radius", "#46,5.,0.244978663127", "#46,5.125,0.244978663127", ""},
		{"wrong axis for the lower inner chamfer", "#163 = DIRECTION('',(-0.,-0.,-1.));", "#163 = DIRECTION('',(0.,0.,1.));", ""},
		{"line phase outside the source budget", "-5.940409537599E-17", "-5.940409537599E-7", "finite line seam displacement"},
		{"reversed signed line", "(0.242535625036,0.,-0.970142500145)", "(-0.242535625036,0.,0.970142500145)", "original line direction"},
		{"same-sense seam", "#37 = ORIENTED_EDGE('',*,*,#21,.F.);", "#37 = ORIENTED_EDGE('',*,*,#21,.T.);", ""},
		{"reversed cap owner", "#115,.F.);", "#115,.T.);", ""},
		{"moved original exact rim", "#33,6.75);", "#33,6.751);", ""},
		{"isolated model point", "ENDSEC;\nEND-ISO-10303-21;", "#300 = CARTESIAN_POINT('',(0.,0.,0.));\nENDSEC;\nEND-ISO-10303-21;", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if bytes.Count(raw, []byte(tc.before)) != 1 {
				t.Fatalf("unexpected source record for mutation %q", tc.before)
			}
			reject(t, bytes.Replace(raw, []byte(tc.before), []byte(tc.after), 1), budget, Limits{}, tc.detail)
		})
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if got, err := RepairEccentricAnnularChamfer(canceled, bytes.NewReader(raw), Limits{}, budget); !errors.Is(err, context.Canceled) || got.OriginalSTEP() != "" {
		t.Fatalf("canceled repair returned material: %v", err)
	}
}

func TestRepairedEccentricAnnularChamferSTEPReceipt(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	raw := eccentricAnnularChamferSTEPFixture(t, "rounded")
	m, err := RepairEccentricAnnularChamfer(ctx, bytes.NewReader(raw), Limits{}, EccentricAnnularChamferRepairOptions{2e-12})
	if err != nil {
		t.Fatal(err)
	}
	var first, repeated bytes.Buffer
	if err := WriteRepairedEccentricAnnularChamferJSON(ctx, &first, m); err != nil {
		t.Fatal(err)
	}
	got, err := ReadRepairedEccentricAnnularChamferJSON(ctx, bytes.NewReader(first.Bytes()), IndependentMaterialJSONLimits{})
	if err != nil || got.OriginalSTEP() != string(raw) || got.CorrectedModelSTEP() != m.CorrectedModelSTEP() || got.Walls() != m.Walls() {
		t.Fatalf("receipt did not replay original source and corrected file: %v", err)
	}
	if err := WriteRepairedEccentricAnnularChamferJSON(ctx, &repeated, got); err != nil || !bytes.Equal(first.Bytes(), repeated.Bytes()) {
		t.Fatalf("receipt did not reproduce bytes: %v", err)
	}
	reject := func(raw []byte, l IndependentMaterialJSONLimits) {
		t.Helper()
		p, err := ReadRepairedEccentricAnnularChamferJSON(ctx, bytes.NewReader(raw), l)
		if err == nil || p.OriginalSTEP() != "" || p.Material().Source != nil {
			t.Fatalf("altered receipt returned source or material: %v", err)
		}
	}
	reject(first.Bytes(), IndependentMaterialJSONLimits{MaxBytes: int64(first.Len() - 1)})
	reject(first.Bytes(), IndependentMaterialJSONLimits{Source: Limits{MaxEntities: 211}})
	for _, name := range []string{"original bytes", "caller budget", "corrected digest", "signed wall axis"} {
		t.Run(name, func(t *testing.T) {
			q, err := readRationalPosedPCurveRepairEnvelope(first.Bytes())
			if err != nil {
				t.Fatal(err)
			}
			var claim eccentricAnnularChamferRepairClaim
			if err := json.Unmarshal(q.Claims, &claim); err != nil {
				t.Fatal(err)
			}
			switch name {
			case "original bytes":
				q.Source = append(q.Source, '\n')
			case "caller budget":
				q.MaxDistance *= 2
			case "corrected digest":
				claim.CorrectedSHA256 = claim.SourceSHA256
			case "signed wall axis":
				claim.Walls[3].AxisSign = 1
			}
			q.Claims, err = json.Marshal(claim)
			if err != nil {
				t.Fatal(err)
			}
			d, err := json.Marshal(q)
			if err != nil {
				t.Fatal(err)
			}
			reject(d, IndependentMaterialJSONLimits{})
		})
	}
}
