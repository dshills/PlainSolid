package step

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"os"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
)

func shearedLensFixture(t testing.TB) SplineLensBody {
	m := splineLensFixture(t)
	m.Name = "Sheared exact lens"
	m.Shear = -.125
	var surfaces [2]*geom.BSplineSurface
	for side, sign := range []float64{1, -1} {
		p := []geom.Vec3{}
		sq := []float64{.25, -.25, .25}
		for i := 0; i < 3; i++ {
			for j := 0; j < 3; j++ {
				u, v := float64(i)/2, float64(j)/2
				p = append(p, geom.Vec3{X: u, Y: v, Z: 2 + sign*(1./16-sq[i]-1.25*sq[j]-(u-.5)*(v-.5))/2})
			}
		}
		s, e := geom.NewBSplineSurface(2, 2, 3, 3, p, []float64{0, 0, 0, 1, 1, 1}, []float64{0, 0, 0, 1, 1, 1}, nil)
		if e != nil {
			t.Fatal(e)
		}
		surfaces[side] = s
	}
	m.Upper, m.Lower = surfaces[0], surfaces[1]
	return m
}
func TestShearedSplineLens(t *testing.T) {
	t.Parallel()
	m := shearedLensFixture(t)
	var b bytes.Buffer
	if e := WriteSplineLensBody(&b, m, CurvedOvalOptions{}); e != nil {
		t.Fatal(e)
	}
	got, e := ReadSplineLensBody(&b, Limits{}, CurvedOvalOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if got.Shear != m.Shear || got.Center != m.Center || got.Radii != m.Radii {
		t.Fatal(got)
	}
	p, e := got.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-9})
	if e != nil || math.Abs(p.Volume-math.Pi/512) > 1e-10 {
		t.Fatal(p, e)
	}
	if _, e = AffineSplineLensFromSolid(got.Solid, got.Center, got.Radii, got.Shear, got.Name, got.LengthUnit, CurvedOvalOptions{}); e != nil {
		t.Fatal(e)
	}
	if _, e = SplineLensFromSolid(got.Solid, got.Center, got.Radii, got.Name, got.LengthUnit, CurvedOvalOptions{}); e == nil {
		t.Fatal("axis-only native recovery accepted sheared body")
	}
}
func TestShearedSplineLensUnrepresentable(t *testing.T) {
	t.Parallel()
	m := shearedLensFixture(t)
	m.Shear = math.SmallestNonzeroFloat64
	var b bytes.Buffer
	if e := WriteSplineLensBody(&b, m, CurvedOvalOptions{}); !errors.Is(e, geom.ErrUnrepresentable) || b.Len() != 0 {
		t.Fatal("rounded affine control emitted", e)
	}
	// A native rational model pole at1/3 has no exact float64/STEP codec pole.
	// Changing a homogeneous weight scale cannot make that Euclidean pole exact.
	controls := [][4]*big.Rat{{big.NewRat(1, 3), new(big.Rat), new(big.Rat), big.NewRat(1, 1)}, {big.NewRat(2, 3), new(big.Rat), new(big.Rat), big.NewRat(1, 1)}}
	c, e := geom.NewExactHomogeneousBezier(context.Background(), controls, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	o, _ := (CurvedOvalOptions{}).defaults()
	if _, _, e = exactBezierFloats(c, o.patch()); !errors.Is(e, geom.ErrUnrepresentable) {
		t.Fatal("native-only rational rounded", e)
	}
}
func TestShearedSplineLensOCCT(t *testing.T) {
	t.Parallel()
	source := t.TempDir() + "/source.step"
	stepOracle(t, source, "write-step-sheared-spline-lens")
	raw, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadSplineLensBody(bytes.NewReader(raw), Limits{}, CurvedOvalOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if m.Shear != -.125 {
		t.Fatal(m.Shear)
	}
	var b bytes.Buffer
	if e = WriteSplineLensBody(&b, m, CurvedOvalOptions{}); e != nil {
		t.Fatal(e)
	}
	path := t.TempDir() + "/native.step"
	if e = os.WriteFile(path, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	native, e := m.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-9})
	if e != nil {
		t.Fatal(e)
	}
	for i, p := range []string{source, path} {
		out := t.TempDir() + "/mass.json"
		stepOracle(t, out, "step-mass-precise", p)
		data, e := os.ReadFile(out)
		if e != nil {
			t.Fatal(e)
		}
		var mass struct{ Volume, Area float64 }
		if e = json.Unmarshal(data, &mass); e != nil {
			t.Fatal(e)
		}
		vt, at := 1e-10, 1e-8
		if i == 0 {
			vt, at = 2e-9, 1e-7
		}
		if math.Abs(mass.Volume-math.Pi/512) > vt || math.Abs(mass.Area-native.Area) > at {
			t.Fatal(i, mass, native)
		}
	}
}
func TestIndependentShearedLens(t *testing.T) {
	t.Parallel()
	raw, e := os.ReadFile("testdata/sheared-spline-lens.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadSplineLensBody(bytes.NewReader(raw), Limits{}, CurvedOvalOptions{})
	if e != nil || m.Shear != -.125 {
		t.Fatal(m.Shear, e)
	}
	f, _ := Parse(bytes.NewReader(raw), Limits{})
	for _, id := range f.Order {
		face, e := simple(f, id, "ADVANCED_FACE", 4)
		if e != nil {
			continue
		}
		sid, _ := ref(face.Args[2])
		surface, _ := simple(f, sid, "B_SPLINE_SURFACE_WITH_KNOTS", 13)
		if surface.Type == "" {
			continue
		}
		rows := surface.Args[3].Items
		pid, _ := ref(rows[1].Items[1])
		p, _ := simple(f, pid, "CARTESIAN_POINT", 2)
		z, _ := numeric(p.Args[1].Items[2])
		p.Args[1].Items[2] = Value{Kind: Number, Text: real(math.Nextafter(z, math.Inf(1)))}
		ovalSet(f, pid, p)
		if _, e = ReadSplineLensBody(bytes.NewReader(serializeWrappedProofFile(f)), Limits{}, CurvedOvalOptions{}); e == nil {
			t.Fatal("source support perturbation accepted")
		}
		return
	}
	t.Fatal("fixture mutation missed")
}
