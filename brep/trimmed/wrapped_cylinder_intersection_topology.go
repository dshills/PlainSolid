package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// WrappedCylinderIntersectionOptions shares the comparison arithmetic quota
// with quotient embedding and native conversion. Zero selects 8M operations,
// 16384 bits and 4096 poles. Comparison flags describe the original charts.
type WrappedCylinderIntersectionOptions struct {
	geom.ExactSurfaceComparisonOptions
	MaxPoles int
}

// WrappedCylinderRectangleSide refers to a CONCEPTUAL exact source rectangle
// boundary, not a preexisting B-rep edge. Owner is 0=A or 1=B. Side is 0=bottom,
// 1=right, 2=top, 3=left in that owner's CCW chart. Parameters are normalized
// [0,1] side coordinates in the stored output edge's direction. UV remains in
// the owner's original lifted chart and is never reduced modulo its period.
type WrappedCylinderRectangleSide struct {
	Owner, Side int
	Parameters  [2]*big.Rat
	UV          [2][2]*big.Rat
}

// WrappedCylinderIntersectionTopology contains two independently embedded,
// physically disjoint native open disk graphs. It is not a Solid, closed
// shell, or materialized annular union. Both use A's original support and
// outward canonical orientation. No input Face.Reversed flags are supplied;
// OwnerOrientations retains the signed chart correspondence. Graph
// EdgeAncestry is empty because no
// native SewFace inputs were supplied: use RectangleBoundaryAncestry instead.
// Exact nonbinary source data is retained, but any unrepresentable native A
// corner or model vertex rejects the whole result without rounding.
type WrappedCylinderIntersectionTopology struct {
	components   [2]*WrappedFaceRegionTopology
	ancestry     [2][][]WrappedCylinderRectangleSide
	shifts       [2][2]*big.Int
	orientations [2]int
	operations   int
}

func (r *WrappedCylinderIntersectionTopology) Components() [2]*WrappedFaceRegionTopology {
	if r == nil {
		return [2]*WrappedFaceRegionTopology{}
	}
	return r.components
}
func (r *WrappedCylinderIntersectionTopology) OwnerOrientations() [2]int {
	if r == nil {
		return [2]int{}
	}
	return r.orientations
}
func (r *WrappedCylinderIntersectionTopology) ExactOperations() int {
	if r == nil {
		return 0
	}
	return r.operations
}
func (r *WrappedCylinderIntersectionTopology) PeriodShifts() [2][2]*big.Int {
	var out [2][2]*big.Int
	if r == nil {
		return out
	}
	for i := range out {
		for axis := range out[i] {
			out[i][axis] = new(big.Int).Set(r.shifts[i][axis])
		}
	}
	return out
}
func (r *WrappedCylinderIntersectionTopology) RectangleBoundaryAncestry(component int) [][]WrappedCylinderRectangleSide {
	if r == nil || component < 0 || component >= 2 {
		return nil
	}
	out := make([][]WrappedCylinderRectangleSide, len(r.ancestry[component]))
	for i, list := range r.ancestry[component] {
		for _, p := range list {
			for end := 0; end < 2; end++ {
				p.Parameters[end] = new(big.Rat).Set(p.Parameters[end])
				for axis := 0; axis < 2; axis++ {
					p.UV[end][axis] = new(big.Rat).Set(p.UV[end][axis])
				}
			}
			out[i] = append(out[i], p)
		}
	}
	return out
}

