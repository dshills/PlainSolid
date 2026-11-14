package step

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"plainsolid/brep"
)

var nurbsNativeModes = []string{nurbsNativeUnit, nurbsNativeScaled, nurbsNativeOblique}

func nurbsNativeFixture(t testing.TB, mode string) nurbsNativeModel {
	t.Helper()
	var raw []byte
	switch mode {
	case nurbsNativeUnit:
		raw = nurbsPlanarFixture(t)
	case nurbsNativeScaled:
		raw = scaledNURBSFixture(t)
	case nurbsNativeOblique:
		raw = obliqueNURBSFixture(t)
	default:
		t.Fatal(mode)
	}
	m, e := reproveNURBSNative(context.Background(), mode, string(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	return m
}
func writeNURBSNativeTest(ctx context.Context, w io.Writer, m nurbsNativeModel) error {
	switch {
	case m.unit != nil:
		return WriteNURBSPlanarAssemblyJSON(ctx, w, *m.unit)
	case m.scaled != nil:
		return WriteScaledNURBSPlanarAssemblyJSON(ctx, w, *m.scaled)
	case m.oblique != nil:
		return WriteObliqueNURBSPlanarAssemblyJSON(ctx, w, *m.oblique)
	}
	return io.ErrUnexpectedEOF
}
func readNURBSNativeTest(t testing.TB, ctx context.Context, r io.Reader, mode string, l IndependentMaterialJSONLimits) (nurbsNativeModel, error) {
	t.Helper()
	switch mode {
	case nurbsNativeUnit:
		m, e := ReadNURBSPlanarAssemblyJSON(ctx, r, l)
		if e != nil {
			if m.original != "" || len(m.Geometry.ProductBodies) != 0 {
				t.Fatal("partial unit result")
			}
			return nurbsNativeModel{}, e
		}
		return nurbsNativeModel{unit: &m}, nil
	case nurbsNativeScaled:
		m, e := ReadScaledNURBSPlanarAssemblyJSON(ctx, r, l)
		if e != nil {
			if m.original != "" || len(m.Geometry.ProductBodies) != 0 {
				t.Fatal("partial scaled result")
			}
			return nurbsNativeModel{}, e
		}
		return nurbsNativeModel{scaled: &m}, nil
	case nurbsNativeOblique:
		m, e := ReadObliqueNURBSPlanarAssemblyJSON(ctx, r, l)
		if e != nil {
			if m.original != "" || len(m.Geometry.ProductBodies) != 0 {
				t.Fatal("partial oblique result")
			}
			return nurbsNativeModel{}, e
		}
		return nurbsNativeModel{oblique: &m}, nil
	}
	return nurbsNativeModel{}, io.ErrUnexpectedEOF
}
func nurbsNativeTestBytes(t testing.TB, mode string) (nurbsNativeModel, []byte) {
	t.Helper()
	m := nurbsNativeFixture(t, mode)
	var b bytes.Buffer
	if e := writeNURBSNativeTest(context.Background(), &b, m); e != nil {
		t.Fatal(e)
	}
	return m, b.Bytes()
}
func nurbsNativeBody(q *nurbsAssemblyEnvelope) *occurrenceColorNativeBody {
	for i := range q.Geometry.Products {
		if len(q.Geometry.Products[i].Bodies) > 0 {
			return &q.Geometry.Products[i].Bodies[0]
		}
	}
	return nil
}
func TestNURBSAssemblyNativeRoundtrip(t *testing.T) {
	for _, mode := range nurbsNativeModes {
		t.Run(mode, func(t *testing.T) {
			m, raw := nurbsNativeTestBytes(t, mode)
			ctx := context.Background()
			n, e := readNURBSNativeTest(t, ctx, bytes.NewReader(raw), mode, IndependentMaterialJSONLimits{})
			if e != nil {
				t.Fatal(e)
			}
			a, e := nurbsAssemblyNativeClaims(ctx, m)
			if e != nil {
				t.Fatal(e)
			}
			b, e := nurbsAssemblyNativeClaims(ctx, n)
			if e != nil || !reflect.DeepEqual(a, b) {
				t.Fatal("claim or source loss", e)
			}
			var again bytes.Buffer
			if e = writeNURBSNativeTest(ctx, &again, n); e != nil || !bytes.Equal(raw, again.Bytes()) {
				t.Fatal("noncanonical bytes", e)
			}
			if a.Format != nurbsPlanarAssemblyNativeFormat || a.Mode != mode || a.Version != 1 || len(a.Geometry.Occurrences) != 3 {
				t.Fatal("wrong format or occurrence ancestry")
			}
			for _, other := range nurbsNativeModes {
				if other == mode {
					continue
				}
				if _, e = readNURBSNativeTest(t, ctx, bytes.NewReader(raw), other, IndependentMaterialJSONLimits{}); e == nil {
					t.Fatal("cross mode accepted", other)
				}
			}
		})
	}
}
func TestNURBSAssemblyNativeTamper(t *testing.T) {
	for _, mode := range nurbsNativeModes {
		t.Run(mode, func(t *testing.T) {
			_, raw := nurbsNativeTestBytes(t, mode)
			mutations := map[string]func(*nurbsAssemblyEnvelope){
				"format":      func(q *nurbsAssemblyEnvelope) { q.Format = occurrenceColorsNativeFormat },
				"version":     func(q *nurbsAssemblyEnvelope) { q.Version++ },
				"mode":        func(q *nurbsAssemblyEnvelope) { q.Mode = "general-nurbs" },
				"unit":        func(q *nurbsAssemblyEnvelope) { q.Geometry.LengthUnit *= 1000 },
				"body name":   func(q *nurbsAssemblyEnvelope) { nurbsNativeBody(q).Name += "forged" },
				"source face": func(q *nurbsAssemblyEnvelope) { nurbsNativeBody(q).SourceFaceIDs[0]++ },
				"source body": func(q *nurbsAssemblyEnvelope) { nurbsNativeBody(q).SourceBodyID++ },
				"vertex":      func(q *nurbsAssemblyEnvelope) { nurbsNativeBody(q).Vertices[0].X++ },
				"edge":        func(q *nurbsAssemblyEnvelope) { nurbsNativeBody(q).Edges[0].End++ },
				"face use": func(q *nurbsAssemblyEnvelope) {
					b := nurbsNativeBody(q)
					b.Faces[0].Boundary[0].Reversed = !b.Faces[0].Boundary[0].Reversed
				},
				"surface color": func(q *nurbsAssemblyEnvelope) { nurbsNativeBody(q).SurfaceColor = &RGB{.25, .5, .75} },
				"face color":    func(q *nurbsAssemblyEnvelope) { nurbsNativeBody(q).FaceColors = map[int]RGB{0: {.25, .5, .75}} },
				"definition":    func(q *nurbsAssemblyEnvelope) { q.Geometry.Definitions[0].Name += "forged" },
				"instance id":   func(q *nurbsAssemblyEnvelope) { q.Geometry.Occurrences[0].ID++ },
				"child":         func(q *nurbsAssemblyEnvelope) { q.Geometry.Occurrences[0].Child++ },
				"placement":     func(q *nurbsAssemblyEnvelope) { q.Geometry.Occurrences[0].Transform[0][3]++ },
				"proof source":  func(q *nurbsAssemblyEnvelope) { q.Geometry.Source += "\nforged" },
				"valid different source appearance": func(q *nurbsAssemblyEnvelope) {
					before := q.OriginalSource
					q.OriginalSource = strings.Replace(before, "DRAUGHTING_PRE_DEFINED_COLOUR('red')", "DRAUGHTING_PRE_DEFINED_COLOUR('green')", 1)
					if before == q.OriginalSource {
						t.Fatal("stale source mutation")
					}
				},
				"invalid original surface": func(q *nurbsAssemblyEnvelope) {
					f, e := Parse(strings.NewReader(q.OriginalSource), Limits{})
					if e != nil {
						t.Fatal(e)
					}
					var pid int64
					switch mode {
					case nurbsNativeUnit:
						pid = q.Unit.Surfaces[0].PoleIDs[3]
					case nurbsNativeScaled:
						pid = q.Scaled.Charts[0].Source.PoleIDs[3]
					case nurbsNativeOblique:
						pid = q.Oblique.Frames[0].PoleIDs[3]
					}
					f.Entities[pid].Records[0].Args[1].Items[0].Text = "999."
					b, e := mappedPlanarSourceBytes(f)
					if e != nil {
						t.Fatal(e)
					}
					q.OriginalSource = string(b)
				},
			}
			switch mode {
			case nurbsNativeUnit:
				mutations["surface chart"] = func(q *nurbsAssemblyEnvelope) { q.Unit.Surfaces[0].Origin[0] = "999" }
				mutations["curve interval"] = func(q *nurbsAssemblyEnvelope) { q.Unit.Curves[0].Knots[0] = "1" }
				mutations["owner"] = func(q *nurbsAssemblyEnvelope) { q.Unit.PCurves[0].PlaneID++ }
				mutations["extraneous mode"] = func(q *nurbsAssemblyEnvelope) { q.Scaled = &nurbsScaledNativeClaims{} }
			case nurbsNativeScaled:
				mutations["normalized source"] = func(q *nurbsAssemblyEnvelope) { q.Scaled.NormalizedSource += "forged" }
				mutations["rational factor"] = func(q *nurbsAssemblyEnvelope) { q.Scaled.Charts[0].Scale[0] = "1/0" }
				mutations["original UV"] = func(q *nurbsAssemblyEnvelope) { q.Scaled.PCurves[0].SourcePoles[0][0] = "999" }
				mutations["normalized equation"] = func(q *nurbsAssemblyEnvelope) { q.Scaled.NormalizedPCurves[0].Derivative[0] = "999" }
				mutations["extraneous mode"] = func(q *nurbsAssemblyEnvelope) { q.Oblique = &nurbsObliqueNativeClaims{} }
			case nurbsNativeOblique:
				mutations["rational basis"] = func(q *nurbsAssemblyEnvelope) { q.Oblique.Frames[0].Basis[0][0] = "999/1" }
				mutations["frame owner"] = func(q *nurbsAssemblyEnvelope) { q.Oblique.Frames[0].DefinitionID++ }
				mutations["source point"] = func(q *nurbsAssemblyEnvelope) { q.Oblique.Points[0].Source[0] = "999" }
				mutations["proof point"] = func(q *nurbsAssemblyEnvelope) { q.Oblique.Points[0].Proof[0] = "999" }
				mutations["extraneous mode"] = func(q *nurbsAssemblyEnvelope) { q.Unit = &nurbsUnitNativeClaims{} }
			}
			for name, mutate := range mutations {
				t.Run(name, func(t *testing.T) {
					var q nurbsAssemblyEnvelope
					if e := json.Unmarshal(raw, &q); e != nil {
						t.Fatal(e)
					}
					mutate(&q)
					bad, e := json.Marshal(q)
					if e != nil {
						t.Fatal(e)
					}
					if _, e = readNURBSNativeTest(t, context.Background(), bytes.NewReader(bad), mode, IndependentMaterialJSONLimits{}); e == nil {
						t.Fatal("forged claims accepted")
					}
				})
			}
			for _, bad := range [][]byte{append(append([]byte(nil), raw...), []byte("{}")...), bytes.Replace(raw, []byte(`"Version":1`), []byte(`"Version":1,"Unknown":true`), 1), bytes.Replace(raw, []byte(`"Geometry":{`), []byte(`"Geometry":{"Unknown":true,`), 1)} {
				if bytes.Equal(bad, raw) {
					t.Fatal("stale invalid JSON mutation")
				}
				if _, e := readNURBSNativeTest(t, context.Background(), bytes.NewReader(bad), mode, IndependentMaterialJSONLimits{}); e == nil {
					t.Fatal("invalid outer JSON accepted")
				}
			}
		})
	}
}
func nurbsNativeGeometry(m nurbsNativeModel) *ColoredAssembly {
	if m.unit != nil {
		return &m.unit.Geometry
	}
	if m.scaled != nil {
		return &m.scaled.Geometry
	}
	return &m.oblique.Geometry
}
func TestNURBSAssemblyNativeLimitsAtomicity(t *testing.T) {
	for _, mode := range nurbsNativeModes {
		t.Run(mode, func(t *testing.T) {
			m, raw := nurbsNativeTestBytes(t, mode)
			ctx := context.Background()
			for _, l := range []IndependentMaterialJSONLimits{{MaxBytes: int64(len(raw) - 1)}, {MaxBytes: -1}, {MaxBytes: 9 << 20}, {Source: Limits{MaxEntities: 600}}, {Source: Limits{MaxBytes: 64}}, {Source: Limits{MaxSolids: 1}}} {
				if _, e := readNURBSNativeTest(t, ctx, bytes.NewReader(raw), mode, l); e == nil {
					t.Fatal("ignored limit", l)
				}
			}
			canceled, cancel := context.WithCancel(ctx)
			cancel()
			var out bytes.Buffer
			if _, e := readNURBSNativeTest(t, canceled, bytes.NewReader(raw), mode, IndependentMaterialJSONLimits{}); !errors.Is(e, context.Canceled) {
				t.Fatal(e)
			}
			if e := writeNURBSNativeTest(canceled, &out, m); !errors.Is(e, context.Canceled) || out.Len() != 0 {
				t.Fatal("canceled write", e)
			}
			during, c := context.WithCancel(ctx)
			if _, e := readNURBSNativeTest(t, during, &occurrenceCancelReader{Reader: bytes.NewReader(raw), cancel: c}, mode, IndependentMaterialJSONLimits{}); !errors.Is(e, context.Canceled) {
				t.Fatal("during-read cancellation", e)
			}
			for _, mutate := range []func(nurbsNativeModel){func(m nurbsNativeModel) { nurbsNativeGeometry(m).Assembly.Occurrences[0].Transform[0][3]++ }, func(m nurbsNativeModel) {
				g := nurbsNativeGeometry(m)
				for id, bs := range g.ProductBodies {
					if len(bs) == 0 {
						continue
					}
					v := bs[0].Body.Solid.Vertices()
					for i := range v {
						v[i].X++
					}
					s, e := brep.NewSolid(v, bs[0].Body.Solid.Edges(), bs[0].Body.Solid.Faces())
					if e != nil {
						t.Fatal(e)
					}
					bs[0].Body.Solid = s
					g.ProductBodies[id] = bs
					return
				}
			}, func(m nurbsNativeModel) {
				if m.unit != nil {
					m.unit.surfaces[0].U[0] = "forged"
				}
				if m.scaled != nil {
					m.scaled.charts[0].Scale[0] = "forged"
				}
				if m.oblique != nil {
					m.oblique.frames[0].Origin[0] = "forged"
				}
			}} {
				q, e := readNURBSNativeTest(t, ctx, bytes.NewReader(raw), mode, IndependentMaterialJSONLimits{})
				if e != nil {
					t.Fatal(e)
				}
				mutate(q)
				out.Reset()
				if e = writeNURBSNativeTest(ctx, &out, q); e == nil || out.Len() != 0 {
					t.Fatal("forged writer input emitted", e)
				}
			}
			if e := writeNURBSNativeTest(ctx, occurrenceShortWriter{}, m); !errors.Is(e, io.ErrShortWrite) {
				t.Fatal(e)
			}
			if _, e := ReadOccurrenceColoredAssemblyJSON(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{}); e == nil {
				t.Fatal("legacy contextual format widened")
			}
			if _, e := ReadIndependentFrustumHierarchyJSON(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{}); e == nil {
				t.Fatal("legacy hierarchy format widened")
			}
		})
	}
	_, old := occurrenceColorsNativeFixture(t)
	for _, mode := range nurbsNativeModes {
		if _, e := readNURBSNativeTest(t, context.Background(), bytes.NewReader(old), mode, IndependentMaterialJSONLimits{}); e == nil {
			t.Fatal("old native format accepted", mode)
		}
	}
}
func TestNURBSAssemblyNativeLive(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_AP242_NURBS_NATIVE")
	if bin == "" {
		t.Skip("set PLAINSOLID_OCCT_AP242_NURBS_NATIVE to the independent oblique observer")
	}
	wants := map[string][5]float64{nurbsNativeUnit: {54, 126, 85.0 / 18, 86.0 / 9, 2.5}, nurbsNativeScaled: {136, 252, 6.5, 180.0 / 17, 57.0 / 17}, nurbsNativeOblique: {1750, 1350, 69.0 / 14, 66.0 / 7, 99.0 / 14}}
	for _, mode := range nurbsNativeModes {
		t.Run(mode, func(t *testing.T) {
			_, raw := nurbsNativeTestBytes(t, mode)
			ctx := context.Background()
			m, e := readNURBSNativeTest(t, ctx, bytes.NewReader(raw), mode, IndependentMaterialJSONLimits{})
			if e != nil {
				t.Fatal(e)
			}
			var source bytes.Buffer
			switch mode {
			case nurbsNativeUnit:
				e = WriteNURBSPlanarAssemblySource(ctx, &source, *m.unit, Limits{})
			case nurbsNativeScaled:
				e = WriteScaledNURBSPlanarAssemblySource(ctx, &source, *m.scaled, Limits{})
			case nurbsNativeOblique:
				e = WriteObliqueNURBSPlanarAssemblySource(ctx, &source, *m.oblique, Limits{})
			}
			if e != nil {
				t.Fatal(e)
			}
			claims, e := nurbsAssemblyNativeClaims(ctx, m)
			if e != nil || source.String() != claims.OriginalSource {
				t.Fatal("source loss", e)
			}
			path := filepath.Join(t.TempDir(), "native-reexport.step")
			if e = os.WriteFile(path, source.Bytes(), 0600); e != nil {
				t.Fatal(e)
			}
			c, cancel := context.WithTimeout(ctx, 30*time.Second)
			out, e := exec.CommandContext(c, bin, "read", path).CombinedOutput()
			cancel()
			if e != nil {
				t.Fatalf("%v: %s", e, out)
			}
			var q struct {
				Volume, Area  float64
				Centroid      [3]float64
				Solids, Faces int
				Colors        [][3]float64
			}
			at := bytes.IndexByte(out, '{')
			if at < 0 || json.Unmarshal(out[at:], &q) != nil {
				t.Fatal(string(out))
			}
			colors := map[[3]float64]int{}
			for _, c := range q.Colors {
				colors[c]++
			}
			if colors[[3]float64{1, 0, 0}] != 1 || colors[[3]float64{0, 0, 1}] != 1 {
				t.Fatal("product color loss", q)
			}
			want := wants[mode]
			if math.Abs(q.Volume-want[0]) > 1e-9 || math.Abs(q.Area-want[1]) > 1e-9 || math.Abs(q.Centroid[0]-want[2]) > 1e-9 || math.Abs(q.Centroid[1]-want[3]) > 1e-9 || math.Abs(q.Centroid[2]-want[4]) > 1e-9 || q.Solids != 3 || q.Faces != 18 || len(q.Colors) != 2 {
				t.Fatal(q)
			}
		})
	}
}
func FuzzNURBSAssemblyNative(f *testing.F) {
	for i, mode := range nurbsNativeModes {
		_, b := nurbsNativeTestBytes(f, mode)
		f.Add(uint8(i), b)
	}
	f.Fuzz(func(t *testing.T, choice uint8, b []byte) {
		if len(b) > 1<<20 {
			t.Skip()
		}
		mode := nurbsNativeModes[int(choice)%3]
		m, e := readNURBSNativeTest(t, context.Background(), bytes.NewReader(b), mode, IndependentMaterialJSONLimits{MaxBytes: 1 << 20, Source: Limits{MaxEntities: 1024}})
		if e != nil {
			return
		}
		var out bytes.Buffer
		if e = writeNURBSNativeTest(context.Background(), &out, m); e != nil {
			t.Fatal(e)
		}
		if _, e = readNURBSNativeTest(t, context.Background(), bytes.NewReader(out.Bytes()), mode, IndependentMaterialJSONLimits{}); e != nil {
			t.Fatal(e)
		}
	})
}
