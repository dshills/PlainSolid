package parametric

import (
	"context"
	"math"
	"math/big"

	"plainsolid/geom"
)

// ObliqueCylinderCommonResult preserves the two original cylindrical supports
// and the exact proof that no finite cap truncates the intersection.
type ObliqueCylinderCommonResult struct {
	Solid         Solid
	CylinderFaces [2][]int
	CapCoverage   geom.CylinderCommonCaps
}

// ObliqueCylinderCommon assembles two cylinders with intersecting raw axes.
// Their radii and raw axis norms may differ. The finite caps must strictly
// clear the full common, which is verified without normalizing either axis.
func ObliqueCylinderCommon(ctx context.Context, center, u, v geom.Vec3, ru, rv float64, uCaps, vCaps [2]float64, tolerance float64) (ObliqueCylinderCommonResult, error) {
	var empty ObliqueCylinderCommonResult
	if ctx == nil || !finite(tolerance) || tolerance <= 0 || !center.IsFinite() ||
		!finite(ru) || !finite(rv) || ru <= 0 || rv <= 0 {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	w := [2][2]*big.Rat{}
	for i, c := range [2][2]float64{uCaps, vCaps} {
		if !finite(c[0]) || !finite(c[1]) || c[0] >= c[1] {
			return empty, geom.ErrInvalid
		}
		w[i] = [2]*big.Rat{q(c[0]), q(c[1])}
	}
	return obliqueCylinderCommonExactWindows(ctx, center, u, v, ru, rv, w, tolerance)
}

// Exact raw windows can be non-dyadic when measured from the common axis foot.
// The untrimmed common only needs their cap-clearance proof, never float caps.
func obliqueCylinderCommonExactWindows(ctx context.Context, center, u, v geom.Vec3, ru, rv float64, w [2][2]*big.Rat, tolerance float64) (ObliqueCylinderCommonResult, error) {
	var empty ObliqueCylinderCommonResult
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	proof, e := geom.ClassifyCylinderCommonCaps(ctx, u, v, ru, rv, &w[0], &w[1], geom.SurfacePatchEvaluationOptions{})
	if e != nil {
		return empty, e
	}
	pad := new(big.Rat).Mul(big.NewRat(8, 1), q(tolerance))
	gap := new(big.Rat).Sub(q(max(ru, rv)), q(min(ru, rv)))
	if !proof.Covers || proof.TouchMask != 0 || q(min(ru, rv)).Cmp(pad) <= 0 || gap.Sign() > 0 && gap.Cmp(pad) <= 0 {
		return empty, geom.ErrConvergence
	}
	walls, e := geom.NewCylinderCommonWalls(ctx, center, u, v, ru, rv, geom.SurfacePatchEvaluationOptions{})
	if e != nil {
		return empty, e
	}
	out := ObliqueCylinderCommonResult{CapCoverage: proof}
	var faces []Face
	for i, s := range walls {
		owner, _, reverse := s.Ownership()
		out.CylinderFaces[owner] = append(out.CylinderFaces[owner], i)
		faces = append(faces, Face{Surface: generalCylinderCommonChart{s, center, u, v, ru, rv, i}, U: Interval{-math.Pi / 2, math.Pi / 2}, V: Interval{-1, 1}, Reversed: reverse})
	}
	out.Solid, e = joinPrimitivePatches(faces, tolerance)
	if e != nil {
		return empty, e
	}
	if e = ctx.Err(); e != nil {
		return empty, e
	}
	return out, nil
}

type generalCylinderCommonChart struct {
	geom.CylinderCommonWall
	center, u, v geom.Vec3
	ru, rv       float64
	index        int
}

func (s generalCylinderCommonChart) secondDerivativeBounds(ctx context.Context, u, v Interval) (float64, float64, float64, error) {
	if e := ctx.Err(); e != nil {
		return 0, 0, 0, e
	}
	if u.Min < -math.Pi/2 || u.Max > math.Pi/2 || v.Min < -1 || v.Max > 1 {
		return 0, 0, 0, geom.ErrDomain
	}
	b, e := s.HessianBounds()
	if e != nil {
		return 0, 0, 0, e
	}
	return upperBoreRat(b.UU), upperBoreRat(b.UV), upperBoreRat(b.VV), nil
}
