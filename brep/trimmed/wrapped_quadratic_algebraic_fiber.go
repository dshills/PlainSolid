package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// AlgebraicVerticalFiberOptions bounds total arithmetic work and event count.
// MaxSubdivisions bounds each exact isolation/refinement stage. MaxRefinements
// bounds rational probe searches per adjacent event pair (default 128).
type AlgebraicVerticalFiberOptions struct {
	geom.SurfacePatchOptions
	MaxEvents, MaxSubdivisions, MaxRefinements int
	// AllowBoundaryIntervals accepts only degree-elevated affine trim spans.
	AllowBoundaryIntervals bool
}

// AlgebraicVerticalTrimSource binds an original normalized trim parameter to
// its exact U-level event. Event coordinates are UV, not model XYZ.
// Parameter always retains the original normalized trim parameter. For an
// opt-in coincident span endpoint, Coincident is true and Event may be nil
// when finite clipping creates an endpoint in the source edge interior.
type AlgebraicVerticalTrimSource struct {
	Loop, Edge int
	Event      *geom.ExactComposedPlaneEvent
	Parameter  *geom.ExactAlgebraicRoot
	Coincident bool
}

// AlgebraicVerticalFiberEvent retains exact line units and original chart V.
// Model XY is constant and recorded on the result; Z=origin.Z+direction.Z*t.
// Source-owned events are proved trim boundary points even when irrational.
type AlgebraicVerticalFiberEvent struct {
	Parameter, V            *geom.ExactAlgebraicRoot
	Sources                 []AlgebraicVerticalTrimSource
	Location                trimvalidate.Location
	FiniteMask, SupportMask uint8
}
type AlgebraicVerticalFiberInterval struct {
	Start, End            *geom.ExactAlgebraicRoot
	FirstEvent, LastEvent int
	// Probe is strictly between the endpoint roots, proved Inside.
	Probe *big.Rat
}

// AlgebraicVerticalBoundarySource keeps source parameters in increasing line-t
// order; the source edge may run oppositely. Loop zero is outer, others holes.
type AlgebraicVerticalBoundarySource struct {
	Loop, Edge int
	Parameters [2]*geom.ExactAlgebraicRoot
}

// AlgebraicVerticalBoundaryInterval is a positive line-length boundary cell,
// with zero face-interior area. It is not a strict material-interior interval.
type AlgebraicVerticalBoundaryInterval struct {
	Start, End            *geom.ExactAlgebraicRoot
	FirstEvent, LastEvent int
	Sources               []AlgebraicVerticalBoundarySource
}
type AlgebraicVerticalFiberClassification struct {
	Complete          bool
	SuppliedU, FaceU  *big.Rat
	PeriodShift       *big.Int
	Origin, Direction [3]*big.Rat
	Events            []AlgebraicVerticalFiberEvent
	Intervals         []AlgebraicVerticalFiberInterval
	BoundaryIntervals []AlgebraicVerticalBoundaryInterval
	ExactOperations   int
}

// ClassifyAlgebraicVerticalFiber intersects the closed trimmed face with a
// finite vertical line. Positive-weight rational quadratic trim roots and
// their exact coordinate images are retained, never rounded. Global quotient
// uniqueness and complete root enumeration precede every rational interval
// probe. Coincident trim spans require an opt-in affine source map proof;
// nonlinear coincident speed remains unsupported. Failure is atomic.
func (r *WrappedQuadraticFace) ClassifyAlgebraicVerticalFiber(ctx context.Context, origin, direction geom.Vec3, u *big.Rat, limits [2]*big.Rat, o AlgebraicVerticalFiberOptions) (AlgebraicVerticalFiberClassification, error) {
	if !origin.IsFinite() || !direction.IsFinite() || direction.X != 0 || direction.Y != 0 {
		return AlgebraicVerticalFiberClassification{}, geom.ErrInvalid
	}
	return r.ClassifyAlgebraicVerticalFiberRational(ctx, [3]*big.Rat{ratInset(origin.X), ratInset(origin.Y), ratInset(origin.Z)},
		ratInset(direction.Z), u, limits, o)
}

