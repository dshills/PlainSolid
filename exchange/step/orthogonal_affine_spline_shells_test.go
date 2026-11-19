package step

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"plainsolid/geom"
	"reflect"
	"strings"
	"testing"
)

func placedAffineShell(t testing.TB, a geom.Affine) OrthogonalAffineSplineShellModel {
	t.Helper()
	m := axisAlignedShellFixture(t)
	for i := range m.Bodies {
		for j := range m.Bodies[i].Faces {
			f := &m.Bodies[i].Faces[j]
			d := f.Geometry.Data()
			for k, p := range d.Poles {
				d.Poles[k] = a.Apply(p)
			}
			g, e := geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
			if e != nil {
				t.Fatal(e)
			}
			f.Geometry = g
		}
	}
	return m
}
func TestOrthogonalAffineSplineShell(t *testing.T) {
	t.Parallel()
	m := placedAffineShell(t, geom.Affine{{3, -4, 0, 7}, {4, 3, 0, 11}, {0, 0, 5, 13}})
	var b bytes.Buffer
	if e := WriteOrthogonalAffineSplineShells(&b, m); e != nil {
		t.Fatal(e)
	}
	raw := append([]byte(nil), b.Bytes()...)
	round, e := ReadOrthogonalAffineSplineShells(&b, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	mass, e := round.Bodies[0].Solid.MassProperties()
	if e != nil || math.Abs(mass.Volume-4500) > 1e-9 || math.Abs(mass.SurfaceArea-1800) > 1e-9 || mass.Centroid.Sub(geom.Vec3{X: 16. / 3, Y: 68. / 3, Z: 20.5}).Length() > 1e-12 {
		t.Fatal(mass, e)
	}
	for i, face := range round.Bodies[0].Faces {
		if !reflect.DeepEqual(face.Geometry.Data(), m.Bodies[0].Faces[i].Geometry.Data()) || !reflect.DeepEqual(face.Boundary, m.Bodies[0].Faces[i].Boundary) {
			t.Fatal("model-space support changed")
		}
	}
	if _, e = ReadAxisAlignedSplineShells(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("axis reader broadened")
	}
	var bad bytes.Buffer
	if e = WriteAxisAlignedSplineShells(&bad, m); e == nil || bad.Len() != 0 {
		t.Fatal("axis writer broadened", e)
	}
}
func TestOrthogonalAffineSplineShellReject(t *testing.T) {
	t.Parallel()
	for _, a := range []geom.Affine{{{1, 1, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}}, {{3, math.Nextafter(-4, 0), 0, 7}, {4, 3, 0, 11}, {0, 0, 5, 13}}, {{-1, 0, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}}} {
		m := placedAffineShell(t, a)
		var b bytes.Buffer
		if e := WriteOrthogonalAffineSplineShells(&b, m); e == nil || b.Len() != 0 {
			t.Fatal("unqualified placement emitted", a, e)
		}
	}
	m := placedAffineShell(t, geom.Affine{{3, -4, 0, 0}, {4, 3, 0, 0}, {0, 0, 2, 0}})
	if !commonOrthogonalAffineSupports(m.Bodies[0].Faces) {
		t.Fatal("exact nonunit orthogonal axes rejected")
	}
}
func TestOCCTOrthogonalAffineSplineShell(t *testing.T) {
	t.Parallel()
	source := filepath.Join(t.TempDir(), "source.step")
	stepOracle(t, source, "write-step-orthogonal-affine-spline-shell")
	raw, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadOrthogonalAffineSplineShells(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteOrthogonalAffineSplineShells(&b, m); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "ours.step")
	if e = os.WriteFile(path, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	for _, p := range []string{source, path} {
		out := filepath.Join(t.TempDir(), "mass.json")
		stepOracle(t, out, "step-mass-precise", p)
		data, e := os.ReadFile(out)
		if e != nil {
			t.Fatal(e)
		}
		var mass struct{ Area, Volume float64 }
		if e = json.Unmarshal(data, &mass); e != nil || math.Abs(mass.Volume-4500) > 1e-8 || math.Abs(mass.Area-1800) > 1e-8 {
			t.Fatal(mass, e)
		}
	}
}

func TestIndependentOrthogonalAffineSplineShell(t *testing.T) {
	t.Parallel()
	raw, e := os.ReadFile("testdata/orthogonal-affine-spline-shell.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadOrthogonalAffineSplineShells(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	mass, e := m.Bodies[0].Solid.MassProperties()
	if e != nil || math.Abs(mass.Volume-4500) > 1e-9 {
		t.Fatal(mass, e)
	}
	for _, pair := range [][2]string{{"#39 = CARTESIAN_POINT('',(4.,2.));", "#39 = CARTESIAN_POINT('',(4.,2.0000000000000004));"}, {"#38 = CARTESIAN_POINT('',(4.,0.));", "#38 = CARTESIAN_POINT('',(4.000000000000001,0.));"}} {
		bad := strings.Replace(string(raw), pair[0], pair[1], 1)
		if bad == string(raw) {
			t.Fatal("missed pcurve mutation")
		}
		if _, e := ReadOrthogonalAffineSplineShells(strings.NewReader(bad), Limits{}); e == nil {
			t.Fatal("inexact source pcurve accepted")
		}
	}
}
func FuzzOrthogonalAffineSplineShell(f *testing.F) {
	raw, e := os.ReadFile("testdata/orthogonal-affine-spline-shell.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(raw)
	f.Fuzz(func(t *testing.T, raw []byte) {
		_, _ = ReadOrthogonalAffineSplineShells(bytes.NewReader(raw), Limits{MaxBytes: 65536, MaxEntities: 1024, MaxVertices: 128, MaxEdgeUses: 128})
	})
}

func TestAffinePCurveSharedDomainReject(t *testing.T) {
	t.Parallel()
	raw, e := os.ReadFile("testdata/orthogonal-affine-spline-shell.step")
	if e != nil {
		t.Fatal(e)
	}
	f, e := Parse(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	curve, e := simple(f, 56, "SURFACE_CURVE", 4)
	if e != nil {
		t.Fatal(e)
	}
	// Reuse an already-normalized p-curve on a different model domain. A proof
	// view LINE must retain the bounded source spline's domain constraint.
	curve.Args[2].Items[0] = Value{Kind: Reference, Ref: 30}
	f.Entities[56] = Entity{ID: 56, Records: []Record{curve}}
	if e = normalizeAffineIsoPCurves(f, 128); e == nil {
		t.Fatal("reused bounded pcurve lost its domain")
	}
}
