//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// AlgebraicLensLineOptions controls exact support composition and root order.
// MaxEvents bounds the total number of distinct isolated events.
type AlgebraicLensLineOptions struct {
	Composition geom.SurfaceCurveAgreementOptions
	Roots       geom.ExactCurvePlaneEventOptions
	MaxEvents   int
}

type AlgebraicLensLineEvent struct {
	Parameter    *geom.ExactAlgebraicRoot // original signed line units
	BoundaryMask uint8                    // 1 upper, 2 lower, 4 conic rim, 8 planar cap, 16 query ends
}
type AlgebraicLensLineInterval struct {
	Start, End   AlgebraicLensLineEvent
	BoundaryMask uint8 // owners that coincide along the entire interval
}
type AlgebraicLensLineSection struct {
	Intervals []AlgebraicLensLineInterval
	Isolated  []AlgebraicLensLineEvent
	Complete  bool
}

type lensLineRoot struct {
	root *geom.ExactAlgebraicRoot
	mask uint8
}

// ClipMaterialLine computes the closed intersection of a finite represented
// model line and one capped lens half. Exact conic, upper/lower spline and
// cap events partition the line; interval and singleton membership use
// exact signs. The line direction is not normalized.
func (s AlgebraicLensPlaneSplit) ClipMaterialLine(ctx context.Context, side int, origin, direction geom.Vec3,
	limits [2]*big.Rat, o AlgebraicLensLineOptions) (AlgebraicLensLineSection, error) {
	var empty AlgebraicLensLineSection
	if ctx == nil || side != 1 && side != -1 || !origin.IsFinite() || !direction.IsFinite() || direction == (geom.Vec3{}) ||
		limits[0] == nil || limits[1] == nil || limits[0].Cmp(limits[1]) > 0 || s.contact[0] == nil || len(s.source.curves) != 2 || o.MaxEvents < 0 || o.MaxEvents > 4096 {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	if o.MaxEvents == 0 {
		o.MaxEvents = 256
	}
	if o.MaxEvents < 2 {
		return empty, geom.ErrConvergence
	}
	if o.Roots.MaxOperations == 0 {
		o.Roots.MaxOperations = 8000000
	}
	if o.Roots.MaxSubdivisions == 0 {
		o.Roots.MaxSubdivisions = 8192
	}
	if o.Roots.MaxOperations < 1 || o.Roots.MaxSubdivisions < 1 {
		return empty, geom.ErrInvalid
	}
	for _, q := range limits {
		if q.Num().BitLen() > 4096 || q.Denom().BitLen() > 4096 {
			return empty, geom.ErrConvergence
		}
	}
	x, d := [3]*big.Rat{ratInset(origin.X), ratInset(origin.Y), ratInset(origin.Z)}, [3]*big.Rat{ratInset(direction.X), ratInset(direction.Y), ratInset(direction.Z)}
	lo, hi := new(big.Rat).Set(limits[0]), new(big.Rat).Set(limits[1])
	one, zero := big.NewRat(1, 1), new(big.Rat)
	at := func(t *big.Rat) (r [3]*big.Rat) {
		for i := range r {
			r[i] = new(big.Rat).Add(x[i], new(big.Rat).Mul(t, d[i]))
		}
		return
	}
	for i := 0; i < 2; i++ {
		if d[i].Sign() == 0 {
			if x[i].Sign() < 0 || x[i].Cmp(one) > 0 {
				return AlgebraicLensLineSection{Complete: true}, nil
			}
			continue
		}
		a := new(big.Rat).Quo(new(big.Rat).Neg(x[i]), d[i])
		b := new(big.Rat).Quo(new(big.Rat).Sub(one, x[i]), d[i])
		if a.Cmp(b) > 0 {
			a, b = b, a
		}
		lo = maxRat(lo, a)
		hi = minRat(hi, b)
	}
	if lo.Cmp(hi) > 0 {
		return AlgebraicLensLineSection{Complete: true}, nil
	}
	linearRoot := func(v *big.Rat) (*geom.ExactAlgebraicRoot, error) {
		return geom.ImportExactRoot(ctx, geom.ExactRootRecipe{Version: 1, Coefficients: []string{new(big.Rat).Neg(v).RatString(), "1"}, Lower: v.RatString(), Upper: v.RatString()},
			geom.ExactRootCodecOptions{ExactCurvePlaneEventOptions: o.Roots})
	}
	if lo.Cmp(hi) == 0 {
		q, e := s.LocateRational(ctx, side, at(lo), o.Composition.SurfacePatchOptions)
		if e != nil {
			return empty, e
		}
		r := AlgebraicLensLineSection{Complete: true}
		if q != parametric.Outside {
			root, e := linearRoot(lo)
			if e != nil {
				return empty, e
			}
			p := at(lo)
			h, _, e := s.VerticalSection(ctx, side, [2]*big.Rat{p[0], p[1]}, o.Composition.SurfacePatchOptions)
			if e != nil {
				return empty, e
			}
			c, v := s.sectionSigns([2]*big.Rat{p[0], p[1]})
			mask := uint8(16)
			if p[2].Cmp(h[1]) == 0 {
				mask |= 1
			}
			if p[2].Cmp(h[0]) == 0 {
				mask |= 2
			}
			if c == 0 {
				mask |= 4
			}
			if v == 0 {
				mask |= 8
			}
			r.Isolated = append(r.Isolated, AlgebraicLensLineEvent{root, mask})
		}
		return r, nil
	}
	width := new(big.Rat).Sub(hi, lo)
	start, end := at(lo), at(hi)
	net := [][4]*big.Rat{{start[0], start[1], start[2], one}, {end[0], end[1], end[2], one}}
	curve, e := geom.NewExactHomogeneousBezier(ctx, net, o.Composition.SurfacePatchOptions)
	if e != nil {
		return empty, e
	}
	loRoot, e := linearRoot(zero)
	if e != nil {
		return empty, e
	}
	hiRoot, e := linearRoot(one)
	if e != nil {
		return empty, e
	}
	rows := []lensLineRoot{{root: loRoot, mask: 16}, {root: hiRoot, mask: 16}}
	spend := func(ops, sub int) { o.Roots.MaxOperations -= ops; o.Roots.MaxSubdivisions -= sub }
	compare := func(a, b *geom.ExactAlgebraicRoot) (geom.ExactCurveEventOrder, error) {
		if o.Roots.MaxOperations < 1 || o.Roots.MaxSubdivisions < 1 {
			return geom.ExactCurveEventOrder{}, geom.ErrConvergence
		}
		q, e := a.Compare(ctx, b, o.Roots)
		spend(q.Operations, q.Subdivisions)
		if e == nil && !q.Proven {
			e = geom.ErrConvergence
		}
		return q, e
	}
	add := func(root *geom.ExactAlgebraicRoot, owner int) error {
		pos := len(rows)
		for pos > 0 {
			q, e := compare(rows[pos-1].root, root)
			if e != nil {
				return e
			}
			if q.Order < 0 {
				break
			}
			if q.Order == 0 {
				rows[pos-1].mask |= 1 << owner
				return nil
			}
			pos--
		}
		if len(rows) >= o.MaxEvents {
			return geom.ErrConvergence
		}
		rows = append(rows, lensLineRoot{})
		copy(rows[pos+1:], rows[pos:])
		rows[pos] = lensLineRoot{root: root, mask: 1 << owner}
		return nil
	}
	var coincident [4][][2]*geom.ExactAlgebraicRoot
	var patches [2]*geom.SurfacePatch
	if o.Composition.MaxOperations == 0 {
		o.Composition.MaxOperations = 2000000
	}
	if o.Composition.MaxSubdivisions == 0 {
		o.Composition.MaxSubdivisions = 8192
	}
	for i, f := range s.source.subdivision.source.faces {
		p, e := exactFaceSupportPatch(f, o.Composition.SurfacePatchOptions)
		if e != nil {
			return empty, e
		}
		patches[i] = p
		if o.Roots.MaxOperations < 1 || o.Roots.MaxSubdivisions < 1 {
			return empty, geom.ErrConvergence
		}
		co := o.Composition
		co.MaxOperations = min(co.MaxOperations, o.Roots.MaxOperations)
		co.MaxSubdivisions = min(co.MaxSubdivisions, o.Roots.MaxSubdivisions)
		r, e := geom.IntersectExactCurveIdentityGraph(ctx, curve, p, co)
		spend(r.ExactOperations, r.Subdivisions)
		if e != nil {
			return empty, e
		}
		if !r.Complete {
			return empty, geom.ErrConvergence
		}
		for _, c := range r.Cells {
			if c.Intersection.Coincident {
				z := [2]*geom.ExactAlgebraicRoot{c.Intersection.Boundary[0].Root(), c.Intersection.Boundary[1].Root()}
				coincident[i] = append(coincident[i], z)
				for _, root := range z {
					if e := add(root, i); e != nil {
						return empty, e
					}
				}
			}
			for _, ev := range c.Intersection.Events {
				if e := add(ev.Root(), i); e != nil {
					return empty, e
				}
			}
		}
	}
	conicValue := func(p [3]*big.Rat) *big.Rat {
		q := s.contact
		x, y := p[0], p[1]
		r := new(big.Rat).Set(q[0])
		for i, t := range []*big.Rat{x, new(big.Rat).Mul(x, x), y, new(big.Rat).Mul(y, y), new(big.Rat).Mul(x, y)} {
			r.Add(r, new(big.Rat).Mul(q[i+1], t))
		}
		return r
	}
	line := s.source.curves[0].Partition.Line
	planeValue := func(p [3]*big.Rat) *big.Rat {
		return new(big.Rat).Add(new(big.Rat).Add(new(big.Rat).Mul(line[0], p[0]), new(big.Rat).Mul(line[1], p[1])), line[2])
	}
	for owner, value := range []func([3]*big.Rat) *big.Rat{conicValue, planeValue} {
		a, b := value(start), value(end)
		m := value(at(new(big.Rat).Quo(new(big.Rat).Add(lo, hi), big.NewRat(2, 1))))
		m.Sub(new(big.Rat).Mul(big.NewRat(2, 1), m), new(big.Rat).Quo(new(big.Rat).Add(a, b), big.NewRat(2, 1)))
		c, e := geom.NewExactHomogeneousBezier(ctx, [][4]*big.Rat{{zero, zero, a, one}, {zero, zero, m, one}, {zero, zero, b, one}}, o.Composition.SurfacePatchOptions)
		if e != nil {
			return empty, e
		}
		cell, e := geom.NewExactComposedBezierCell(ctx, c, o.Composition.SurfacePatchOptions)
		if e != nil {
			return empty, e
		}
		if o.Roots.MaxOperations < 1 || o.Roots.MaxSubdivisions < 1 {
			return empty, geom.ErrConvergence
		}
		r, e := cell.IntersectRawPlane(ctx, [4]*big.Rat{nil, nil, one, nil}, o.Roots)
		spend(r.Operations, r.Subdivisions)
		if e != nil {
			return empty, e
		}
		if !r.Complete {
			return empty, geom.ErrConvergence
		}
		if r.Coincident {
			coincident[owner+2] = append(coincident[owner+2], [2]*geom.ExactAlgebraicRoot{loRoot, hiRoot})
		}
		for _, ev := range r.Events {
			if e := add(ev.Root(), owner+2); e != nil {
				return empty, e
			}
		}
	}
	inside := func(v [4]int) bool { return v[0] <= 0 && v[1] >= 0 && v[2] <= 0 && v[3]*side >= 0 }
	type interval struct {
		signs   [4]int
		mask    uint8
		present bool
	}
	intervals := make([]interval, len(rows)-1)
	for i := range intervals {
		q, e := compare(rows[i].root, rows[i+1].root)
		if e != nil {
			return empty, e
		}
		if q.Order != -1 || q.Intervals[0][1].Cmp(q.Intervals[1][0]) >= 0 {
			return empty, geom.ErrConvergence
		}
		w := new(big.Rat).Quo(new(big.Rat).Add(q.Intervals[0][1], q.Intervals[1][0]), big.NewRat(2, 1))
		p := at(new(big.Rat).Add(lo, new(big.Rat).Mul(width, w)))
		c := &intervals[i]
		c.signs[2], c.signs[3] = s.sectionSigns([2]*big.Rat{p[0], p[1]})
		for j, patch := range patches {
			if o.Roots.MaxOperations < 1 {
				return empty, geom.ErrConvergence
			}
			x, e := patch.EvaluateRational(ctx, p[0], p[1], geom.SurfacePatchEvaluationOptions{MaxOperations: o.Roots.MaxOperations, MaxCoefficientBits: o.Roots.MaxCoefficientBits})
			spend(x.ExactOperations, 0)
			if e != nil {
				return empty, e
			}
			c.signs[j] = p[2].Cmp(x.Point[2])
		}
		for j, sign := range c.signs {
			if sign != 0 {
				continue
			}
			proved := false
			for _, span := range coincident[j] {
				l, e := compare(span[0], rows[i].root)
				if e != nil {
					return empty, e
				}
				if l.Order > 0 {
					continue
				}
				h, e := compare(rows[i+1].root, span[1])
				if e != nil {
					return empty, e
				}
				if h.Order <= 0 {
					proved = true
					break
				}
			}
			if !proved {
				return empty, geom.ErrConvergence
			}
			c.mask |= 1 << j
		}
		c.present = inside(c.signs)
	}
	events := make([]AlgebraicLensLineEvent, len(rows))
	for i, r := range rows {
		if o.Roots.MaxOperations < 1 {
			return empty, geom.ErrConvergence
		}
		p, n, e := r.root.AffineMap(ctx, lo, width, o.Roots)
		spend(n, 0)
		if e != nil {
			return empty, e
		}
		mask := r.mask
		if i > 0 {
			mask |= intervals[i-1].mask
		}
		if i < len(intervals) {
			mask |= intervals[i].mask
		}
		events[i] = AlgebraicLensLineEvent{p, mask}
	}
	var out AlgebraicLensLineSection
	for i, c := range intervals {
		if c.present {
			out.Intervals = append(out.Intervals, AlgebraicLensLineInterval{events[i], events[i+1], c.mask})
		}
	}
	for i, e := range events {
		if i > 0 && intervals[i-1].present || i < len(intervals) && intervals[i].present {
			continue
		}
		near := min(i, len(intervals)-1)
		signs := intervals[near].signs
		for j := range signs {
			if e.BoundaryMask&(1<<j) != 0 {
				signs[j] = 0
			}
		}
		if inside(signs) {
			out.Isolated = append(out.Isolated, e)
		}
	}
	out.Complete = true
	return out, nil
}
