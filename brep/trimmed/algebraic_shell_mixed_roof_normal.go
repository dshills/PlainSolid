//go:build research

package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// MixedRoofNormal is one original support's outward normal at a proved
// material boundary point, with cutter supports reversed for Difference.
// Direction is an exact nonzero vector; it is not normalized. Multiple
// original normals describe incident supports at seams or corners. At a
// piecewise crease the same owner may have distinct one-sided normals.
type MixedRoofNormal struct {
	Owner     MixedRoofFaceOwner
	Direction [3]*big.Rat
}

// MixedRoofBoundaryNormals classifies the point and lists each exposed
// original support's normal if the point is on the material boundary.
type MixedRoofBoundaryNormals struct {
	Location        parametric.Location
	Normals         []MixedRoofNormal
	ExactOperations int
}

// BoundaryNormalsRational evaluates outward normals on the original rational
// roof, floor and side supports. Buried supports do not emit normals.
func (q *MixedRoofMaterialQuery) BoundaryNormalsRational(ctx context.Context, p [3]*big.Rat, o geom.SurfacePatchEvaluationOptions) (MixedRoofBoundaryNormals, error) {
	var zero MixedRoofBoundaryNormals
	o, err := q.evaluationOptions(ctx, o, p[:]...)
	if err != nil {
		return zero, err
	}
	location, err := q.LocateRational(ctx, p, o)
	if err != nil {
		return zero, err
	}
	out := MixedRoofBoundaryNormals{Location: location.Location, ExactOperations: location.ExactOperations}
	if location.Location != parametric.Boundary {
		return out, nil
	}
	for i, where := range location.Operands {
		if where != parametric.Boundary {
			continue
		}
		b := q.box[i]
		_, sides := b.footprintSides([2]*big.Rat{p[0], p[1]})
		opt := o
		opt.MaxOperations -= out.ExactOperations
		if opt.MaxOperations < 1 {
			return zero, geom.ErrConvergence
		}
		j, err := b.roof.EvaluateRational(ctx, p[0], p[1], opt)
		if err != nil {
			return zero, err
		}
		out.ExactOperations += j.ExactOperations
		add := func(face int, x, y, z *big.Rat) {
			d := [3]*big.Rat{x, y, z}
			if i == 1 && q.op == parametric.Difference {
				for _, v := range d {
					v.Neg(v)
				}
			}
			out.Normals = append(out.Normals, MixedRoofNormal{MixedRoofFaceOwner{i, face}, d})
		}
		if p[2].Cmp(b.floor) == 0 {
			add(b.floorFace, new(big.Rat), new(big.Rat), big.NewRat(-1, 1))
		}
		if p[2].Cmp(j.Point[2]) == 0 {
			if len(b.roofSpans) == 0 {
				add(b.roofFace, new(big.Rat).Neg(j.Du[2]), new(big.Rat).Neg(j.Dv[2]), big.NewRat(1, 1))
			} else {
				var slopes [][2]*big.Rat
				for _, span := range b.roofSpans {
					if span.footprint([2]*big.Rat{p[0], p[1]}) == parametric.Outside {
						continue
					}
					opt := o
					opt.MaxOperations -= out.ExactOperations
					if opt.MaxOperations < 1 {
						return zero, geom.ErrConvergence
					}
					v, err := span.roof.EvaluateRational(ctx, p[0], p[1], opt)
					if err != nil {
						return zero, err
					}
					out.ExactOperations += v.ExactOperations
					if v.Point[2].Cmp(p[2]) != 0 {
						return zero, geom.ErrUnrepresentable
					}
					same := false
					for _, d := range slopes {
						same = same || v.Du[2].Cmp(d[0]) == 0 && v.Dv[2].Cmp(d[1]) == 0
					}
					if !same {
						slopes = append(slopes, [2]*big.Rat{v.Du[2], v.Dv[2]})
						add(b.roofFace, new(big.Rat).Neg(v.Du[2]), new(big.Rat).Neg(v.Dv[2]), big.NewRat(1, 1))
					}
				}
			}
		}
		for k, face := range b.sideFaces {
			if sides&(1<<k) == 0 {
				continue
			}
			d := [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}
			d[k/2].SetInt64(int64(2*(k%2) - 1))
			add(face, d[0], d[1], d[2])
		}
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	if len(out.Normals) == 0 {
		return zero, geom.ErrConvergence
	}
	return out, nil
}

// BoundaryNormalsRational evaluates exact outward normals at a placed world
// point. Translation affects the query position only.
func (q PlacedRoofMaterialQuery) BoundaryNormalsRational(ctx context.Context, world [3]*big.Rat, o geom.SurfacePatchEvaluationOptions) (MixedRoofBoundaryNormals, error) {
	var zero MixedRoofBoundaryNormals
	o, err := q.source.evaluationOptions(ctx, o, world[:]...)
	if err != nil {
		return zero, err
	}
	o.MaxOperations -= roofPosePointOperations
	if o.MaxOperations < 1 {
		return zero, geom.ErrConvergence
	}
	p, err := q.pose.TransformPoint(ctx, world, true, o.MaxCoefficientBits)
	if err != nil {
		return zero, err
	}
	out, err := q.source.BoundaryNormalsRational(ctx, p, o)
	if err != nil {
		return zero, err
	}
	cost := len(out.Normals) * roofPoseVectorOperations
	if o.MaxOperations-out.ExactOperations < cost {
		return zero, geom.ErrConvergence
	}
	for i, n := range out.Normals {
		out.Normals[i].Direction, err = q.pose.TransformVector(ctx, n.Direction, false, o.MaxCoefficientBits)
		if err != nil {
			return zero, err
		}
	}
	if err = ctx.Err(); err != nil {
		return zero, err
	}
	out.ExactOperations += roofPosePointOperations + cost
	return out, nil
}
