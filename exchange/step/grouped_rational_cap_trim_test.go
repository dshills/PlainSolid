package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/geom"
	"testing"
)

func TestGroupedCapFiniteTrim(t *testing.T) {
	b, e := os.ReadFile("testdata/occt-both-default-oblique-prism-fillet.step")
	if e != nil {
		t.Fatal(e)
	}
	q, e := InspectGroupedConvexArcFiniteTrims(context.Background(), bytes.NewReader(b), Limits{}, ConvexArcRepairOptions{1e-12}, geom.ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for _, v := range q.Trims {
		t.Log(v.SourceEnd.Lower.FloatString(16), v.SourceEnd.Upper.FloatString(16), v.EndPolynomial, q.RootOperations, q.RootSubdivisions)
		if v.Wraps || !v.StartsAtSeam || v.CapCoedgeID == v.CylinderCoedgeID || v.SourceStart.Sign() != 0 {
			t.Fatal(v)
		}
		s, e := v.EndStation.SignPolynomial(context.Background(), []*big.Rat{big.NewRat(-2, 1), big.NewRat(2, 1), big.NewRat(1, 1)}, geom.ExactCurvePlaneEventOptions{})
		if e != nil || s.Sign != 0 {
			t.Fatal("sqrt3-1", s, e)
		}
	}
}
func TestGroupedCapFiniteTrimReject(t *testing.T) {
	b, e := os.ReadFile("testdata/occt-both-default-oblique-prism-fillet.step")
	if e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	q, e := InspectGroupedConvexArcFiniteTrims(ctx, bytes.NewReader(b), Limits{}, ConvexArcRepairOptions{1e-12}, geom.ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for _, o := range []geom.ExactCurvePlaneEventOptions{{MaxOperations: 1}, {MaxOperations: q.RootOperations - 1}, {MaxSubdivisions: 1}, {MaxCoefficientBits: 1}} {
		v, e := InspectGroupedConvexArcFiniteTrims(ctx, bytes.NewReader(b), Limits{}, ConvexArcRepairOptions{1e-12}, o)
		if e == nil || len(v.Trims) > 0 {
			t.Fatal("root budget", o, e)
		}
	}
	for _, name := range []string{"endpoint ray", "endpoint order", "circle axis", "one coedge"} {
		t.Run(name, func(t *testing.T) {
			f, e := Parse(bytes.NewReader(b), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			trim := q.Trims[0]
			switch name {
			case "endpoint ray":
				v := f.Entities[trim.EndVertexID].Records[0]
				f.Entities[v.Args[1].Ref].Records[0].Args[1].Items[1].Text = "0.25000000000000001"
			case "endpoint order":
				r := f.Entities[trim.CapOwner.EdgeID].Records[0]
				r.Args[1], r.Args[2] = r.Args[2], r.Args[1]
			case "circle axis":
				r := f.Entities[trim.CircleID].Records[0]
				p := f.Entities[r.Args[1].Ref].Records[0]
				f.Entities[p.Args[2].Ref].Records[0].Args[1].Items[2].Text = "1."
			case "one coedge":
				r := f.Entities[trim.CapCoedgeID].Records[0]
				if r.Args[4].Text == "T" {
					r.Args[4].Text = "F"
				} else {
					r.Args[4].Text = "T"
				}
			}
			raw, e := mappedPlanarSourceBytes(f)
			if e != nil {
				t.Fatal(e)
			}
			v, e := InspectGroupedConvexArcFiniteTrims(ctx, bytes.NewReader(raw), Limits{}, ConvexArcRepairOptions{1e-12}, geom.ExactCurvePlaneEventOptions{})
			if e == nil || len(v.Trims) > 0 {
				t.Fatal("accepted", e)
			}
		})
	}
	c, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = InspectGroupedConvexArcFiniteTrims(c, bytes.NewReader(b), Limits{}, ConvexArcRepairOptions{1e-12}, geom.ExactCurvePlaneEventOptions{}); e != context.Canceled {
		t.Fatal(e)
	}
}
func TestGroupedCapFiniteTrimLive(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_CONVEX_ARC_ORIGINAL")
	if bin == "" {
		t.Skip("set pinned author")
	}
	path := filepath.Join(t.TempDir(), "grouped.step")
	if out, e := exec.Command(bin, "write-both-default", path).CombinedOutput(); e != nil {
		t.Fatal(e, string(out))
	}
	b, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	q, e := InspectGroupedConvexArcFiniteTrims(context.Background(), bytes.NewReader(b), Limits{}, ConvexArcRepairOptions{1e-12}, geom.ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	out, e := exec.Command(bin, "cap-trim", path).CombinedOutput()
	if e != nil {
		t.Fatal(e, string(out))
	}
	at := bytes.LastIndex(out, []byte("{\"Caps\""))
	if at < 0 {
		t.Fatal(string(out))
	}
	var v struct {
		Caps                                        int
		MinStation, MaxStation, MaxEndpointDistance float64
	}
	if e = json.Unmarshal(bytes.TrimSpace(out[at:]), &v); e != nil {
		t.Fatal(e)
	}
	if v.Caps != 2 || v.MinStation != v.MaxStation {
		t.Fatal(v)
	}
	for _, trim := range q.Trims {
		lo, _ := trim.SourceEnd.Lower.Float64()
		hi, _ := trim.SourceEnd.Upper.Float64()
		bound, _ := trim.EndpointDistanceUpper[1].Float64()
		if v.MinStation < lo || v.MinStation > hi || v.MaxEndpointDistance > bound+1e-15 {
			t.Fatal(v, lo, hi, bound)
		}
	}
}
func FuzzGroupedCapFiniteTrim(f *testing.F) {
	b, e := os.ReadFile("testdata/occt-both-default-oblique-prism-fillet.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(b)
	f.Add([]byte("ISO-10303-21;"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<20 {
			t.Skip()
		}
		q, e := InspectGroupedConvexArcFiniteTrims(context.Background(), bytes.NewReader(b), Limits{}, ConvexArcRepairOptions{1e-12}, geom.ExactCurvePlaneEventOptions{})
		if e != nil && len(q.Trims) > 0 {
			t.Fatal("partial trim")
		}
	})
}
