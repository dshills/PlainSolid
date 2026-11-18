package trimmed

import (
	"plainsolid/geom"
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
