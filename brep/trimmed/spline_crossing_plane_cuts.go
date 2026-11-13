package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// SplineCapIntersection retains exact original cutter-chart intervals and model
// endpoints in the returned edge's Start-to-End direction. The UV segment is
// UV[k][0]+t*(UV[k][1]-UV[k][0]), 0<=t<=1, on original cutter face Faces[k].
// Returned rationals are caller-owned and are not retained by any proof object.
type SplineCapIntersection struct {
	Edge   int
	Faces  [2]int
	Points [2][3]*big.Rat
	UV     [2][2][2]*big.Rat
}

// SplineCrossingPlaneCutsResult extends the shared two-cut ancestry with one
// CapIntersectionEdges entry and its exact two-chart parameter correspondence.
type SplineCrossingPlaneCutsResult = SplineParallelSlab

// CombineSplineCrossingPlaneCuts intersects stock with two active halfspaces
// whose exact convex cap polygons cross transversely in one segment. Every cap
// vertex must be strictly off the opposite plane; precisely two cap edges cross.
// Both independently constructed model segments must agree exactly. Each cut
// otherwise satisfies CombineSplinePlaneClip. Crossing original curved faces,
// tangent/coincident cap events and unrepresentable new vertices are rejected.
// CapIntersectionEdges is distinct from stock ancestry and both SectionEdges.
func CombineSplineCrossingPlaneCuts(ctx context.Context, stock Solid, tools [2]Solid, active [2]int, o SplineRectangleEmbeddingOptions) (SplineCrossingPlaneCutsResult, error) {
	return combineSplinePlaneCuts(ctx, stock, tools, active, o, false, true)
}

func clipSplineCrossingCaps(ctx context.Context, clips [2]SplinePlaneClipResult, active [2]int, patches [2]*geom.SurfacePatch, origins, normals [2][3]*big.Rat, o SplineRectangleEmbeddingOptions) ([2]SewFace, *SplineCapIntersection, error) {
	var empty [2]SewFace
	var parts [2]SewFace
	proof := &SplineCapIntersection{Edge: -1, Faces: active}
	var points [2][2][3]*big.Rat
	for k := 0; k < 2; k++ {
		cap := clips[k].Solid.faces[clips[k].ToolFaces[active[k]][0]]
		uv := slabFaceUV(cap)
		if !slabConvexCCW(uv) {
			return empty, nil, geom.ErrConvergence
		}
		heights := make([]*big.Rat, len(uv))
		eval := func(q [2]*big.Rat) ([3]*big.Rat, error) {
			j, e := patches[k].EvaluateRational(ctx, q[0], q[1], geom.SurfacePatchEvaluationOptions{MaxOperations: o.Patches.MaxOperations, MaxCoefficientBits: o.Patches.MaxCoefficientBits})
			return j.Point, e
		}
		for i, q := range uv {
			p, e := eval(q)
			if e != nil {
				return empty, nil, e
			}
			h := new(big.Rat)
			for axis := 0; axis < 3; axis++ {
				h.Add(h, new(big.Rat).Mul(normals[1-k][axis], new(big.Rat).Sub(p[axis], origins[1-k][axis])))
			}
			if h.Sign() == 0 {
				return empty, nil, geom.ErrConvergence
			}
			heights[i] = h
		}
		var polygon [][2]*big.Rat
		count := 0
		for i, a := range uv {
			j := (i + 1) % len(uv)
			b := uv[j]
			ha, hb := heights[i], heights[j]
			if ha.Sign() < 0 {
				polygon = append(polygon, a)
			}
			if ha.Sign()*hb.Sign() < 0 {
				if count == 2 {
					return empty, nil, geom.ErrConvergence
				}
				alpha := new(big.Rat).Quo(ha, new(big.Rat).Sub(ha, hb))
				var q [2]*big.Rat
				for axis := range q {
					q[axis] = new(big.Rat).Add(a[axis], new(big.Rat).Mul(alpha, new(big.Rat).Sub(b[axis], a[axis])))
				}
				p, e := eval(q)
				if e != nil {
					return empty, nil, e
				}
				points[k][count] = p
				proof.UV[k][count] = q
				count++
				polygon = append(polygon, q)
			}
		}
		if count != 2 || !slabConvexCCW(polygon) {
			return empty, nil, geom.ErrConvergence
		}
		var e error
		parts[k], e = slabSewFace(ctx, cap, patches[k], polygon, o)
		if e != nil {
			return empty, nil, e
		}
	}
	equal := func(a, b [3]*big.Rat) bool {
		for i := range a {
			if a[i].Cmp(b[i]) != 0 {
				return false
			}
		}
		return true
	}
	if equal(points[0][0], points[0][1]) {
		return empty, nil, geom.ErrConvergence
	}
	if !equal(points[0][0], points[1][0]) {
		points[1][0], points[1][1] = points[1][1], points[1][0]
		proof.UV[1][0], proof.UV[1][1] = proof.UV[1][1], proof.UV[1][0]
	}
	if !equal(points[0][0], points[1][0]) || !equal(points[0][1], points[1][1]) {
		return empty, nil, geom.ErrConvergence
	}
	proof.Points = points[0]
	return parts, proof, nil
}
