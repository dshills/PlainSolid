//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// FilletedFrustumNearestCircle certifies that EVERY point
// Center+Radius*(U*cos(theta)+V*sin(theta)) is a globally nearest retained
// boundary point. U,V are exact orthonormal vectors. This is a full circular
// subset of minimizers, not necessarily the entire nearest locus: another
// original face can tie. Face identifies the original periodic support owner;
// the creating method specifies the support family.
type FilletedFrustumNearestCircle struct {
	Center, U, V            [3]*big.Rat
	Radius, SquaredDistance *big.Rat
	Face                    int
}

// FilletedFrustumAxisCircleDistance retains a conservative all-face baseline.
// Circle is nonnil only for a proved continuum of nearest contacts. Nil means
// this bounded axis certificate did not apply, not unique nearest contact.
type FilletedFrustumAxisCircleDistance struct {
	Baseline FilletedFrustumDistance
	Circle   *FilletedFrustumNearestCircle
}

// BoundBoreAxisNearestCircleRational performs one bounded all-support query,
// then at most16 exact axis/height tests on immutable native cuts. Strict axial
// interior and the original disjoint-cut proof retain the complete circle;
// separation against every other face establishes its global nearest status.
// Cap/floor equalities, closer faces and off-axis queries retain only Baseline.
func (q FilletedFrustumMaterial) BoundBoreAxisNearestCircleRational(ctx context.Context, p [3]*big.Rat, o FilletedFrustumDistanceOptions) (out FilletedFrustumAxisCircleDistance, err error) {
	defer func() {
		if err != nil {
			out = FilletedFrustumAxisCircleDistance{}
		}
	}()
	baseline, e := q.BoundBoundaryDistanceRational(ctx, p, o)
	if e != nil {
		return out, e
	}
	out.Baseline = baseline
	for _, c := range q.cuts {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		if p[0].Cmp(c.center[0]) != 0 || p[1].Cmp(c.center[1]) != 0 || p[2].Cmp(c.z[0]) <= 0 || p[2].Cmp(c.z[1]) >= 0 {
			continue
		}
		d := fdSquare(c.radius)
		if !baseline.Exact || baseline.Lower.Cmp(d) != 0 {
			continue
		}
		separated := true
		for _, f := range baseline.PerFace {
			if f.Face != c.wall && f.SquaredLower.Cmp(d) < 0 {
				separated = false
				break
			}
		}
		if !separated {
			continue
		}
		out.Circle = &FilletedFrustumNearestCircle{Center: [3]*big.Rat{new(big.Rat).Set(p[0]), new(big.Rat).Set(p[1]), new(big.Rat).Set(p[2])}, U: [3]*big.Rat{big.NewRat(1, 1), new(big.Rat), new(big.Rat)}, V: [3]*big.Rat{new(big.Rat), big.NewRat(1, 1), new(big.Rat)}, Radius: new(big.Rat).Set(c.radius), SquaredDistance: d, Face: c.wall}
		return out, nil
	}
	return out, nil
}

// FilletedFrustumOccurrenceAxisCircleDistance keeps its baseline explicitly
// in the original frame. WorldCircle describes the whole placed continuum.
type FilletedFrustumOccurrenceAxisCircleDistance struct {
	Original    FilletedFrustumAxisCircleDistance
	WorldCircle *FilletedFrustumNearestCircle
}

// BoundBoreAxisNearestCircleRational applies one original query and transforms
// exact circle center/basis through the proper rational rigid pose. No sampled
// azimuths or rounded support reconstruction are used. Errors are atomic.
func (q FilletedFrustumOccurrence) BoundBoreAxisNearestCircleRational(ctx context.Context, p [3]*big.Rat, o FilletedFrustumDistanceOptions) (out FilletedFrustumOccurrenceAxisCircleDistance, err error) {
	defer func() {
		if err != nil {
			out = FilletedFrustumOccurrenceAxisCircleDistance{}
		}
	}()
	if q.source.recipe == nil {
		return out, geom.ErrInvalid
	}
	local, e := q.pose.TransformPoint(ctx, p, true, 4096)
	if e != nil {
		return out, e
	}
	original, e := q.source.BoundBoreAxisNearestCircleRational(ctx, local, o)
	if e != nil {
		return out, e
	}
	out.Original = original
	if original.Circle == nil {
		return out, nil
	}
	c := original.Circle
	center, e := q.pose.TransformPoint(ctx, c.Center, false, 4096)
	if e != nil {
		return out, e
	}
	u, e := q.pose.TransformVector(ctx, c.U, false, 4096)
	if e != nil {
		return out, e
	}
	v, e := q.pose.TransformVector(ctx, c.V, false, 4096)
	if e != nil {
		return out, e
	}
	out.WorldCircle = &FilletedFrustumNearestCircle{Center: center, U: u, V: v, Radius: new(big.Rat).Set(c.Radius), SquaredDistance: new(big.Rat).Set(c.SquaredDistance), Face: c.Face}
	return out, nil
}
