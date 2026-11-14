package step

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// ClipIndependentCircularFrustumHierarchyLine clips the original line parameter
// against each retained body, not a regularized union. Complete coincident wall
// and cap intervals remain explicit boundary spans. Noncircular frusta fail.
func ClipIndependentCircularFrustumHierarchyLine(ctx context.Context, m IndependentFrustumHierarchy, origin, direction [3]*big.Rat, lower, upper *big.Rat, o RationalHierarchyLineOptions) (RationalHierarchyLine, error) {
	view := RationalSphericalHierarchy{Source: m.Assembly.Source, Products: m.Products, LengthUnit: m.Assembly.LengthUnit}
	if len(m.Instances) > 64 {
		return RationalHierarchyLine{}, geom.ErrInvalid
	}
	for _, u := range m.Instances {
		if len(u.Path) < 1 || len(u.Path) > 8 || !planarSplineRigid(u.Transform) {
			return RationalHierarchyLine{}, geom.ErrInvalid
		}
		b := mappedRationalBasis{}
		for i := 0; i < 3; i++ {
			b.Origin[i] = new(big.Rat).SetFloat64(u.Transform[i][3])
			b.X[i] = new(big.Rat).SetFloat64(u.Transform[i][0])
			b.Y[i] = new(big.Rat).SetFloat64(u.Transform[i][1])
			b.Z[i] = new(big.Rat).SetFloat64(u.Transform[i][2])
		}
		view.Instances = append(view.Instances, RationalHierarchyInstance{ProductID: u.ProductID, Path: append([]int64(nil), u.Path...), pose: b})
	}
	return clipHierarchyLineWithLocations(ctx, view, origin, direction, lower, upper, o, func(q RationalHierarchyPointOptions) (RationalHierarchyPointLocations, error) {
		return LocateIndependentCircularFrustumHierarchyPoint(ctx, m, origin, q)
	}, true, true)
}
func clipHierarchyCircularFrustum(ctx context.Context, source *File, p IndependentExactMaterial, leaf HierarchyLineLeaf, x, d [3]*big.Rat, lower, upper *big.Rat, a *hierarchyPointArithmetic, out *RationalHierarchyLine, opts func() (geom.ExactCurvePlaneEventOptions, error), compare func(*geom.ExactAlgebraicRoot, *geom.ExactAlgebraicRoot) (int, error), addHit func(*geom.ExactAlgebraicRoot, HierarchyLineOwner) error) (HierarchyLineLeaf, error) {
	fail := func(e error) (HierarchyLineLeaf, error) { return HierarchyLineLeaf{}, e }
	q := p.Frustum.Embedding
	if !q.Complete || !q.Circular {
		return fail(geom.ErrDomain)
	}
	faces, e := circularFrustumSourceFaces(source, p, leaf.Owner.SourceFaceIDs)
	if e != nil {
		return fail(e)
	}
	h := a.sub(q.UpperZ, q.LowerZ)
	delta := a.sub(q.UpperRadius, q.LowerRadius)
	n0, n1 := a.add(a.mul(q.LowerRadius, h), a.mul(delta, x[2])), a.mul(delta, d[2])
	h2 := a.mul(h, h)
	radial := []*big.Rat{
		a.sub(a.mul(h2, a.add(a.mul(x[0], x[0]), a.mul(x[1], x[1]))), a.mul(n0, n0)),
		a.mul(big.NewRat(2, 1), a.sub(a.mul(h2, a.add(a.mul(x[0], d[0]), a.mul(x[1], d[1]))), a.mul(n0, n1))),
		a.sub(a.mul(h2, a.add(a.mul(d[0], d[0]), a.mul(d[1], d[1]))), a.mul(n1, n1))}
	seamX := []*big.Rat{a.sub(a.mul(h, x[0]), n0), a.sub(a.mul(h, d[0]), n1)}
	seamY := []*big.Rat{x[1], d[1]}
	if a.err != nil {
		return fail(a.err)
	}
	sign := func(root *geom.ExactAlgebraicRoot, poly []*big.Rat) (int, error) {
		op, e := opts()
		if e != nil {
			return 0, e
		}
		r, e := root.SignPolynomial(ctx, poly, op)
		out.Operations += r.Operations
		out.Subdivisions += r.Subdivisions
		if e != nil {
			return 0, e
		}
		return r.Sign, nil
	}
	enriched := func(root *geom.ExactAlgebraicRoot, own HierarchyLineOwner) error {
		wall, low, up := false, false, false
		for i := range own.Supports {
			support := &own.Supports[i]
			switch support.Role {
			case "wall":
				support.SourceFaceID = faces[0]
				wall = true
			case "lower-cap":
				support.SourceFaceID = faces[1]
				low = true
			case "upper-cap":
				support.SourceFaceID = faces[2]
				up = true
			}
			own.ContactFaceIDs = append(own.ContactFaceIDs, support.SourceFaceID)
		}
		if wall && low {
			own.ContactEdgeIDs = append(own.ContactEdgeIDs, p.Frustum.Face.SideEdges[0])
		}
		if wall && up {
			own.ContactEdgeIDs = append(own.ContactEdgeIDs, p.Frustum.Face.SideEdges[2])
		}
		if wall {
			sx, e := sign(root, seamX)
			if e != nil {
				return e
			}
			sy, e := sign(root, seamY)
			if e != nil {
				return e
			}
			if sx == 0 && sy == 0 {
				own.ContactEdgeIDs = append(own.ContactEdgeIDs, p.Frustum.Face.SideEdges[1])
				if low {
					own.ContactVertexIDs = append(own.ContactVertexIDs, p.Frustum.Face.CornerVertices[0])
				}
				if up {
					own.ContactVertexIDs = append(own.ContactVertexIDs, p.Frustum.Face.CornerVertices[2])
				}
			}
		}
		return addHit(root, own)
	}
	leaf, e = clipHierarchyAxialQuadratic(ctx, leaf, x, d, h, lower, upper, radial, a, out, opts, compare, enriched)
	if e != nil {
		return fail(e)
	}
	for i := range leaf.BoundarySpans {
		s := &leaf.BoundarySpans[i]
		for _, role := range s.Roles {
			switch role {
			case "wall":
				s.SourceFaceIDs = append(s.SourceFaceIDs, faces[0])
				if seamX[0].Sign() == 0 && seamX[1].Sign() == 0 && seamY[0].Sign() == 0 && seamY[1].Sign() == 0 {
					s.SourceEdgeIDs = append(s.SourceEdgeIDs, p.Frustum.Face.SideEdges[1])
				}
			case "lower-cap":
				s.SourceFaceIDs = append(s.SourceFaceIDs, faces[1])
			case "upper-cap":
				s.SourceFaceIDs = append(s.SourceFaceIDs, faces[2])
			}
		}
	}
	return leaf, nil
}
