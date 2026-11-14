package geom

import (
	"context"
	"math"
	"math/big"
	"testing"
)

func TestWrappedLiftedRectangleInjectivityAcrossSeam(t *testing.T) {
	ctx := context.Background()
	w, _, e := NewWrappedSurfacePatch(ctx, wrappedCylinderPatch(t), WrappedSurfaceOptions{WrapU: true, ContinuityOrder: 1})
	if e != nil {
		t.Fatal(e)
	}
	opts := SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}}
	for _, turn := range []int{0, -2, 3} {
		d := ParameterRectangle{UMin: float64(4*turn) - .25, UMax: float64(4*turn) + .25, VMin: -1, VMax: 2}
		r, e := w.CertifyLiftedRectangleInjectivity(ctx, d, opts)
		if e != nil || !r.ProvenInjective || r.FundamentalPieces != 2 || r.ProjectionOrientation != 1 {
			t.Fatal(turn, r, e)
		}
		area := new(big.Rat)
		for _, cell := range r.Cells {
			if !cell.Source.StrictPositive {
				t.Fatal("nonpositive source proof")
			}
			width := new(big.Rat).Sub(cell.LiftedDomain[0][1], cell.LiftedDomain[0][0])
			height := new(big.Rat).Sub(cell.LiftedDomain[1][1], cell.LiftedDomain[1][0])
			area.Add(area, new(big.Rat).Mul(width, height))
			sourceBounds := [2][2]float64{{cell.Source.Domain.UMin, cell.Source.Domain.UMax}, {cell.Source.Domain.VMin, cell.Source.Domain.VMax}}
			for axis := 0; axis < 2; axis++ {
				for end := 0; end < 2; end++ {
					want := new(big.Rat).SetFloat64(sourceBounds[axis][end])
					if period := w.period[axis]; period != nil {
						want.Add(want, new(big.Rat).Mul(new(big.Rat).SetInt(cell.PeriodShift[axis]), period))
					}
					if want.Cmp(cell.LiftedDomain[axis][end]) != 0 {
						t.Fatal("lost exact lifted owner")
					}
				}
			}
		}
		if area.Cmp(big.NewRat(3, 2)) != 0 {
			t.Fatal("incomplete rectangular coverage", area)
		}
		for _, u := range []*big.Rat{big.NewRat(int64(32*turn-1), 8), big.NewRat(int64(4*turn), 1), big.NewRat(int64(32*turn+1), 8)} {
			j, e := w.EvaluateRational(ctx, u, big.NewRat(1, 3), SurfacePatchEvaluationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			v := new(big.Rat).Sub(u, big.NewRat(int64(4*turn), 1))
			if v.Sign() < 0 {
				v.Add(v, big.NewRat(4, 1))
			}
			original, e := w.patch.EvaluateRational(ctx, v, big.NewRat(1, 3), SurfacePatchEvaluationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			for axis := 0; axis < 3; axis++ {
				if j.Point[axis].Cmp(original.Point[axis]) != 0 {
					t.Fatal("different model occurrence")
				}
			}
		}
		bad := opts
		bad.MaxOperations = r.ExactOperations - 1
		out, e := w.CertifyLiftedRectangleInjectivity(ctx, d, bad)
		if e == nil || out.ProvenInjective || len(out.Cells) != 0 || out.ExactOperations != 0 {
			t.Fatal("late quota published partial proof", out, e)
		}
	}
	reject := func(d ParameterRectangle, o SurfaceInjectivityOptions) {
		t.Helper()
		r, e := w.CertifyLiftedRectangleInjectivity(ctx, d, o)
		if e == nil || r.ProvenInjective || len(r.Cells) != 0 {
			t.Fatal("accepted unsupported", r, e)
		}
	}
	reject(ParameterRectangle{UMin: -.25, UMax: 3.75, VMin: 0, VMax: 1}, opts)                         // equal endpoint images over a full period
	reject(ParameterRectangle{UMin: -math.SmallestNonzeroFloat64, UMax: .125, VMin: 0, VMax: 1}, opts) // exact 4−2^-1074 source cut cannot be stored as float
	bad := opts
	bad.ProjectionAxes = [2]int{0, 2}
	bad.MaxDepth = 2
	reject(ParameterRectangle{UMin: -.25, UMax: .25, VMin: 0, VMax: 1}, bad)
	bad = opts
	bad.MaxCells = 1
	reject(ParameterRectangle{UMin: -.25, UMax: .25, VMin: 0, VMax: 1}, bad)
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	out, e := w.CertifyLiftedRectangleInjectivity(canceled, ParameterRectangle{UMin: -.25, UMax: .25, VMin: 0, VMax: 1}, opts)
	if e == nil || out.ProvenInjective {
		t.Fatal(out, e)
	}
}

func TestWrappedLiftedRectangleInjectivityBothSeams(t *testing.T) {
	w := locationToroid(t, false)
	d := ParameterRectangle{UMin: -.0625, UMax: .0625, VMin: -.0625, VMax: .0625}
	r, e := w.CertifyLiftedRectangleInjectivity(context.Background(), d, SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}})
	if e != nil || !r.ProvenInjective || r.FundamentalPieces != 4 {
		t.Fatal(r, e)
	}
	area := new(big.Rat)
	shifts := map[[2]int64]bool{}
	for _, c := range r.Cells {
		shifts[[2]int64{c.PeriodShift[0].Int64(), c.PeriodShift[1].Int64()}] = true
		area.Add(area, new(big.Rat).Mul(new(big.Rat).Sub(c.LiftedDomain[0][1], c.LiftedDomain[0][0]), new(big.Rat).Sub(c.LiftedDomain[1][1], c.LiftedDomain[1][0])))
	}
	if len(shifts) != 4 || area.Cmp(big.NewRat(1, 64)) != 0 {
		t.Fatal("incomplete double seam", shifts, area)
	}
}

