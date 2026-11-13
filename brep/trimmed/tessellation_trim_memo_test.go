package trimmed

import (
	"context"
	"errors"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"reflect"
	"sync"
	"testing"
)

type countedDisplayTrim struct {
	calls int
	shift float64
	fail  bool
}

func (c *countedDisplayTrim) EvalUV(t float64) (geom.Vec2, error) {
	c.calls++
	if c.fail {
		return geom.Vec2{}, geom.ErrDomain
	}
	return geom.Vec2{X: t, Y: c.shift}, nil
}
func TestTessellationTrimMemoIdentityAndBounds(t *testing.T) {
	memo := &tessellationTrimMemo{limit: 2}
	a, b := &countedDisplayTrim{}, &countedDisplayTrim{shift: 1}
	first := tessellationMemoCurve{a, memo, 0}
	second := tessellationMemoCurve{b, memo, 1}
	for i := 0; i < 4; i++ {
		p, e := first.EvalUV(.25)
		q, f := second.EvalUV(.25)
		if e != nil || f != nil || p.Y != 0 || q.Y != 1 {
			t.Fatal(p, q, e, f)
		}
	}
	if a.calls != 1 || b.calls != 1 || memo.hits != 6 {
		t.Fatal("seam alias or missing reuse", a, b, memo)
	}
	for i := 0; i < 4; i++ {
		_, _ = first.EvalUV(.5)
	}
	if len(memo.values) != 2 || a.calls != 5 {
		t.Fatal("capacity", a.calls)
	}
	reverse := trimSide{edge: 0, use: Use{Reversed: true, Curve: first}}
	p, e := reverse.point(.75)
	if e != nil || p.X != .25 || a.calls != 5 {
		t.Fatal("reversed parameter", p, e)
	}
	bad := &countedDisplayTrim{fail: true}
	curve := tessellationMemoCurve{bad, &tessellationTrimMemo{limit: 10}, 2}
	for i := 0; i < 2; i++ {
		if _, e := curve.EvalUV(.2); e == nil {
			t.Fatal("error lost")
		}
	}
	if bad.calls != 2 {
		t.Fatal("cached error")
	}
}
func TestTessellationTrimMemoEquivalence(t *testing.T) {
	ctx := context.Background()
	frame, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	s, e := ExtrudeDiskWithHoles(ctx, frame, ExtrusionCircle{Radius: 4}, []ExtrusionCircle{{Center: geom.Vec2{X: 1}, Radius: 1}}, nil, 3)
	if e != nil {
		t.Fatal(e)
	}
	before := s.Faces()
	for _, o := range []parametric.TessellationOptions{{Deflection: .1, Angle: .2}, {Deflection: .01, MaxTriangles: 1}, {Deflection: .1, AnalyticBound: true}} {
		old, oe := s.tessellateWithTrimMemo(ctx, o, nil)
		memo := &tessellationTrimMemo{limit: tessellationTrimMemoLimit}
		got, ge := s.tessellateWithTrimMemo(ctx, o, memo)
		if !errors.Is(ge, oe) || !reflect.DeepEqual(old, got) {
			t.Fatal("changed mesh or error", oe, ge)
		}
		if oe == nil {
			if memo.hits == 0 {
				t.Fatal("no reuse")
			}
			if e = got.ValidateSolid(); e != nil {
				t.Fatal(e)
			}
			tiny, te := s.tessellateWithTrimMemo(ctx, o, &tessellationTrimMemo{limit: 1})
			if te != nil || !reflect.DeepEqual(old, tiny) {
				t.Fatal("saturation changed mesh", te)
			}
		}
	}
	if !reflect.DeepEqual(before, s.Faces()) {
		t.Fatal("source mutation")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, e = s.Tessellate(canceled, parametric.TessellationOptions{Deflection: .1})
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := s.Tessellate(ctx, parametric.TessellationOptions{Deflection: .3})
			if e != nil {
				t.Error(e)
			}
		}()
	}
	wg.Wait()
}
