package iges

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"plainsolid/geom"
	"reflect"
	"testing"
)

func curvedOvalFixture(t testing.TB) CurvedOvalSurface {
	t.Helper()
	ps := []geom.Vec3{}
	sq := []float64{.25, -.25, .25}
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			ps = append(ps, geom.Vec3{X: float64(i) / 2, Y: float64(j) / 2, Z: 3 * (sq[i] + sq[j])})
		}
	}
	s, e := geom.NewBSplineSurface(2, 2, 3, 3, ps, []float64{0, 0, 0, 1, 1, 1}, []float64{0, 0, 0, 1, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	m, e := NewCurvedOvalSurface(s, QuadraticOval{Center: geom.Vec2{X: .5, Y: .5}, Radii: geom.Vec2{X: 3. / 16, Y: 3. / 16}, QuarterWeight: .5}, []QuadraticOval{{Center: geom.Vec2{X: .5, Y: .5}, Radii: geom.Vec2{X: 3. / 32, Y: 3. / 32}, QuarterWeight: .5}}, .001)
	if e != nil {
		t.Fatal(e)
	}
	return m
}
func TestCurvedOvalSurface(t *testing.T) {
	m := curvedOvalFixture(t)
	var b bytes.Buffer
	if e := WriteCurvedOvalSurface(&b, m); e != nil {
		t.Fatal(e)
	}
	copy, e := ReadCurvedOvalSurface(bytes.NewReader(b.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if copy.Outer != m.Outer || !reflect.DeepEqual(copy.Holes, m.Holes) || !reflect.DeepEqual(copy.Model.Entities[0].Surface.Geometry.Data(), m.Model.Entities[0].Surface.Geometry.Data()) {
		t.Fatal("source chart/trim changed")
	}
	for _, c := range m.Model.CurvesOnSurfaces {
		a, z := m.Model.Entities[c.ModelCurve].Spline.Curve, copy.Model.Entities[c.ModelCurve].Spline.Curve
		if !reflect.DeepEqual(a.Poles(), z.Poles()) || !reflect.DeepEqual(a.Knots(), z.Knots()) || !reflect.DeepEqual(a.Weights(), z.Weights()) {
			t.Fatal("model coefficients changed")
		}
	}
	if _, e = Read(bytes.NewReader(b.Bytes()), Limits{}); e == nil {
		t.Fatal("generic affine API broadened")
	}
}

func TestCurvedOvalIndependentFixture(t *testing.T) {
	for _, name := range []string{"curved-oval", "curved-oval-annulus"} {
		t.Run(name, func(t *testing.T) {
			raw, e := os.ReadFile("testdata/" + name + ".igs")
			if e != nil {
				t.Fatal(e)
			}
			face, e := ReadCurvedOvalSurface(bytes.NewReader(raw), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			if len(face.Holes) != map[string]int{"curved-oval": 0, "curved-oval-annulus": 1}[name] {
				t.Fatal("hole count")
			}
			expected := curvedOvalFixture(t)
			if !reflect.DeepEqual(face.Model.Entities[face.Model.TrimmedSurfaces[0].Surface].Surface.Geometry.Data(), expected.Model.Entities[0].Surface.Geometry.Data()) {
				t.Fatal("independent chart mismatch")
			}
			var b bytes.Buffer
			if e = WriteCurvedOvalSurface(&b, face); e != nil {
				t.Fatal(e)
			}
			if _, e = ReadCurvedOvalSurface(bytes.NewReader(b.Bytes()), Limits{}); e != nil {
				t.Fatal(e)
			}
			if os.Getenv("PLAINSOLID_OCCT_IGES") != "" {
				path := filepath.Join(t.TempDir(), "target.igs")
				if e = os.WriteFile(path, b.Bytes(), 0600); e != nil {
					t.Fatal(e)
				}
				a, z := rectangleOracleArea(t, "testdata/"+name+".igs"), rectangleOracleArea(t, path)
				if math.Abs(a-z) > 1e-9 {
					t.Fatal(a, z)
				}
			}
		})
	}
}
func TestCurvedOvalReject(t *testing.T) {
	for _, kind := range []string{"model", "rounded", "unowned", "shared", "hole-owner", "domain", "reverse", "support-domain", "support-rounded"} {
		t.Run(kind, func(t *testing.T) {
			s := curvedOvalFixture(t)
			m := &s.Model
			c := m.CurvesOnSurfaces[0]
			switch kind {
			case "model":
				q := m.Entities[c.ModelCurve].Spline
				ps := q.Curve.Poles()
				ps[1].Z = math.Nextafter(ps[1].Z, math.Inf(1))
				q.Curve, _ = geom.NewBSpline(q.Curve.Degree(), ps, q.Curve.Knots(), q.Curve.Weights())
			case "rounded":
				m.Entities[c.ModelCurve].sourceWeightsRounded = true
			case "unowned":
				m.Entities = append(m.Entities, m.Entities[0])
			case "shared":
				m.CurvesOnSurfaces[1].ModelCurve = c.ModelCurve
			case "hole-owner":
				m.TrimmedSurfaces[0].Inner = []int{0}
			case "domain":
				m.Entities[c.ModelCurve].Spline.End = 3
			case "support-domain":
				m.Entities[0].Surface.UEnd = .6
			case "support-rounded":
				m.Entities[0].sourceWeightsRounded = true
			case "reverse":
				m.TrimmedSurfaces[0].Outer = 1
				m.TrimmedSurfaces[0].Inner = []int{0}
			}
			var b bytes.Buffer
			if e := WriteCurvedOvalSurface(&b, s); e == nil || b.Len() != 0 {
				t.Fatal("invalid graph emitted", e)
			}
		})
	}
	s := curvedOvalFixture(t)
	h := s.Holes[0]
	h.Center.X = .75
	if _, e := NewCurvedOvalSurface(s.Model.Entities[0].Surface.Geometry, s.Outer, []QuadraticOval{h}, .001); e == nil {
		t.Fatal("outside hole")
	}
	if _, e := NewCurvedOvalSurface(s.Model.Entities[0].Surface.Geometry, s.Outer, []QuadraticOval{s.Holes[0], s.Holes[0]}, .001); e == nil {
		t.Fatal("overlapping holes")
	}
}
func TestCurvedOvalLimits(t *testing.T) {
	b, e := os.ReadFile("testdata/curved-oval-annulus.igs")
	if e != nil {
		t.Fatal(e)
	}
	for _, l := range []Limits{{MaxBytes: 100}, {MaxRecords: 3}, {MaxEntities: 2}, {MaxValues: 10}, {MaxControlPoints: 4}} {
		if _, e := ReadCurvedOvalSurface(bytes.NewReader(b), l); e == nil {
			t.Fatal("resource limit ignored", l)
		}
	}
}
func FuzzCurvedOvalSurface(f *testing.F) {
	for _, n := range []string{"curved-oval", "curved-oval-annulus"} {
		b, e := os.ReadFile("testdata/" + n + ".igs")
		if e != nil {
			f.Fatal(e)
		}
		f.Add(b)
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = ReadCurvedOvalSurface(bytes.NewReader(b), Limits{MaxBytes: 128 << 10, MaxRecords: 2000, MaxEntities: 64, MaxValues: 20000, MaxControlPoints: 1024})
	})
}
