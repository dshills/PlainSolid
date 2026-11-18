//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
	"sort"
)

type WrappedVerticalFiberOptions struct {
	geom.SurfacePatchOptions
	MaxEvents int
	// AllowBoundaryIntervals accepts only degree-elevated affine trim spans.
	AllowBoundaryIntervals bool
}

// WrappedVerticalTrimSource retains original loop/edge and its exact normalized
// quadratic parameter. Several source endpoints can own one physical event.
type WrappedVerticalTrimSource struct {
	Loop, Edge int
	Parameter  *big.Rat
}

// WrappedVerticalFiberEvent includes both trim events and clipped endpoints.
// FiniteMask1/2 refers to requested t bounds; SupportMask1/2 to lower/upper Z.
// Location distinguishes an isolated boundary touch from an outside endpoint.
type WrappedVerticalFiberEvent struct {
	Parameter               *big.Rat
	UV                      [2]*big.Rat
	Point                   [3]*big.Rat
	Sources                 []WrappedVerticalTrimSource
	Location                trimvalidate.Location
	FiniteMask, SupportMask uint8
}

// WrappedVerticalFiberInterval is a positive-length retained closed overlap.
// Its open interior is strictly inside the face; endpoints index exact Events.
type WrappedVerticalFiberInterval struct {
	Start, End            *big.Rat
	FirstEvent, LastEvent int
}

// WrappedVerticalBoundarySource retains parameters in increasing line-t order;
// they decrease when the source edge runs against the line. Loop zero is the
// outer boundary; subsequent loops are holes with their original orientation.
type WrappedVerticalBoundarySource struct {
	Loop, Edge int
	Parameters [2]*big.Rat
}

// WrappedVerticalBoundaryInterval lies on the trim, not in the face interior.
// These positive line-length cells have zero two-dimensional material measure.
type WrappedVerticalBoundaryInterval struct {
	Start, End            *big.Rat
	FirstEvent, LastEvent int
	Sources               []WrappedVerticalBoundarySource
}
type WrappedVerticalFiberClassification struct {
	Complete          bool
	SuppliedU, FaceU  *big.Rat
	PeriodShift       *big.Int
	Events            []WrappedVerticalFiberEvent
	Intervals         []WrappedVerticalFiberInterval
	BoundaryIntervals []WrappedVerticalBoundaryInterval
	ExactOperations   int
}

// ClassifyVerticalFiber proves the closed face's intersection with a finite
// vertical model line using an explicit original rational U witness. Physical
// XY is corroborated exactly; global cylinder embedding excludes other lifts.
// Every quadratic trim event must have rational parameter. Coincident trim
// spans reject unless AllowBoundaryIntervals proves an affine source map;
// unrecovered algebraic roots always reject atomically. Exact interval
// witnesses are used only after complete polynomial root enumeration proves
// that each open cell contains no boundary; this is not sampled parity.
// Isolated touches are retained as Boundary Events without a material interval.
func (r *WrappedQuadraticFace) ClassifyVerticalFiber(ctx context.Context, origin, direction geom.Vec3, u *big.Rat, limits [2]*big.Rat, o WrappedVerticalFiberOptions) (WrappedVerticalFiberClassification, error) {
	if !origin.IsFinite() || !direction.IsFinite() || direction.X != 0 || direction.Y != 0 {
		return WrappedVerticalFiberClassification{}, geom.ErrInvalid
	}
	return r.ClassifyVerticalFiberRational(ctx, [3]*big.Rat{ratInset(origin.X), ratInset(origin.Y), ratInset(origin.Z)},
		ratInset(direction.Z), u, limits, o)
}

