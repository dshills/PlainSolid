package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/geom"
	"reflect"
	"testing"
)

func TestOCCTAnalyticHierarchy(t *testing.T) {
	for _, name := range []string{"independent-analytic-hierarchy-units-nonzero-origin.step", "independent-analytic-hierarchy-occt-frames.step"} {
		t.Run(name, func(t *testing.T) {
			raw, e := os.ReadFile("testdata/" + name)
			if e != nil {
				t.Fatal(e)
			}
			ctx := context.Background()
			target := IndependentMaterialSIUnit(RoundMillimetre)
			m, e := ReadOCCTNormalizedAnalyticHierarchy(ctx, bytes.NewReader(raw), target, Limits{}, 1e-9)
			if e != nil {
				t.Fatal(e)
			}
			if len(m.Mappings) != 6 || len(m.Normalized.Hierarchy.Instances) != 5 {
				t.Fatal("counts")
			}
			if _, e = ReadNormalizedIndependentAnalyticHierarchy(ctx, bytes.NewReader(raw), target, Limits{}, 1e-9); e == nil {
				t.Fatal("strict widened")
			}
			for _, p := range m.Mappings {
				tr, e := simple(m.Normalized.OriginalSource, p.TransformationID, "ITEM_DEFINED_TRANSFORMATION", 4)
				if e != nil || tr.Args[2].Ref != p.FromFrameID || tr.Args[3].Ref != p.ToFrameID {
					t.Fatal("ancestry")
				}
				for _, id := range []int64{p.FromFrameID, p.ToFrameID} {
					original := m.Normalized.OriginalSource.Entities[id]
					derived := m.Normalized.Hierarchy.Assembly.Source.Entities[id]
					if !reflect.DeepEqual(original, derived) {
						t.Fatal("frame records changed")
					}
				}
			}
			var native bytes.Buffer
			if e = WriteOCCTNormalizedAnalyticHierarchyJSON(ctx, &native, m); e != nil {
				t.Fatal(e)
			}
			n, e := ReadOCCTNormalizedAnalyticHierarchyJSON(ctx, bytes.NewReader(native.Bytes()), IndependentMaterialJSONLimits{})
			if e != nil {
				t.Fatal(e)
			}
			var stable bytes.Buffer
			if e = WriteOCCTNormalizedAnalyticHierarchyJSON(ctx, &stable, n); e != nil || !bytes.Equal(native.Bytes(), stable.Bytes()) {
				t.Fatal("stable", e)
			}
			if _, e = ReadNormalizedIndependentAnalyticHierarchyJSON(ctx, bytes.NewReader(native.Bytes()), IndependentMaterialJSONLimits{}); e == nil {
				t.Fatal("old tag")
			}
			var source, normalized bytes.Buffer
			if e = WriteOCCTNormalizedAnalyticHierarchySource(ctx, &source, n.Normalized.OriginalSource, target, Limits{}, 1e-9); e != nil {
				t.Fatal(e)
			}
			if e = WriteIndependentAnalyticHierarchySource(ctx, &normalized, n.Normalized.Hierarchy.Assembly.Source, Limits{}, 1e-9); e != nil {
				t.Fatal(e)
			}
			// Independently compute world bounds/first moments of the known original
			// local solids under each reported native occurrence (no STEP pose helpers).
			bounds := [6]float64{math.Inf(1), math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1), math.Inf(-1)}
			moment := [3]float64{}
			area := 52 + 24*math.Pi*math.Pi + 40*math.Pi
			volume := 24 + 12*math.Pi*math.Pi + 24*math.Pi
			for _, u := range n.Normalized.Hierarchy.Instances {
				p := n.Normalized.Hierarchy.Products[u.ProductID]
				var box [6]float64
				var center [3]float64
				var a float64
				switch {
				case p.Planar != nil:
					box = [6]float64{-10, -4, -2, -8, -1, 2}
					center = [3]float64{-9, -2.5, 0}
					a = 52
				case p.Torus != nil:
					box = [6]float64{-3, -2, 2, 5, 6, 4}
					center = [3]float64{1, 2, 3}
					a = 12 * math.Pi * math.Pi
				case p.Cylinder != nil:
					box = [6]float64{-2, -2, 0, 2, 2, 3}
					center = [3]float64{0, 0, 1.5}
					a = 20 * math.Pi
				default:
					t.Fatal("family")
				}
				for i := 0; i < 3; i++ {
					c := u.Transform[i][3]
					for j := 0; j < 3; j++ {
						c += u.Transform[i][j] * center[j]
					}
					moment[i] += a * c
				}
				for bits := 0; bits < 8; bits++ {
					for i := 0; i < 3; i++ {
						v := u.Transform[i][3]
						for j := 0; j < 3; j++ {
							v += u.Transform[i][j] * box[j+3*((bits>>j)&1)]
						}
						bounds[i] = math.Min(bounds[i], v)
						bounds[i+3] = math.Max(bounds[i+3], v)
					}
				}
			}
			want := [6]float64{-10, -4, -2, 30, 31, 9}
			if name == "independent-analytic-hierarchy-occt-frames.step" {
				want = [6]float64{-10, -7, -2, 27, 7, 24}
			}
			if bounds != want {
				t.Fatal("independent bounds", bounds)
			}
			if oracle := os.Getenv("PLAINSOLID_OCCT_INDEPENDENT_TORUS_ASSEMBLY"); oracle != "" {
				for _, b := range [][]byte{raw, source.Bytes(), normalized.Bytes()} {
					path := filepath.Join(t.TempDir(), "source.step")
					os.WriteFile(path, b, 0600)
					data, e := exec.Command(oracle, "read", path).CombinedOutput()
					if e != nil {
						t.Fatal(e, string(data))
					}
					at := bytes.LastIndex(data, []byte("{"))
					var q struct {
						Area, Volume            float64
						Solids, Faces, EdgeUses int
						Bounds                  [6]float64
						SurfaceCenter           [3]float64
					}
					if at < 0 || json.Unmarshal(data[at:], &q) != nil {
						t.Fatal(string(data))
					}
					if q.Solids != 5 || q.Faces != 20 || q.EdgeUses != 80 || math.Abs(q.Area-area) > 1e-7 || math.Abs(q.Volume-volume) > 1e-7 {
						t.Fatal(q)
					}
					for i, x := range bounds {
						if math.Abs(q.Bounds[i]-x) > 2e-7 {
							t.Fatal("bounds", q.Bounds, bounds)
						}
					}
					for i, x := range moment {
						if math.Abs(q.SurfaceCenter[i]-x/area) > 1e-7 {
							t.Fatal("moment", q.SurfaceCenter, moment)
						}
					}
				}
			}
		})
	}
}
func TestOCCTAnalyticHierarchyReject(t *testing.T) {
	ctx := context.Background()
	raw, _ := os.ReadFile("testdata/independent-analytic-hierarchy-occt-frames.step")
	target := IndependentMaterialSIUnit(RoundMillimetre)
	m, e := ReadOCCTNormalizedAnalyticHierarchy(ctx, bytes.NewReader(raw), target, Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	var n bytes.Buffer
	if e = WriteOCCTNormalizedAnalyticHierarchyJSON(ctx, &n, m); e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"frame", "derived", "transform", "pose", "tag", "native"} {
		t.Run(name, func(t *testing.T) {
			var q occtAnalyticHierarchyEnvelope
			json.Unmarshal(n.Bytes(), &q)
			switch name {
			case "frame":
				q.Mappings[0].FromFrameID++
			case "derived":
				q.Mappings[0].DerivedToFrameID++
			case "transform":
				q.Mappings[0].Transform[0][3]++
			case "pose":
				q.Proof.Proof.Instances[0].Transform[0][3]++
			case "tag":
				q.Format = normalizedAnalyticHierarchyFormat
			case "native":
				q.Proof.Proof.Proof.Members[0].Native = json.RawMessage(`{}`)
			}
			b, _ := json.Marshal(q)
			if x, e := ReadOCCTNormalizedAnalyticHierarchyJSON(ctx, bytes.NewReader(b), IndependentMaterialJSONLimits{}); e == nil || x.Normalized.OriginalSource != nil {
				t.Fatal("accepted")
			}
		})
	}
	for _, name := range []string{"owner", "rotation", "exponent"} {
		t.Run(name, func(t *testing.T) {
			f, _ := Parse(bytes.NewReader(raw), Limits{})
			p := m.Mappings[1]
			switch name {
			case "owner":
				en := f.Entities[p.TransformationID]
				en.Records[0].Args[2].Ref = p.ToFrameID
				f.Entities[p.TransformationID] = en
			case "rotation", "exponent":
				fr, _ := simple(f, p.FromFrameID, "AXIS2_PLACEMENT_3D", 4)
				en := f.Entities[fr.Args[3].Ref]
				en.Records[0].Args[1].Items[0].Text = "1."
				en.Records[0].Args[1].Items[1].Text = "1."
				if name == "exponent" {
					en.Records[0].Args[1].Items[0].Text = "1.E-999999999"
				}
				f.Entities[fr.Args[3].Ref] = en
			}
			b, _ := mappedPlanarSourceBytes(f)
			if x, e := ReadOCCTNormalizedAnalyticHierarchy(ctx, bytes.NewReader(b), target, Limits{}, 1e-9); e == nil || x.Normalized.OriginalSource != nil {
				t.Fatal("accepted")
			}
		})
	}
	m.Mappings[0].Transform = geom.Affine{}
	var out bytes.Buffer
	if e = WriteOCCTNormalizedAnalyticHierarchyJSON(ctx, &out, m); e == nil || out.Len() != 0 {
		t.Fatal("stale")
	}
}
func FuzzOCCTAnalyticHierarchy(f *testing.F) {
	raw, _ := os.ReadFile("testdata/independent-analytic-hierarchy-occt-frames.step")
	f.Add(raw)
	f.Add([]byte("ISO-10303-21;"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 131072 {
			return
		}
		m, e := ReadOCCTNormalizedAnalyticHierarchy(context.Background(), bytes.NewReader(b), IndependentMaterialSIUnit(RoundMillimetre), Limits{MaxBytes: 131072, MaxEntities: 1024, MaxValues: 16384, MaxDepth: 16}, 1e-9)
		if e != nil && m.Normalized.OriginalSource != nil {
			t.Fatal("partial")
		}
	})
}
func FuzzOCCTAnalyticHierarchyNative(f *testing.F) {
	raw, _ := os.ReadFile("testdata/independent-analytic-hierarchy-occt-frames.step")
	m, e := ReadOCCTNormalizedAnalyticHierarchy(context.Background(), bytes.NewReader(raw), IndependentMaterialSIUnit(RoundMillimetre), Limits{}, 1e-9)
	if e != nil {
		f.Fatal(e)
	}
	var out bytes.Buffer
	if e = WriteOCCTNormalizedAnalyticHierarchyJSON(context.Background(), &out, m); e != nil {
		f.Fatal(e)
	}
	f.Add(out.Bytes())
	f.Add([]byte(`{}`))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<20 {
			return
		}
		m, e := ReadOCCTNormalizedAnalyticHierarchyJSON(context.Background(), bytes.NewReader(b), IndependentMaterialJSONLimits{MaxBytes: 1 << 20, Source: Limits{MaxBytes: 131072, MaxEntities: 1024, MaxValues: 16384, MaxDepth: 16}})
		if e != nil && m.Normalized.OriginalSource != nil {
			t.Fatal("partial")
		}
	})
}
func TestOCCTAnalyticHierarchyLimits(t *testing.T) {
	raw, _ := os.ReadFile("testdata/independent-analytic-hierarchy-occt-frames.step")
	ctx := context.Background()
	target := IndependentMaterialSIUnit(RoundMillimetre)
	m, e := ReadOCCTNormalizedAnalyticHierarchy(ctx, bytes.NewReader(raw), target, Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	f, _ := Parse(bytes.NewReader(raw), Limits{})
	for _, l := range []Limits{{MaxBytes: 32}, {MaxEntities: len(f.Entities) + 47}} {
		if _, e = ReadOCCTNormalizedAnalyticHierarchy(ctx, bytes.NewReader(raw), target, l, 1e-9); e == nil {
			t.Fatal("limits including derived frames")
		}
	}
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = ReadOCCTNormalizedAnalyticHierarchy(cancelCtx, bytes.NewReader(raw), target, Limits{}, 1e-9); e == nil {
		t.Fatal("cancel")
	}
	var out bytes.Buffer
	if e = WriteOCCTNormalizedAnalyticHierarchyJSON(cancelCtx, &out, m); e == nil || out.Len() != 0 {
		t.Fatal("cancel write")
	}
	if e = WriteOCCTNormalizedAnalyticHierarchyJSON(ctx, &out, m); e != nil {
		t.Fatal(e)
	}
	for _, l := range []IndependentMaterialJSONLimits{{MaxBytes: 16}, {Source: Limits{MaxEntities: 16}}} {
		if _, e = ReadOCCTNormalizedAnalyticHierarchyJSON(ctx, bytes.NewReader(out.Bytes()), l); e == nil {
			t.Fatal("native limit")
		}
	}
	if _, e = ReadOCCTNormalizedAnalyticHierarchyJSON(cancelCtx, bytes.NewReader(out.Bytes()), IndependentMaterialJSONLimits{}); e == nil {
		t.Fatal("native cancel")
	}
}
