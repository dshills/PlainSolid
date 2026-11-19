package trimmed

import (
	"context"
	"math/big"
	"runtime"

	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// Exact native Bézier uses have a separate trim proof path. Ordinary spline
// evaluation alone is not a substitute for the original homogeneous controls.
func hasNativeExactSplineTrim(f Face) bool {
	for _, ring := range append([][]Use{f.Outer}, f.Holes...) {
		for _, u := range ring {
			switch u.Curve.(type) {
			case ExactQuadraticUV, ExactBezierUV:
				return true
			}
		}
	}
	return false
}

// SplineFaceLineReport retains all proved interior contacts, excluded
// outside-trim contacts, and unresolved contacts. Complete is true only
// when the support solver and every trim membership proof are exhaustive.
type SplineFaceLineReport struct {
	Face       int
	Hits       []geom.LineSplineSurfaceHit
	Excluded   []geom.LineSplineSurfaceHit
	Unresolved []geom.LineSplineSurfaceHit
	Support    geom.LineSplineSurfaceResult
	Complete   bool
}

type SplineFaceLineOptions struct {
	Support geom.LineSplineSurfaceOptions
	Patch   geom.SurfacePatchOptions
	Trim    trimvalidate.Options
	// MaxTrimBoxes bounds rational Bézier control-hull subdivision
	// for each candidate root box. Zero selects 256.
	MaxTrimBoxes int
}

// IntersectSplineFaceLine finds regular finite line contacts with one
// trimmed spline face. Each support contact's entire enclosing UV box is
// classified against exact trim loops. A box reaching a trim boundary
// remains unresolved; a rounded UV midpoint never decides membership.
func (s Solid) IntersectSplineFaceLine(ctx context.Context, face int, line geom.Line, o SplineFaceLineOptions) (SplineFaceLineReport, error) {
	out := SplineFaceLineReport{Face: face}
	if ctx == nil || face < 0 || face >= len(s.faces) || o.MaxTrimBoxes < 0 || o.MaxTrimBoxes > 65536 {
		return out, geom.ErrInvalid
	}
	if o.MaxTrimBoxes == 0 {
		o.MaxTrimBoxes = 256
	}
	f := s.faces[face]
	p, e := exactFaceSupportPatch(f, o.Patch)
	if e != nil {
		return out, e
	}
	support, e := geom.IntersectLineSplineSurfaceContext(ctx, line, p, o.Support)
	out.Support = support
	if e != nil && e != geom.ErrConvergence {
		return out, e
	}
	for _, h := range support.Hits {
		loc, proof, e := classifySplineFaceRootBox(ctx, f, h.UV, o.Trim, o.MaxTrimBoxes)
		if e != nil {
			return out, e
		}
		if !proof {
			out.Unresolved = append(out.Unresolved, h)
		} else if loc == trimvalidate.Outside {
			out.Excluded = append(out.Excluded, h)
		} else {
			out.Hits = append(out.Hits, h)
		}
	}
	out.Complete = support.Complete && len(out.Unresolved) == 0
	if !out.Complete {
		return out, geom.ErrConvergence
	}
	return out, nil
}

func classifySplineFaceRootBox(ctx context.Context, face Face, box geom.UVBox, o trimvalidate.Options, trimBoxes int) (trimvalidate.Location, bool, error) {
	proof, e := ValidateFaceTrimLoops(ctx, face, o)
	if e != nil || proof.Status != trimvalidate.Certified {
		return trimvalidate.Unknown, false, e
	}
	return classifyCertifiedSplineFaceBox(ctx, face, box, o, trimBoxes)
}

func classifyCertifiedSplineFaceBox(ctx context.Context, face Face, box geom.UVBox, o trimvalidate.Options, trimBoxes int) (trimvalidate.Location, bool, error) {
	// Exact point boxes can be classified directly, including trim hits.
	if box.Min == box.Max {
		loc, proof, e := LocateInFaceTrim(ctx, face, box.Min, o)
		return loc, proof.Status == trimvalidate.Certified && loc != trimvalidate.Unknown, e
	}
	// A conservative rectangle cannot intersect any p-curve whose full
	// bound is disjoint. Straight edges are clipped exactly to avoid
	// false ambiguity from a diagonal chord's enclosing rectangle.
	for _, ring := range append([][]Use{face.Outer}, face.Holes...) {
		for _, u := range ring {
			if u.Curve == nil {
				if lineCrossesUVBox(u.UVStart, u.UVEnd, box) {
					return trimvalidate.Unknown, false, nil
				}
				continue
			}
			a, b, e := trimBounds(u)
			if e != nil {
				return trimvalidate.Unknown, false, nil
			}
			if b.X >= box.Min.X && a.X <= box.Max.X && b.Y >= box.Min.Y && a.Y <= box.Max.Y {
				var h [][3]*big.Rat
				switch c := u.Curve.(type) {
				case SplineUV:
					var quadratic [3][3]*big.Rat
					quadratic, e = quadraticTrimSourceControls(c)
					if e != nil {
						return trimvalidate.Unknown, false, nil
					}
					h = quadratic[:]
				case ExactQuadraticUV:
					h = c.control[:]
				case ExactBezierUV:
					h = c.control
				default:
					return trimvalidate.Unknown, false, nil
				}
				disjoint, e := bezierTrimExcludesUVBox(ctx, h, box, &trimBoxes)
				if e != nil || !disjoint {
					return trimvalidate.Unknown, false, e
				}
			}
		}
	}
	p := [2]*big.Rat{
		new(big.Rat).Quo(new(big.Rat).Add(ratInset(box.Min.X), ratInset(box.Max.X)), big.NewRat(2, 1)),
		new(big.Rat).Quo(new(big.Rat).Add(ratInset(box.Min.Y), ratInset(box.Max.Y)), big.NewRat(2, 1)),
	}
	loc, proof, e := LocateRationalInFaceTrim(ctx, face, p, o)
	return loc, proof.Status == trimvalidate.Certified && (loc == trimvalidate.Inside || loc == trimvalidate.Outside), e
}

func lineCrossesUVBox(a, b geom.Vec2, r geom.UVBox) bool {
	lo, hi := new(big.Rat), big.NewRat(1, 1)
	start, end := [2]float64{a.X, a.Y}, [2]float64{b.X, b.Y}
	left, right := [2]float64{r.Min.X, r.Min.Y}, [2]float64{r.Max.X, r.Max.Y}
	for i, x := range start {
		d := new(big.Rat).Sub(ratInset(end[i]), ratInset(x))
		if d.Sign() == 0 {
			if x < left[i] || x > right[i] {
				return false
			}
			continue
		}
		l := new(big.Rat).Quo(new(big.Rat).Sub(ratInset(left[i]), ratInset(x)), d)
		h := new(big.Rat).Quo(new(big.Rat).Sub(ratInset(right[i]), ratInset(x)), d)
		if l.Cmp(h) > 0 {
			l, h = h, l
		}
		lo, hi = maxRat(lo, l), minRat(hi, h)
		if lo.Cmp(hi) > 0 {
			return false
		}
	}
	return true
}

func bezierTrimExcludesUVBox(ctx context.Context, h [][3]*big.Rat, box geom.UVBox, left *int) (bool, error) {
	if len(h) < 3 || len(h) > 5 || h[0][0] == nil {
		return false, nil
	}
	type item struct {
		net   [][3]*big.Rat
		depth int
	}
	q := []item{{net: h}}
	limits := [2][2]*big.Rat{{ratInset(box.Min.X), ratInset(box.Max.X)}, {ratInset(box.Min.Y), ratInset(box.Max.Y)}}
	half := big.NewRat(1, 2)
	for len(q) > 0 {
		if e := ctx.Err(); e != nil {
			return false, e
		}
		if *left == 0 {
			return false, nil
		}
		*left--
		x := q[len(q)-1]
		q = q[:len(q)-1]
		excluded := false
		for i, span := range limits {
			below, above := true, true
			for _, v := range x.net {
				below = below && v[i].Cmp(new(big.Rat).Mul(span[0], v[2])) < 0
				above = above && v[i].Cmp(new(big.Rat).Mul(span[1], v[2])) > 0
				runtime.KeepAlive(&v) // golang/go#82026: keeps v in the stack map. Remove once the Go fix is in use.
			}
			excluded = excluded || below || above
		}
		if excluded {
			continue
		}
		if x.depth == 40 {
			return false, nil
		}
		lower, upper := splitHomogeneousUV(x.net, half)
		q = append(q, item{upper, x.depth + 1}, item{lower, x.depth + 1})
	}
	return true, nil
}
