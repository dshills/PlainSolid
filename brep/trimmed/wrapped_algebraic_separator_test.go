package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
	"testing"
)

func wrappedDifferentBandSupport(t *testing.T, scale float64) *geom.WrappedSurfacePatch {
	t.Helper()
	profile := []geom.Vec2{{X: 2}, {X: 2, Y: 2}, {Y: 2}, {X: -2, Y: 2}, {X: -2}, {X: -2, Y: -2}, {Y: -2}, {X: 2, Y: -2}, {X: 2}}
	var poles []geom.Vec3
	for _, p := range profile {
		for j, v := range []float64{-1, 0, 1} {
			x := .25
			if j == 1 {
				x = -.25
			}
			poles = append(poles, geom.Vec3{X: p.X + x, Y: p.Y, Z: scale * v})
		}
	}
	b, e := geom.NewBSplineSurface(2, 2, 9, 3, poles, []float64{0, 0, 0, 1, 1, 2, 2, 3, 3, 4, 4, 4}, []float64{-1, -1, -1, 1, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	p, e := geom.NewSurfacePatch(b, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	w, _, e := geom.NewWrappedSurfacePatch(context.Background(), p, geom.WrappedSurfaceOptions{WrapU: true, ContinuityOrder: 1})
	if e != nil {
		t.Fatal(e)
	}
	return w
}
func wrappedBandLoop(t *testing.T, umin, umax, vmin, vmax float64) []ExactQuadraticUV {
	t.Helper()
	xy := [][2]float64{{umin, vmin}, {umax, vmin}, {umax, vmax}, {umin, vmax}}
	var loop []ExactQuadraticUV
	for i, p := range xy {
		q := xy[(i+1)%4]
		edge, e := quadraticLine([2]*big.Rat{new(big.Rat).SetFloat64(p[0]), new(big.Rat).SetFloat64(p[1])}, [2]*big.Rat{new(big.Rat).SetFloat64(q[0]), new(big.Rat).SetFloat64(q[1])})
		if e != nil {
			t.Fatal(e)
		}
		loop = append(loop, edge)
	}
	return loop
}
func wrappedSeparatedFixture(t *testing.T, negativeScale, sameSide bool) (*WrappedAlgebraicOpenShell, WrappedCoordinateSeparator) {
	t.Helper()
	ctx := context.Background()
	opts := WrappedQuadraticFaceOptions{SurfaceInjectivityOptions: geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}}}
	a, e := NewWrappedAlgebraicQuadraticFace(ctx, wrappedOverlapSupport(t), wrappedBandLoop(t, -.25, .25, -1, 0), nil, opts)
	if e != nil {
		t.Fatal(e)
	}
	scale := 2.0
	lo, hi := 0., .5
	be := 0
	rev := true
	if negativeScale || sameSide {
		lo, hi = -.5, 0
		be = 2
		rev = false
		opts.Reversed = true
	}
	if negativeScale {
		scale = -2
	}
	b, e := NewWrappedAlgebraicQuadraticFace(ctx, wrappedDifferentBandSupport(t, scale), wrappedBandLoop(t, 3.75, 4.25, lo, hi), nil, opts)
	if e != nil {
		t.Fatal(e)
	}
	shell, e := NewWrappedAlgebraicOpenShell(ctx, a, b, [2]int{0, 2}, [2]int{0, be}, rev, edgecheck.Options{})
	if e != nil {
		t.Fatal(e)
	}
	return shell, WrappedCoordinateSeparator{Axis: 2, Level: new(big.Rat), ParameterAxes: [2]int{1, 1}, Scale: [2]*big.Rat{big.NewRat(1, 1), new(big.Rat).SetFloat64(scale)}, Offset: [2]*big.Rat{new(big.Rat), new(big.Rat)}}
}
func TestWrappedAlgebraicSeparatedPair(t *testing.T) {
	ctx := context.Background()
	for _, negative := range []bool{false, true} {
		s, hint := wrappedSeparatedFixture(t, negative, false)
		p, e := CertifyWrappedAlgebraicSeparatedPair(ctx, s, hint, geom.SurfaceAffineCoordinateOptions{})
		if e != nil {
			t.Fatal(negative, e)
		}
		if p.Shell() != s || p.Sides() != ([2]int{-1, 1}) {
			t.Fatal("exact sides")
		}
		if q, e := CertifyWrappedAlgebraicOpenShellEmbedding(ctx, s, geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}}); q != nil || e == nil {
			t.Fatal("fixture must have different chart maps")
		}
		native := Solid{vertices: s.Vertices(), edges: s.Edges(), faces: s.Faces(), tolerance: 1e-9}
		r, e := native.ValidateEdgeAgreement(ctx, edgecheck.Options{})
		if e != nil || !r.Complete {
			t.Fatal(r, e)
		}
		hint.Scale[0].SetInt64(99)
		if p.Separator().Scale[0].Cmp(big.NewRat(1, 1)) != 0 {
			t.Fatal("mutable input")
		}
		copy := p.Separator()
		copy.Level.SetInt64(2)
		if p.Separator().Level.Sign() != 0 {
			t.Fatal("mutable result")
		}
		opts := geom.SurfaceAffineCoordinateOptions{SurfacePatchOptions: geom.SurfacePatchOptions{MaxOperations: p.ExactOperations() - 1}}
		if q, e := CertifyWrappedAlgebraicSeparatedPair(ctx, s, p.Separator(), opts); q != nil || e == nil {
			t.Fatal("atomic work")
		}
	}
}
func TestWrappedAlgebraicSeparatorRejects(t *testing.T) {
	ctx := context.Background()
	s, h := wrappedSeparatedFixture(t, false, true)
	if p, e := CertifyWrappedAlgebraicSeparatedPair(ctx, s, h, geom.SurfaceAffineCoordinateOptions{}); p != nil || e == nil {
		t.Fatal("same halfspaces accepted")
	}
	s, h = wrappedSeparatedFixture(t, false, false)
	h.Scale[1].SetInt64(1)
	if p, e := CertifyWrappedAlgebraicSeparatedPair(ctx, s, h, geom.SurfaceAffineCoordinateOptions{}); p != nil || e == nil {
		t.Fatal("wrong coordinate formula")
	}
	h.Scale[1].SetInt64(2)
	h.Level.SetInt64(1)
	if p, e := CertifyWrappedAlgebraicSeparatedPair(ctx, s, h, geom.SurfaceAffineCoordinateOptions{}); p != nil || e == nil {
		t.Fatal("wrong separator plane")
	}
	h.Level.SetInt64(0)
	h.ParameterAxes[1] = 0
	if p, e := CertifyWrappedAlgebraicSeparatedPair(ctx, s, h, geom.SurfaceAffineCoordinateOptions{}); p != nil || e == nil {
		t.Fatal("wrapped linear coordinate")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if p, e := CertifyWrappedAlgebraicSeparatedPair(canceled, s, h, geom.SurfaceAffineCoordinateOptions{}); p != nil || e == nil {
		t.Fatal("cancel")
	}
}

func TestWrappedAlgebraicSeparatorRejectsExtraPlanePoint(t *testing.T) {
	ctx := context.Background()
	base, h := wrappedSeparatedFixture(t, false, false)
	xy := [][2]float64{{-.25, 0}, {.25, 0}, {.375, .5}, {.5, 0}, {.75, .75}, {-.375, .75}}
	var loop []ExactQuadraticUV
	for i, p := range xy {
		q := xy[(i+1)%len(xy)]
		edge, e := quadraticLine([2]*big.Rat{new(big.Rat).SetFloat64(p[0] + 4), new(big.Rat).SetFloat64(p[1])}, [2]*big.Rat{new(big.Rat).SetFloat64(q[0] + 4), new(big.Rat).SetFloat64(q[1])})
		if e != nil {
			t.Fatal(e)
		}
		loop = append(loop, edge)
	}
	b, e := NewWrappedAlgebraicQuadraticFace(ctx, base.sources[1].NativeFace().Face.Surface.(*geom.WrappedSurfacePatch), loop, nil, WrappedQuadraticFaceOptions{SurfaceInjectivityOptions: geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}}})
	if e != nil {
		t.Fatal("individually embedded source", e)
	}
	s, e := NewWrappedAlgebraicOpenShell(ctx, base.sources[0], b, [2]int{0, 2}, [2]int{0, 0}, true, edgecheck.Options{})
	if e != nil {
		t.Fatal(e)
	}
	if p, e := CertifyWrappedAlgebraicSeparatedPair(ctx, s, h, geom.SurfaceAffineCoordinateOptions{}); p != nil || e == nil {
		t.Fatal("extra separator-plane boundary point accepted")
	}
}