// NewWrappedCylinderIntersectionTopology re-proves the source relation and
// complete two-component quotient overlap; it never trusts a mutable Complete
// report. The supports and exact rectangles are the source geometry. The two
// output graphs keep separate vertex/edge indices: their model images are
// proved disjoint by global cylinder embedding, not merged by rounded points.
func NewWrappedCylinderIntersectionTopology(ctx context.Context, a, b *geom.WrappedSurfacePatch, rectA, rectB [2][2]*big.Rat, o WrappedCylinderIntersectionOptions) (*WrappedCylinderIntersectionTopology, error) {
	if ctx == nil {
		return nil, geom.ErrInvalid
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
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxPoles < 4 {
		return nil, geom.ErrInvalid
	}
	relation, comparison, e := geom.CompareWrappedSurfaceCharts(ctx, a, b, o.ExactSurfaceComparisonOptions)
	if e != nil {
		return nil, e
	}
	if relation == nil || !comparison.Complete || !comparison.Equal {
		return nil, geom.ErrConvergence
	}
	used := comparison.ExactOperations
	if used >= o.MaxOperations {
		return nil, geom.ErrConvergence
	}
	proof, e := relation.CertifyCylinderTwoRectangleOverlap(ctx, rectA, rectB, geom.WrappedCylinderTwoOverlapOptions{WrappedRectangleOverlapOptions: geom.WrappedRectangleOverlapOptions{MaxOperations: o.MaxOperations - used, MaxCoefficientBits: o.MaxCoefficientBits, MaxComponents: 2}, MaxPoles: o.MaxPoles})
	if e != nil {
		return nil, e
	}
	used += proof.ExactOperations
	result := &WrappedCylinderIntersectionTopology{orientations: proof.OwnerOrientations}
	check := func(q *big.Rat) error {
		used++
		if used > o.MaxOperations || q.Num().BitLen() > o.MaxCoefficientBits || q.Denom().BitLen() > o.MaxCoefficientBits {
			return geom.ErrConvergence
		}
		return ctx.Err()
	}
	rects := [2][2][2]*big.Rat{rectA, rectB}
	for ci, component := range proof.Components {
		graph := &WrappedFaceRegionTopology{ancestry: make([][]WrappedFaceFragmentOwner, 4)}
		for axis, k := range component.Overlap.TargetPeriodShift {
			result.shifts[ci][axis] = new(big.Int).Set(k)
		}
		loops := [2][4][2]*big.Rat{component.ALoop, component.BLoop}
		var corners [4]geom.Vec2
		for i, uv := range component.ALoop {
			corners[i], e = saddleRepresentedUV(uv)
			if e != nil {
				return nil, e
			}
			if used >= o.MaxOperations {
				return nil, geom.ErrConvergence
			}
			jet, err := a.EvaluateRational(ctx, uv[0], uv[1], geom.SurfacePatchEvaluationOptions{MaxOperations: o.MaxOperations - used, MaxCoefficientBits: o.MaxCoefficientBits})
			if err != nil {
				return nil, err
			}
			used += jet.ExactOperations
			point, err := geom.RepresentedExactPoint(jet.Point)
			if err != nil {
				return nil, err
			}
			graph.vertices = append(graph.vertices, point)
		}
		face := Face{Surface: a}
		result.ancestry[ci] = make([][]WrappedCylinderRectangleSide, 4)
		for side, start := range corners {
			end := corners[(side+1)%4]
			graph.edges = append(graph.edges, parametric.Edge{Start: side, End: (side + 1) % 4, Curve: surfaceTrimCurve{surface: a, a: start, b: end}, Range: parametric.Interval{Max: 1}})
			face.Outer = append(face.Outer, Use{Edge: side, UVStart: start, UVEnd: end})
			for owner, rect := range rects {
				source := [4][2]*big.Rat{{rect[0][0], rect[1][0]}, {rect[0][1], rect[1][0]}, {rect[0][1], rect[1][1]}, {rect[0][0], rect[1][1]}}
				uv := [2][2]*big.Rat{loops[owner][side], loops[owner][(side+1)%4]}
				for original, s := range source {
					t := source[(original+1)%4]
					axis := 0
					if s[0].Cmp(t[0]) == 0 {
						axis = 1
					}
					fixed := axis ^ 1
					if uv[0][fixed].Cmp(s[fixed]) != 0 || uv[1][fixed].Cmp(s[fixed]) != 0 {
						continue
					}
					var parameters [2]*big.Rat
					inside := true
					for end := 0; end < 2; end++ {
						q := new(big.Rat).Quo(new(big.Rat).Sub(uv[end][axis], s[axis]), new(big.Rat).Sub(t[axis], s[axis]))
						if e := check(q); e != nil {
							return nil, e
						}
						if q.Sign() < 0 || q.Cmp(big.NewRat(1, 1)) > 0 {
							inside = false
							break
						}
						parameters[end] = q
					}
					if inside {
						result.ancestry[ci][side] = append(result.ancestry[ci][side], WrappedCylinderRectangleSide{Owner: owner, Side: original, Parameters: parameters, UV: uv})
					}
				}
			}
			if len(result.ancestry[ci][side]) == 0 {
				return nil, geom.ErrConvergence
			}
		}
		graph.faces = []Face{face}
		graph.cells = []WrappedRegionCellAncestry{{SourceCell: ci, Owners: 3, OwnerUV: loops}}
		cycles, err := validateWrappedOpenRegionCycles(ctx, 4, graph.edges, graph.faces, 1)
		if err != nil {
			return nil, err
		}
		graph.boundaries = cycles
		graph.exterior = cycles[0]
		result.components[ci] = graph
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	for _, g := range result.components {
		g.operations = used
	}
	result.operations = used
	return result, nil
}
