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
	"testing"
)

func TestGroupedRationalCapEvidence(t *testing.T) {
	t.Parallel()
	raw, e := os.ReadFile("testdata/occt-both-default-oblique-prism-fillet.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := InspectGroupedConvexArcPCurves(context.Background(), bytes.NewReader(raw), Limits{}, ConvexArcRepairOptions{1e-12})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Caps) != 2 || len(m.OtherOwners) != 28 {
		t.Fatal(m)
	}
	for _, q := range m.Caps {
		a, _ := q.RadialDistanceUpper.Float64()
		b, _ := q.WitnessDistanceSquaredLower.Float64()
		t.Log(q.Owner.EdgeID, q.Owner.PCurveID, q.Domain, q.WitnessParameter, a, b)
		if q.RadialDistanceUpper.Sign() < 0 || q.RadialDistanceUpper.Cmp(big.NewRat(1, 100000000000)) >= 0 || q.WitnessDistanceSquaredLower.Cmp(big.NewRat(1, 1000000)) <= 0 {
			t.Fatal("bounds")
		}
	}
}

func TestGroupedRationalCapEvidenceReject(t *testing.T) {
	t.Parallel()
	raw, e := os.ReadFile("testdata/occt-both-default-oblique-prism-fillet.step")
	if e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	base, e := InspectGroupedConvexArcPCurves(ctx, bytes.NewReader(raw), Limits{}, ConvexArcRepairOptions{1e-12})
	if e != nil {
		t.Fatal(e)
	}
	cap := base.Caps[0]
	mutations := map[string]func(*File){
		"changed interior pole": func(f *File) { f.Entities[cap.PoleIDs[2]].Records[0].Args[1].Items[0].Text = "-1.4" },
		"closure":               func(f *File) { f.Entities[cap.PoleIDs[6]].Records[0].Args[1].Items[0].Text = "-1.24" },
		"negative weight": func(f *File) {
			for _, r := range f.Entities[cap.Owner.GeometryID].Records {
				if r.Type == "RATIONAL_B_SPLINE_CURVE" {
					r.Args[0].Items[1].Text = "-0.5"
				}
			}
		},
		"wrong knot multiplicity": func(f *File) {
			for _, r := range f.Entities[cap.Owner.GeometryID].Records {
				if r.Type == "B_SPLINE_CURVE_WITH_KNOTS" {
					r.Args[0].Items[2].Text = "1"
				}
			}
		},
		"unordered knots": func(f *File) {
			for _, r := range f.Entities[cap.Owner.GeometryID].Records {
				if r.Type == "B_SPLINE_CURVE_WITH_KNOTS" {
					r.Args[1].Items[2].Text = "-1."
				}
			}
		},
		"exponent": func(f *File) {
			for _, r := range f.Entities[cap.Owner.GeometryID].Records {
				if r.Type == "RATIONAL_B_SPLINE_CURVE" {
					r.Args[0].Items[1].Text = "1.E-999999999"
				}
			}
		},
		"wrong owner":  func(f *File) { f.Entities[cap.Owner.PCurveID].Records[0].Args[1].Ref = base.Caps[1].Owner.SupportID },
		"circle sense": func(f *File) { f.Entities[cap.Owner.EdgeID].Records[0].Args[4].Text = "F" },
		"one coedge": func(f *File) {
			id := base.Reversals[0].OrientedUseIDs[0]
			r := f.Entities[id].Records[0]
			if r.Args[4].Text == "T" {
				r.Args[4].Text = "F"
			} else {
				r.Args[4].Text = "T"
			}
		},
		"opposite traversal": func(f *File) {
			for _, r := range f.Entities[cap.Owner.GeometryID].Records {
				if r.Type == "B_SPLINE_CURVE" {
					for i, j := 0, 6; i < j; i, j = i+1, j-1 {
						r.Args[1].Items[i], r.Args[1].Items[j] = r.Args[1].Items[j], r.Args[1].Items[i]
					}
				}
			}
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			f, e := Parse(bytes.NewReader(raw), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			mutate(f)
			b, e := mappedPlanarSourceBytes(f)
			if e != nil {
				t.Fatal(e)
			}
			q, e := InspectGroupedConvexArcPCurves(ctx, bytes.NewReader(b), Limits{}, ConvexArcRepairOptions{1e-12})
			if e == nil || len(q.Caps) != 0 || len(q.OtherOwners) != 0 {
				t.Fatal("non-atomic acceptance", e)
			}
		})
	}
	if _, e = InspectGroupedConvexArcPCurves(ctx, bytes.NewReader(raw), Limits{}, ConvexArcRepairOptions{1e-14}); e == nil {
		t.Fatal("tolerance")
	}
	if _, e = InspectGroupedConvexArcPCurves(ctx, bytes.NewReader(raw), Limits{MaxEntities: 100}, ConvexArcRepairOptions{1e-12}); e == nil {
		t.Fatal("limit")
	}
	c, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = InspectGroupedConvexArcPCurves(c, bytes.NewReader(raw), Limits{}, ConvexArcRepairOptions{1e-12}); e != context.Canceled {
		t.Fatal(e)
	}
}

