package iges

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"testing"
)

func TestConical45FrustumMaterial(t *testing.T) {
	ctx := context.Background()
	sources := [][]byte{conical45FrustumSource(t)}
	var authored bytes.Buffer
	if e := WriteConical45FrustumSolid(ctx, &authored, Conical45FrustumDescriptor{Origin: geom.Vec3{X: .25, Y: -2, Z: 3}, Radius: 2, Height: 1.5}, .001); e != nil {
		t.Fatal(e)
	}
	sources = append(sources, authored.Bytes())
	for _, raw := range sources {
		m, e := ReadConical45FrustumMaterial(ctx, bytes.NewReader(raw), Limits{}, 1e-9)
		if e != nil {
			t.Fatal(e)
		}
		if len(m.Solid.Faces()) != 6 || m.SourceRimCurveIDs != [2]int{19, 17} {
			t.Fatal("source partition")
		}
		var native bytes.Buffer
		if e = WriteConical45FrustumMaterialJSON(ctx, &native, m); e != nil {
			t.Fatal(e)
		}
		p, e := ReadConical45FrustumMaterialJSON(ctx, bytes.NewReader(native.Bytes()), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		if !bytes.Equal(p.source, raw) {
			t.Fatal("source lost")
		}
		var retained bytes.Buffer
		if e = WriteConical45FrustumMaterialSource(ctx, &retained, p); e != nil || !bytes.Equal(retained.Bytes(), raw) {
			t.Fatal("source reproof", e)
		}
		var plain bytes.Buffer
		if e = trimmed.WriteJSON(&plain, p.Solid); e != nil {
			t.Fatal(e)
		}
		s, e := trimmed.ReadJSON(&plain, trimmed.JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		for _, solid := range []trimmed.Solid{m.Solid, s} {
			proof, e := solid.ValidateEdgeAgreement(ctx, edgecheck.Options{})
			if e != nil || !proof.Complete {
				t.Fatal("edge proof", e)
			}
			for _, u := range proof.Uses {
				if u.Report.SquaredUpperBound == nil || u.Report.SquaredUpperBound.Sign() != 0 {
					t.Fatal("nonexact seam")
				}
			}
		}
		d := m.Descriptor
		r, h := d.Radius, d.Height
		rt := r + h
		sum := r*r + r*rt + rt*rt
		volume := math.Pi * h * sum / 3
		area := math.Pi * (r*r + rt*rt + (r+rt)*h*math.Sqrt2)
		center := d.Origin.Add(geom.Vec3{Z: h * (r*r + 2*r*rt + 3*rt*rt) / (4 * sum)})
		mass, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
		if e != nil || math.Abs(mass.Volume-volume) > 1e-7 || math.Abs(mass.Area-area) > 1e-7 || mass.Centroid.Distance(center) > 1e-8 {
			t.Fatal(mass, e)
		}
		mesh, e := s.Tessellate(ctx, parametric.TessellationOptions{Deflection: .1, MaxSubdivisions: 128, MaxTriangles: 100000})
		if e != nil {
			t.Fatal(e)
		}
		if e = mesh.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		var q conical45MaterialEnvelope
		if e = json.Unmarshal(native.Bytes(), &q); e != nil {
			t.Fatal(e)
		}
		q.Native = json.RawMessage(`{}`)
		bad, e := json.Marshal(q)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = ReadConical45FrustumMaterialJSON(ctx, bytes.NewReader(bad), Limits{}); e == nil {
			t.Fatal("forged native geometry")
		}
		if e = json.Unmarshal(native.Bytes(), &q); e != nil {
			t.Fatal(e)
		}
		q.Descriptor.Radius++
		bad, _ = json.Marshal(q)
		if _, e = ReadConical45FrustumMaterialJSON(ctx, bytes.NewReader(bad), Limits{}); e == nil {
			t.Fatal("forged descriptor")
		}
		if oracle := os.Getenv("PLAINSOLID_OCCT_IGES_SOLID"); oracle != "" {
			var output bytes.Buffer
			if e = WriteConical45FrustumSolid(ctx, &output, p.Descriptor, p.LengthUnit); e != nil {
				t.Fatal(e)
			}
			for _, data := range [][]byte{raw, output.Bytes()} {
				path := filepath.Join(t.TempDir(), "frustum.igs")
				if e = os.WriteFile(path, data, 0600); e != nil {
					t.Fatal(e)
				}
				b, e := exec.Command(oracle, path).CombinedOutput()
				if e != nil {
					t.Fatalf("%s %v", b, e)
				}
				var q struct {
					Faces, Solids int
					Area, Volume  float64
					Center        [3]float64
				}
				at := bytes.LastIndex(b, []byte("{"))
				if at < 0 || json.Unmarshal(b[at:], &q) != nil || q.Faces != 3 || q.Solids != 1 || math.Abs(q.Area-area) > 1e-6 || math.Abs(q.Volume-volume) > 1e-7 || (geom.Vec3{X: q.Center[0], Y: q.Center[1], Z: q.Center[2]}).Distance(center) > 1e-8 {
					t.Fatalf("%s", b)
				}
			}
		}
	}
	vendor, e := os.ReadFile("testdata/occt-rounded-conical45-frustum.igs")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ReadConical45FrustumMaterial(ctx, bytes.NewReader(vendor), Limits{}, 1e-9); e == nil {
		t.Fatal("rounded vendor source promoted")
	}
	for _, l := range []Limits{{MaxEntities: 19}, {MaxRecords: 2}, {MaxBytes: 80}, {MaxValues: 8}} {
		if _, e = ReadConical45FrustumMaterial(ctx, bytes.NewReader(sources[0]), l, 1e-9); e == nil {
			t.Fatal("limit")
		}
	}
}
func FuzzConical45FrustumMaterial(f *testing.F) {
	f.Add(conical45FrustumSource(f))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 32768 {
			return
		}
		_, _ = ReadConical45FrustumMaterial(context.Background(), bytes.NewReader(b), Limits{}, 1e-9)
	})
}
