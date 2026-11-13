package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/geom"
)

// HelicalOverlapEndReliefs opens both ends of a phase-staggered threaded tube. Entry
// coefficients include both disjoint axial relief regions, with groove overlap
// subtracted exactly. Coefficients describe ideal unit-frame geometry.
type HelicalOverlapEndReliefs struct {
	HelicalOverlapRelief
	UpperReliefWall, UpperShoulder int
}

// GrooveWithPhasedEndReliefs qualifies tube stock with one widened radius at
// both ends. Each relief passes the complete corresponding groove end profile;
// a nonempty grooved interval remains. The original wall can split into several
// angular strips, each retaining the same analytic cylindrical support.
func GrooveWithPhasedEndReliefs(ctx context.Context, r HelicalGrooveRecipe, initialPhase, reliefRadius, lowerDepth, upperDepth, maxChartError float64) (HelicalOverlapEndReliefs, error) {
	fail := func(e error) (HelicalOverlapEndReliefs, error) { return HelicalOverlapEndReliefs{}, e }
	if ctx == nil || r.BoreRadius <= 0 || !finite(reliefRadius) || !finite(lowerDepth) || !finite(upperDepth) || lowerDepth <= 0 || upperDepth <= 0 {
		return fail(geom.ErrInvalid)
	}
	original, proof, e := GrooveWithPhasedStarts(ctx, r, initialPhase, maxChartError)
	if e != nil {
		return fail(e)
	}
	pad := 8 * original.tolerance
	loR, hiR := math.Inf(1), math.Inf(-1)
	frontEnd, rearStart := math.Inf(-1), math.Inf(1)
	for _, p := range r.Profile {
		loR = min(loR, p.X)
		hiR = max(hiR, p.X)
		frontEnd = max(frontEnd, p.Y+r.Lead*r.Start)
		rearStart = min(rearStart, p.Y+r.Lead*r.End)
	}
	if reliefRadius <= hiR+pad || reliefRadius >= r.OuterRadius-pad || lowerDepth <= frontEnd+pad || r.Height-upperDepth >= rearStart-pad || lowerDepth+upperDepth >= r.Height-pad {
		return fail(geom.ErrDomain)
	}
	for _, p := range original.vertices {
		z := r.Frame.ToLocal(p).Z
		if math.Abs(z-lowerDepth) <= pad || math.Abs(z-(r.Height-upperDepth)) <= pad {
			return fail(geom.ErrDomain)
		}
	}
	lower, e := clipHelicalEntry(ctx, original, r, lowerDepth)
	if e != nil {
		return fail(e)
	}
	// This private open shell is consumed only by the second clipper. It is
	// never exposed or measured; both reliefs are sewn before NewSolid validates it.
	open := Solid{vertices: lower.vertices, edges: lower.edges, faces: lower.faces, tolerance: original.tolerance}
	upperRecipe := r
	upperRecipe.Frame, e = geom.NewFrame(r.Frame.ToWorld(geom.Vec3{Z: r.Height}), r.Frame.XAxis(), r.Frame.ZAxis().Scale(-1))
	if e != nil {
		return fail(e)
	}
	upper, e := clipHelicalEntryAxis(ctx, open, upperRecipe, upperDepth, true)
	if e != nil {
		return fail(e)
	}
	upperWall, upperShoulder, e := upper.appendEntryRelief(reliefRadius)
	if e != nil {
		return fail(e)
	}
	upper.r, upper.depth = r, lowerDepth
	wall, shoulder, e := upper.appendEntryRelief(reliefRadius)
	if e != nil {
		return fail(e)
	}
	solid, e := upper.solid()
	if e != nil {
		return fail(e)
	}
	groove, below, e := helicalOverlapVolume(r, lowerDepth)
	if e != nil {
		return fail(e)
	}
	_, belowUpper, e := helicalOverlapVolumeExactDepth(r, new(big.Rat).Sub(ratInset(r.Height), ratInset(upperDepth)))
	if e != nil {
		return fail(e)
	}
	above := new(big.Rat).Sub(new(big.Rat).Set(groove), belowUpper)
	starts := big.NewRat(int64(r.Starts), 1)
	groove.Mul(groove, starts)
	below.Mul(below, starts)
	above.Mul(above, starts)
	annulus := new(big.Rat).Sub(new(big.Rat).Mul(ratInset(reliefRadius), ratInset(reliefRadius)), new(big.Rat).Mul(ratInset(r.BoreRadius), ratInset(r.BoreRadius)))
	depths := new(big.Rat).Add(ratInset(lowerDepth), ratInset(upperDepth))
	entry := new(big.Rat).Sub(new(big.Rat).Mul(annulus, depths), new(big.Rat).Add(below, above))
	if entry.Sign() <= 0 {
		return fail(geom.ErrDomain)
	}
	result := HelicalOverlapRelief{solid, original, proof, wall, shoulder, groove, entry}
	return HelicalOverlapEndReliefs{result, upperWall, upperShoulder}, nil
}