func TestGroupedRationalCapEvidenceLive(t *testing.T) {
	t.Parallel()
	bin := os.Getenv("PLAINSOLID_OCCT_CONVEX_ARC_ORIGINAL")
	if bin == "" {
		t.Skip("set pinned OCCT author")
	}
	path := filepath.Join(t.TempDir(), "original.step")
	if out, e := exec.Command(bin, "write-both-default", path).CombinedOutput(); e != nil {
		t.Fatal(e, string(out))
	}
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	q, e := InspectGroupedConvexArcPCurves(context.Background(), bytes.NewReader(raw), Limits{}, ConvexArcRepairOptions{1e-12})
	if e != nil || len(q.Caps) != 2 {
		t.Fatal(e)
	}
	out, e := exec.Command(bin, "read", path).CombinedOutput()
	if e != nil {
		t.Fatal(e, string(out))
	}
	at := bytes.LastIndex(out, []byte("{\"Volume\""))
	if at < 0 {
		t.Fatal(string(out))
	}
	var m struct {
		Volume, Area  float64
		Faces, Solids int
	}
	if e = json.Unmarshal(bytes.TrimSpace(out[at:]), &m); e != nil {
		t.Fatal(e)
	}
	if m.Faces != 7 || m.Solids != 1 || math.Abs(m.Volume-29.077978807115535) > 1e-8 || math.Abs(m.Area-65.622954970084379) > 1e-8 {
		t.Fatal(m)
	}
	witness, e := exec.Command(bin, "cap-witness", path).CombinedOutput()
	if e != nil {
		t.Fatal(e, string(witness))
	}
	at = bytes.LastIndex(witness, []byte("{\"Caps\""))
	if at < 0 {
		t.Fatal(string(witness))
	}
	var w struct {
		Caps                   int
		MinSquared, MaxSquared float64
	}
	if e = json.Unmarshal(bytes.TrimSpace(witness[at:]), &w); e != nil {
		t.Fatal(e)
	}
	if w.Caps != 2 || math.Abs(w.MinSquared-9.2489838783647467e-5) > 1e-15 || w.MinSquared != w.MaxSquared {
		t.Fatal(w)
	}
	for _, c := range q.Caps {
		lower, _ := c.WitnessDistanceSquaredLower.Float64()
		if lower > w.MinSquared+1e-16 || w.MinSquared-lower > 1e-15 {
			t.Fatal("independent raw source evaluation", lower, w)
		}
	}
	// Neither existing ingress route may mistake evidence for a repaired source.
	if _, e = RepairPCurveOCCTConvexArcExtrusion(context.Background(), bytes.NewReader(raw), Limits{}, ConvexArcRepairOptions{1e-12}); e == nil {
		t.Fatal("single route")
	}
	if _, e = RepairGroupedOCCTConvexArcExtrusion(context.Background(), bytes.NewReader(raw), Limits{}, ConvexArcRepairOptions{1e-12}); e == nil {
		t.Fatal("model-only route")
	}
}
func FuzzGroupedRationalCapEvidence(f *testing.F) {
	raw, e := os.ReadFile("testdata/occt-both-default-oblique-prism-fillet.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(raw)
	f.Add([]byte("ISO-10303-21;"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<20 {
			t.Skip()
		}
		q, e := InspectGroupedConvexArcPCurves(context.Background(), bytes.NewReader(b), Limits{}, ConvexArcRepairOptions{1e-12})
		if e != nil && (len(q.Caps) > 0 || len(q.OtherOwners) > 0) {
			t.Fatal("partial evidence")
		}
	})
}
