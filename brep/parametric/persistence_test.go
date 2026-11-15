package parametric

import (
	"bytes"
	"context"
	"math"
	"reflect"
	"testing"

	"plainsolid/geom"
)

func TestNativeRoundTrip(t *testing.T) {
	f := frameAt(t, geom.Vec3{X: 2, Y: 3, Z: -1})
	plane, e := geom.NewPlane(f.Origin(), f.ZAxis())
	if e != nil {
		t.Fatal(e)
	}
	for _, build := range []func() (Solid, error){
		func() (Solid, error) { return Sphere(f.Origin(), 1) },
		func() (Solid, error) { return Cone(f, .6, 3) },
		func() (Solid, error) { return Cylinder(f, 1, 3) },
		func() (Solid, error) { return Torus(f, 2, .5) },
		func() (Solid, error) {
			return Revolve(f, []geom.Vec2{{X: 1, Y: 0}, {X: 2, Y: 0}, {X: 2, Y: 2}, {X: 1, Y: 1}})
		},
		func() (Solid, error) {
			return CombineSpheres(Ball{f.Origin(), 1}, Ball{f.ToWorld(geom.Vec3{X: 1}), 1.5}, Difference, 1e-10)
		},
		func() (Solid, error) { return ClipSphere(Ball{f.Origin(), 1}, plane, 1e-10) },
	} {
		s, e := build()
		if e != nil {
			t.Fatal(e)
		}
		var buf bytes.Buffer
		if e = WriteJSON(&buf, s); e != nil {
			t.Fatal(e)
		}
		got, e := ReadJSON(bytes.NewReader(buf.Bytes()), JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		if !reflect.DeepEqual(got.Vertices(), s.Vertices()) || len(got.Edges()) != len(s.Edges()) || len(got.Faces()) != len(s.Faces()) {
			t.Fatal("topology changed")
		}
		for i, f := range s.Faces() {
			h := got.Faces()[i]
			if f.Boundary != h.Boundary || f.Reversed != h.Reversed || f.U != h.U || f.V != h.V {
				t.Fatal("face metadata changed")
			}
			for j := range 17 {
				u, v := float64(j)/16, float64((j*7)%17)/16
				p, e := f.point(u, v)
				q, ferr := h.point(u, v)
				if e != nil || ferr != nil || p.Sub(q).Length() > 1e-12 {
					t.Fatalf("surface changed %v %v %v %v", p, q, e, ferr)
				}
			}
		}
		ctx := context.Background()
		x, e := s.MassProperties(ctx, IntegrationOptions{})
		y, ferr := got.MassProperties(ctx, IntegrationOptions{})
		if e != nil || ferr != nil || math.Abs(x.Volume-y.Volume) > 1e-9 {
			t.Fatalf("volume changed %v %v %v %v", x, y, e, ferr)
		}
		for _, limit := range []JSONLimits{{MaxBytes: 1}, {MaxEntities: 1}} {
			if _, e = ReadJSON(bytes.NewReader(buf.Bytes()), limit); e == nil {
				t.Fatal("accepted over-budget input")
			}
		}
	}
}
func TestSplineDataCopies(t *testing.T) {
	s, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1, Z: 1}}, []float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}, []float64{1, 2, 3, 1})
	if e != nil {
		t.Fatal(e)
	}
	d, e := encodeSurface(s)
	if e != nil {
		t.Fatal(e)
	}
	n := 4
	reconstructed, e := d.decode(&n)
	if e != nil || n != 0 {
		t.Fatalf("decode %d %v", n, e)
	}
	p, e := reconstructed.Eval(.3, .6)
	q, f := s.Eval(.3, .6)
	if e != nil || f != nil || p.Sub(q).Length() > 1e-14 {
		t.Fatalf("spline mismatch: %v %v %v %v", p, q, e, f)
	}
	d.Spline.Poles[0].Z = 100
	z, _ := s.Eval(0, 0)
	if z.Z != 0 {
		t.Fatal("input data leaked")
	}
	if _, e = d.decode(&n); e == nil {
		t.Fatal("control point budget ignored")
	}
}

