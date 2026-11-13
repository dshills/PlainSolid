package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"plainsolid/geom"
	"testing"
)

func TestWrappedRimFilletSTEP(t *testing.T) {
	for _, r := range [][2]float64{{.5, 1}, {0, 1}, {.5, 0}} {
		t.Run(real(r[0])+"/"+real(r[1]), func(t *testing.T) {
			m := WrappedRimFilletBody{Name: "Fillet", Center: geom.Vec3{X: 2, Y: 4, Z: 8}, Radius: 3, Height: 5, Fillets: r, LengthUnit: .001, CapNames: [2]string{"Lower", "Upper"}}
			var b bytes.Buffer
			if e := WriteWrappedRimFillet(&b, m); e != nil {
				t.Fatal(e)
			}
			got, e := ReadWrappedRimFillet(bytes.NewReader(b.Bytes()), Limits{}, WrappedSurfaceOptions{})
			if e != nil {
				t.Fatal(e)
			}
			if got.Center != m.Center || got.Radius != m.Radius || got.Height != m.Height || got.Fillets != r || got.CapNames != m.CapNames {
				t.Fatal(got)
			}
			if _, e = ReadWrappedStationLoft(bytes.NewReader(b.Bytes()), Limits{}, WrappedSurfaceOptions{}); e == nil {
				t.Fatal("loft accepted curved meridian")
			}
			got.Fillets[0] += .125
			var bad bytes.Buffer
			if e = WriteWrappedRimFillet(&bad, got); e == nil || bad.Len() != 0 {
				t.Fatal("mismatched retained support accepted")
			}
		})
	}
}

func TestOCCTWrappedRimFilletSTEP(t *testing.T) {
	source := t.TempDir() + "/source.step"
	stepOracle(t, source, "write-step-wrapped-rim-fillet")
	raw, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadWrappedRimFillet(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if m.Radius != 3 || m.Height != 5 || m.Fillets != ([2]float64{.5, 1}) {
		t.Fatal(m)
	}
	var b bytes.Buffer
	if e = WriteWrappedRimFillet(&b, m); e != nil {
		t.Fatal(e)
	}
	native := t.TempDir() + "/native.step"
	if e = os.WriteFile(native, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	volume, area := 9*3.5*math.Pi, 2*math.Pi*3*3.5
	for _, r := range []float64{.5, 1} {
		major := 3 - r
		volume += math.Pi * (major*major*r + major*r*r*math.Pi/2 + 2*r*r*r/3)
		area += math.Pi*major*major + 2*math.Pi*r*(major*math.Pi/2+r)
	}
	for _, path := range []string{source, native} {
		out := t.TempDir() + "/mass.json"
		stepOracle(t, out, "step-mass-precise", path)
		raw, e := os.ReadFile(out)
		if e != nil {
			t.Fatal(e)
		}
		var p struct{ Area, Volume float64 }
		if e = json.Unmarshal(raw, &p); e != nil {
			t.Fatal(e)
		}
		if math.Abs(p.Volume-volume) > 1e-7 || math.Abs(p.Area-area) > 1e-7 {
			t.Fatal(p, area, volume)
		}
	}
	recovered, e := WrappedRimFilletFromSolid(context.Background(), m.Solid, "Native", m.LengthUnit, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	b.Reset()
	if e = WriteWrappedRimFillet(&b, recovered); e != nil {
		t.Fatal(e)
	}
}

func TestIndependentWrappedRimFillet(t *testing.T) {
	raw, e := os.ReadFile("testdata/wrapped-rim-fillet.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadWrappedRimFillet(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Walls) != 3 || m.Walls[0].SideEdges[2] != m.Walls[1].SideEdges[0] || m.Walls[1].SideEdges[2] != m.Walls[2].SideEdges[0] {
		t.Fatal("rim ancestry")
	}
	for _, kind := range []string{"interior", "owner", "unshared"} {
		t.Run(kind, func(t *testing.T) {
			f, e := Parse(bytes.NewReader(raw), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			changed := false
			if kind == "interior" {
				for _, id := range f.Order {
					r, ok := component(f.Entities[id], "B_SPLINE_SURFACE")
					if !ok || len(r.Args) < 3 || len(r.Args[2].Items) < 2 || len(r.Args[2].Items[1].Items) != 3 {
						continue
					}
					pid := r.Args[2].Items[1].Items[1].Ref
					p, e := ovalRecord(f, pid, "CARTESIAN_POINT", 2)
					if e != nil {
						t.Fatal(e)
					}
					p.Args[1].Items = append([]Value(nil), p.Args[1].Items...)
					v, e := numeric(p.Args[1].Items[2])
					if e != nil {
						t.Fatal(e)
					}
					p.Args[1].Items[2] = Value{Kind: Number, Text: real(v + .0625)}
					ovalSet(f, pid, p)
					changed = true
					break
				}
			}
			if kind == "owner" {
				for _, id := range f.Order {
					r, e := ovalRecord(f, id, "SURFACE_CURVE", 4)
					if e == nil && len(r.Args[2].Items) == 2 {
						r.Args[2] = Value{Kind: Aggregate, Items: []Value{r.Args[2].Items[0], r.Args[2].Items[0]}}
						ovalSet(f, id, r)
						changed = true
						break
					}
				}
			}
			if kind == "unshared" {
				target := m.Walls[0].SideEdges[2]
				clone := f.Entities[target]
				clone.ID = 999999
				f.Entities[clone.ID] = clone
				f.Order = append(f.Order, clone.ID)
				for _, id := range f.Order {
					r, e := ovalRecord(f, id, "ORIENTED_EDGE", 5)
					if e == nil && r.Args[3].Ref == target {
						r.Args[3] = Value{Kind: Reference, Ref: clone.ID}
						ovalSet(f, id, r)
						changed = true
						break
					}
				}
			}
			if !changed {
				t.Fatal("mutation missing")
			}
			if _, e = ReadWrappedRimFillet(bytes.NewReader(serializeWrappedProofFile(f)), Limits{}, WrappedSurfaceOptions{}); e == nil {
				t.Fatal("accepted", kind)
			}
		})
	}
}

func FuzzWrappedRimFillet(f *testing.F) {
	if raw, e := os.ReadFile("testdata/wrapped-rim-fillet.step"); e == nil {
		f.Add(raw)
	}
	f.Fuzz(func(t *testing.T, raw []byte) {
		_, _ = ReadWrappedRimFillet(bytes.NewReader(raw), Limits{MaxBytes: 65536, MaxEntities: 500, MaxVertices: 500, MaxFaces: 6, MaxEdgeUses: 32, MaxDepth: 24}, WrappedSurfaceOptions{MaxSurfaces: 3, MaxPoles: 128, MaxOperations: 1000000, MaxCoefficientBits: 4096})
	})
}
