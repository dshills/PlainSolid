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
	"plainsolid/brep/trimmed"
	"plainsolid/internal/slowtest"
	"testing"
)

func nonplanarCavityFixture(t testing.TB) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/occt-spline-roof-void.step")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestNonplanarSplineCavityReject(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	raw := nonplanarCavityFixture(t)
	m, e := ReadNonplanarSplineCavitySource(ctx, bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	moveInner := func(f *File, axis int, delta *big.Rat) {
		cl, e := nauoClosure(f, []int64{m.shellIDs[1]}, 32)
		if e != nil {
			t.Fatal(e)
		}
		for id := range cl {
			r, e := simple(f, id, "CARTESIAN_POINT", 2)
			if e != nil || len(r.Args[1].Items) != 3 {
				continue
			}
			q, e := affineSplineSourceRat(r.Args[1].Items[axis])
			if e != nil {
				t.Fatal(e)
			}
			r.Args[1].Items[axis].Text = new(big.Rat).Add(q, delta).FloatString(4)
		}
	}
	for name, edit := range map[string]func(*File){
		"outward-void": func(f *File) { f.Entities[m.orientedID].Records[0].Args[3].Text = "T" },
		"cross-shell":  func(f *File) { f.Entities[m.orientedID].Records[0].Args[2].Ref = m.shellIDs[0] },
		"shared-support": func(f *File) {
			f.Entities[m.faces[1][0]].Records[0].Args[2].Ref = f.Entities[m.faces[0][0]].Records[0].Args[2].Ref
		},
		"outside":        func(f *File) { moveInner(f, 0, big.NewRat(10, 1)) },
		"touching-floor": func(f *File) { moveInner(f, 2, big.NewRat(-1, 2)) },
		"inverted-nesting": func(f *File) {
			f.Entities[m.bodyID].Records[0].Args[1].Ref = m.shellIDs[1]
			f.Entities[m.orientedID].Records[0].Args[2].Ref = m.shellIDs[0]
		},
		"wrong-inner-owner": func(f *File) { f.Entities[m.curves[1][0].PCurveID].Records[0].Args[1].Ref = m.curves[0][0].SurfaceID },
		"wrong-inner-curve": func(f *File) { f.Entities[m.curves[1][0].PoleIDs[1]].Records[0].Args[1].Items[0].Text = "99" },
	} {
		t.Run(name, func(t *testing.T) {
			f, e := Parse(bytes.NewReader(raw), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			edit(f)
			b, e := mappedPlanarSourceBytes(f)
			if e != nil {
				t.Fatal(e)
			}
			q, e := ReadNonplanarSplineCavitySource(ctx, bytes.NewReader(b), Limits{})
			if e == nil || q.original != "" || len(q.Solid.Faces()) != 0 {
				t.Fatal("accepted", e)
			}
		})
	}
	var native bytes.Buffer
	if e = WriteNonplanarSplineCavitySourceJSON(ctx, &native, m); e != nil {
		t.Fatal(e)
	}
	for _, edit := range []func(*nonplanarSplineCavityEnvelope){func(q *nonplanarSplineCavityEnvelope) { q.Format = nonplanarSplineShellNativeFormat }, func(q *nonplanarSplineCavityEnvelope) { q.History.BReversed = false }, func(q *nonplanarSplineCavityEnvelope) { q.ShellIDs[1] = q.ShellIDs[0] }, func(q *nonplanarSplineCavityEnvelope) { q.PCurves[1][0].EdgeID++ }, func(q *nonplanarSplineCavityEnvelope) { q.Native = json.RawMessage(`{}`) }, func(q *nonplanarSplineCavityEnvelope) { q.Generated = "fake" }} {
		var q nonplanarSplineCavityEnvelope
		json.Unmarshal(native.Bytes(), &q)
		edit(&q)
		b, _ := json.Marshal(q)
		if _, e = ReadNonplanarSplineCavitySourceJSON(ctx, bytes.NewReader(b), IndependentMaterialJSONLimits{}); e == nil {
			t.Fatal("native forgery")
		}
	}
	for _, l := range []Limits{{MaxBytes: 10}, {MaxEntities: 100}, {MaxValues: 100}, {MaxDepth: 1}, {MaxFaces: 11}, {MaxEdgeUses: 47}, {MaxVertices: 10}} {
		if _, e = ReadNonplanarSplineCavitySource(ctx, bytes.NewReader(raw), l); e == nil {
			t.Fatal("limits")
		}
	}
	cctx, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = ReadNonplanarSplineCavitySource(cctx, bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("cancel")
	}
	if _, e = ReadNonplanarSplineCavitySourceJSON(ctx, bytes.NewReader(native.Bytes()), IndependentMaterialJSONLimits{MaxBytes: 10}); e == nil {
		t.Fatal("native limit")
	}
	m.Solid = trimmed.Solid{}
	var out bytes.Buffer
	if e = WriteNonplanarSplineCavity(ctx, &out, m, Limits{}); e == nil || out.Len() != 0 {
		t.Fatal("stale material")
	}
}
func TestNonplanarSplineCavityLive(t *testing.T) {
	t.Parallel()
	bin := os.Getenv("PLAINSOLID_OCCT_SPLINE_ROOF_VOID")
	if bin == "" {
		t.Skip("independent OCCT")
	}
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "original.step")
	if b, e := exec.Command(bin, "write", path).CombinedOutput(); e != nil {
		t.Fatal(e, string(b))
	}
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadNonplanarSplineCavitySource(ctx, bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var native bytes.Buffer
	if e = WriteNonplanarSplineCavitySourceJSON(ctx, &native, m); e != nil {
		t.Fatal(e)
	}
	m, e = ReadNonplanarSplineCavitySourceJSON(ctx, bytes.NewReader(native.Bytes()), IndependentMaterialJSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"source", "generated"} {
		var out bytes.Buffer
		if kind == "source" {
			e = WriteNonplanarSplineCavitySource(ctx, &out, m, Limits{})
		} else {
			e = WriteNonplanarSplineCavity(ctx, &out, m, Limits{})
		}
		if e != nil {
			t.Fatal(e)
		}
		p := filepath.Join(t.TempDir(), kind+".step")
		if e = os.WriteFile(p, out.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
		b, e := exec.Command(bin, "read", p).CombinedOutput()
		if e != nil {
			t.Fatal(e, string(b))
		}
		var report struct {
			Volume, Area  float64
			Faces, Solids int
			Centroid      [3]float64
		}
		found := false
		for _, line := range bytes.Split(b, []byte("\n")) {
			if json.Unmarshal(line, &report) == nil && report.Solids == 1 {
				found = true
				break
			}
		}
		if !found || report.Faces != 12 || math.Abs(report.Volume-27.5625) > 1e-8 || math.Abs(report.Area-61.78375078112386) > 1e-7 || math.Abs(report.Centroid[0]-2.1524943310657596) > 1e-8 {
			t.Fatal(kind, string(b))
		}
	}
}
func FuzzNonplanarSplineCavitySource(f *testing.F) {
	f.Add(nonplanarCavityFixture(f))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<20 {
			t.Skip()
		}
		m, e := ReadNonplanarSplineCavitySource(context.Background(), bytes.NewReader(b), Limits{})
		if e != nil {
			if m.original != "" || len(m.Solid.Faces()) != 0 {
				t.Fatal("partial")
			}
			return
		}
		if len(m.Solid.Topology().Shells) != 2 || !m.History().BReversed {
			t.Fatal("unproved void")
		}
	})
}
