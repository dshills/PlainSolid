package step

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"strings"
	"testing"
)

func TestEllipticalPrismMaterial(t *testing.T) {
	shell, e := RepairRoundedEllipticalPrismShell(bytes.NewReader(ellipticalPrismFixture(t)), Limits{}, EllipticalWireRepairOptions{MaxDistance: 1e-11})
	if e != nil {
		t.Fatal(e)
	}
	m, e := PromoteRepairedEllipticalPrism(context.Background(), shell.Source, Limits{}, EllipticalWireRepairOptions{MaxDistance: 1e-11}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Solid.Faces()) != 6 || m.NativeFaceSourceIDs != ([6]int64{108, 112, 17, 17, 17, 17}) || m.SourceRimEdgeIDs != ([2]int64{82, 54}) {
		t.Fatal(m.NativeFaceSourceIDs)
	}
	var b bytes.Buffer
	if e = trimmed.WriteJSON(&b, m.Solid); e != nil {
		t.Fatal(e)
	}
	loaded, e := trimmed.ReadJSON(&b, trimmed.JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	for _, s := range []trimmed.Solid{m.Solid, loaded} {
		proof, e := s.ValidateEdgeAgreement(context.Background(), edgecheck.Options{})
		for _, u := range proof.Uses {
			if u.Report.SquaredUpperBound == nil || u.Report.SquaredUpperBound.Sign() != 0 {
				t.Fatal("nonzero continuous discrepancy", u)
			}
		}
		if e != nil || !proof.Complete {
			t.Fatal(proof, e)
		}
		p, e := s.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-8})
		if e != nil {
			t.Fatal(e)
		}
		if math.Abs(p.Volume-24*math.Pi) > 1e-6 || p.Centroid.Distance(geom.Vec3{X: 1, Y: 3.5, Z: 3}) > 1e-7 {
			t.Fatal(p)
		}
	}
	mesh, e := loaded.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .02, MaxSubdivisions: 64, MaxTriangles: 100000})
	if e != nil {
		t.Fatal(e)
	}
	if e = mesh.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
	var native bytes.Buffer
	if e = WriteEllipticalPrismMaterialJSON(context.Background(), &native, m); e != nil {
		t.Fatal(e)
	}
	raw := append([]byte(nil), native.Bytes()...)
	r, e := ReadEllipticalPrismMaterialJSON(context.Background(), &native, Limits{})
	if e != nil || r.NativeRimEdges != m.NativeRimEdges {
		t.Fatal(e)
	}
	var twice bytes.Buffer
	if e = WriteEllipticalPrismMaterialJSON(context.Background(), &twice, r); e != nil || !bytes.Equal(raw, twice.Bytes()) {
		t.Fatal("unstable recipe", e)
	}
	for _, key := range []string{"Native", "Faces", "RimEdges", "SourceRims", "RepairTolerance", "NativeTolerance", "version"} {
		var q map[string]any
		if json.Unmarshal(raw, &q) != nil {
			t.Fatal("json")
		}
		q[key] = 0
		b, _ := json.Marshal(q)
		out, e := ReadEllipticalPrismMaterialJSON(context.Background(), bytes.NewReader(b), Limits{})
		if e == nil || out.Shell.Source != nil {
			t.Fatal("accepted tamper", key)
		}
	}
	m.NativeRimEdges[0][0]++
	var bad bytes.Buffer
	if WriteEllipticalPrismMaterialJSON(context.Background(), &bad, m) == nil || bad.Len() != 0 {
		t.Fatal("stale ancestry")
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = PromoteRepairedEllipticalPrism(canceled, shell.Source, Limits{}, EllipticalWireRepairOptions{MaxDistance: 1e-11}, 1e-9); e == nil {
		t.Fatal("cancellation")
	}
}

func TestEllipticalPrismMaterialUnrepresentable(t *testing.T) {
	raw := string(ellipticalPrismFixture(t))
	raw = strings.ReplaceAll(raw, "(0.,0.,-1.)", "(0.,0.,1.)")
	raw = strings.ReplaceAll(raw, "(1.,2.,3.)", "(1.,2.,-36028797018963968.)")
	raw = strings.ReplaceAll(raw, "(1.,5.,3.)", "(1.,5.,-36028797018963968.)")
	raw = strings.ReplaceAll(raw, "(1.,2.,-1.)", "(1.,2.,-36028797018963964.)")
	raw = strings.ReplaceAll(raw, "(1.,5.,-1.)", "(1.,5.,-36028797018963964.)")
	o := EllipticalWireRepairOptions{MaxDistance: 1e-11}
	shell, e := RepairRoundedEllipticalPrismShell(strings.NewReader(raw), Limits{}, o)
	if e != nil {
		t.Fatal("source graph is exactly represented", e)
	}
	q, e := PromoteRepairedEllipticalPrism(context.Background(), shell.Source, Limits{}, o, 1e-9)
	if !errors.Is(e, geom.ErrUnrepresentable) || q.Shell.Source != nil {
		t.Fatal("rounded hidden negative pole", e)
	}
}
func FuzzEllipticalPrismMaterialNative(f *testing.F) {
	o := EllipticalWireRepairOptions{MaxDistance: 1e-11}
	shell, e := RepairRoundedEllipticalPrismShell(bytes.NewReader(ellipticalPrismFixture(f)), Limits{}, o)
	if e != nil {
		f.Fatal(e)
	}
	m, e := PromoteRepairedEllipticalPrism(context.Background(), shell.Source, Limits{}, o, 1e-9)
	if e != nil {
		f.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteEllipticalPrismMaterialJSON(context.Background(), &b, m); e != nil {
		f.Fatal(e)
	}
	f.Add(b.Bytes())
	f.Add([]byte(`{}`))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 512<<10 {
			return
		}
		_, _ = ReadEllipticalPrismMaterialJSON(context.Background(), bytes.NewReader(b), Limits{MaxBytes: 65536, MaxEntities: 121, MaxValues: 4096, MaxDepth: 16})
	})
}
