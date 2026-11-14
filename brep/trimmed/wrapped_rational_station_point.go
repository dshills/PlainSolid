package trimmed

import (
	"context"
	"math/big"
	"sort"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// WrappedRationalStationPointOptions separates native source authentication
// from one exact query. Source retains its documented per-band/shared-edge
// quotas (at most16 bands). MaxPointOperations bounds all query arithmetic and
// the one original conic classification, default4096, maximum1M. Query rational
// coefficients are bounded by MaxCoefficientBits, default16384, maximum65536.
type WrappedRationalStationPointOptions struct {
	Source                                 geom.SurfacePatchOptions
	MaxPointOperations, MaxCoefficientBits int
}

// WrappedRationalStationPoint reports material classification and every original
// face attaining a boundary point. Faces is empty for Inside/Outside. An internal
// station disk is not boundary; its rim has both original adjacent wall owners.
// Any failure returns an empty result and must not be interpreted as Outside.
type WrappedRationalStationPoint struct {
	Location parametric.Location
	Faces    []int
}

// LocateWrappedRationalStationLoftRational authenticates the native rational
// profile and classifies an exact point without circular substitution. A
// homothety maps the original cross section to the interior of one original
// band; its guarded exact conic predicate then distinguishes lateral boundary
// from the artificial cap of a local band. Whole-rim source identity proves
// the same predicate applies to both owners at a station crease.
func (s Solid) LocateWrappedRationalStationLoftRational(ctx context.Context, point [3]*big.Rat, o WrappedRationalStationPointOptions) (WrappedRationalStationPoint, error) {
	empty := WrappedRationalStationPoint{}
	if o.MaxPointOperations == 0 {
		o.MaxPointOperations = 4096
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if ctx == nil || o.MaxPointOperations < 1 || o.MaxPointOperations > 1000000 || o.MaxCoefficientBits < 64 || o.MaxCoefficientBits > 65536 {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	check := func(q *big.Rat) error {
		if q == nil {
			return geom.ErrInvalid
		}
		if q.Num().BitLen() > o.MaxCoefficientBits || q.Denom().BitLen() > o.MaxCoefficientBits {
			return geom.ErrConvergence
		}
		return nil
	}
	var query [3]*big.Rat
	for i, q := range point {
		if e := check(q); e != nil {
			return empty, e
		}
		query[i] = new(big.Rat).Set(q)
	}
	p, bands, e := s.validateWrappedProfileStationLoft(ctx, o.Source, false)
	if e != nil {
		return empty, e
	}
	used := 0
	take := func() error {
		if e := ctx.Err(); e != nil {
			return e
		}
		used++
		if used > o.MaxPointOperations {
			return geom.ErrConvergence
		}
		return nil
	}
	compare := func(a, b *big.Rat) (int, error) {
		if e := take(); e != nil {
			return 0, e
		}
		if e := check(a); e != nil {
			return 0, e
		}
		if e := check(b); e != nil {
			return 0, e
		}
		return a.Cmp(b), nil
	}
	arithmetic := func(op byte, a, b *big.Rat) (*big.Rat, error) {
		if e := take(); e != nil {
			return nil, e
		}
		if e := check(a); e != nil {
			return nil, e
		}
		if e := check(b); e != nil {
			return nil, e
		}
		q := new(big.Rat)
		switch op {
		case '+':
			q.Add(a, b)
		case '-':
			q.Sub(a, b)
		case '*':
			q.Mul(a, b)
		case '/':
			if b.Sign() == 0 {
				return nil, geom.ErrDomain
			}
			q.Quo(a, b)
		default:
			return nil, geom.ErrInvalid
		}
		if e := check(q); e != nil {
			return nil, e
		}
		return q, nil
	}
	station := -1
	band := -1
	for i, h := range p.Heights {
		c, e := compare(query[2], h)
		if e != nil {
			return empty, e
		}
		if c == 0 {
			station = i
			if i == 0 {
				band = 0
			} else {
				band = i - 1
			}
			break
		}
		if c < 0 {
			if i == 0 {
				return WrappedRationalStationPoint{Location: parametric.Outside}, nil
			}
			band = i - 1
			break
		}
	}
	if band < 0 {
		return WrappedRationalStationPoint{Location: parametric.Outside}, nil
	}
	// The query-side arithmetic is charged individually; the conic call receives
	// only the remaining allowance, including its own support reproof.
	dz, e := arithmetic('-', p.Heights[band+1], p.Heights[band])
	if e != nil {
		return empty, e
	}
	z, e := arithmetic('-', query[2], p.Heights[band])
	if e != nil {
		return empty, e
	}
	t, e := arithmetic('/', z, dz)
	if e != nil {
		return empty, e
	}
	dr, e := arithmetic('-', p.Radii[band+1], p.Radii[band])
	if e != nil {
		return empty, e
	}
	offset, e := arithmetic('*', t, dr)
	if e != nil {
		return empty, e
	}
	radius, e := arithmetic('+', p.Radii[band], offset)
	if e != nil {
		return empty, e
	}
	sum, e := arithmetic('+', p.Radii[band], p.Radii[band+1])
	if e != nil {
		return empty, e
	}
	middleRadius, e := arithmetic('/', sum, big.NewRat(2, 1))
	if e != nil {
		return empty, e
	}
	scale, e := arithmetic('/', middleRadius, radius)
	if e != nil {
		return empty, e
	}
	var lifted [3]*big.Rat
	for axis := 0; axis < 2; axis++ {
		d, e := arithmetic('-', query[axis], p.Center[axis])
		if e != nil {
			return empty, e
		}
		d, e = arithmetic('*', d, scale)
		if e != nil {
			return empty, e
		}
		lifted[axis], e = arithmetic('+', p.Center[axis], d)
		if e != nil {
			return empty, e
		}
	}
	sum, e = arithmetic('+', p.Heights[band], p.Heights[band+1])
	if e != nil {
		return empty, e
	}
	lifted[2], e = arithmetic('/', sum, big.NewRat(2, 1))
	if e != nil {
		return empty, e
	}
	if used >= o.MaxPointOperations {
		return empty, geom.ErrConvergence
	}
	options := o.Source
	options.MaxOperations = o.MaxPointOperations - used
	options.MaxCoefficientBits = o.MaxCoefficientBits
	location, e := geom.LocateWrappedFrustumRational(ctx, bands[band], lifted, options)
	if e != nil {
		return empty, e
	}
	result := WrappedRationalStationPoint{Location: parametric.Inside}
	switch location {
	case geom.CylinderOutside:
		result.Location = parametric.Outside
	case geom.CylinderInside:
	case geom.CylinderBoundary:
		result.Location = parametric.Boundary
		result.Faces = append(result.Faces, p.History.WallFaces[band])
		if station > 0 && station < len(p.Heights)-1 {
			result.Faces = append(result.Faces, p.History.WallFaces[station])
		}
	default:
		return empty, geom.ErrConvergence
	}
	if location != geom.CylinderOutside && (station == 0 || station == len(p.Heights)-1) {
		cap := 0
		if station != 0 {
			cap = 1
		}
		result.Location = parametric.Boundary
		result.Faces = append(result.Faces, p.History.CapFaces[cap])
	}
	sort.Ints(result.Faces)
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	return result, nil
}
