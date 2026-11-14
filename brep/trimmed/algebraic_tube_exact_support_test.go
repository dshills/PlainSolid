package trimmed

import (
	"bytes"
	"context"
	"math/big"
	"testing"

	"plainsolid/geom"
	"plainsolid/intersect"
)

func TestAlgebraicSectionRetainsNonbinarySupports(t *testing.T) {
	ctx := context.Background()
	var h [][4]*big.Rat
	for x := int64(1); x <= 2; x++ {
		for y := int64(0); y <= 1; y++ {
			h = append(h, [4]*big.Rat{big.NewRat(x, 3), big.NewRat(y, 1), big.NewRat((x-1)*y, 3), big.NewRat(1, 1)})
		}
	}
	p, e := geom.NewExactHomogeneousBezierSurface(ctx, 1, 1, h, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	a, e := geom.NewExactBezierSurface(p)
	if e != nil {
		t.Fatal(e)
	}
	k := []float64{0, 0, 1, 1}
	b, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{Y: .5, Z: -1}, {Y: .5, Z: 1}, {X: 1, Y: .5, Z: -1}, {X: 1, Y: .5, Z: 1}}, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	patch := func(s geom.Surface) intersect.Patch { return intersect.Patch{Surface: s, Max: geom.Vec2{X: 1, Y: 1}} }
	trace := intersect.SplinePairTraceOptions{}
	options := geom.TubeAlgebraicGraphEndpointOptions{}
	r, e := SectionSplinePairAlgebraic(ctx, patch(a), patch(b), nil, trace, options)
	if e != nil || !r.Complete || len(r.Open) != 1 {
		t.Fatal("the original curved chart did not qualify", len(r.Open), e)
	}
	var out bytes.Buffer
	if e = WriteAlgebraicSplineSectionJSON(&out, r); e != nil {
		t.Fatal(e)
	}
	reload, e := ReadAlgebraicSplineSectionJSON(ctx, bytes.NewReader(out.Bytes()), JSONLimits{}, trace, options)
	if e != nil || !reload.Complete || len(reload.Open) != 1 {
		t.Fatal("native reload did not independently reprove the section", len(reload.Open), e)
	}
	for _, s := range []AlgebraicSplinePairSection{r, reload} {
		var x [2]string
		for i, end := range s.Open[0].Ends {
			v, e := end.Certificate.Parameters[2].Classify(ctx, geom.ExactCurvePlaneEventOptions{})
			if e != nil || v.Kind != geom.ExactRootKindRational || end.Operand != 0 {
				t.Fatal("exact source terminal", v, end.Operand, e)
			}
			x[i] = v.Rational.RatString()
		}
		if x != [2]string{"1/3", "2/3"} && x != [2]string{"2/3", "1/3"} {
			t.Fatal("nonrepresentable parameter was lost", x)
		}
	}
	if _, e = ReadAlgebraicSplineSectionJSON(ctx, bytes.NewReader(out.Bytes()), JSONLimits{MaxControlPoints: 3}, trace, options); e == nil {
		t.Fatal("nonbinary support bypassed the serialized control budget")
	}
}
