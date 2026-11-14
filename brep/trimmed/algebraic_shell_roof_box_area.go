package trimmed

import (
	"context"
	"fmt"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// MixedRoofAreaOptions bounds strict nested or separated roof-box boundary
// integration. The default absolute gap is 1/10 square model unit, with 32768
// evaluated cells and four million exact operations for all retained faces.
type MixedRoofAreaOptions struct {
	geom.SurfacePatchEvaluationOptions
	AbsoluteGap        float64
	MaxDepth, MaxCells int
}

// MixedRoofArea holds total and original face area intervals. A deleted face
// has exact zero area. A retained cutter face is reversed for Difference.
type MixedRoofArea struct {
	Area                   geom.ExactParameterInterval
	Faces                  []MixedRoofAreaFace
	Cells, ExactOperations int
}

type MixedRoofAreaFace struct {
	Owner              MixedRoofFaceOwner
	Retained, Reversed bool
	Area               geom.ExactParameterInterval
}

// BoundStrictSurfaceArea first proves strict containment or separation of the
// original material using exact support hull comparisons. In these cases every
// retained original face is whole. Each such face contributes its certified
// parametric area. Crossing or unproved relations return ErrUnrepresentable;
// exhausting the integration budget or cancellation publishes no result.
func (q *MixedRoofMaterialQuery) BoundStrictSurfaceArea(ctx context.Context, o MixedRoofAreaOptions) (MixedRoofArea, error) {
	var zero MixedRoofArea
	if o.MaxOperations == 0 {
		o.MaxOperations = 4000000
	}
	var err error
	o.SurfacePatchEvaluationOptions, err = q.evaluationOptions(ctx, o.SurfacePatchEvaluationOptions)
	if err != nil {
		return zero, err
	}
	if o.AbsoluteGap == 0 {
		o.AbsoluteGap = .1
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 24
	}
	if o.MaxCells == 0 {
		o.MaxCells = 32768
	}
	if !finite(o.AbsoluteGap) || o.AbsoluteGap <= 0 || o.MaxDepth < 1 || o.MaxDepth > 64 || o.MaxCells < 1 || o.MaxCells > 262144 {
		return zero, geom.ErrInvalid
	}
	var hull [2]geom.Box
	for i, b := range q.box {
		hull[i], err = b.roof.Bounds()
		if err != nil {
			return zero, err
		}
	}
	var kept, reverse [2]bool
	strictlyInside := func(k, j int) bool {
		a, b := q.box[k], q.box[j]
		return a.domain.UMin > b.domain.UMin && a.domain.UMax < b.domain.UMax && a.domain.VMin > b.domain.VMin && a.domain.VMax < b.domain.VMax &&
			a.floor.Cmp(b.floor) > 0 && hull[k].Max.Z < hull[j].Min.Z
	}
	a, b := q.box[0], q.box[1]
	disjoint := a.domain.UMax < b.domain.UMin || b.domain.UMax < a.domain.UMin || a.domain.VMax < b.domain.VMin || b.domain.VMax < a.domain.VMin ||
		ratInset(hull[0].Max.Z).Cmp(b.floor) < 0 || ratInset(hull[1].Max.Z).Cmp(a.floor) < 0
	if disjoint {
		kept = [2]bool{q.op != parametric.Intersection, q.op == parametric.Union}
	} else {
		inner := -1
		for i := range 2 {
			if strictlyInside(i, 1-i) {
				inner = i
				break
			}
		}
		if inner < 0 {
			return zero, geom.ErrUnrepresentable
		}
		switch q.op {
		case parametric.Intersection:
			kept[inner] = true
		case parametric.Union:
			kept[1-inner] = true
		case parametric.Difference:
			if inner == 1 {
				kept = [2]bool{true, true}
				reverse[1] = true
			}
		}
	}
	count := 0
	for _, use := range kept {
		if use {
			count += 6
		}
	}
	out := MixedRoofArea{Area: mixedRoofZeroInterval(), Faces: make([]MixedRoofAreaFace, 0, 12)}
	for i, b := range q.box {
		for j := range b.patches {
			row := MixedRoofAreaFace{Owner: MixedRoofFaceOwner{i, j}, Retained: kept[i], Reversed: reverse[i], Area: mixedRoofZeroInterval()}
			if kept[i] && j == b.floorFace {
				if o.MaxOperations-out.ExactOperations < 5 || out.Cells >= o.MaxCells {
					return zero, geom.ErrConvergence
				}
				d := b.domain
				x := new(big.Rat).Sub(ratInset(d.UMax), ratInset(d.UMin))
				y := new(big.Rat).Sub(ratInset(d.VMax), ratInset(d.VMin))
				v := new(big.Rat).Mul(x, y)
				if !mixedRoofFits(x, o.MaxCoefficientBits) || !mixedRoofFits(y, o.MaxCoefficientBits) || !mixedRoofFits(v, o.MaxCoefficientBits) {
					return zero, geom.ErrConvergence
				}
				row.Area.Lower.Set(v)
				row.Area.Upper.Set(v)
				out.ExactOperations += 5
				out.Cells++
				out.Area = addBooleanMeasureIntervals(out.Area, row.Area)
			} else if kept[i] && j == b.roofFace {
				opt := o
				opt.MaxCells -= out.Cells
				opt.MaxOperations -= out.ExactOperations
				var cells, operations int
				row.Area, cells, operations, err = mixedRoofGraphArea(ctx, b, o.AbsoluteGap/float64(count), opt)
				if err != nil {
					return zero, fmt.Errorf("measure original roof %d: %w", i, err)
				}
				out.Cells += cells
				out.ExactOperations += operations
				out.Area = addBooleanMeasureIntervals(out.Area, row.Area)
			} else if kept[i] {
				if out.Cells >= o.MaxCells || out.ExactOperations >= o.MaxOperations {
					return zero, geom.ErrConvergence
				}
				side := -1
				for k, id := range b.sideFaces {
					if id == j {
						side = k
						break
					}
				}
				if side < 0 {
					return zero, geom.ErrUnrepresentable
				}
				opt := o
				opt.MaxCells -= out.Cells
				opt.MaxOperations -= out.ExactOperations
				var cells, operations int
				row.Area, cells, operations, err = mixedRoofWallArea(ctx, b, side, o.AbsoluteGap/float64(count), opt)
				if err != nil {
					return zero, fmt.Errorf("measure original wall %d:%d: %w", i, j, err)
				}
				out.ExactOperations += operations
				out.Cells += cells
				out.Area = addBooleanMeasureIntervals(out.Area, row.Area)
			}
			if out.ExactOperations > o.MaxOperations || out.Cells > o.MaxCells || !mixedRoofFits(out.Area.Lower, o.MaxCoefficientBits) || !mixedRoofFits(out.Area.Upper, o.MaxCoefficientBits) {
				return zero, geom.ErrConvergence
			}
			out.Faces = append(out.Faces, row)
		}
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	if new(big.Rat).Sub(out.Area.Upper, out.Area.Lower).Cmp(ratInset(o.AbsoluteGap)) > 0 {
		return zero, geom.ErrConvergence
	}
	return out, nil
}

// BoundStrictSurfaceArea is invariant under the exact proper rigid pose.
func (q PlacedRoofMaterialQuery) BoundStrictSurfaceArea(ctx context.Context, o MixedRoofAreaOptions) (MixedRoofArea, error) {
	return q.source.BoundStrictSurfaceArea(ctx, o)
}