func TestWrappedSplineAndConicPersistence(t *testing.T) {
	f, e := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	c, _ := geom.NewEllipse(f, 2, 1)
	p, _ := c.RationalBSpline()
	w, _, e := geom.NewWrappedBSpline(p, geom.ClosedCurveOptions{DerivativeOrder: 1})
	if e != nil {
		t.Fatal(e)
	}
	a, _ := geom.NewParabola(f, 2)
	h, _ := geom.NewHyperbolaBranch(f, 2, 3, -1)
	for _, c := range []geom.Curve{w, a, h} {
		raw, e := EncodeCurve(c)
		if e != nil {
			t.Fatal(e)
		}
		d, e := DecodeCurve(raw, 4096)
		if e != nil {
			t.Fatal(e)
		}
		for _, v := range []float64{-.6, 0, .6} {
			a, e := c.Eval(v)
			if e != nil {
				t.Fatal(e)
			}
			b, e := d.Eval(v)
			if e != nil || a.Distance(b) > 1e-14 {
				t.Fatal(a, b, e)
			}
		}
	}
}
func FuzzReadJSON(f *testing.F) {
	var b bytes.Buffer
	s, e := Sphere(geom.Vec3{}, 1)
	if e != nil {
		f.Fatal(e)
	}
	if e = WriteJSON(&b, s); e != nil {
		f.Fatal(e)
	}
	f.Add(b.Bytes())
	f.Add([]byte(`{"version":1,"kind":"parametric-brep"}`))
	f.Fuzz(func(t *testing.T, b []byte) {
		s, e := ReadJSON(bytes.NewReader(b), JSONLimits{MaxBytes: 8192, MaxEntities: 64, MaxControlPoints: 64})
		if e != nil {
			return
		}
		if e = WriteJSON(&bytes.Buffer{}, s); e != nil {
			t.Fatal(e)
		}
	})
}

func TestPeriodicGeometryCodec(t *testing.T) {
	s, e := geom.NewPeriodicBSpline(2, []geom.Vec3{{X: 1}, {Y: 1}, {X: -1}, {Y: -1}}, []float64{1, .8, 1, .8}, -2, 4)
	if e != nil {
		t.Fatal(e)
	}
	b, e := EncodeCurve(s)
	if e != nil {
		t.Fatal(e)
	}
	d, e := DecodeCurve(b, 4)
	if e != nil {
		t.Fatal(e)
	}
	for i := -20; i <= 20; i++ {
		u := float64(i) * .37
		a, e := s.Eval(u)
		b, err := d.Eval(u)
		if e != nil || err != nil || a.Sub(b).Length() > 1e-14 {
			t.Fatalf("%v %v %v %v", a, b, e, err)
		}
	}
}

func TestPeriodicSplineSurfaceCodec(t *testing.T) {
	data := geom.BSplineSurfaceData{
		DegreeU: 1, DegreeV: 1, CountU: 3, CountV: 2,
		KnotsV:  []float64{-3, -3, 2, 2},
		Poles:   []geom.Vec3{{}, {Z: 3}, {X: 2}, {X: 2, Z: 3}, {Y: 2}, {Y: 2, Z: 3}},
		Weights: []float64{2, 1, 1, 3, 4, 5},
	}
	surface, err := geom.NewPeriodicBSplineSurface(data, &geom.PeriodicSplineAxis{Start: 1, Period: 2}, nil)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := EncodeSurface(surface)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeSurface(raw, 6)
	if err != nil {
		t.Fatal(err)
	}
	reencoded, err := EncodeSurface(decoded)
	if err != nil || !bytes.Equal(raw, reencoded) {
		t.Fatalf("source codec changed on reload: %s (%v)", reencoded, err)
	}
	for _, u := range []float64{-15.75, -.8, 1, 3, 4.7, 20.32} {
		p, err := surface.Eval(u, .25)
		q, decodeErr := decoded.Eval(u, .25)
		du, dv, partialErr := surface.Partials(u, .25)
		ru, rv, reloadErr := decoded.Partials(u, .25)
		if err != nil || decodeErr != nil || partialErr != nil || reloadErr != nil || p != q || du != ru || dv != rv {
			t.Fatalf("geometry at %v changed on reload: %v and %v", u, p, q)
		}
	}
	if _, err := DecodeSurface(raw, 5); err == nil {
		t.Fatal("original poles exceeded the native control budget")
	}
	for _, change := range [][2]string{
		{`"Period":2`, `"Period":0`}, {`"periodic-spline-surface"`, `"bspline"`},
		{`"Start":1`, `"Start":1,"Unrecognized":true`}, {`"Kind":`, `"A":1,"Kind":`},
	} {
		altered := bytes.Replace(raw, []byte(change[0]), []byte(change[1]), 1)
		if bytes.Equal(altered, raw) {
			t.Fatalf("failed to alter fixture with %v", change)
		}
		if _, err := DecodeSurface(altered, 6); err == nil {
			t.Fatalf("accepted changed periodic recipe %s", altered)
		}
	}
}
