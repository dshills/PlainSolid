package parametric

import (
	"bytes"
	"context"
	"math"
	"math/big"
	"plainsolid/collision"
	"plainsolid/geom"
	"testing"
)

func TestNativeTorusCylinderUnion(t *testing.T) {
	ctx := context.Background()
	for _, caps := range [][2]float64{{-3, 4}, {-4, 3}} {
		s, e := NewTorusCylinderUnionSet(ctx, 5, 2, 6, caps[0], caps[1], 1e-9)
		if e != nil {
			t.Fatal(e)
		}
		native := s.Solid()
		if len(native.Faces()) != 5 || len(native.Edges()) != 11 || len(native.Vertices()) != 6 {
			t.Fatal("topology")
		}
		owners, e := s.SourceFaces()
		if e != nil || len(owners[1]) != 2 || owners[2][0] != 3 || owners[3][0] != 4 {
			t.Fatal("owners", e)
		}
		owners[1][0] = 99
		fresh, _ := s.SourceFaces()
		if fresh[1][0] != 1 {
			t.Fatal("alias")
		}
		_, h2, roots, e := s.RimProof()
		if e != nil || h2.Cmp(big.NewRat(3, 1)) != 0 {
			t.Fatal(e)
		}
		for _, root := range roots {
			p, e := root.SignPolynomial(ctx, []*big.Rat{big.NewRat(-3, 1), new(big.Rat), big.NewRat(1, 1)}, geom.ExactCurvePlaneEventOptions{})
			if e != nil || p.Sign != 0 {
				t.Fatal("rim", e)
			}
		}
		p, e := native.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-8})
		if e != nil {
			t.Fatal(e)
		}
		v, e := s.Query().BoundVolume(ctx, geom.TorusCylinderVolumeOptions{VolumeTolerance: big.NewRat(1, 100)})
		if e != nil {
			t.Fatal(e)
		}
		a, e := s.Query().BoundArea(ctx, geom.TorusCylinderAreaOptions{AreaTolerance: big.NewRat(1, 1000000)})
		if e != nil {
			t.Fatal(e)
		}
		for _, test := range []struct {
			x    float64
			l, h *big.Rat
		}{{p.Volume, v.Lower, v.Upper}, {p.Area, a.Lower, a.Upper}} {
			lo, _ := test.l.Float64()
			hi, _ := test.h.Float64()
			if test.x < lo-1e-6 || test.x > hi+1e-6 {
				t.Fatal("independent measure", test.x, lo, hi)
			}
		}
		// Only the complete finite cylinder contributes first Z moment; the torus
		// and removed common are symmetric about z=0.
		wantFirst := math.Pi * 36 * (caps[1]*caps[1] - caps[0]*caps[0]) / 2
		if math.Abs(p.Centroid.Z*p.Volume-wantFirst) > 1e-6 || math.Hypot(p.Centroid.X, p.Centroid.Y) > 1e-8 {
			t.Fatal("independent first moment", p)
		}
		var b bytes.Buffer
		if e = WriteJSON(&b, native); e != nil {
			t.Fatal(e)
		}
		reload, e := ReadJSON(bytes.NewReader(b.Bytes()), JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		var again bytes.Buffer
		if e = WriteJSON(&again, reload); e != nil || !bytes.Equal(b.Bytes(), again.Bytes()) {
			t.Fatal("canonical", e)
		}
		needle := []byte("\"Cap\":4")
		if caps[1] == 3 {
			needle = []byte("\"Cap\":3")
		}
		tamper := bytes.ReplaceAll(b.Bytes(), needle, []byte("\"Cap\":2"))
		if bytes.Equal(tamper, b.Bytes()) {
			t.Fatal("tamper missing")
		}
		if _, e = ReadJSON(bytes.NewReader(tamper), JSONLimits{}); e == nil {
			t.Fatal("cap tangency accepted")
		}
		box, e := reload.Bounds()
		if e != nil {
			t.Fatal(e)
		}
		_ = box
		mesh, e := reload.Tessellate(ctx, TessellationOptions{Deflection: .5, AnalyticBound: true})
		if e != nil {
			t.Fatal(e)
		}
		if e = mesh.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		if mesh.SignedVolume() <= 0 || math.Abs(mesh.SignedVolume()-p.Volume) > .04*p.Volume {
			t.Fatal("mesh orientation/volume")
		}
		report, e := collision.SelfIntersectionsBVHContext(ctx, mesh, collision.Options{})
		if e != nil || !report.Complete || len(report.Contacts) != 0 {
			t.Fatal("global contact", e, len(report.Contacts))
		}
		// Independently count displayed Euler characteristic (collapsed native polar
		// edges are not geometric edges and must not be counted in V-E+F).
		edgeSet := map[[2]int]bool{}
		for _, tri := range mesh.Triangles {
			for j := 0; j < 3; j++ {
				a, b := tri[j], tri[(j+1)%3]
				if a > b {
					a, b = b, a
				}
				edgeSet[[2]int{a, b}] = true
			}
		}
		if len(mesh.Vertices)-len(edgeSet)+len(mesh.Triangles) != 2 {
			t.Fatal("genus")
		}
		t.Log(len(mesh.Triangles), p.Volume, p.Area, p.Centroid.Z)
	}
	if _, e := NewTorusCylinderUnionSet(ctx, 5, 2, 6, -2, 4, 1e-9); e == nil {
		t.Fatal("cap intersects torus")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := NewTorusCylinderUnionSet(canceled, 5, 2, 6, -3, 4, 1e-9); e == nil {
		t.Fatal("cancel")
	}
}

// The outer meridian crosses both torus latitude extrema when c<R; its height
// is not monotone. The wall joining law must not replace that arc by a chord.
func TestNativeTorusCylinderUnionBeyondLatitudeExtrema(t *testing.T) {
	ctx := context.Background()
	s, e := NewTorusCylinderUnionSet(ctx, 5, 2, 4, -3, 4, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	face := s.Solid().Faces()[0]
	p, e := face.Surface.Eval(0, 1/math.Sqrt(3))
	if e != nil || math.Abs(p.X-5) > 1e-12 || math.Abs(p.Z-2) > 1e-12 {
		t.Fatal("retained latitude maximum", p, e)
	}
	native, e := s.Solid().MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-8})
	if e != nil {
		t.Fatal(e)
	}
	v, e := s.Query().BoundVolume(ctx, geom.TorusCylinderVolumeOptions{VolumeTolerance: big.NewRat(1, 100)})
	if e != nil {
		t.Fatal(e)
	}
	lo, _ := v.Lower.Float64()
	hi, _ := v.Upper.Float64()
	if native.Volume < lo-1e-6 || native.Volume > hi+1e-6 {
		t.Fatal(native.Volume, lo, hi)
	}
}
