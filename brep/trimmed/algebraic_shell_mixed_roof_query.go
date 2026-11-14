package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// MixedRoofMaterialQuery contains the independently proved original roof boxes
// used by a mixed Boolean. It supports exact rational point membership at
// arbitrary scale in the original XY chart units. Surfaces not satisfying the
// six-face graph-box proof remain outside this preparation path.
type MixedRoofMaterialQuery struct {
	source *MixedAlgebraicShellBoolean
	box    [2]*mixedRoofBox
	work   int
}

// MixedRoofQueryOptions bounds preparation separately from later evaluations.
// MaxPreparationOperations defaults to 2M (maximum 16M).
type MixedRoofQueryOptions struct{ MaxPreparationOperations int }

// PrepareRoofMaterialQuery detects the unique nonplanar original support of
// each operand, then proves that each original shell is an embedded roof box.
// It uses the original immutable Boolean and requires no caller face IDs.
func (s *MixedAlgebraicShellBoolean) PrepareRoofMaterialQuery(ctx context.Context, o MixedRoofQueryOptions) (*MixedRoofMaterialQuery, error) {
	if s == nil || s.section == nil || s.boundary.source == nil || ctx == nil || o.MaxPreparationOperations < 0 || o.MaxPreparationOperations > 16000000 {
		return nil, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if o.MaxPreparationOperations == 0 {
		o.MaxPreparationOperations = 2000000
	}
	q := &MixedRoofMaterialQuery{source: s}
	for k, body := range [2]Solid{s.boundary.source.a, s.boundary.source.b} {
		if len(body.faces) != 6 {
			return nil, geom.ErrUnrepresentable
		}
		roof := -1
		left := o.MaxPreparationOperations - q.work
		for i, f := range body.faces {
			if left <= 0 {
				return nil, geom.ErrConvergence
			}
			p, err := exactFaceSupportPatch(f, geom.SurfacePatchOptions{MaxOperations: left, MaxCoefficientBits: 4096})
			if err != nil {
				return nil, err
			}
			left -= p.ExactOperations()
			_, yes, err := mixedExactPlaneProposal(ctx, p, i, &left)
			if err != nil {
				return nil, err
			}
			if !yes {
				if roof >= 0 {
					return nil, geom.ErrUnrepresentable
				}
				roof = i
			}
		}
		if roof < 0 {
			return nil, geom.ErrUnrepresentable
		}
		if left <= 0 {
			return nil, geom.ErrConvergence
		}
		var err error
		q.box[k], err = proveMixedRoofBox(ctx, body, roof, left)
		if err != nil {
			return nil, err
		}
		q.work = o.MaxPreparationOperations - left + q.box[k].operations
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return q, nil
}

// PreparationOperations reports work spent proving the original box supports.
func (q *MixedRoofMaterialQuery) PreparationOperations() int {
	if q == nil {
		return 0
	}
	return q.work
}

// MixedRoofMaterialPoint reports exact original material states and the
// regularized Boolean classification at one rational point.
type MixedRoofMaterialPoint struct {
	Location        parametric.Location
	Operands        [2]parametric.Location
	ExactOperations int
}

// LocateRational classifies directly against each proved floor, footprint,
// and exact graph. On a simultaneous source contact the original regular
// section certificate supplies the transverse boundary condition.
func (q *MixedRoofMaterialQuery) LocateRational(ctx context.Context, p [3]*big.Rat, o geom.SurfacePatchEvaluationOptions) (MixedRoofMaterialPoint, error) {
	var z MixedRoofMaterialPoint
	if ctx == nil || q == nil || q.source == nil || o.MaxOperations < 0 || o.MaxOperations > 16000000 || o.MaxCoefficientBits < 0 || o.MaxCoefficientBits > 16384 || o.MaxCoefficientBits > 0 && o.MaxCoefficientBits < 64 {
		return z, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return z, err
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 4096
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 1000000
	}
	for _, v := range p {
		if v == nil {
			return z, geom.ErrInvalid
		}
		if v.Num().BitLen() > o.MaxCoefficientBits || v.Denom().BitLen() > o.MaxCoefficientBits {
			return z, geom.ErrConvergence
		}
	}
	var out MixedRoofMaterialPoint
	for i, b := range q.box {
		d := b.domain
		a, c := [2]float64{d.UMin, d.VMin}, [2]float64{d.UMax, d.VMax}
		side, outside := false, false
		for j := range a {
			lo, hi := p[j].Cmp(ratInset(a[j])), p[j].Cmp(ratInset(c[j]))
			outside = outside || lo < 0 || hi > 0
			side = side || lo == 0 || hi == 0
		}
		floor := p[2].Cmp(b.floor)
		if outside || floor < 0 {
			continue
		}
		opt := o
		opt.MaxOperations -= out.ExactOperations
		if opt.MaxOperations <= 0 {
			return z, geom.ErrConvergence
		}
		j, err := b.roof.EvaluateRational(ctx, p[0], p[1], opt)
		if err != nil {
			return z, err
		}
		out.ExactOperations += j.ExactOperations
		roof := p[2].Cmp(j.Point[2])
		if roof > 0 {
			continue
		}
		if side || floor == 0 || roof == 0 {
			out.Operands[i] = parametric.Boundary
		} else {
			out.Operands[i] = parametric.Inside
		}
	}
	if err := ctx.Err(); err != nil {
		return z, err
	}
	a, b := out.Operands[0], out.Operands[1]
	inside, boundary := parametric.Inside, parametric.Boundary
	switch q.source.boundary.op {
	case parametric.Union:
		if a == inside || b == inside {
			out.Location = inside
		} else if a == boundary || b == boundary {
			out.Location = boundary
		}
	case parametric.Difference:
		if a != parametric.Outside && b != inside {
			out.Location = inside
			if a == boundary || b == boundary {
				out.Location = boundary
			}
		}
	case parametric.Intersection:
		if a != parametric.Outside && b != parametric.Outside {
			out.Location = inside
			if a == boundary || b == boundary {
				out.Location = boundary
			}
		}
	default:
		return z, geom.ErrInvalid
	}
	return out, nil
}
