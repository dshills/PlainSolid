package trimmed

import (
	"bytes"
	"context"
	"math"
	"math/big"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestObtuseConicalRollingFillets(t *testing.T) {
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	for _, upper := range []bool{false, true} {
		R, change := 16.0, -8.0
		if upper {
			R, change = 8, 8
		}
		w, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, R, 6, o)
		if e != nil {
			t.Fatal(e)
		}
		w, _, e = geom.DraftWrappedFrustum(ctx, w, 0, change, o)
		if e != nil {
			t.Fatal(e)
		}
		source, _, e := NewWrappedFrustum(ctx, w, 1e-9, o)
		if e != nil {
			t.Fatal(e)
		}
		f := source.Faces()
		source, e = NewSolid(source.Vertices(), source.Edges(), []Face{f[2], f[0], f[1]}, source.Tolerance())
		if e != nil {
			t.Fatal(e)
		}
		out, h, e := FilletWrappedFrustumObtuseRim(ctx, source, 15.0/8, upper, o)
		if e != nil {
			t.Fatal(upper, e)
		}
		end, centerZ, sideZ, normalZ := 0, big.NewRat(15, 8), big.NewRat(27, 8), big.NewRat(4, 5)
		if upper {
			end, centerZ, sideZ, normalZ = 1, big.NewRat(33, 8), big.NewRat(21, 8), big.NewRat(-4, 5)
		}
		if out.Topology().Faces != 4 || out.Topology().EulerCharacteristic != 2 || h.Blends[end] < 0 || h.Blends[1-end] != -1 || h.MeridianCenter[0].Cmp(big.NewRat(83, 8)) != 0 || h.MeridianCenter[1].Cmp(centerZ) != 0 || h.Tangency[0].Cmp(big.NewRat(23, 2)) != 0 || h.Tangency[1].Cmp(sideZ) != 0 || h.ConeNormal[1].Cmp(normalZ) != 0 {
			t.Fatal(h)
		}
		if len(h.SourceFaces[h.Blends[end]]) != 2 || h.SourceFaces[h.Lateral][0] != 1 {
			t.Fatal("original ancestry", h)
		}
		// Independent full right-half-circle antiderivative, including the point
		// of maximal radial extent. The lower case is its exact Z reflection.
		M, r := 83.0/8, 15.0/8
		lo, hi := -math.Atan2(4, 3), math.Pi/2
		primitive := func(a float64) float64 {
			s, c := math.Sin(a), math.Cos(a)
			return math.Pi * r * (M*M*s + M*r*(a+s*c) + r*r*(s-s*s*s/3))
		}
		V := primitive(hi) - primitive(lo) + math.Pi*(21.0/8)*(64+8*11.5+11.5*11.5)/3
		A := 2*math.Pi*r*(M*(hi-lo)+r*(math.Sin(hi)-math.Sin(lo))) + math.Pi*(8+11.5)*math.Hypot(21.0/8, 3.5) + math.Pi*(64+M*M)
		mass, e := out.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
		if e != nil || math.Abs(mass.Volume-V) > 1e-7 || math.Abs(mass.Area-A) > 1e-7 {
			t.Fatal(mass, V, A, e)
		}
		var b bytes.Buffer
		if e = WriteJSON(&b, out); e != nil {
			t.Fatal(e)
		}
		back, e := ReadJSONContext(ctx, &b, JSONLimits{})
		if e != nil || back.Topology().Faces != 4 {
			t.Fatal(e)
		}
		if bad, h, e := FilletWrappedFrustumObtuseRim(ctx, source, 15.0/8, !upper, o); e == nil || bad.Topology().Faces != 0 || h.Radius != nil {
			t.Fatal("wrong corner class", e)
		}
		if bad, h, e := FilletWrappedFrustumObtuseRim(ctx, source, 1.5, upper, o); e == nil || bad.Topology().Faces != 0 || h.Radius != nil {
			t.Fatal("nonrepresentable tangent circle", e)
		}
		start, delta := 11.0, -3.0
		if upper {
			start, delta = 8, 3
		}
		angular, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, start, 4, o)
		if e != nil {
			t.Fatal(e)
		}
		angular, _, e = geom.DraftWrappedFrustum(ctx, angular, 0, delta, o)
		if e != nil {
			t.Fatal(e)
		}
		unsupported, _, e := NewWrappedFrustum(ctx, angular, 1e-9, o)
		if e != nil {
			t.Fatal(e)
		}
		if bad, h, e := FilletWrappedFrustumObtuseRim(ctx, unsupported, 15.0/8, upper, o); e != geom.ErrUnrepresentable || bad.Topology().Faces != 0 || h.Radius != nil {
			t.Fatal("nonrepresentable angle", e)
		}
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		if bad, h, e := FilletWrappedFrustumObtuseRim(cancelled, source, 15.0/8, upper, o); e != context.Canceled || bad.Topology().Faces != 0 || h.Radius != nil {
			t.Fatal(e)
		}
		if bad, h, e := FilletWrappedFrustumObtuseRim(ctx, source, 15.0/8, upper, geom.SurfacePatchOptions{MaxOperations: 1}); e == nil || bad.Topology().Faces != 0 || h.Radius != nil {
			t.Fatal("budget", e)
		}

	}
}

func TestExactFilletMeridianPhase(t *testing.T) {
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	w, e := geom.NewCanonicalWrappedCircularToroid(ctx, geom.Vec3{Z: 33.0 / 8}, 83.0/8, 15.0/8, 15.0/8, o)
	if e != nil {
		t.Fatal(e)
	}
	shifted, e := halfTurnFilletMeridian(ctx, w, o)
	if e != nil {
		t.Fatal(e)
	}
	for _, u := range []*big.Rat{big.NewRat(1, 3), big.NewRat(5, 4), big.NewRat(19, 7)} {
		for _, v := range []*big.Rat{big.NewRat(0, 1), big.NewRat(1, 3), big.NewRat(3, 2), big.NewRat(2, 1), big.NewRat(8, 3), big.NewRat(4, 1)} {
			original := new(big.Rat).Add(v, big.NewRat(2, 1))
			if original.Cmp(big.NewRat(4, 1)) > 0 {
				original.Sub(original, big.NewRat(4, 1))
			}
			a, e := w.EvaluateRational(ctx, u, original, geom.SurfacePatchEvaluationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			b, e := shifted.EvaluateRational(ctx, u, v, geom.SurfacePatchEvaluationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			for axis := 0; axis < 3; axis++ {
				if a.Point[axis].Cmp(b.Point[axis]) != 0 || a.Du[axis].Cmp(b.Du[axis]) != 0 || a.Dv[axis].Cmp(b.Dv[axis]) != 0 {
					t.Fatal("phase changed exact surface", u, v, axis)
				}
			}
		}
	}
}