// ClassifyAlgebraicVerticalFiberRational classifies the same exact source
// trim arrangement for p(t)=(origin.X, origin.Y, origin.Z+t*scale). Model
// coordinates, parameterization, and all recovered trim events stay exact.
func (r *WrappedQuadraticFace) ClassifyAlgebraicVerticalFiberRational(ctx context.Context, origin [3]*big.Rat,
	scale, u *big.Rat, limits [2]*big.Rat, o AlgebraicVerticalFiberOptions) (AlgebraicVerticalFiberClassification, error) {
	var zero AlgebraicVerticalFiberClassification
	if ctx == nil || r == nil || len(r.loops) == 0 || u == nil || scale == nil || limits[0] == nil || limits[1] == nil {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
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
	if o.MaxEvents == 0 {
		o.MaxEvents = 128
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 8192
	}
	if o.MaxRefinements == 0 {
		o.MaxRefinements = 128
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxPoles < 4 || o.MaxEvents < 1 || o.MaxEvents > 256 || o.MaxSubdivisions < 1 || o.MaxSubdivisions > 65536 || o.MaxRefinements < 1 || o.MaxRefinements > 1024 {
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
	opts := func() geom.ExactCurvePlaneEventOptions {
		remaining := o.MaxOperations - used
		if remaining <= 0 {
			remaining = -1
		}
		return geom.ExactCurvePlaneEventOptions{MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits, MaxSubdivisions: o.MaxSubdivisions}
	}
	patchOpts := func() geom.SurfacePatchOptions {
		p := o.SurfacePatchOptions
		p.MaxOperations = opts().MaxOperations
		return p
	}
	for _, x := range []*big.Rat{u, limits[0], limits[1], origin[0], origin[1], origin[2], scale} {
		if e := check(x); e != nil {
			return zero, e
		}
	}
	if scale.Sign() == 0 || limits[0].Cmp(limits[1]) > 0 {
		return zero, geom.ErrInvalid
	}
	out := AlgebraicVerticalFiberClassification{SuppliedU: new(big.Rat).Set(u), Direction: [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat).Set(scale)}}
	for i := range origin {
		out.Origin[i] = new(big.Rat).Set(origin[i])
	}
	finish := func() (AlgebraicVerticalFiberClassification, error) {
		if e := ctx.Err(); e != nil {
			return zero, e
		}
		if used > o.MaxOperations {
			return zero, geom.ErrConvergence
		}
		out.Complete = true
		out.ExactOperations = used
		return out, nil
	}
	w, ok := r.native.Face.Surface.(*geom.WrappedSurfacePatch)
	if !ok {
		return zero, geom.ErrConvergence
	}
	d := w.Domain()
	// A source endpoint V lies in both the support and trim hull; using the
	// support's VMin could incorrectly report no lift merely due to height.
	seedV := new(big.Rat).Quo(r.loops[0][0].control[0][1], r.loops[0][0].control[0][2])
	if e := check(seedV); e != nil {
		return zero, e
	}
	lift, e := r.LocateCylinderRationalUV(ctx, [2]*big.Rat{u, seedV}, patchOpts())
	if e != nil {
		return zero, e
	}
	used += lift.ExactOperations
	if lift.Point[0].Cmp(out.Origin[0]) != 0 || lift.Point[1].Cmp(out.Origin[1]) != 0 {
		return zero, geom.ErrDomain
	}
	if !lift.MatchedLift {
		return finish()
	}
	out.FaceU = new(big.Rat).Set(lift.FaceUV[0])
	out.PeriodShift = new(big.Int).Set(lift.PeriodShift[0])
	emb, e := geom.CertifyWrappedCylinderEmbedding(ctx, w, patchOpts())
	if e != nil {
		return zero, e
	}
	used += emb.ExactOperations
	if !emb.Complete {
		return zero, geom.ErrConvergence
	}
	// t = offset + tPerV*V, in the original unnormalized line units.
	tPerV := new(big.Rat).Quo(new(big.Rat).Sub(emb.UpperZ, emb.LowerZ), new(big.Rat).Mul(new(big.Rat).Sub(ratInset(d.VMax), ratInset(d.VMin)), out.Direction[2]))
	offset := new(big.Rat).Sub(new(big.Rat).Quo(new(big.Rat).Sub(emb.LowerZ, out.Origin[2]), out.Direction[2]), new(big.Rat).Mul(tPerV, ratInset(d.VMin)))
	zt := func(z *big.Rat) *big.Rat {
		return new(big.Rat).Quo(new(big.Rat).Sub(z, out.Origin[2]), out.Direction[2])
	}
	lowerZ, upperZ := zt(emb.LowerZ), zt(emb.UpperZ)
	lo, hi := new(big.Rat).Set(lowerZ), new(big.Rat).Set(upperZ)
	if lo.Cmp(hi) > 0 {
		lo, hi = hi, lo
	}
	if lo.Cmp(limits[0]) < 0 {
		lo.Set(limits[0])
	}
	if hi.Cmp(limits[1]) > 0 {
		hi.Set(limits[1])
	}
	for _, x := range []*big.Rat{tPerV, offset, lowerZ, upperZ, lo, hi} {
		if e := check(x); e != nil {
			return zero, e
		}
	}
	if lo.Cmp(hi) > 0 {
		return finish()
	}
	rational := func(x *big.Rat) (*geom.ExactAlgebraicRoot, error) {
		root, n, e := geom.NewExactRationalRoot(ctx, x, opts())
		used += n
		return root, e
	}
	compare := func(a, b *geom.ExactAlgebraicRoot) (int, error) {
		c, e := a.Compare(ctx, b, opts())
		used += c.Operations
		if e != nil {
			return 0, e
		}
		if !c.Proven {
			return 0, geom.ErrConvergence
		}
		return c.Order, nil
	}
	lr, e := rational(lo)
	if e != nil {
		return zero, e
	}
	hr, e := rational(hi)
	if e != nil {
		return zero, e
	}
	add := func(event AlgebraicVerticalFiberEvent) error {
		c, e := compare(event.Parameter, lr)
		if e != nil {
			return e
		}
		if c < 0 {
			return nil
		}
		c, e = compare(event.Parameter, hr)
		if e != nil {
			return e
		}
		if c > 0 {
			return nil
		}
		for i := range out.Events {
			c, e = compare(event.Parameter, out.Events[i].Parameter)
			if e != nil {
				return e
			}
			if c == 0 {
				out.Events[i].Sources = append(out.Events[i].Sources, event.Sources...)
				out.Events[i].FiniteMask |= event.FiniteMask
				out.Events[i].SupportMask |= event.SupportMask
				if len(event.Sources) > 0 {
					out.Events[i].Location = trimvalidate.Boundary
				}
				return nil
			}
			if c < 0 {
				if len(out.Events) >= o.MaxEvents {
					return geom.ErrConvergence
				}
				out.Events = append(out.Events, AlgebraicVerticalFiberEvent{})
				copy(out.Events[i+1:], out.Events[i:])
				out.Events[i] = event
				return nil
			}
		}
		if len(out.Events) >= o.MaxEvents {
			return geom.ErrConvergence
		}
		out.Events = append(out.Events, event)
		return nil
	}
	locate := func(t *big.Rat) (trimvalidate.Location, error) {
		v := new(big.Rat).Quo(new(big.Rat).Sub(t, offset), tPerV)
		if e := check(v); e != nil {
			return trimvalidate.Unknown, e
		}
		q, e := r.LocateCylinderRationalUV(ctx, [2]*big.Rat{out.FaceU, v}, patchOpts())
		used += q.ExactOperations
		return q.Location, e
	}
	for j, t := range []*big.Rat{lo, hi} {
		if j == 1 && lo.Cmp(hi) == 0 {
			continue
		}
		tr := lr
		if j == 1 {
			tr = hr
		}
		vr, n, e := tr.AffineMap(ctx, new(big.Rat).Quo(new(big.Rat).Neg(offset), tPerV), new(big.Rat).Inv(tPerV), opts())
		used += n
		if e != nil {
			return zero, e
		}
		loc, e := locate(t)
		if e != nil {
			return zero, e
		}
		ev := AlgebraicVerticalFiberEvent{Parameter: tr, V: vr, Location: loc}
		if t.Cmp(limits[0]) == 0 {
			ev.FiniteMask |= 1
		}
		if t.Cmp(limits[1]) == 0 {
			ev.FiniteMask |= 2
		}
		if t.Cmp(lowerZ) == 0 {
			ev.SupportMask |= 1
		}
		if t.Cmp(upperZ) == 0 {
			ev.SupportMask |= 2
		}
		if e := add(ev); e != nil {
			return zero, e
		}
	}
	type affineSpan struct {
		loop, edge int
		t0, t1     *big.Rat
		lo, hi     *geom.ExactAlgebraicRoot
	}
	var spans []affineSpan
	for li, loop := range r.loops {
		for ei, q := range loop {
			curve, e := q.modelParameterCurve(ctx, patchOpts())
			if e != nil {
				return zero, e
			}
			used += curve.ExactOperations()
			cells, e := curve.ModelContactCells(ctx, patchOpts(), 1)
			if e != nil {
				return zero, e
			}
			used += cells.ExactOperations
			if !cells.Complete || len(cells.Cells) != 1 {
				return zero, geom.ErrConvergence
			}
			contacts, e := cells.Cells[0].IntersectRawPlane(ctx, [4]*big.Rat{big.NewRat(1, 1), nil, nil, new(big.Rat).Neg(out.FaceU)}, opts())
			used += contacts.Operations
			if e != nil {
				return zero, e
			}
			if !contacts.Complete {
				return zero, geom.ErrConvergence
			}
			if contacts.Coincident {
				if !o.AllowBoundaryIntervals || q.control[0][2].Cmp(q.control[1][2]) != 0 || q.control[0][2].Cmp(q.control[2][2]) != 0 || new(big.Rat).Mul(big.NewRat(2, 1), q.control[1][1]).Cmp(new(big.Rat).Add(q.control[0][1], q.control[2][1])) != 0 {
					return zero, geom.ErrConvergence
				}
				var ts [2]*big.Rat
				for j, ci := range []int{0, 2} {
					v := new(big.Rat).Quo(q.control[ci][1], q.control[ci][2])
					ts[j] = new(big.Rat).Add(offset, new(big.Rat).Mul(tPerV, v))
					if e := check(v); e != nil {
						return zero, e
					}
					if e := check(ts[j]); e != nil {
						return zero, e
					}
				}
				if ts[0].Cmp(ts[1]) == 0 {
					return zero, geom.ErrConvergence
				}
				low, high := ts[0], ts[1]
				if low.Cmp(high) > 0 {
					low, high = high, low
				}
				if low.Cmp(lo) < 0 {
					low = lo
				}
				if high.Cmp(hi) > 0 {
					high = hi
				}
				if low.Cmp(high) <= 0 {
					lowRoot, e := rational(low)
					if e != nil {
						return zero, e
					}
					highRoot, e := rational(high)
					if e != nil {
						return zero, e
					}
					spans = append(spans, affineSpan{li, ei, ts[0], ts[1], lowRoot, highRoot})
					for j, t := range []*big.Rat{low, high} {
						if j == 1 && low.Cmp(high) == 0 {
							continue
						}
						tr := lowRoot
						if j == 1 {
							tr = highRoot
						}
						s := new(big.Rat).Quo(new(big.Rat).Sub(t, ts[0]), new(big.Rat).Sub(ts[1], ts[0]))
						if e := check(s); e != nil {
							return zero, e
						}
						sr, e := rational(s)
						if e != nil {
							return zero, e
						}
						vr, n, e := tr.AffineMap(ctx, new(big.Rat).Quo(new(big.Rat).Neg(offset), tPerV), new(big.Rat).Inv(tPerV), opts())
						used += n
						if e != nil {
							return zero, e
						}
						source := AlgebraicVerticalTrimSource{Loop: li, Edge: ei, Parameter: sr, Coincident: true}
						if s.Sign() == 0 {
							source.Event = contacts.Boundary[0]
						} else if s.Cmp(big.NewRat(1, 1)) == 0 {
							source.Event = contacts.Boundary[1]
						}
						if e := add(AlgebraicVerticalFiberEvent{Parameter: tr, V: vr, Location: trimvalidate.Boundary, Sources: []AlgebraicVerticalTrimSource{source}}); e != nil {
							return zero, e
						}
					}
				}
				continue
			}
			for _, event := range contacts.Events {
				v, n, e := event.CoordinateRoot(ctx, 1, opts())
				used += n
				if e != nil {
					return zero, e
				}
				t, n, e := v.AffineMap(ctx, offset, tPerV, opts())
				used += n
				if e != nil {
					return zero, e
				}
				if e := add(AlgebraicVerticalFiberEvent{Parameter: t, V: v, Location: trimvalidate.Boundary, Sources: []AlgebraicVerticalTrimSource{{Loop: li, Edge: ei, Event: event, Parameter: event.Root()}}}); e != nil {
					return zero, e
				}
			}
		}
	}
	for i := 0; i+1 < len(out.Events); i++ {
		a, b := out.Events[i].Parameter, out.Events[i+1].Parameter
		width := big.NewRat(1, 1)
		var probe *big.Rat
		for step := 0; step < o.MaxRefinements; step++ {
			aa, e := a.Enclose(ctx, width, opts())
			used += aa.Operations
			if e != nil {
				return zero, e
			}
			bb, e := b.Enclose(ctx, width, opts())
			used += bb.Operations
			if e != nil {
				return zero, e
			}
			if aa.Interval.Upper.Cmp(bb.Interval.Lower) < 0 {
				probe = new(big.Rat).Quo(new(big.Rat).Add(aa.Interval.Upper, bb.Interval.Lower), big.NewRat(2, 1))
				break
			}
			width.Quo(width, big.NewRat(2, 1))
			if e := check(width); e != nil {
				return zero, e
			}
		}
		if probe == nil {
			return zero, geom.ErrConvergence
		}
		loc, e := locate(probe)
		if e != nil {
			return zero, e
		}
		switch loc {
		case trimvalidate.Inside:
			if out.Events[i].Location == trimvalidate.Outside || out.Events[i+1].Location == trimvalidate.Outside {
				return zero, geom.ErrConvergence
			}
			out.Intervals = append(out.Intervals, AlgebraicVerticalFiberInterval{Start: a, End: b, FirstEvent: i, LastEvent: i + 1, Probe: probe})
		case trimvalidate.Outside:
		case trimvalidate.Boundary:
			interval := AlgebraicVerticalBoundaryInterval{Start: a, End: b, FirstEvent: i, LastEvent: i + 1}
			for _, span := range spans {
				c, e := compare(a, span.lo)
				if e != nil {
					return zero, e
				}
				if c < 0 {
					continue
				}
				c, e = compare(b, span.hi)
				if e != nil {
					return zero, e
				}
				if c > 0 {
					continue
				}
				source := AlgebraicVerticalBoundarySource{Loop: span.loop, Edge: span.edge}
				delta := new(big.Rat).Sub(span.t1, span.t0)
				for j, t := range []*geom.ExactAlgebraicRoot{a, b} {
					root, n, e := t.AffineMap(ctx, new(big.Rat).Quo(new(big.Rat).Neg(span.t0), delta), new(big.Rat).Inv(delta), opts())
					used += n
					if e != nil {
						return zero, e
					}
					source.Parameters[j] = root
				}
				interval.Sources = append(interval.Sources, source)
			}
			if len(interval.Sources) == 0 || out.Events[i].Location != trimvalidate.Boundary || out.Events[i+1].Location != trimvalidate.Boundary {
				return zero, geom.ErrConvergence
			}
			out.BoundaryIntervals = append(out.BoundaryIntervals, interval)
		default:
			return zero, geom.ErrConvergence
		}
	}
	return finish()
}
