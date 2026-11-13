package iges

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"plainsolid/geom"
	"testing"
)

func filledOvalSheet(t testing.TB) (QuadraticOvalFilledSheet, Model, []Model) {
	t.Helper()
	s := planarMappingModel(t).Entities[0].Surface.Geometry
	outer := QuadraticOval{Center: geom.Vec2{X: .5, Y: .5}, Radii: geom.Vec2{X: .25, Y: .25}, QuarterWeight: .75}
	holes := []QuadraticOval{{Center: geom.Vec2{X: .375, Y: .5}, Radii: geom.Vec2{X: .03125, Y: .03125}, QuarterWeight: .5}, {Center: geom.Vec2{X: .625, Y: .5}, Radii: geom.Vec2{X: .03125, Y: .03125}, QuarterWeight: .75}}
	base, e := NewQuadraticOvalSurfaceWithHoles(s, outer, holes, .001)
	if e != nil {
		t.Fatal(e)
	}
	disks := []Model{}
	for i, h := range holes {
		scale := math.Ldexp(2, i)
		support, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 3 * scale}, {X: 2 * scale}, {X: 2 * scale, Y: 3 * scale}}, []float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}, nil)
		if e != nil {
			t.Fatal(e)
		}
		h.Center.X /= scale
		h.Center.Y /= scale
		h.Radii.X /= scale
		h.Radii.Y /= scale
		d, e := NewQuadraticOvalSurface(support, h, .001)
		if e != nil {
			t.Fatal(e)
		}
		disks = append(disks, d)
	}
	disks[0], disks[1] = disks[1], disks[0]
	q, e := NewQuadraticOvalFilledSheet(base, disks)
	if e != nil {
		t.Fatal(e)
	}
	return q, base, disks
}
func TestQuadraticOvalFilledSheet(t *testing.T) {
	q, base, disks := filledOvalSheet(t)
	if q.Base != 0 || len(q.Seams) != 2 || q.Seams[0].Disk != 2 || q.Seams[1].Disk != 1 {
		t.Fatal(q.Seams)
	}
	var b bytes.Buffer
	if e := WriteQuadraticOvalFilledSheet(&b, q); e != nil {
		t.Fatal(e)
	}
	raw := append([]byte(nil), b.Bytes()...)
	got, e := ReadQuadraticOvalFilledSheet(&b, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(got.Seams) != 2 || got.Seams[0].Disk != 2 {
		t.Fatal(got.Seams)
	}
	if _, e = ReadQuadraticOvalSheet(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("old API broadened")
	}
	if _, e = ReadQuadraticOvalFilledSheet(bytes.NewReader(raw), Limits{MaxEntities: 1}); e == nil {
		t.Fatal("limit ignored")
	}
	if _, e = NewQuadraticOvalFilledSheet(base, []Model{disks[0], disks[0]}); e == nil {
		t.Fatal("duplicate disk")
	}
	if _, e = NewQuadraticOvalFilledSheet(base, disks[:1]); e == nil {
		t.Fatal("missing disk")
	}
	disks[0].LengthUnit = .01
	if _, e = NewQuadraticOvalFilledSheet(base, disks); e == nil {
		t.Fatal("units")
	}
	for _, kind := range []string{"unowned", "duplicate-contour", "model", "nested"} {
		t.Run(kind, func(t *testing.T) {
			q, _, _ := filledOvalSheet(t)
			switch kind {
			case "unowned":
				q.Model.Entities = append(q.Model.Entities, q.Model.Entities[0])
			case "duplicate-contour":
				q.Model.TrimmedSurfaces[2].Outer = q.Model.TrimmedSurfaces[1].Outer
			case "model":
				q.Model.CurvesOnSurfaces[4].ModelCurve = q.Model.CurvesOnSurfaces[3].ModelCurve
			case "nested":
				q.Model.TrimmedSurfaces[0].Inner[1] = q.Model.TrimmedSurfaces[0].Inner[0]
			}
			var out bytes.Buffer
			if e := WriteQuadraticOvalFilledSheet(&out, q); e == nil || out.Len() != 0 {
				t.Fatal("invalid sheet emitted")
			}
		})
	}
}
func TestQuadraticOvalFilledSheetOCCT(t *testing.T) {
	if os.Getenv("PLAINSOLID_OCCT_IGES") == "" {
		t.Skip("optional OCCT")
	}
	q, _, _ := filledOvalSheet(t)
	var b bytes.Buffer
	if e := WriteQuadraticOvalFilledSheet(&b, q); e != nil {
		t.Fatal(e)
	}
	path := t.TempDir() + "/sheet.igs"
	if e := os.WriteFile(path, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	out := t.TempDir() + "/area.json"
	oracleCommand(t, out, "iges-surface", path)
	raw, e := os.ReadFile(out)
	if e != nil {
		t.Fatal(e)
	}
	var got struct {
		Faces int
		Area  float64
	}
	if e = json.Unmarshal(raw, &got); e != nil || got.Faces != 3 {
		t.Fatal(got, e)
	}
	ref := rectangleOracleArea(t, "testdata/quadratic-trim.igs")
	if math.Abs(got.Area-ref) > 1e-6 {
		t.Fatal(got.Area, ref)
	}
}

func FuzzQuadraticOvalFilledSheet(f *testing.F) {
	q, _, _ := filledOvalSheet(f)
	var b bytes.Buffer
	if e := WriteQuadraticOvalFilledSheet(&b, q); e != nil {
		f.Fatal(e)
	}
	f.Add(b.Bytes())
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = ReadQuadraticOvalFilledSheet(bytes.NewReader(data), Limits{MaxBytes: 256 << 10, MaxRecords: 4096, MaxEntities: 256, MaxValues: 16384, MaxControlPoints: 2048})
	})
}