// ClassifyVerticalFiberRational preserves a rational model generator and its
// original finite parameterization. It requires rational trim events and
// shares the proof and boundary-interval contract of ClassifyVerticalFiber.
func (r *WrappedQuadraticFace) ClassifyVerticalFiberRational(ctx context.Context, ori [3]*big.Rat,
	dz, u *big.Rat, limits [2]*big.Rat, o WrappedVerticalFiberOptions) (WrappedVerticalFiberClassification, error) {
	var zero WrappedVerticalFiberClassification
	if ctx == nil || r == nil || len(r.loops) == 0 || u == nil || dz == nil || limits[0] == nil || limits[1] == nil {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	if o.MaxEvents == 0 {
		o.MaxEvents = 128
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxPoles == 0 {
		o.MaxPoles = 4096
	}
	if o.MaxEvents < 1 || o.MaxEvents > 256 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxPoles < 4 {
		return zero, geom.ErrInvalid
	}
	used := 0
	check := func(q *big.Rat) error {
		if q == nil {
			return geom.ErrInvalid
		}
		used++
		if used > o.MaxOperations || q.Num().BitLen() > o.MaxCoefficientBits || q.Denom().BitLen() > o.MaxCoefficientBits {
			return geom.ErrConvergence
		}
		return ctx.Err()
	}
	for _, q := range []*big.Rat{u, limits[0], limits[1], ori[0], ori[1], ori[2], dz} {
		if e := check(q); e != nil {
			return zero, e
		}
	}
	if dz.Sign() == 0 || limits[0].Cmp(limits[1]) > 0 {
		return zero, geom.ErrInvalid
	}
	w, ok := r.native.Face.Surface.(*geom.WrappedSurfacePatch)
	if !ok {
		return zero, geom.ErrConvergence
	}
	if used >= o.MaxOperations {
		return zero, geom.ErrConvergence
	}
	child := o.SurfacePatchOptions
	child.MaxOperations -= used
	embedding, e := geom.CertifyWrappedCylinderEmbedding(ctx, w, child)
	if e != nil {
		return zero, e
	}
	if !embedding.Complete {
		return zero, geom.ErrConvergence
	}
	used += embedding.ExactOperations
	d := w.Domain()
	if used >= o.MaxOperations {
		return zero, geom.ErrConvergence
	}
	jet, e := w.EvaluateRational(ctx, u, ratInset(d.VMin), geom.SurfacePatchEvaluationOptions{MaxOperations: o.MaxOperations - used, MaxCoefficientBits: o.MaxCoefficientBits})
	if e != nil {
		return zero, e
	}
	used += jet.ExactOperations
	if jet.Point[0].Cmp(ori[0]) != 0 || jet.Point[1].Cmp(ori[1]) != 0 {
		return zero, geom.ErrDomain
	}
	result := WrappedVerticalFiberClassification{SuppliedU: new(big.Rat).Set(u)}
	finish := func() (WrappedVerticalFiberClassification, error) {
		if e := ctx.Err(); e != nil {
			return zero, e
		}
		result.Complete = true
		result.ExactOperations = used
		return result, nil
	}
	var lowU, highU *big.Rat
	for _, loop := range r.loops {
		for _, q := range loop {
			for _, h := range q.control {
				v := new(big.Rat).Quo(h[0], h[2])
				if e := check(v); e != nil {
					return zero, e
				}
				if lowU == nil || v.Cmp(lowU) < 0 {
					lowU = v
				}
				if highU == nil || v.Cmp(highU) > 0 {
					highU = v
				}
			}
		}
	}
	period := w.Periods()[0]
	if period == nil || new(big.Rat).Sub(highU, lowU).Cmp(period) >= 0 {
		return zero, geom.ErrConvergence
	}
	quotient := new(big.Rat).Quo(new(big.Rat).Sub(lowU, u), period)
	if e := check(quotient); e != nil {
		return zero, e
	}
	k, rem := new(big.Int), new(big.Int)
	k.QuoRem(quotient.Num(), quotient.Denom(), rem)
	if rem.Sign() > 0 {
		k.Add(k, big.NewInt(1))
	}
	faceU := new(big.Rat).Add(u, new(big.Rat).Mul(new(big.Rat).SetInt(k), period))
	if e := check(faceU); e != nil {
		return zero, e
	}
	if faceU.Cmp(highU) > 0 {
		return finish()
	}
	result.FaceU = faceU
	result.PeriodShift = k
	zToT := func(z *big.Rat) *big.Rat { return new(big.Rat).Quo(new(big.Rat).Sub(z, ori[2]), dz) }
	tlo, thi := zToT(embedding.LowerZ), zToT(embedding.UpperZ)
	for _, q := range []*big.Rat{tlo, thi} {
		if e := check(q); e != nil {
			return zero, e
		}
	}
	if tlo.Cmp(thi) > 0 {
		tlo, thi = thi, tlo
	}
	if tlo.Cmp(limits[0]) < 0 {
		tlo = new(big.Rat).Set(limits[0])
	}
	if thi.Cmp(limits[1]) > 0 {
		thi = new(big.Rat).Set(limits[1])
	}
	if tlo.Cmp(thi) > 0 {
		return finish()
	}
	type coincident struct {
		loop, edge     int
		t0, t1, lo, hi *big.Rat
	}
	var spans []coincident
	events := map[string]*WrappedVerticalFiberEvent{}
	add := func(t *big.Rat, source *WrappedVerticalTrimSource) error {
		if t.Cmp(tlo) < 0 || t.Cmp(thi) > 0 {
			return nil
		}
		key := t.RatString()
		event := events[key]
		if event == nil {
			if len(events) >= o.MaxEvents {
				return geom.ErrConvergence
			}
			event = &WrappedVerticalFiberEvent{Parameter: new(big.Rat).Set(t)}
			events[key] = event
		}
		if source != nil {
			event.Sources = append(event.Sources, *source)
		}
		return nil
	}
	if e := add(tlo, nil); e != nil {
		return zero, e
	}
	if e := add(thi, nil); e != nil {
		return zero, e
	}
	for li, loop := range r.loops {
		for ei, q := range loop {
			var b [3]*big.Rat
			for i, h := range q.control {
				b[i] = new(big.Rat).Sub(h[0], new(big.Rat).Mul(faceU, h[2]))
				if e := check(b[i]); e != nil {
					return zero, e
				}
			}
			p0 := b[0]
			p1 := new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Sub(b[1], b[0]))
			p2 := new(big.Rat).Add(new(big.Rat).Sub(b[0], new(big.Rat).Mul(big.NewRat(2, 1), b[1])), b[2])
			for _, p := range []*big.Rat{p1, p2} {
				if e := check(p); e != nil {
					return zero, e
				}
			}
			var roots []*big.Rat
			if p2.Sign() == 0 {
				if p1.Sign() == 0 {
					if p0.Sign() == 0 {
						if !o.AllowBoundaryIntervals || q.control[0][2].Cmp(q.control[1][2]) != 0 || q.control[0][2].Cmp(q.control[2][2]) != 0 || new(big.Rat).Mul(big.NewRat(2, 1), q.control[1][1]).Cmp(new(big.Rat).Add(q.control[0][1], q.control[2][1])) != 0 {
							return zero, geom.ErrConvergence
						}
						var ts [2]*big.Rat
						for j, ci := range []int{0, 2} {
							v := new(big.Rat).Quo(q.control[ci][1], q.control[ci][2])
							z := new(big.Rat).Add(embedding.LowerZ, new(big.Rat).Mul(new(big.Rat).Sub(embedding.UpperZ, embedding.LowerZ), new(big.Rat).Quo(new(big.Rat).Sub(v, ratInset(d.VMin)), new(big.Rat).Sub(ratInset(d.VMax), ratInset(d.VMin)))))
							ts[j] = zToT(z)
							for _, x := range []*big.Rat{v, z, ts[j]} {
								if e := check(x); e != nil {
									return zero, e
								}
							}
						}
						if ts[0].Cmp(ts[1]) == 0 {
							return zero, geom.ErrConvergence
						}
						lo, hi := ts[0], ts[1]
						if lo.Cmp(hi) > 0 {
							lo, hi = hi, lo
						}
						if lo.Cmp(tlo) < 0 {
							lo = tlo
						}
						if hi.Cmp(thi) > 0 {
							hi = thi
						}
						if lo.Cmp(hi) <= 0 {
							spans = append(spans, coincident{li, ei, ts[0], ts[1], lo, hi})
							for j, t := range []*big.Rat{lo, hi} {
								if j == 1 && lo.Cmp(hi) == 0 {
									continue
								}
								s := new(big.Rat).Quo(new(big.Rat).Sub(t, ts[0]), new(big.Rat).Sub(ts[1], ts[0]))
								if e := check(s); e != nil {
									return zero, e
								}
								if e := add(t, &WrappedVerticalTrimSource{Loop: li, Edge: ei, Parameter: s}); e != nil {
									return zero, e
								}
							}
						}
					}
					continue
				}
				roots = append(roots, new(big.Rat).Quo(new(big.Rat).Neg(p0), p1))
			} else {
				disc := new(big.Rat).Sub(new(big.Rat).Mul(p1, p1), new(big.Rat).Mul(big.NewRat(4, 1), new(big.Rat).Mul(p0, p2)))
				if e := check(disc); e != nil {
					return zero, e
				}
				if disc.Sign() < 0 {
					continue
				}
				sn, sd := new(big.Int).Sqrt(disc.Num()), new(big.Int).Sqrt(disc.Denom())
				if new(big.Int).Mul(sn, sn).Cmp(disc.Num()) != 0 || new(big.Int).Mul(sd, sd).Cmp(disc.Denom()) != 0 {
					return zero, geom.ErrConvergence
				}
				sqrt := new(big.Rat).SetFrac(sn, sd)
				for _, sign := range []int64{-1, 1} {
					root := new(big.Rat).Quo(new(big.Rat).Add(new(big.Rat).Neg(p1), new(big.Rat).Mul(big.NewRat(sign, 1), sqrt)), new(big.Rat).Mul(big.NewRat(2, 1), p2))
					if len(roots) == 0 || root.Cmp(roots[0]) != 0 {
						roots = append(roots, root)
					}
				}
			}
			for _, s := range roots {
				if e := check(s); e != nil {
					return zero, e
				}
				if s.Sign() < 0 || s.Cmp(big.NewRat(1, 1)) > 0 {
					continue
				}
				one := new(big.Rat).Sub(big.NewRat(1, 1), s)
				weights := [3]*big.Rat{new(big.Rat).Mul(one, one), new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(s, one)), new(big.Rat).Mul(s, s)}
				var h [3]*big.Rat
				for axis := 0; axis < 3; axis++ {
					h[axis] = new(big.Rat)
					for i := 0; i < 3; i++ {
						h[axis].Add(h[axis], new(big.Rat).Mul(weights[i], q.control[i][axis]))
					}
					if e := check(h[axis]); e != nil {
						return zero, e
					}
				}
				if h[2].Sign() <= 0 || new(big.Rat).Quo(h[0], h[2]).Cmp(faceU) != 0 {
					return zero, geom.ErrConvergence
				}
				v := new(big.Rat).Quo(h[1], h[2])
				z := new(big.Rat).Add(embedding.LowerZ, new(big.Rat).Mul(new(big.Rat).Sub(embedding.UpperZ, embedding.LowerZ), new(big.Rat).Quo(new(big.Rat).Sub(v, ratInset(d.VMin)), new(big.Rat).Sub(ratInset(d.VMax), ratInset(d.VMin)))))
				t := zToT(z)
				for _, q := range []*big.Rat{v, z, t} {
					if e := check(q); e != nil {
						return zero, e
					}
				}
				if e := add(t, &WrappedVerticalTrimSource{Loop: li, Edge: ei, Parameter: new(big.Rat).Set(s)}); e != nil {
					return zero, e
				}
			}
		}
	}
	for _, event := range events {
		result.Events = append(result.Events, *event)
	}
	sort.Slice(result.Events, func(i, j int) bool { return result.Events[i].Parameter.Cmp(result.Events[j].Parameter) < 0 })
	locate := func(t *big.Rat) (WrappedFacePointLocation, error) {
		z := new(big.Rat).Add(ori[2], new(big.Rat).Mul(t, dz))
		v := new(big.Rat).Add(ratInset(d.VMin), new(big.Rat).Mul(new(big.Rat).Sub(ratInset(d.VMax), ratInset(d.VMin)), new(big.Rat).Quo(new(big.Rat).Sub(z, embedding.LowerZ), new(big.Rat).Sub(embedding.UpperZ, embedding.LowerZ))))
		for _, q := range []*big.Rat{z, v} {
			if e := check(q); e != nil {
				return WrappedFacePointLocation{}, e
			}
		}
		if used >= o.MaxOperations {
			return WrappedFacePointLocation{}, geom.ErrConvergence
		}
		child := o.SurfacePatchOptions
		child.MaxOperations -= used
		location, e := r.ClassifyCylinderRationalPoint(ctx, [3]*big.Rat{ori[0], ori[1], z}, [2]*big.Rat{faceU, v}, child)
		if e == nil {
			used += location.ExactOperations
		}
		return location, e
	}
	for i := range result.Events {
		event := &result.Events[i]
		location, e := locate(event.Parameter)
		if e != nil {
			return zero, e
		}
		event.UV = [2]*big.Rat{new(big.Rat).Set(faceU), new(big.Rat).Set(location.SuppliedUV[1])}
		event.Point = location.Point
		event.Location = location.Location
		if len(event.Sources) > 0 && event.Location != trimvalidate.Boundary {
			return zero, geom.ErrConvergence
		}
		if event.Parameter.Cmp(limits[0]) == 0 {
			event.FiniteMask |= 1
		}
		if event.Parameter.Cmp(limits[1]) == 0 {
			event.FiniteMask |= 2
		}
		if event.Point[2].Cmp(embedding.LowerZ) == 0 {
			event.SupportMask |= 1
		}
		if event.Point[2].Cmp(embedding.UpperZ) == 0 {
			event.SupportMask |= 2
		}
	}
	for i := 0; i < len(result.Events)-1; i++ {
		a, b := result.Events[i].Parameter, result.Events[i+1].Parameter
		mid := new(big.Rat).Quo(new(big.Rat).Add(a, b), big.NewRat(2, 1))
		if e := check(mid); e != nil {
			return zero, e
		}
		location, e := locate(mid)
		if e != nil {
			return zero, e
		}
		switch location.Location {
		case trimvalidate.Inside:
			if result.Events[i].Location == trimvalidate.Outside || result.Events[i+1].Location == trimvalidate.Outside {
				return zero, geom.ErrConvergence
			}
			result.Intervals = append(result.Intervals, WrappedVerticalFiberInterval{Start: new(big.Rat).Set(a), End: new(big.Rat).Set(b), FirstEvent: i, LastEvent: i + 1})
		case trimvalidate.Outside:
		case trimvalidate.Boundary:
			interval := WrappedVerticalBoundaryInterval{Start: new(big.Rat).Set(a), End: new(big.Rat).Set(b), FirstEvent: i, LastEvent: i + 1}
			for _, span := range spans {
				if a.Cmp(span.lo) < 0 || b.Cmp(span.hi) > 0 {
					continue
				}
				source := WrappedVerticalBoundarySource{Loop: span.loop, Edge: span.edge}
				for j, t := range []*big.Rat{a, b} {
					source.Parameters[j] = new(big.Rat).Quo(new(big.Rat).Sub(t, span.t0), new(big.Rat).Sub(span.t1, span.t0))
					if e := check(source.Parameters[j]); e != nil {
						return zero, e
					}
				}
				interval.Sources = append(interval.Sources, source)
			}
			if len(interval.Sources) == 0 || result.Events[i].Location != trimvalidate.Boundary || result.Events[i+1].Location != trimvalidate.Boundary {
				return zero, geom.ErrConvergence
			}
			result.BoundaryIntervals = append(result.BoundaryIntervals, interval)
		default:
			return zero, geom.ErrConvergence
		}
	}
	return finish()
}
