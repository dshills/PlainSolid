//go:build research

package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep"
	"plainsolid/csg/poly"
	"plainsolid/geom"
)

// AffineSplinePlaneSplitSide maps original faces and edges to fragments on
// one side of the cut. CapFaces identifies the new affine spline faces.
// PlanarEdges preserves exact retained parameter intervals; a generated
// interval can be a cap boundary or a conforming triangulation diagonal.
type AffineSplinePlaneSplitSide struct {
	Faces, Edges [][]int
	CapFaces     []int
	PlanarEdges  []poly.EdgeAncestry
}

// AffineSplinePlaneSplitHistory uses output-local IDs on each side.
type AffineSplinePlaneSplitHistory struct {
	Negative, Positive AffineSplinePlaneSplitSide
}

// SplitAffineSplineShellPlane partitions an embedded affine spline solid by
// the exact represented origin and nonzero (unnormalized) normal. Original
// support charts are retained, while generated cap triangles use affine
// bilinear charts on the cutter. A split requiring rounded model or UV
// coordinates is rejected, including an unrepresentable cap chart corner.
// Outputs can be used in other affine spline operations. The original solid
// is returned directly on its occupied side when the plane misses or touches
// the boundary without cutting material.
func SplitAffineSplineShellPlane(ctx context.Context, s Solid, origin, normal geom.Vec3, o AffineSplineShellBooleanOptions) (Solid, Solid, AffineSplinePlaneSplitHistory, error) {
	empty := Solid{}
	var h AffineSplinePlaneSplitHistory
	if ctx == nil || !origin.IsFinite() || !normal.IsFinite() || normal == (geom.Vec3{}) {
		return empty, empty, h, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, empty, h, e
	}
	planar, charts, e := affineSplinePlanarBoundary(ctx, s, o)
	if e != nil {
		return empty, empty, h, e
	}
	o.Planar.RequireExactConversion = true
	a, b, history, e := poly.SplitPlaneWithHistory(ctx, planar, origin, normal, o.Planar)
	if e != nil {
		return empty, empty, h, e
	}
	lift := func(p brep.Solid, proof poly.PlaneSplitSideHistory, split *AffineSplinePlaneSplitSide) (Solid, error) {
		split.Faces, split.Edges = make([][]int, len(s.faces)), make([][]int, len(s.edges))
		split.PlanarEdges = proof.Edges
		for j, a := range proof.Edges {
			seen := map[int]bool{}
			for _, x := range a.Overlaps {
				if !seen[x.Edge] {
					split.Edges[x.Edge] = append(split.Edges[x.Edge], j)
					seen[x.Edge] = true
				}
			}
		}
		for j, src := range proof.Faces {
			if src.Operand == poly.OperandA {
				split.Faces[src.Face] = append(split.Faces[src.Face], j)
			} else {
				split.CapFaces = append(split.CapFaces, j)
			}
		}
		if len(proof.Faces) == 0 {
			return empty, nil
		}
		if proof.Faces[0].Operand == poly.OperandA && proof.Faces[0].Triangle == -1 {
			return s, nil
		}
		return liftAffinePlanarBoundary(ctx, p, s.tolerance, o,
			func(i int, corners []geom.Vec3) (Face, *geom.AffineSurfaceChart, error) {
				src := proof.Faces[i]
				if src.Operand == poly.OperandA {
					f := s.faces[src.Face]
					return Face{Surface: f.Surface, Reversed: f.Reversed != src.Reversed}, charts[src.Face], nil
				}
				return affineCapTriangle(ctx, corners, o)
			})
	}
	x, e := lift(a, history.Negative, &h.Negative)
	if e != nil {
		return empty, empty, AffineSplinePlaneSplitHistory{}, e
	}
	y, e := lift(b, history.Positive, &h.Positive)
	if e != nil {
		return empty, empty, AffineSplinePlaneSplitHistory{}, e
	}
	return x, y, h, nil
}

func affineCapTriangle(ctx context.Context, corners []geom.Vec3, o AffineSplineShellBooleanOptions) (Face, *geom.AffineSurfaceChart, error) {
	if len(corners) != 3 {
		return Face{}, nil, geom.ErrInvalid
	}
	p, q, r := corners[0], corners[1], corners[2]
	fourth := func(a, b, c float64) *big.Rat {
		return new(big.Rat).Sub(new(big.Rat).Add(ratInset(b), ratInset(c)), ratInset(a))
	}
	s, e := geom.RepresentedExactPoint([3]*big.Rat{fourth(p.X, q.X, r.X), fourth(p.Y, q.Y, r.Y), fourth(p.Z, q.Z, r.Z)})
	if e != nil {
		return Face{}, nil, e
	}
	k := []float64{0, 0, 1, 1}
	surface, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{p, r, q, s}, k, k, nil)
	if e != nil {
		return Face{}, nil, e
	}
	patch, e := geom.NewSurfacePatch(surface, o.Patches)
	if e != nil {
		return Face{}, nil, e
	}
	chart, e := geom.NewAffineSurfaceChart(ctx, patch, o.Charts)
	if e != nil {
		return Face{}, nil, e
	}
	return Face{Surface: surface}, chart, nil
}
