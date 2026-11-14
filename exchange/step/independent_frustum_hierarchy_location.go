package step

import (
	"bytes"
	"context"
	"math/big"
	"plainsolid/geom"
	"reflect"
)

// LocateIndependentCircularFrustumHierarchyPoint classifies each complete
// retained occurrence independently. This is not regularized union membership.
// Frustum boundary contacts retain original cap/wall faces, rim/seam edges and
// their source endpoint vertices. Noncircular frusta fail with no partial result.
func LocateIndependentCircularFrustumHierarchyPoint(ctx context.Context, m IndependentFrustumHierarchy, point [3]*big.Rat, o RationalHierarchyPointOptions) (RationalHierarchyPointLocations, error) {
	bad := func(e error) (RationalHierarchyPointLocations, error) { return RationalHierarchyPointLocations{}, e }
	if ctx == nil {
		return bad(geom.ErrInvalid)
	}
	if e := ctx.Err(); e != nil {
		return bad(e)
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 4096
	}
	if o.MaxCoefficientBits < 256 || o.MaxCoefficientBits > 8192 {
		return bad(geom.ErrInvalid)
	}
	for _, p := range point {
		if p == nil {
			return bad(geom.ErrInvalid)
		}
		if p.Num().BitLen() > o.MaxCoefficientBits || p.Denom().BitLen() > o.MaxCoefficientBits {
			return bad(geom.ErrConvergence)
		}
	}
	point = copyHierarchyRats(point)
	actual, c, e := frustumHierarchyEnvelope(ctx, m)
	if e != nil {
		return bad(e)
	}
	proof, e := ReadIndependentFrustumHierarchy(ctx, bytes.NewBufferString(actual.Proof.Source), o.Source, actual.Proof.Tolerance)
	if e != nil {
		return bad(e)
	}
	expected, d, e := frustumHierarchyEnvelope(ctx, proof)
	if e != nil {
		return bad(e)
	}
	if !reflect.DeepEqual(actual, expected) || !reflect.DeepEqual(c, d) {
		return bad(geom.ErrInvalid)
	}
	instances := make([]RationalHierarchyInstance, 0, len(proof.Instances))
	for _, u := range proof.Instances {
		b := mappedRationalBasis{}
		for i := 0; i < 3; i++ {
			b.Origin[i] = new(big.Rat).SetFloat64(u.Transform[i][3])
			b.X[i] = new(big.Rat).SetFloat64(u.Transform[i][0])
			b.Y[i] = new(big.Rat).SetFloat64(u.Transform[i][1])
			b.Z[i] = new(big.Rat).SetFloat64(u.Transform[i][2])
		}
		instances = append(instances, RationalHierarchyInstance{ProductID: u.ProductID, Path: append([]int64(nil), u.Path...), pose: b})
	}
	return locateAuthenticatedHierarchyPoint(ctx, proof.Assembly.Source, proof.Products, instances, proof.Assembly.LengthUnit, point, o, true, true)
}
func locateAuthenticatedCircularFrustum(source *File, p IndependentExactMaterial, v RationalHierarchyPointLeaf, a *hierarchyPointArithmetic) (RationalHierarchyPointLeaf, error) {
	bad := func(e error) (RationalHierarchyPointLeaf, error) { return RationalHierarchyPointLeaf{}, e }
	f := p.Frustum
	if f == nil || !f.Embedding.Complete || !f.Embedding.Circular {
		return bad(geom.ErrDomain)
	}
	q := f.Embedding
	var wall, lower, upper int64
	// Recover face ownership from the certified original supports, never sorted
	// IDs or shell-list position. The source codec already proves their trims.
	for _, id := range v.SourceFaceIDs {
		face, e := simple(source, id, "ADVANCED_FACE", 4)
		if e != nil {
			return bad(e)
		}
		sid := face.Args[2].Ref
		if plane, e := simple(source, sid, "PLANE", 2); e == nil {
			frame, e := mappedRationalFrame(source, plane.Args[1].Ref)
			if e != nil {
				return bad(e)
			}
			if frame.Origin[2].Cmp(q.LowerZ) == 0 {
				if lower != 0 {
					return bad(geom.ErrInvalid)
				}
				lower = id
			} else if frame.Origin[2].Cmp(q.UpperZ) == 0 {
				if upper != 0 {
					return bad(geom.ErrInvalid)
				}
				upper = id
			} else {
				return bad(geom.ErrInvalid)
			}
		} else {
			found := false
			for _, r := range source.Entities[sid].Records {
				if r.Type == "B_SPLINE_SURFACE_WITH_KNOTS" {
					found = true
				}
			}
			if !found || wall != 0 {
				return bad(geom.ErrInvalid)
			}
			wall = id
		}
	}
	if wall == 0 || lower == 0 || upper == 0 || len(v.SourceFaceIDs) != 3 {
		return bad(geom.ErrInvalid)
	}
	x, y, z := a.sub(v.product[0], q.CenterX), a.sub(v.product[1], q.CenterY), a.sub(v.product[2], q.LowerZ)
	v.local = [3]*big.Rat{x, y, z}
	h := a.sub(q.UpperZ, q.LowerZ)
	numerator := a.add(a.mul(q.LowerRadius, h), a.mul(a.sub(q.UpperRadius, q.LowerRadius), z))
	radial := a.add(a.mul(x, x), a.mul(y, y))
	residual := a.sub(a.mul(a.mul(h, h), radial), a.mul(numerator, numerator))
	if a.err != nil {
		return bad(a.err)
	}
	v.Location = AnalyticPointOutside
	if z.Sign() < 0 || z.Cmp(h) > 0 || residual.Sign() > 0 {
		return v, nil
	}
	v.Location = AnalyticPointInside
	onWall, onLower, onUpper := residual.Sign() == 0, z.Sign() == 0, z.Cmp(h) == 0
	if onWall {
		v.BoundaryRoles = append(v.BoundaryRoles, "wall")
		v.ContactFaceIDs = append(v.ContactFaceIDs, wall)
	}
	if onLower {
		v.BoundaryRoles = append(v.BoundaryRoles, "lower-cap")
		v.ContactFaceIDs = append(v.ContactFaceIDs, lower)
	}
	if onUpper {
		v.BoundaryRoles = append(v.BoundaryRoles, "upper-cap")
		v.ContactFaceIDs = append(v.ContactFaceIDs, upper)
	}
	if len(v.BoundaryRoles) == 0 {
		return v, nil
	}
	v.Location = AnalyticPointBoundary
	if onWall && onLower {
		v.ContactEdgeIDs = append(v.ContactEdgeIDs, f.Face.SideEdges[0])
	}
	if onWall && onUpper {
		v.ContactEdgeIDs = append(v.ContactEdgeIDs, f.Face.SideEdges[2])
	}
	// Canonical full-period support starts on the positive local X ray. This
	// selects the physical shared seam, without guessing an authored parameter.
	seam := onWall && y.Sign() == 0 && a.mul(x, h).Cmp(numerator) == 0
	if seam {
		v.ContactEdgeIDs = append(v.ContactEdgeIDs, f.Face.SideEdges[1])
		if onLower {
			v.ContactVertexIDs = append(v.ContactVertexIDs, f.Face.CornerVertices[0])
		}
		if onUpper {
			v.ContactVertexIDs = append(v.ContactVertexIDs, f.Face.CornerVertices[2])
		}
	}
	if a.err != nil {
		return bad(a.err)
	}
	return v, nil
}
