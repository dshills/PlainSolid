package iges

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"plainsolid/geom"
	"testing"
)

func curvedSheetFixture(t *testing.T) CurvedOvalSheet {
	t.Helper()
	a := curvedOvalFixture(t)
	// The independent disk chart covers model x,y in [0,2], so its UV trim
	// has half the annulus chart coordinates and radii.
	ps := []geom.Vec3{}
	sq := []float64{.25, -.75, 2.25}
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			ps = append(ps, geom.Vec3{X: float64(i), Y: float64(j), Z: 3 * (sq[i] + sq[j])})
		}
	}
	s, e := geom.NewBSplineSurface(2, 2, 3, 3, ps, []float64{0, 0, 0, 1, 1, 1}, []float64{0, 0, 0, 1, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	d, e := NewCurvedOvalSurface(s, QuadraticOval{Center: geom.Vec2{X: .25, Y: .25}, Radii: geom.Vec2{X: 3. / 64, Y: 3. / 64}, QuarterWeight: .5}, nil, .001)
	if e != nil {
		t.Fatal(e)
	}
	q, e := NewCurvedOvalSheet(a.Model, d.Model)
	if e != nil {
		t.Fatal(e)
	}
	return q
}
func TestCurvedOvalSheet(t *testing.T) {
	q := curvedSheetFixture(t)
	var b bytes.Buffer
	if e := WriteCurvedOvalSheet(&b, q); e != nil {
		t.Fatal(e)
	}
	z, e := ReadCurvedOvalSheet(bytes.NewReader(b.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if z.Annulus != 0 || z.Disk != 1 || z.SeamModelCurves[0] == z.SeamModelCurves[1] {
		t.Fatal(z)
	}
	if _, e = ReadCurvedOvalSurface(bytes.NewReader(b.Bytes()), Limits{}); e == nil {
		t.Fatal("single face API broadened")
	}
	for _, kind := range []string{"model", "shared", "unowned", "same-chart", "owner"} {
		t.Run(kind, func(t *testing.T) {
			q := curvedSheetFixture(t)
			m := &q.Model
			switch kind {
			case "model":
				c := m.Entities[q.SeamModelCurves[1]].Spline
				p := c.Curve.Poles()
				p[1].Z = math.Nextafter(p[1].Z, math.Inf(1))
				c.Curve, _ = geom.NewBSpline(c.Curve.Degree(), p, c.Curve.Knots(), c.Curve.Weights())
			case "shared":
				m.CurvesOnSurfaces[2].ModelCurve = q.SeamModelCurves[0]
			case "unowned":
				m.Entities = append(m.Entities, m.Entities[0])
			case "same-chart":
				m.TrimmedSurfaces[1].Surface = m.TrimmedSurfaces[0].Surface
			case "owner":
				m.TrimmedSurfaces[1].Outer = m.TrimmedSurfaces[0].Outer
			}
			var out bytes.Buffer
			if e := WriteCurvedOvalSheet(&out, q); e == nil || out.Len() != 0 {
				t.Fatal("invalid graph emitted", e)
			}
		})
	}
}

func TestCurvedOvalSheetIndependent(t *testing.T) {
	raw, e := os.ReadFile("testdata/curved-oval-sheet.igs")
	if e != nil {
		t.Fatal(e)
	}
	q, e := ReadCurvedOvalSheet(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if q.Model.Entities[q.Model.TrimmedSurfaces[q.Annulus].Surface].Surface.Geometry.Data().Poles[8] == q.Model.Entities[q.Model.TrimmedSurfaces[q.Disk].Surface].Surface.Geometry.Data().Poles[8] {
		t.Fatal("same source chart")
	}
	var b bytes.Buffer
	if e = WriteCurvedOvalSheet(&b, q); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadCurvedOvalSheet(bytes.NewReader(b.Bytes()), Limits{}); e != nil {
		t.Fatal(e)
	}
	if os.Getenv("PLAINSOLID_OCCT_IGES") == "" {
		return
	}
	area := func(path string) float64 {
		out := filepath.Join(t.TempDir(), "area.json")
		oracleCommand(t, out, "iges-surface", path)
		data, e := os.ReadFile(out)
		if e != nil {
			t.Fatal(e)
		}
		var r struct {
			Faces int
			Area  float64
		}
		if e = json.Unmarshal(data, &r); e != nil || r.Faces != 2 || r.Area <= 0 {
			t.Fatal(r, e)
		}
		return r.Area
	}
	path := filepath.Join(t.TempDir(), "target.igs")
	if e = os.WriteFile(path, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	a, z := area("testdata/curved-oval-sheet.igs"), area(path)
	if math.Abs(a-z) > 1e-9 {
		t.Fatal(a, z)
	}
	// Filling the annulus must recover the independently authored outer disk.
	disk := rectangleOracleArea(t, "testdata/curved-oval.igs")
	if math.Abs(a-disk) > 1e-7 {
		t.Fatal(a, disk)
	}
}
func FuzzCurvedOvalSheet(f *testing.F) {
	b, e := os.ReadFile("testdata/curved-oval-sheet.igs")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(b)
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = ReadCurvedOvalSheet(bytes.NewReader(b), Limits{MaxBytes: 128 << 10, MaxRecords: 2000, MaxEntities: 64, MaxValues: 20000, MaxControlPoints: 1024})
	})
}
func TestCurvedOvalSheetValidButDifferentSeam(t *testing.T) {
	q := curvedSheetFixture(t)
	a := curvedOvalFixture(t)
	s := q.Model.Entities[q.Model.TrimmedSurfaces[q.Disk].Surface].Surface.Geometry
	disk, e := NewCurvedOvalSurface(s, QuadraticOval{Center: geom.Vec2{X: .25, Y: .25}, Radii: geom.Vec2{X: 3. / 128, Y: 3. / 128}, QuarterWeight: .5}, nil, .001)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = NewCurvedOvalSheet(a.Model, disk.Model); e == nil {
		t.Fatal("different individually valid model seam accepted")
	}
}
