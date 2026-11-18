//go:build research

package trimmed

import (
	"context"
	"math/big"
	"testing"

	"plainsolid/geom"
)

func TestMaterializeWrappedRectangleFaceCells(t *testing.T) {
	ctx := context.Background()
	w := wrappedOverlapSupport(t)
	refined, _, e := w.InsertKnot(ctx, 0, .125, 1, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	a := wrappedOverlapFace(t, w, geom.ParameterRectangle{UMin: 4.125, UMax: 4.375, VMin: -1, VMax: 1})
	b := wrappedOverlapFace(t, refined, geom.ParameterRectangle{UMin: .25, UMax: .5, VMin: 0, VMax: 2})
	b.Face.Reversed = true
	original := a.Edges[0].Curve.(surfaceTrimCurve)
	original.a, original.b = original.b, original.a
	a.Edges[0].Curve = original
	a.Edges[0].Start, a.Edges[0].End = a.Edges[0].End, a.Edges[0].Start
	a.Face.Outer[0].Reversed = true
	opts := WrappedRectangleFaceOverlapOptions{ProjectionAxes: [2]int{1, 2}}
	r, e := MaterializeWrappedRectangleFaceCells(ctx, a, b, opts)
	if e != nil || !r.Complete || len(r.Cells) != 7 || !r.FaceReversed[1] {
		t.Fatal(r, e)
	}
	area, common := new(big.Rat), new(big.Rat)
	mates, boundaries := 0, 0
	for ci, c := range r.Cells {
		self, err := CertifyWrappedRectangleFaceOverlap(ctx, c.Face, c.Face, opts)
		if err != nil || !self.Complete {
			t.Fatal("materialized native face failed independent revalidation", ci, err)
		}
		if c.Face.Face.Surface != w || c.Face.Face.Reversed != a.Face.Reversed {
			t.Fatal("model representative changed")
		}
		loop := c.Face.Face.Outer
		cellArea := new(big.Rat)
		for i, u := range loop {
			cross := new(big.Rat).Sub(new(big.Rat).Mul(ratInset(u.UVStart.X), ratInset(u.UVEnd.Y)), new(big.Rat).Mul(ratInset(u.UVStart.Y), ratInset(u.UVEnd.X)))
			cellArea.Add(cellArea, cross)
			edge := c.Face.Edges[i]
			side := c.Sides[i]
			if side.MateCell >= 0 {
				mates++
				other := r.Cells[side.MateCell]
				mate := other.Sides[side.MateSide]
				if mate.MateCell != ci || mate.MateSide != i {
					t.Fatal("asymmetric mate")
				}
				oe := other.Face.Edges[side.MateSide]
				for _, p := range []float64{0, .125, .5, .875, 1} {
					x, err := edge.Curve.Eval(p)
					y, otherErr := oe.Curve.Eval(1 - p)
					if err != nil || otherErr != nil || x != y {
						t.Fatal("different exact affine mate", x, y)
					}
				}
			} else {
				boundaries++
				if len(side.Original) == 0 {
					t.Fatal("unowned exposed edge")
				}
			}
			for _, owner := range side.Original {
				source := a
				if owner.Face == 1 {
					source = b
				}
				original := source.Edges[owner.Edge]
				for end, p := range owner.Parameters {
					f, err := geom.RepresentedExactScalar(p)
					if err != nil {
						t.Fatal(err)
					}
					model, err := original.Curve.Eval(f)
					if err != nil {
						t.Fatal(err)
					}
					vertex := c.Face.Vertices[edge.Start]
					if end == 1 {
						vertex = c.Face.Vertices[edge.End]
					}
					if model != vertex {
						t.Fatal("lost original range")
					}
				}
			}
		}
		cellArea.Quo(cellArea, big.NewRat(2, 1))
		if cellArea.Sign() <= 0 {
			t.Fatal("nonpositive cell")
		}
		area.Add(area, cellArea)
		if c.Owners == 3 {
			common.Add(common, cellArea)
		}
		for owner := 0; owner < 2; owner++ {
			if c.Owners&(1<<owner) == 0 {
				continue
			}
			source := w
			if owner == 1 {
				source = refined
			}
			for i, uv := range c.OwnerUV[owner] {
				jet, err := source.EvaluateRational(ctx, uv[0], uv[1], geom.SurfacePatchEvaluationOptions{})
				if err != nil {
					t.Fatal(err)
				}
				p, err := geom.RepresentedExactPoint(jet.Point)
				if err != nil || p != c.Face.Vertices[i] {
					t.Fatal("owner coordinates", err)
				}
			}
		}
	}
	if area.Cmp(big.NewRat(7, 8)) != 0 || common.Cmp(big.NewRat(1, 8)) != 0 || mates != 16 || boundaries != 12 {
		t.Fatal(area, common, mates, boundaries)
	}
	bad := opts
	bad.MaxOperations = r.ExactOperations - 1
	out, e := MaterializeWrappedRectangleFaceCells(ctx, a, b, bad)
	if e == nil || out.Complete || len(out.Cells) != 0 {
		t.Fatal("late quota publication", out, e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	out, e = MaterializeWrappedRectangleFaceCells(canceled, a, b, opts)
	if e == nil || out.Complete {
		t.Fatal(out, e)
	}
}

// A rational graph whose original eight face corners are represented exactly,
// but the new grid corner (1/4,1/2) has an odd denominator in model Z.
// X=u(1-u), Y=v, Z=1/W; W=1+u(1-u)(u-1/8)(u-3/8)(v-1/4)(v-3/4).
// The U endpoints agree exactly, giving a C0 wrapped support.
func wrappedNonrepresentableFragmentSupport(t *testing.T) *geom.WrappedSurfacePatch {
	t.Helper()
	multiply := func(a, b []*big.Rat) []*big.Rat {
		out := make([]*big.Rat, len(a)+len(b)-1)
		for i := range out {
			out[i] = new(big.Rat)
		}
		for i, x := range a {
			for j, y := range b {
				out[i+j].Add(out[i+j], new(big.Rat).Mul(x, y))
			}
		}
		return out
	}
	u := []*big.Rat{big.NewRat(0, 1), big.NewRat(1, 1), big.NewRat(-1, 1)}
	fu := multiply(multiply(u, []*big.Rat{big.NewRat(-1, 8), big.NewRat(1, 1)}), []*big.Rat{big.NewRat(-3, 8), big.NewRat(1, 1)})
	fv := multiply([]*big.Rat{big.NewRat(-1, 4), big.NewRat(1, 1)}, []*big.Rat{big.NewRat(-3, 4), big.NewRat(1, 1)})
	var power [4][7][4]*big.Rat
	for axis := range power {
		for i := range power[axis] {
			for j := range power[axis][i] {
				power[axis][i][j] = new(big.Rat)
			}
		}
	}
	for i, x := range fu {
		for j, y := range fv {
			power[3][i][j].Mul(x, y)
		}
	}
	power[3][0][0].Add(power[3][0][0], big.NewRat(1, 1))
	power[2][0][0].SetInt64(1)
	for i := 0; i <= 4; i++ {
		for j := 0; j <= 2; j++ {
			power[0][i+1][j].Add(power[0][i+1][j], power[3][i][j])
			power[0][i+2][j].Sub(power[0][i+2][j], power[3][i][j])
			power[1][i][j+1].Set(power[3][i][j])
		}
	}
	controls := make([][4]*big.Rat, 28)
	for i := 0; i <= 6; i++ {
		for j := 0; j <= 3; j++ {
			for axis := 0; axis < 4; axis++ {
				c := new(big.Rat)
				for k := 0; k <= i; k++ {
					for l := 0; l <= j; l++ {
						scale := new(big.Rat).SetFrac(new(big.Int).Binomial(int64(i), int64(k)), new(big.Int).Binomial(6, int64(k)))
						scale.Mul(scale, new(big.Rat).SetFrac(new(big.Int).Binomial(int64(j), int64(l)), new(big.Int).Binomial(3, int64(l))))
						c.Add(c, new(big.Rat).Mul(scale, power[axis][k][l]))
					}
				}
				controls[i*4+j][axis] = c
			}
		}
	}
	patch, e := geom.NewExactHomogeneousBezierSurface(context.Background(), 6, 3, controls, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	w, _, e := geom.NewWrappedSurfacePatch(context.Background(), patch, geom.WrappedSurfaceOptions{WrapU: true})
	if e != nil {
		t.Fatal(e)
	}
	return w
}

func TestWrappedRectangleFaceCellsRejectNonrepresentedNewCorner(t *testing.T) {
	ctx := context.Background()
	w := wrappedNonrepresentableFragmentSupport(t)
	a := wrappedOverlapFace(t, w, geom.ParameterRectangle{UMin: .125, UMax: .375, VMin: 0, VMax: .5})
	b := wrappedOverlapFace(t, w, geom.ParameterRectangle{UMin: .25, UMax: .4375, VMin: .25, VMax: .75})
	opts := WrappedRectangleFaceOverlapOptions{ProjectionAxes: [2]int{0, 1}}
	proof, e := CertifyWrappedRectangleFaceOverlap(ctx, a, b, opts)
	if e != nil || !proof.Complete {
		t.Fatal("exact region must qualify", e)
	}
	jet, e := w.EvaluateRational(ctx, big.NewRat(1, 4), big.NewRat(1, 2), geom.SurfacePatchEvaluationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = geom.RepresentedExactPoint(jet.Point); e == nil {
		t.Fatal("fixture corner is representable")
	}
	out, e := MaterializeWrappedRectangleFaceCells(ctx, a, b, opts)
	if e == nil || out.Complete || len(out.Cells) != 0 {
		t.Fatal("rounded new vertex accepted", out, e)
	}
}