func TestWrappedLiftedRectangleRejectsFoldedSeam(t *testing.T) {
	// X=u(1-u), Y=v, Z=0 is C0-periodic but folds at its seam: u=t and
	// u=-t have equal images. The two locally injective sides have opposite
	// orientation, so their certificates must never be combined as global proof.
	var poles []Vec3
	for _, x := range []float64{0, .5, 0} {
		for _, y := range []float64{0, 1} {
			poles = append(poles, Vec3{X: x, Y: y})
		}
	}
	s, e := NewBSplineSurface(2, 1, 3, 2, poles, []float64{0, 0, 0, 1, 1, 1}, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	p, e := NewSurfacePatch(s, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	w, _, e := NewWrappedSurfacePatch(context.Background(), p, WrappedSurfaceOptions{WrapU: true})
	if e != nil {
		t.Fatal(e)
	}
	a, _ := w.EvaluateRational(context.Background(), big.NewRat(-1, 8), big.NewRat(1, 2), SurfacePatchEvaluationOptions{})
	b, _ := w.EvaluateRational(context.Background(), big.NewRat(1, 8), big.NewRat(1, 2), SurfacePatchEvaluationOptions{})
	for axis := 0; axis < 3; axis++ {
		if a.Point[axis].Cmp(b.Point[axis]) != 0 {
			t.Fatal("fixture is not overlapping")
		}
	}
	for _, d := range []ParameterRectangle{{UMin: .75, UMax: 1, VMin: 0, VMax: 1}, {UMin: 0, UMax: .25, VMin: 0, VMax: 1}} {
		local, err := p.Trim(d)
		if err != nil {
			t.Fatal(err)
		}
		proof, err := CertifySurfaceInjectivity(local, SurfaceInjectivityOptions{ProjectionAxes: [2]int{0, 1}})
		if err != nil || !proof.ProvenInjective {
			t.Fatal("fixture side is not locally injective", err)
		}
	}
	r, e := w.CertifyLiftedRectangleInjectivity(context.Background(), ParameterRectangle{UMin: -.25, UMax: .25, VMin: 0, VMax: 1}, SurfaceInjectivityOptions{ProjectionAxes: [2]int{0, 1}, MaxDepth: 2})
	if e == nil || r.ProvenInjective || len(r.Cells) != 0 {
		t.Fatal("independent local proofs accepted a fold", r, e)
	}
}

func TestWrappedLiftedRectangleC0DerivativeJump(t *testing.T) {
	profile := []Vec2{{X: 2}, {X: 2, Y: 2}, {Y: 2}, {X: -2, Y: 2}, {X: -2}, {X: -2, Y: -2}, {Y: -2}, {X: 2, Y: -1}, {X: 2}}
	var poles []Vec3
	for _, p := range profile {
		for _, z := range []float64{0, 1} {
			poles = append(poles, Vec3{X: p.X, Y: p.Y, Z: z})
		}
	}
	s, e := NewBSplineSurface(2, 1, 9, 2, poles, []float64{0, 0, 0, 1, 1, 2, 2, 3, 3, 4, 4, 4}, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	p, e := NewSurfacePatch(s, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	w, _, e := NewWrappedSurfacePatch(ctx, p, WrappedSurfaceOptions{WrapU: true})
	if e != nil {
		t.Fatal(e)
	}
	a, e := p.EvaluateRational(ctx, big.NewRat(0, 1), big.NewRat(1, 2), SurfacePatchEvaluationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	b, e := p.EvaluateRational(ctx, big.NewRat(4, 1), big.NewRat(1, 2), SurfacePatchEvaluationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if a.Du[1].Cmp(b.Du[1]) == 0 {
		t.Fatal("fixture has no seam derivative jump")
	}
	result, e := w.CertifyLiftedRectangleInjectivity(ctx, ParameterRectangle{UMin: -.125, UMax: .125, VMin: 0, VMax: 1}, SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}})
	if e != nil || !result.ProvenInjective {
		t.Fatal(result, e)
	}
}

func TestWrappedLiftedRectangleRejectsRankCollapsedSupport(t *testing.T) {
	poles := []Vec3{{}, {}, {X: .5}, {X: .5}, {}, {}}
	s, e := NewBSplineSurface(2, 1, 3, 2, poles, []float64{0, 0, 0, 1, 1, 1}, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	p, e := NewSurfacePatch(s, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	w, _, e := NewWrappedSurfacePatch(ctx, p, WrappedSurfaceOptions{WrapU: true})
	if e != nil {
		t.Fatal(e)
	}
	r, e := w.CertifyLiftedRectangleInjectivity(ctx, ParameterRectangle{UMin: -.25, UMax: .25, VMin: 0, VMax: 1}, SurfaceInjectivityOptions{})
	if e == nil || r.ProvenInjective || len(r.Cells) != 0 {
		t.Fatal("rank-collapsed support accepted", r, e)
	}
}
