package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// MixedRoofMaterialQuery contains the independently proved original roof boxes
// used by a mixed Boolean. It supports exact rational points and vertical rows
// in the original XY chart units. Both operands satisfy the six-face graph-box
// proof.
type MixedRoofMaterialQuery struct {
	op   parametric.BooleanOp
	box  [2]*mixedRoofBox
	work int
}

// MixedRoofQueryOptions bounds preparation separately from later evaluations.
// MaxPreparationOperations defaults to 2M (maximum 16M).
type MixedRoofQueryOptions struct{ MaxPreparationOperations int }

// PrepareRoofMaterialQuery detects the unique upward original graph support of
// each operand, then proves that each original shell is an embedded roof box.
// It uses the original immutable Boolean and requires no caller face IDs.
func (s *MixedAlgebraicShellBoolean) PrepareRoofMaterialQuery(ctx context.Context, o MixedRoofQueryOptions) (*MixedRoofMaterialQuery, error) {
	if s == nil || s.section == nil {
		return nil, geom.ErrInvalid
	}
	return s.boundary.PrepareRoofMaterialQuery(ctx, o)
}

// PrepareRoofMaterialQuery detects and proves both original graph boxes of
// the complete regular Boolean. Public diagnostic face and edge tables
// are not used as proof inputs.
func (s AlgebraicShellBoolean) PrepareRoofMaterialQuery(ctx context.Context, o MixedRoofQueryOptions) (*MixedRoofMaterialQuery, error) {
	if s.source == nil || ctx == nil || o.MaxPreparationOperations < 0 || o.MaxPreparationOperations > 16000000 {
		return nil, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if o.MaxPreparationOperations == 0 {
		o.MaxPreparationOperations = 2000000
	}
	q := &MixedRoofMaterialQuery{op: s.op}
	for k, body := range [2]Solid{s.source.a, s.source.b} {
		if len(body.faces) != 6 {
			return nil, geom.ErrUnrepresentable
		}
		roof := -1
		left := o.MaxPreparationOperations - q.work
		// Among outward faces only the roof can have identity model XY:
		// the floor is reversed and every side has a constant X or Y.
		for i, f := range body.faces {
			if f.Reversed {
				continue
			}
			if left < 1 {
				return nil, geom.ErrConvergence
			}
			p, err := exactFaceSupportPatch(f, geom.SurfacePatchOptions{MaxOperations: left, MaxCoefficientBits: 4096})
			if err != nil {
				return nil, err
			}
			left -= p.ExactOperations()
			matched := true
			for axis := 0; axis < 2; axis++ {
				if left < 1 {
					return nil, geom.ErrConvergence
				}
				proof, err := geom.CertifySurfaceAffineCoordinate(ctx, p, axis, axis, big.NewRat(1, 1), new(big.Rat),
					geom.SurfaceAffineCoordinateOptions{SurfacePatchOptions: geom.SurfacePatchOptions{MaxOperations: left, MaxCoefficientBits: 4096}})
				if err != nil {
					return nil, err
				}
				left -= proof.ExactOperations
				if !proof.Proven {
					matched = false
					break
				}
			}
			if matched {
				if roof >= 0 {
					return nil, geom.ErrUnrepresentable
				}
				roof = i
			}
		}
		if left <= 0 {
			return nil, geom.ErrConvergence
		}
		if roof < 0 {
			return nil, geom.ErrUnrepresentable
		}
		var err error
		q.box[k], err = proveMixedRoofBoxMode(ctx, body, roof, left, true)
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
	o, err := q.evaluationOptions(ctx, o, p[:]...)
	if err != nil {
		return z, err
	}
	var out MixedRoofMaterialPoint
	for i, b := range q.box {
		where := b.footprint([2]*big.Rat{p[0], p[1]})
		floor := p[2].Cmp(b.floor)
		if where == parametric.Outside || floor < 0 {
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
		if where == parametric.Boundary || floor == 0 || roof == 0 {
			out.Operands[i] = parametric.Boundary
		} else {
			out.Operands[i] = parametric.Inside
		}
	}
	if err := ctx.Err(); err != nil {
		return z, err
	}
	out.Location, err = mixedRoofBooleanLocation(q.op, out.Operands)
	if err != nil {
		return z, err
	}
	return out, nil
}

func (q *MixedRoofMaterialQuery) evaluationOptions(ctx context.Context, o geom.SurfacePatchEvaluationOptions, inputs ...*big.Rat) (geom.SurfacePatchEvaluationOptions, error) {
	if ctx == nil || q == nil || q.box[0] == nil || q.box[1] == nil || o.MaxOperations < 0 || o.MaxOperations > 16000000 || o.MaxCoefficientBits < 0 || o.MaxCoefficientBits > 16384 || o.MaxCoefficientBits > 0 && o.MaxCoefficientBits < 64 {
		return o, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return o, err
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 4096
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 1000000
	}
	for _, v := range inputs {
		if v == nil {
			return o, geom.ErrInvalid
		}
		if !mixedRoofFits(v, o.MaxCoefficientBits) {
			return o, geom.ErrConvergence
		}
	}
	return o, nil
}

func mixedRoofFits(v *big.Rat, bits int) bool {
	return v.Num().BitLen() <= bits && v.Denom().BitLen() <= bits
}

func (b *mixedRoofBox) footprint(xy [2]*big.Rat) parametric.Location {
	where, _ := b.footprintSides(xy)
	return where
}

func (b *mixedRoofBox) footprintSides(xy [2]*big.Rat) (parametric.Location, uint8) {
	d := b.domain
	lo, hi := [2]float64{d.UMin, d.VMin}, [2]float64{d.UMax, d.VMax}
	where := parametric.Inside
	var sides uint8
	for j, v := range xy {
		a, c := v.Cmp(ratInset(lo[j])), v.Cmp(ratInset(hi[j]))
		if a < 0 || c > 0 {
			return parametric.Outside, 0
		}
		if a == 0 {
			sides |= 1 << (2 * j)
			where = parametric.Boundary
		}
		if c == 0 {
			sides |= 1 << (2*j + 1)
			where = parametric.Boundary
		}
	}
	return where, sides
}

// Both original surfaces at a simultaneous contact are part of the complete
// regular section. The admissible source proof excludes coincident and
// non-transverse face contacts.
func mixedRoofBooleanLocation(op parametric.BooleanOp, operands [2]parametric.Location) (parametric.Location, error) {
	a, b := operands[0], operands[1]
	inside, boundary := parametric.Inside, parametric.Boundary
	switch op {
	case parametric.Union:
		if a == inside || b == inside {
			return inside, nil
		} else if a == boundary || b == boundary {
			return boundary, nil
		}
	case parametric.Difference:
		if a != parametric.Outside && b != inside {
			if a == boundary || b == boundary {
				return boundary, nil
			}
			return inside, nil
		}
	case parametric.Intersection:
		if a != parametric.Outside && b != parametric.Outside {
			if a == boundary || b == boundary {
				return boundary, nil
			}
			return inside, nil
		}
	default:
		return parametric.Outside, geom.ErrInvalid
	}
	return parametric.Outside, nil
}
