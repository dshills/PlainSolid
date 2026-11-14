package step

import (
	"bytes"
	"math"
	"math/big"
	"os"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"strings"
	"testing"
)

func extrusionFaceFixture(t testing.TB) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/occt-ellipse-extrusion.step")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestRepairEllipseExtrusion(t *testing.T) {
	raw := extrusionFaceFixture(t)
	f, e := RepairRoundedEllipseExtrusionFace(bytes.NewReader(raw), Limits{}, EllipticalWireRepairOptions{MaxDistance: 1e-11})
	if e != nil {
		t.Fatal(e)
	}
	if f.Height != 3 || f.MajorRadius != 4 || f.MinorRadius != 2 || f.LengthUnit != .001 || f.SourceEdgeIDs != ([3]int64{21, 54, 70}) || f.SourceSeamPCurveIDs != ([2]int64{31, 46}) || len(f.Face.Outer) != 4 || f.Face.Outer[0].Edge != f.Face.Outer[2].Edge || !f.Face.Outer[2].Reversed || f.Bounds.SignedEndResidual[0].Sign() <= 0 {
		t.Fatal(f)
	}
	encoded, e := parametric.EncodeSurface(f.Face.Surface)
	if e != nil {
		t.Fatal(e)
	}
	surface, e := parametric.DecodeSurface(encoded, 100)
	if e != nil {
		t.Fatal(e)
	}
	for _, u := range []float64{0, .375, 1, math.Pi, 2 * math.Pi} {
		for _, v := range []float64{0, -1, -3} {
			p, e := surface.Eval(u, v)
			if e != nil {
				t.Fatal(e)
			}
			want := geom.Vec3{X: 1 - 2*math.Sin(u), Y: 2 - v, Z: 3 - 4*math.Cos(u)}
			if p.Distance(want) > 1e-13 {
				t.Fatal(p, want)
			}
		}
	}
	if _, e = ReadEllipticalDiskFace(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("old disk accepted side shell")
	}
	// Both seam coedges are authenticated independently, including multiplicity.
	mutations := [][2]string{{"#68 = ORIENTED_EDGE('',*,*,#21,.F.);", "#68 = ORIENTED_EDGE('',*,*,#21,.T.);"}, {"#20 = ORIENTED_EDGE('',*,*,#21,.T.);", "#20 = ORIENTED_EDGE('',*,*,#21,.F.);"}, {"(#20,#53,#68,#69)", "(#20,#53,#20,#69)"}, {"#46 = PCURVE('',#32,#47);", "#46 = PCURVE('',#33,#47);"}, {"6.28318530718", "6.2831853071"}, {"(0.,-3.)", "(0.,-2.)"}, {"(-0.,-1.,-0.)", "(0.,1.,0.)"}, {"#56 = ELLIPSE('',#57,4.,2.);", "#56 = ELLIPSE('',#57,5.,2.);"}, {"(1.,5.,-1.)", "(1.,5.,0.)"}, {"#17 = ADVANCED_FACE('',(#18),#32,.T.);", "#17 = ADVANCED_FACE('',(#18),#32,.F.);"}, {".MILLI.,.METRE.", ".CENTI.,.METRE."}}
	for _, m := range mutations {
		t.Run(m[0], func(t *testing.T) {
			s := strings.Replace(string(raw), m[0], m[1], 1)
			if s == string(raw) {
				t.Fatal("stale mutation")
			}
			q, e := RepairRoundedEllipseExtrusionFace(strings.NewReader(s), Limits{}, EllipticalWireRepairOptions{MaxDistance: 1e-11})
			if e == nil || q.Source != nil {
				t.Fatal("accepted mutation")
			}
		})
	}
	for _, tol := range []float64{0, -1, math.NaN(), 1e-14, 1e-4} {
		if _, e := RepairRoundedEllipseExtrusionFace(bytes.NewReader(raw), Limits{}, EllipticalWireRepairOptions{MaxDistance: tol}); e == nil {
			t.Fatal("tolerance", tol)
		}
	}
	if _, e := RepairRoundedEllipseExtrusionFace(bytes.NewReader(raw), Limits{MaxEntities: 88}, EllipticalWireRepairOptions{MaxDistance: 1e-11}); e == nil {
		t.Fatal("limit")
	}
}
func FuzzRepairEllipseExtrusion(f *testing.F) {
	f.Add(extrusionFaceFixture(f))
	f.Add([]byte("ISO-10303-21;"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 65536 {
			return
		}
		_, _ = RepairRoundedEllipseExtrusionFace(bytes.NewReader(b), Limits{MaxBytes: 65536, MaxEntities: 89, MaxValues: 4096, MaxDepth: 16}, EllipticalWireRepairOptions{MaxDistance: 1e-11})
	})
}

func TestRepairEllipseExtrusionPoseAndHeight(t *testing.T) {
	raw := strings.ReplaceAll(string(extrusionFaceFixture(t)), "(1.,5.,", "(1.,7.,")
	raw = strings.ReplaceAll(raw, "(0.,-3.)", "(0.,-5.)")
	f, e := Parse(strings.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	for id, en := range f.Entities {
		if id >= 12 && id <= 14 || len(en.Records) != 1 {
			continue
		}
		r := en.Records[0]
		if (r.Type != "CARTESIAN_POINT" && r.Type != "DIRECTION") || len(r.Args) != 2 || len(r.Args[1].Items) != 3 {
			continue
		}
		v, e := mappedExactCoordinate(f, id, r.Type, 3)
		if e != nil {
			t.Fatal(e)
		}
		v = geom.Vec3{X: -v.Y, Y: v.Z, Z: -v.X}
		if r.Type == "CARTESIAN_POINT" {
			v = v.Add(geom.Vec3{X: 5, Y: 7, Z: 9})
		}
		for i, x := range []float64{v.X, v.Y, v.Z} {
			s, e := mappedTerminatingDecimal(new(big.Rat).SetFloat64(x))
			if e != nil {
				t.Fatal(e)
			}
			en.Records[0].Args[1].Items[i] = Value{Kind: Number, Text: s}
		}
		f.Entities[id] = en
	}
	b, e := mappedPlanarSourceBytes(f)
	if e != nil {
		t.Fatal(e)
	}
	m, e := RepairRoundedEllipseExtrusionFace(bytes.NewReader(b), Limits{}, EllipticalWireRepairOptions{MaxDistance: 1e-11})
	if e != nil {
		t.Fatal(e)
	}
	if m.Height != 5 {
		t.Fatal(m.Height)
	}
	p, e := m.Face.Surface.Eval(0, -5)
	if e != nil || p != (geom.Vec3{X: -2, Y: 6, Z: 8}) {
		t.Fatal(p, e)
	}
}
