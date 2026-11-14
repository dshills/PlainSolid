package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestWrappedCylinderTwoRectangleOverlap(t *testing.T) {
	ctx := context.Background()
	w, _, e := NewWrappedSurfacePatch(ctx, wrappedCylinderPatch(t), WrappedSurfaceOptions{WrapU: true, ContinuityOrder: 1})
	if e != nil {
		t.Fatal(e)
	}
	target, _, e := w.InsertKnot(ctx, 0, .125, 1, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	rect := func(u0, u1, v0, v1 *big.Rat) [2][2]*big.Rat { return [2][2]*big.Rat{{u0, u1}, {v0, v1}} }
	a := rect(big.NewRat(0, 1), big.NewRat(10, 3), big.NewRat(-1, 1), big.NewRat(2, 1))
	b := rect(big.NewRat(2, 1), big.NewRat(5, 1), big.NewRat(0, 1), big.NewRat(3, 1))
	for _, reverse := range []bool{false, true} {
		other := target
		comparison := ExactSurfaceComparisonOptions{}
		ownerB := b
		if reverse {
			other, _, e = target.Reparameterize(ctx, ParameterRectangle{UMin: 2, UMax: 10, VMin: -3, VMax: 7}, true, false, SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
			comparison = ExactSurfaceComparisonOptions{NormalizeDomains: true, ReverseBU: true}
			ownerB = rect(big.NewRat(0, 1), big.NewRat(6, 1), big.NewRat(1, 1), big.NewRat(7, 1))
		}
		relation, _, e := CompareWrappedSurfaceCharts(ctx, w, other, comparison)
		if e != nil {
			t.Fatal(e)
		}
		result, e := relation.CertifyCylinderTwoRectangleOverlap(ctx, a, ownerB, WrappedCylinderTwoOverlapOptions{})
		if e != nil || !result.Complete || !result.UnionAnnular || !result.UnionConnected || result.IntersectionComponents != 2 || result.UnionBoundaryComponents != 2 {
			t.Fatal(result, e)
		}
		if result.UnionAngularLength.Cmp(big.NewRat(4, 1)) != 0 || result.AngularGaps[0].Cmp(big.NewRat(1, 1)) != 0 || result.AngularGaps[1].Cmp(big.NewRat(2, 3)) != 0 || result.CommonSourceV[0].Sign() != 0 || result.CommonSourceV[1].Cmp(big.NewRat(2, 1)) != 0 {
			t.Fatal("wrong exact quotient coverage", result)
		}
		if result.Components[0].Overlap.A[0][0].Sign() != 0 || result.Components[0].Overlap.A[0][1].Cmp(big.NewRat(1, 1)) != 0 || result.Components[1].Overlap.A[0][0].Cmp(big.NewRat(2, 1)) != 0 || result.Components[1].Overlap.A[0][1].Cmp(big.NewRat(10, 3)) != 0 {
			t.Fatal("lost nonbinary original interval")
		}
		for _, component := range result.Components {
			loops := [2][4][2]*big.Rat{component.ALoop, component.BLoop}
			for owner, loop := range loops {
				area := new(big.Rat)
				for i, p := range loop {
					q := loop[(i+1)%4]
					area.Add(area, new(big.Rat).Sub(new(big.Rat).Mul(p[0], q[1]), new(big.Rat).Mul(p[1], q[0])))
				}
				if area.Sign() != result.OwnerOrientations[owner] {
					t.Fatal("wrong directed owner loop")
				}
			}
			for i, p := range component.ALoop {
				ja, e := w.EvaluateRational(ctx, p[0], p[1], SurfacePatchEvaluationOptions{})
				if e != nil {
					t.Fatal(e)
				}
				q := component.BLoop[i]
				jb, e := other.EvaluateRational(ctx, q[0], q[1], SurfacePatchEvaluationOptions{})
				if e != nil {
					t.Fatal(e)
				}
				for axis := 0; axis < 3; axis++ {
					if ja.Point[axis].Cmp(jb.Point[axis]) != 0 {
						t.Fatal("different model corner")
					}
				}
			}
		}
		if result.OwnerOrientations != [2]int{1, relation.ChartOrientation()} {
			t.Fatal("owner orientation")
		}
		// A single projected rectangle cannot prove this full quotient annulus.
		if legacy, e := relation.ArrangeRectanglesRational(ctx, a, ownerB, WrappedRectangleOverlapOptions{}); e == nil || legacy.Complete {
			t.Fatal("general arrangement silently broadened")
		}
		for _, options := range []WrappedCylinderTwoOverlapOptions{{WrappedRectangleOverlapOptions: WrappedRectangleOverlapOptions{MaxComponents: 1}}, {WrappedRectangleOverlapOptions: WrappedRectangleOverlapOptions{MaxOperations: result.ExactOperations - 1}}} {
			out, e := relation.CertifyCylinderTwoRectangleOverlap(ctx, a, ownerB, options)
			if e == nil || out.Complete || len(out.Components) != 0 {
				t.Fatal("missing occurrence/partial quota accepted", out, e)
			}
		}
	}
	relation, _, e := CompareWrappedSurfaceCharts(ctx, w, target, ExactSurfaceComparisonOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for name, ar := range map[string][2][2]*big.Rat{"repeated full period": rect(big.NewRat(0, 1), big.NewRat(4, 1), big.NewRat(-1, 1), big.NewRat(2, 1)), "only one branch": rect(big.NewRat(2, 1), big.NewRat(3, 1), big.NewRat(-1, 1), big.NewRat(2, 1))} {
		out, e := relation.CertifyCylinderTwoRectangleOverlap(ctx, ar, b, WrappedCylinderTwoOverlapOptions{})
		if e == nil || out.Complete {
			t.Fatal(name, out, e)
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if out, e := relation.CertifyCylinderTwoRectangleOverlap(canceled, a, b, WrappedCylinderTwoOverlapOptions{}); e == nil || out.Complete {
		t.Fatal(out, e)
	}
	collapsed, _, e := w.Transformed(ctx, Scaling(Vec3{X: 1, Z: 1}), WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	relation, _, e = CompareWrappedSurfaceCharts(ctx, collapsed, collapsed, ExactSurfaceComparisonOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if out, e := relation.CertifyCylinderTwoRectangleOverlap(ctx, a, b, WrappedCylinderTwoOverlapOptions{}); e == nil || out.Complete {
		t.Fatal("unembedded quotient accepted", out, e)
	}
}

func TestWrappedCylinderTwoOverlapSwappedOwnerAndContacts(t *testing.T) {
	ctx := context.Background()
	p := wrappedCylinderPatch(t)
	a, _, e := NewWrappedSurfacePatch(ctx, p, WrappedSurfaceOptions{WrapU: true})
	if e != nil {
		t.Fatal(e)
	}
	spline, e := p.ToBSplineSurface()
	if e != nil {
		t.Fatal(e)
	}
	spline, e = spline.Transposed()
	if e != nil {
		t.Fatal(e)
	}
	transposed, e := NewSurfacePatch(spline, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	b, _, e := NewWrappedSurfacePatch(ctx, transposed, WrappedSurfaceOptions{WrapV: true})
	if e != nil {
		t.Fatal(e)
	}
	relation, _, e := CompareWrappedSurfaceCharts(ctx, a, b, ExactSurfaceComparisonOptions{SwapB: true})
	if e != nil {
		t.Fatal(e)
	}
	ar := [2][2]*big.Rat{{big.NewRat(0, 1), big.NewRat(3, 1)}, {big.NewRat(-1, 1), big.NewRat(2, 1)}}
	br := [2][2]*big.Rat{{big.NewRat(0, 1), big.NewRat(3, 1)}, {big.NewRat(2, 1), big.NewRat(5, 1)}}
	result, e := relation.CertifyCylinderTwoRectangleOverlap(ctx, ar, br, WrappedCylinderTwoOverlapOptions{})
	if e != nil || !result.Complete || result.OwnerOrientations != [2]int{1, -1} {
		t.Fatal(result, e)
	}
	for _, c := range result.Components {
		for i, uv := range c.ALoop {
			bv := c.BLoop[i]
			if uv[1].Cmp(bv[0]) != 0 {
				t.Fatal("wrong swapped axial owner")
			}
			expected := new(big.Rat).Add(bv[1], new(big.Rat).Mul(new(big.Rat).SetInt(c.Overlap.TargetPeriodShift[1]), big.NewRat(4, 1)))
			if expected.Cmp(uv[0]) != 0 {
				t.Fatal("wrong swapped periodic owner")
			}
		}
	}
	ar[0][1] = big.NewRat(2, 1)
	br[1][1] = big.NewRat(4, 1)
	// Two line contacts are not two positive-area components.
	if out, e := relation.CertifyCylinderTwoRectangleOverlap(ctx, ar, br, WrappedCylinderTwoOverlapOptions{}); e == nil || out.Complete {
		t.Fatal("contact-only occurrences accepted", out, e)
	}
}
