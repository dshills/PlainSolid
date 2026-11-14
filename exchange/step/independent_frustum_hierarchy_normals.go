package step

import (
	"bytes"
	"context"
	"math/big"
	"plainsolid/geom"
	"reflect"
)

// BoundaryNormalsIndependentCircularFrustumHierarchy retains every incident
// original face's exact outward ray. Rims are not averaged; shared occurrences
// are not regularized into a union. Noncircular frusta are explicitly rejected.
func BoundaryNormalsIndependentCircularFrustumHierarchy(ctx context.Context, m IndependentFrustumHierarchy, point [3]*big.Rat, o RationalHierarchyNormalOptions) (RationalHierarchyNormals, error) {
	bad := func(e error) (RationalHierarchyNormals, error) { return RationalHierarchyNormals{}, e }
	if ctx == nil {
		return bad(geom.ErrInvalid)
	}
	if e := ctx.Err(); e != nil {
		return bad(e)
	}
	if o.MaxNormals == 0 {
		o.MaxNormals = 4096
	}
	if o.MaxNormals < 1 || o.MaxNormals > 65536 {
		return bad(geom.ErrInvalid)
	}
	actual, c, e := frustumHierarchyEnvelope(ctx, m)
	if e != nil {
		return bad(e)
	}
	proof, e := ReadIndependentFrustumHierarchy(ctx, bytes.NewBufferString(actual.Proof.Source), o.Point.Source, actual.Proof.Tolerance)
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
	locations, e := LocateIndependentCircularFrustumHierarchyPoint(ctx, proof, point, o.Point)
	if e != nil {
		return bad(e)
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
	return normalsAuthenticatedHierarchy(ctx, proof.Assembly.Source, proof.Products, instances, locations, o)
}
