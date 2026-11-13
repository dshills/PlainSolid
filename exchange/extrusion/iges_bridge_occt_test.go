package extrusion

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/exchange/iges"
	"plainsolid/exchange/step"
	"testing"
	"time"
)

func decodeMMFixture(t *testing.T, b []byte) step.RationalLinearExtrusionModel {
	t.Helper()
	f, e := step.Parse(bytes.NewReader(b), step.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	lengthID := int64(0)
	for id, entity := range f.Entities {
		length := false
		var si *step.Record
		for _, r := range entity.Records {
			if r.Type == "LENGTH_UNIT" {
				length = true
			}
			if r.Type == "SI_UNIT" {
				copy := r
				si = &copy
			}
		}
		if length {
			if lengthID != 0 || si == nil || len(si.Args) != 2 || si.Args[0].Text != "MILLI" || si.Args[1].Text != "METRE" {
				t.Fatal("fixture length unit is not unambiguous mm")
			}
			lengthID = id
		}
	}
	contexts := 0
	for _, entity := range f.Entities {
		for _, r := range entity.Records {
			if r.Type != "GLOBAL_UNIT_ASSIGNED_CONTEXT" {
				continue
			}
			if len(r.Args) != 1 {
				t.Fatal("context")
			}
			found := false
			for _, v := range r.Args[0].Items {
				found = found || v.Kind == step.Reference && v.Ref == lengthID
			}
			if !found || lengthID == 0 {
				t.Fatal("fixture context unit mismatch")
			}
			contexts++
		}
	}
	if contexts == 0 {
		t.Fatal("missing unit context")
	}
	m := step.RationalLinearExtrusionModel{LengthUnit: .001}
	for _, id := range f.Order {
		for _, r := range f.Entities[id].Records {
			if r.Type == "RECTANGULAR_TRIMMED_SURFACE" {
				s, e := step.DecodeRationalLinearExtrusionSurface(f, id, 64)
				if e != nil {
					t.Fatal(e)
				}
				m.Surfaces = append(m.Surfaces, s)
			}
		}
	}
	if len(m.Surfaces) != 1 {
		t.Fatal(len(m.Surfaces))
	}
	return m
}
func TestExtrusionBridgeRetainedSTEP(t *testing.T) {
	b, e := os.ReadFile("testdata/tabulated-compatible.step")
	if e != nil {
		t.Fatal(e)
	}
	m := decodeMMFixture(t, b)
	out, e := ToIGES(m)
	if e != nil {
		t.Fatal(e)
	}
	back, e := FromIGES(out)
	if e != nil {
		t.Fatal(e)
	}
	if back.LengthUnit != m.LengthUnit || back.Surfaces[0].VEnd != 5 {
		t.Fatal("units/domain changed")
	}
	a, _ := m.Surfaces[0].Surface()
	bSurface, _ := back.Surfaces[0].Surface()
	p, _ := a.Eval(.5, 1.25)
	q, _ := bSurface.Eval(.5, 1.25)
	if p != q {
		t.Fatal(p, q)
	}
}
func TestOCCTExtrusionBridge(t *testing.T) {
	ig := os.Getenv("PLAINSOLID_OCCT_IGES_TABULATED")
	st := os.Getenv("PLAINSOLID_OCCT_EXTRUSION_BRIDGE")
	if ig == "" || st == "" {
		t.Skip("set both standalone extrusion helpers")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for _, mode := range []string{"write", "write-negative"} {
		dir := t.TempDir()
		sourceIG := filepath.Join(dir, "source.igs")
		if b, e := exec.CommandContext(ctx, ig, mode, sourceIG).CombinedOutput(); e != nil {
			t.Fatal(e, string(b))
		}
		raw, e := os.ReadFile(sourceIG)
		if e != nil {
			t.Fatal(e)
		}
		im, e := iges.ReadTabulatedSurfaces(bytes.NewReader(raw), iges.Limits{})
		if e != nil {
			t.Fatal(e)
		}
		if im.LengthUnit != .001 {
			t.Fatal("fixture must explicitly use mm")
		}
		m, e := FromIGES(im)
		if e != nil {
			t.Fatal(e)
		}
		var b bytes.Buffer
		if e = step.WriteRationalLinearExtrusions(&b, m); e != nil {
			t.Fatal(e)
		}
		targetST := filepath.Join(dir, "native.step")
		if e = os.WriteFile(targetST, b.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
		sourceST := filepath.Join(dir, "source.step")
		if data, e := exec.CommandContext(ctx, st, mode, sourceST).CombinedOutput(); e != nil {
			t.Fatal(e, string(data))
		}
		raw, e = os.ReadFile(sourceST)
		if e != nil {
			t.Fatal(e)
		}
		m = decodeMMFixture(t, raw)
		im, e = ToIGES(m)
		if e != nil {
			t.Fatal(e)
		}
		targetIG := filepath.Join(dir, "native.igs")
		b.Reset()
		if e = iges.WriteTabulatedSurfaces(&b, im); e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(targetIG, b.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
		type measure struct {
			Faces         int
			Area          float64
			Point, Du, Dv [3]float64
		}
		var first *measure
		for _, tc := range []struct {
			binary, path string
			scale        float64
		}{{ig, sourceIG, 1}, {st, targetST, 5}, {st, sourceST, 5}, {ig, targetIG, 1}} {
			raw, e = exec.CommandContext(ctx, tc.binary, "read", tc.path).Output()
			if e != nil {
				t.Fatal(e, string(raw))
			}
			start := bytes.LastIndexByte(raw, '{')
			if start < 0 {
				t.Fatal(string(raw))
			}
			var v measure
			if e = json.Unmarshal(bytes.TrimSpace(raw[start:]), &v); e != nil {
				t.Fatal(e)
			}
			for i := range v.Dv {
				v.Dv[i] *= tc.scale
			}
			if v.Faces != 1 {
				t.Fatal(v)
			}
			if first == nil {
				first = &v
				continue
			}
			if math.Abs(first.Area-v.Area) > 1e-9 {
				t.Fatal(first, v)
			}
			for i := 0; i < 3; i++ {
				for _, q := range [][2]float64{{first.Point[i], v.Point[i]}, {first.Du[i], v.Du[i]}, {first.Dv[i], v.Dv[i]}} {
					if math.Abs(q[0]-q[1]) > 1e-10 {
						t.Fatal("parameter-domain correspondence", q)
					}
				}
			}
		}
	}
}
