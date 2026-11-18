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
	"time"
)

func nonplanarVoidsFixture(t testing.TB) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/occt-spline-roof-voids.step")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestNonplanarSplineVoidsReject(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	raw := nonplanarVoidsFixture(t)
	m, e := ReadNonplanarSplineVoidsSource(ctx, bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	transformSecond := func(f *File, scale *big.Rat, offset [3]*big.Rat) {
		cl, e := nauoClosure(f, []int64{m.shellIDs[2]}, 32)
		if e != nil {
			t.Fatal(e)
		}
		for id := range cl {
			r, e := simple(f, id, "CARTESIAN_POINT", 2)
			if e != nil || len(r.Args[1].Items) != 3 {
				continue
			}
			for j, v := range r.Args[1].Items {
				q, e := affineSplineSourceRat(v)
				if e != nil {
					t.Fatal(e)
				}
				r.Args[1].Items[j].Text = new(big.Rat).Add(new(big.Rat).Mul(q, scale), offset[j]).FloatString(6)
			}
		}
	}
	for name, edit := range map[string]func(*File){
		"outward-void":         func(f *File) { f.Entities[m.orientedIDs[1]].Records[0].Args[3].Text = "T" },
		"duplicate-occurrence": func(f *File) { f.Entities[m.bodyID].Records[0].Args[2].Items[1].Ref = m.orientedIDs[0] },
		"shared-shell":         func(f *File) { f.Entities[m.orientedIDs[1]].Records[0].Args[2].Ref = m.shellIDs[1] },
		"shared-support": func(f *File) {
			f.Entities[m.faces[2][0]].Records[0].Args[2].Ref = f.Entities[m.faces[1][0]].Records[0].Args[2].Ref
		},
		"overlap-voids": func(f *File) {
			transformSecond(f, big.NewRat(1, 1), [3]*big.Rat{big.NewRat(-1, 1), big.NewRat(-1, 2), big.NewRat(0, 1)})
		},
		"nested-voids": func(f *File) {
			transformSecond(f, big.NewRat(1, 4), [3]*big.Rat{big.NewRat(1, 8), big.NewRat(3, 8), big.NewRat(9, 16)})
		},
		"touching-voids": func(f *File) {
			transformSecond(f, big.NewRat(1, 1), [3]*big.Rat{big.NewRat(-1, 2), big.NewRat(0, 1), big.NewRat(0, 1)})
		},
		"excess-voids": func(f *File) {
			a := &f.Entities[m.bodyID].Records[0].Args[2]
			a.Items = append(a.Items, a.Items[0], a.Items[0], a.Items[0])
		},
		"recursive-shell": func(f *File) { f.Entities[m.orientedIDs[1]].Records[0].Args[2].Ref = m.orientedIDs[1] },
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
			bounded, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()
			q, e := ReadNonplanarSplineVoidsSource(bounded, bytes.NewReader(b), Limits{})
			if e == nil || q.original != "" || len(q.Solid.Faces()) != 0 {
				t.Fatal("accepted", e)
			}
		})
	}
	var native bytes.Buffer
	if e = WriteNonplanarSplineVoidsSourceJSON(ctx, &native, m); e != nil {
		t.Fatal(e)
	}
	for _, edit := range []func(*nonplanarSplineVoidsEnvelope){func(q *nonplanarSplineVoidsEnvelope) { q.Format = nonplanarSplineCavityNativeFormat }, func(q *nonplanarSplineVoidsEnvelope) { q.ShellIDs[2] = q.ShellIDs[1] }, func(q *nonplanarSplineVoidsEnvelope) { q.NativeFaces[2][0] = 6 }, func(q *nonplanarSplineVoidsEnvelope) { q.PCurves[2][0].SurfaceID++ }, func(q *nonplanarSplineVoidsEnvelope) { q.BodiesNative[2] = json.RawMessage(`{}`) }, func(q *nonplanarSplineVoidsEnvelope) { q.Native = json.RawMessage(`{}`) }} {
		var q nonplanarSplineVoidsEnvelope
		json.Unmarshal(native.Bytes(), &q)
		edit(&q)
		b, _ := json.Marshal(q)
		if _, e = ReadNonplanarSplineVoidsSourceJSON(ctx, bytes.NewReader(b), IndependentMaterialJSONLimits{}); e == nil {
			t.Fatal("native forgery")
		}
	}
	for _, l := range []Limits{{MaxBytes: 10}, {MaxEntities: 700}, {MaxValues: 100}, {MaxDepth: 1}, {MaxFaces: 17}, {MaxEdgeUses: 71}, {MaxVertices: 10}} {
		if _, e = ReadNonplanarSplineVoidsSource(ctx, bytes.NewReader(raw), l); e == nil {
			t.Fatal("cumulative limits")
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = ReadNonplanarSplineVoidsSource(cancelled, bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("cancel")
	}
	if _, e = ReadNonplanarSplineVoidsSourceJSON(ctx, bytes.NewReader(native.Bytes()), IndependentMaterialJSONLimits{MaxBytes: 10}); e == nil {
		t.Fatal("native size")
	}
	m.Solid = trimmed.Solid{}
	var out bytes.Buffer
	if e = WriteNonplanarSplineVoids(ctx, &out, m, Limits{}); e == nil || out.Len() != 0 {
		t.Fatal("stale claim")
	}
}
func TestNonplanarSplineVoidsLive(t *testing.T) {
	t.Parallel()
	bin := os.Getenv("PLAINSOLID_OCCT_SPLINE_ROOF_VOIDS")
	if bin == "" {
		t.Skip("independent OCCT")
	}
	for _, mode := range []string{"write", "write-three", "write-four"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			path := filepath.Join(t.TempDir(), "original.step")
			b, e := exec.Command(bin, mode, path).CombinedOutput()
			if e != nil {
				t.Fatal(e, string(b))
			}
			raw, e := os.ReadFile(path)
			if e != nil {
				t.Fatal(e)
			}
			m, e := ReadNonplanarSplineVoidsSource(ctx, bytes.NewReader(raw), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			want := 26.0859375
			area := 69.96101191391968
			if mode != "write" {
				want -= 28. / 512
				area += 58.14941249988128 / 64
			}
			if mode == "write-four" {
				want -= 28. / 512
				area += 58.14941249988128 / 64
			}
			var native bytes.Buffer
			if e = WriteNonplanarSplineVoidsSourceJSON(ctx, &native, m); e != nil {
				t.Fatal(e)
			}
			m, e = ReadNonplanarSplineVoidsSourceJSON(ctx, bytes.NewReader(native.Bytes()), IndependentMaterialJSONLimits{})
			if e != nil {
				t.Fatal(e)
			}
			for _, kind := range []string{"source", "generated"} {
				var out bytes.Buffer
				if kind == "source" {
					e = WriteNonplanarSplineVoidsSource(ctx, &out, m, Limits{})
				} else {
					e = WriteNonplanarSplineVoids(ctx, &out, m, Limits{})
				}
				if e != nil {
					t.Fatal(e)
				}
				p := filepath.Join(t.TempDir(), kind+".step")
				os.WriteFile(p, out.Bytes(), 0600)
				b, e := exec.Command(bin, "read", p).CombinedOutput()
				if e != nil {
					t.Fatal(e, string(b))
				}
				var q struct {
					Volume, Area  float64
					Faces, Solids int
				}
				found := false
				for _, line := range bytes.Split(b, []byte("\n")) {
					if json.Unmarshal(line, &q) == nil && q.Solids == 1 {
						found = true
						break
					}
				}
				if !found || q.Faces != 6*(len(m.Voids)+1) || math.Abs(q.Volume-want) > 1e-8 || math.Abs(q.Area-area) > 1e-7 {
					t.Fatal(mode, kind, string(b))
				}
			}
		})
	}
}
func FuzzNonplanarSplineVoidsSource(f *testing.F) {
	f.Add(nonplanarVoidsFixture(f))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<20 {
			t.Skip()
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		m, e := ReadNonplanarSplineVoidsSource(ctx, bytes.NewReader(b), Limits{})
		if e != nil {
			if m.original != "" || len(m.Solid.Faces()) != 0 {
				t.Fatal("partial")
			}
			return
		}
		if len(m.Voids) < 2 || len(m.Voids) > 4 || len(m.Solid.Topology().Shells) != len(m.Voids)+1 {
			t.Fatal("material")
		}
	})
}
