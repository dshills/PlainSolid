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
	"slices"
	"strings"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestRepairConicalRimFilletBoreOCCT(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	original := conicalRimFilletBoreFixture(t, "rounded")
	repaired, err := RepairConicalRimFilletBore(ctx, bytes.NewReader(original), Limits{}, ConicalRimFilletBoreRepairOptions{1e-11})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(original, []byte(repaired.OriginalSTEP())) {
		t.Fatal("original OCCT bytes changed")
	}
	e := repaired.Evidence()
	if e.ConeSupportID != 45 || e.ConeFaceID != 17 || e.SeamCurveID != 26 ||
		e.ConeRimEdgeIDs != [2]int64{39, 31} || e.ConeSeamVertexIDs != [2]int64{22, 24} ||
		e.OriginalAngle != "0.927295218002" || e.ExactSlope != "4/3" ||
		e.OriginalRimStations != [2]string{"3/16", "21/8"} ||
		e.ReversedFaceID != 50 || e.ReversedBoundID != 51 || e.ReversedRimEdgeIDs != [2]int64{39, 65} ||
		e.OriginalRimOwnerFaceIDs != [2][2]int64{{17, 50}, {50, 102}} || e.OriginalRimUseSenses != [2]bool{true, false} {
		t.Fatalf("wrong original topology or cone evidence: %+v", e)
	}
	checkBound := func(value string, lo, hi float64) {
		t.Helper()
		q, ok := new(big.Rat).SetString(value)
		if !ok || q.Cmp(sideRat(lo)) <= 0 || q.Cmp(sideRat(hi)) >= 0 {
			t.Fatalf("source-unit displacement bound %s outside (%g, %g)", value, lo, hi)
		}
	}
	checkBound(e.ConeDisplacementBound, 2.8e-12, 2.9e-12)
	checkBound(e.SeamDisplacementBound, 8e-16, 9e-16)
	wantFrames := [][5]int64{{71, 72, 0, 0, 75}, {97, 98, 0, 0, 101}, {32, 33, 0, 0, 36},
		{40, 41, 0, 0, 44}, {57, 58, 54, 60, 61}, {66, 67, 0, 0, 70}, {83, 84, 80, 86, 87}, {92, 93, 0, 0, 96}}
	var gotFrames [][5]int64
	for _, f := range e.Frames {
		gotFrames = append(gotFrames, [5]int64{f.SupportID, f.PlacementID, f.EdgeID, f.CorrectedDirectionIDs[0], f.CorrectedDirectionIDs[1]})
		checkBound(f.DisplacementBound, 5e-16, 3e-15)
	}
	if !reflect.DeepEqual(gotFrames, wantFrames) {
		t.Fatalf("wrong rounded support/seam owners: %v", gotFrames)
	}
	raw, err := Parse(bytes.NewReader(original), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	m := repaired.Material()
	changed := []int64{26, 36, 44, 45, 51, 60, 61, 70, 75, 86, 87, 96, 101}
	removed := []int64{27, 28, 29, 46, 49}
	for _, id := range raw.Order {
		got, kept := m.Source.Entities[id]
		if slices.Contains(removed, id) {
			if kept {
				t.Fatalf("obsolete original support #%d remained reachable", id)
			}
			continue
		}
		if !kept || reflect.DeepEqual(got, raw.Entities[id]) == slices.Contains(changed, id) {
			t.Fatalf("unexpected correction ledger for original entity #%d", id)
		}
	}
	if m.LengthUnit != .001 || m.SourceBodyID != 15 || m.SourceShellID != 16 ||
		!reflect.DeepEqual(m.NativeFaceSourceIDs, []int64{17, 50, 102, 76, 122, 142}) {
		t.Fatalf("wrong original unit, body, shell or face owners: %g #%d #%d %v", m.LengthUnit, m.SourceBodyID, m.SourceShellID, m.NativeFaceSourceIDs)
	}
	for i, id := range m.NativeVertexSourceIDs {
		if id == 0 {
			continue
		}
		vertex, err := simple(raw, id, "VERTEX_POINT", 2)
		if err != nil {
			t.Fatal(err)
		}
		want, err := mappedExactCoordinate(raw, vertex.Args[1].Ref, "CARTESIAN_POINT", 3)
		if err != nil || m.Solid.Vertices()[i] != want {
			t.Fatalf("original vertex #%d moved: %v %v", id, want, err)
		}
	}
	edges := map[int64]int{}
	for _, id := range m.NativeEdgeSourceIDs {
		edges[id]++
	}
	wantEdges := map[int64]int{21: 1, 31: 4, 39: 4, 54: 1, 65: 4, 80: 1, 91: 4, 109: 4, 129: 4, 147: 1}
	if !reflect.DeepEqual(edges, wantEdges) {
		t.Fatalf("wrong original edge owners: %v", edges)
	}
	exact, err := ReadConicalRimFilletBore(ctx, bytes.NewReader(conicalRimFilletBoreFixture(t, "exact")), Limits{}, geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := ReadConicalRimFilletBore(ctx, strings.NewReader(repaired.CorrectedModelSTEP()), Limits{}, geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatalf("corrected model strict reopen: %v", err)
	}
	native, err := prismNativeBytes(m.Solid)
	if err != nil {
		t.Fatal(err)
	}
	for _, peer := range []ConicalRimFilletBoreSTEP{exact, reopened} {
		same, err := prismNativeBytes(peer.Solid)
		if err != nil || !bytes.Equal(native, same) {
			t.Fatalf("same exact native body did not serialize identically: %v", err)
		}
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_CONICAL_RIM_BORE_STEP_READER"); oracle != "" {
		mass, err := m.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
		if err != nil {
			t.Fatal(err)
		}
		points := map[[3]float64]bool{}
		for i, id := range m.NativeVertexSourceIDs {
			if id != 0 {
				q := m.Solid.Vertices()[i]
				points[[3]float64{q.X, q.Y, q.Z}] = true
			}
		}
		verifyConicalRimFilletBoreComparator(t, oracle, []byte(repaired.CorrectedModelSTEP()), points, mass)
	}
}

func TestRepairConicalRimFilletBoreReject(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	raw := conicalRimFilletBoreFixture(t, "rounded")
	tests := []struct {
		name, from, to, message string
		distance                float64
		limits                  Limits
	}{
		{name: "tight angle budget", distance: 1e-12, message: "finite cone angle displacement"},
		{name: "NaN budget", distance: math.NaN()},
		{name: "angle moved farther", distance: 1e-11, from: "0.927295218002", to: "0.92730", message: "finite cone angle displacement"},
		{name: "source cone root radius", distance: 1e-11, from: "#46,8.,", to: "#46,8.125,"},
		{name: "seam outside limit", distance: 1e-11, from: "-1.959434878636E-16", to: "-1.959434878636E-9", message: "finite line seam displacement"},
		{name: "source seam direction", distance: 1e-11, from: "(0.8,-1.959434878636E-16,0.6)", to: "(-0.8,1.959434878636E-16,-0.6)"},
		{name: "meridian frame outside limit", distance: 1e-11, from: "(5.551115123126E-17,1.,0.));\n#61 = DIRECTION('',(1.,-5.551115123126E-17,0.))", to: "(5.551115123126E-8,1.,0.));\n#61 = DIRECTION('',(1.,-5.551115123126E-8,0.))", message: "circular frame #57 exceeds"},
		{name: "complementary meridian", distance: 1e-11, from: "(5.551115123126E-17,1.,0.)", to: "(-5.551115123126E-17,-1.,0.)"},
		{name: "off-support meridian", distance: 1e-11, from: "#58,0.9375)", to: "#58,0.925)"},
		{name: "lost original torus tangency", distance: 1e-11, from: "#72,7.6875,0.9375)", to: "#72,7.75,0.9375)"},
		{name: "different lower bound", distance: 1e-11, from: "#52,.F.)", to: "#52,.T.)"},
		{name: "third edge uses same sense", distance: 1e-11, from: "#91,.T.)", to: "#91,.F.)"},
		{name: "isolated source geometry", distance: 1e-11, from: "ENDSEC;\nEND-ISO", to: "#400 = CARTESIAN_POINT('',(0.,0.,0.));\nENDSEC;\nEND-ISO"},
		{name: "too few bytes", distance: 1e-11, limits: Limits{MaxBytes: int64(len(raw) - 1)}},
		{name: "too few entities", distance: 1e-11, limits: Limits{MaxEntities: 163}},
		{name: "too few edge uses", distance: 1e-11, limits: Limits{MaxEdgeUses: 19}},
		{name: "implicit repair budget"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			source := raw
			if tc.from != "" {
				if bytes.Count(raw, []byte(tc.from)) != 1 {
					t.Fatalf("negative did not identify one original record: %q", tc.from)
				}
				source = bytes.Replace(source, []byte(tc.from), []byte(tc.to), 1)
			}
			got, err := RepairConicalRimFilletBore(ctx, bytes.NewReader(source), tc.limits, ConicalRimFilletBoreRepairOptions{tc.distance})
			if err == nil || !reflect.DeepEqual(got, RepairedConicalRimFilletBoreSTEP{}) {
				t.Fatalf("unsupported input returned partial material: %v", err)
			}
			if tc.message != "" && !strings.Contains(err.Error(), tc.message) {
				t.Fatalf("unexpected failed proof: %v", err)
			}
		})
	}
	exact := conicalRimFilletBoreFixture(t, "exact")
	if got, err := RepairConicalRimFilletBore(ctx, bytes.NewReader(exact), Limits{}, ConicalRimFilletBoreRepairOptions{1e-11}); err == nil || got.OriginalSTEP() != "" {
		t.Fatalf("repair accepted a different source dialect: %v", err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if got, err := RepairConicalRimFilletBore(canceled, bytes.NewReader(raw), Limits{}, ConicalRimFilletBoreRepairOptions{1e-11}); !errors.Is(err, context.Canceled) || got.OriginalSTEP() != "" {
		t.Fatalf("canceled repair returned material: %v", err)
	}
}

func TestRepairedConicalRimFilletBoreReceipt(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	raw := conicalRimFilletBoreFixture(t, "rounded")
	m, err := RepairConicalRimFilletBore(ctx, bytes.NewReader(raw), Limits{}, ConicalRimFilletBoreRepairOptions{1e-11})
	if err != nil {
		t.Fatal(err)
	}
	var first, second bytes.Buffer
	if err := WriteRepairedConicalRimFilletBoreJSON(ctx, &first, m); err != nil {
		t.Fatal(err)
	}
	got, err := ReadRepairedConicalRimFilletBoreJSON(ctx, bytes.NewReader(first.Bytes()), IndependentMaterialJSONLimits{})
	if err != nil || got.OriginalSTEP() != string(raw) || got.CorrectedModelSTEP() != m.CorrectedModelSTEP() || !reflect.DeepEqual(got.Evidence(), m.Evidence()) {
		t.Fatalf("receipt did not reconstruct original source and proof: %v", err)
	}
	if err := WriteRepairedConicalRimFilletBoreJSON(ctx, &second, got); err != nil || !bytes.Equal(first.Bytes(), second.Bytes()) {
		t.Fatalf("receipt did not replay byte-identically: %v", err)
	}
	expectRejected := func(name string, raw []byte, limits IndependentMaterialJSONLimits) {
		t.Helper()
		x, err := ReadRepairedConicalRimFilletBoreJSON(ctx, bytes.NewReader(raw), limits)
		if err == nil || x.OriginalSTEP() != "" || x.Material().Source != nil {
			t.Fatalf("%s returned partial material: %v", name, err)
		}
	}
	for _, name := range []string{"raw bytes", "bound", "sufficient changed budget", "insufficient changed budget", "corrected digest", "native owner"} {
		q, err := readRationalPosedPCurveRepairEnvelope(first.Bytes())
		if err != nil {
			t.Fatal(err)
		}
		var c conicalRimFilletBoreRepairClaim
		if err := json.Unmarshal(q.Claims, &c); err != nil {
			t.Fatal(err)
		}
		switch name {
		case "raw bytes":
			q.Source = append(q.Source, '\n')
		case "bound":
			c.Evidence.SeamDisplacementBound = "0"
		case "sufficient changed budget":
			q.MaxDistance *= 2
		case "insufficient changed budget":
			q.MaxDistance /= 10
		case "corrected digest":
			c.CorrectedSHA256 = c.SourceSHA256
		case "native owner":
			c.EdgeIDs[0]++
		}
		q.Claims, err = json.Marshal(c)
		if err != nil {
			t.Fatal(err)
		}
		change, err := json.Marshal(q)
		if err != nil {
			t.Fatal(err)
		}
		expectRejected(name, change, IndependentMaterialJSONLimits{})
	}
	expectRejected("receipt byte limit", first.Bytes(), IndependentMaterialJSONLimits{MaxBytes: int64(first.Len() - 1)})
	expectRejected("original entity limit", first.Bytes(), IndependentMaterialJSONLimits{Source: Limits{MaxEntities: 163}})
}
