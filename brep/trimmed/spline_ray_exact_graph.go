package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// A vertical line has at most one contact with an identity XY graph under a
// placement that preserves vertical lines. Exact projection avoids losing a
// simple root when it lies on a split boundary of the generic isolator.
func exactVerticalGraphFaceRay(ctx context.Context, result *SplineRayResult, face int, f Face, base *geom.BSplineSurface,
	m geom.Affine, line geom.Line, o SplineRayOptions) (bool, error) {
	d, origin := line.Direction(), line.Origin()
	if d.X != 0 || d.Y != 0 || m[0][2] != 0 || m[1][2] != 0 {
		return false, nil
	}
	if err := requireIdentitySplineChart(ctx, base, o.Patch); err != nil {
		return false, ctx.Err()
	}
	a, b, c, e := ratInset(m[0][0]), ratInset(m[0][1]), ratInset(m[1][0]), ratInset(m[1][1])
	det := new(big.Rat).Sub(new(big.Rat).Mul(a, e), new(big.Rat).Mul(b, c))
	if det.Sign() == 0 {
		return false, nil
	}
	x, y := new(big.Rat).Sub(ratInset(origin.X), ratInset(m[0][3])), new(big.Rat).Sub(ratInset(origin.Y), ratInset(m[1][3]))
	u := new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).Mul(e, x), new(big.Rat).Mul(b, y)), det)
	v := new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).Mul(a, y), new(big.Rat).Mul(c, x)), det)
	if u.Sign() < 0 || u.Cmp(big.NewRat(1, 1)) > 0 || v.Sign() < 0 || v.Cmp(big.NewRat(1, 1)) > 0 {
		return true, nil
	}
	p, err := exactFaceSupportPatch(f, o.Patch)
	if err != nil {
		return false, err
	}
	jet, err := p.EvaluateRational(ctx, u, v, geom.SurfacePatchEvaluationOptions{
		MaxOperations: o.Patch.MaxOperations, MaxCoefficientBits: o.Patch.MaxCoefficientBits,
	})
	if err != nil {
		return false, err
	}
	t := new(big.Rat).Quo(new(big.Rat).Sub(jet.Point[2], ratInset(origin.Z)), ratInset(d.Z))
	if t.Cmp(ratInset(o.Line.MinParameter)) < 0 || t.Cmp(ratInset(o.Line.MaxParameter)) > 0 {
		return true, nil
	}
	loc, proof, err := LocateRationalInFaceTrim(ctx, f, [2]*big.Rat{u, v}, o.Trim)
	if err != nil {
		return false, err
	}
	if loc == trimvalidate.Outside && proof.Status == trimvalidate.Certified {
		return true, nil
	}
	umin, umax, err := outwardTrimRational(u)
	if err != nil {
		return false, err
	}
	vmin, vmax, err := outwardTrimRational(v)
	if err != nil {
		return false, err
	}
	uv := geom.UVBox{Min: geom.Vec2{X: umin, Y: vmin}, Max: geom.Vec2{X: umax, Y: vmax}}
	if loc != trimvalidate.Inside || proof.Status != trimvalidate.Certified {
		result.Issues = append(result.Issues, SplineRayIssue{Face: face, Domain: p.Domain(), UV: uv, Reason: "exact root on unresolved trim or shared boundary"})
		return true, nil
	}
	tmin, tmax, err := outwardTrimRational(t)
	if err != nil {
		return false, err
	}
	zmin, zmax, err := outwardTrimRational(jet.Point[2])
	if err != nil {
		return false, err
	}
	sign := det.Sign() * ratInset(d.Z).Sign()
	if f.Reversed {
		sign = -sign
	}
	result.Hits = append(result.Hits, SplineRayHit{Face: face, Direction: sign, LineSplineSurfaceHit: geom.LineSplineSurfaceHit{
		Cell: p.Domain(), LineRange: geom.ParameterSpan{Start: o.Line.MinParameter, End: o.Line.MaxParameter},
		UV: uv, ParameterMin: tmin, ParameterMax: tmax,
		Bounds: geom.Box{Min: geom.Vec3{X: origin.X, Y: origin.Y, Z: zmin}, Max: geom.Vec3{X: origin.X, Y: origin.Y, Z: zmax}},
	}})
	return true, nil
}
