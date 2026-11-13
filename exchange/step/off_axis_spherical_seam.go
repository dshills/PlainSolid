package step

import (
	"bytes"
	"io"
	"math/big"

	"plainsolid/geom"
)

// OffAxisSphericalSeamEvidence describes a full supporting circle and two
// points, not the trimmed edge interval. All values use exact source decimals.
type OffAxisSphericalSeamEvidence struct {
	EdgeID, CircleID, SphereID                                  int64
	VertexIDs                                                   [2]int64
	FullCircleSphereSquaredRadiusResidual                       *big.Rat
	MeridianContainsSphereAxis                                  bool
	DirectionNormSquared, ReferenceNormSquared                  *big.Rat
	EndpointSphereResiduals, EndpointCircleRadialResiduals      [2]*big.Rat
	EndpointPlaneSquaredDistances, EndpointAxisSquaredDistances [2]*big.Rat
}

// InspectOffAxisSphericalSeam requires a concentric model circle and sphere,
// nonzero orthogonal circle directions and a signed-coordinate sphere frame.
// Raw circle directions need not have unit length: plane distances divide by
// the exact squared normal norm, matching direction normalization without sqrt.
// Residuals are not repaired or compared to source uncertainty metadata.
func InspectOffAxisSphericalSeam(r io.Reader, l Limits, bodyID int64) (out OffAxisSphericalSeamEvidence, err error) {
	if r == nil {
		return out, geom.ErrDomain
	}
	l, err = l.defaults()
	if err != nil {
		return
	}
	raw, e := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if e != nil {
		return out, e
	}
	if int64(len(raw)) > l.MaxBytes {
		return out, geom.ErrDomain
	}
	top, e := ValidateOffAxisBoreTopology(bytes.NewReader(raw), l, bodyID)
	if e != nil {
		return out, e
	}
	f, e := Parse(bytes.NewReader(raw), l)
	if e != nil {
		return out, e
	}
	for _, face := range top.Faces {
		if face.SurfaceKind == "SPHERICAL_SURFACE" {
			out.SphereID = face.SurfaceID
		}
	}
	for _, edge := range top.EdgeCurves {
		if edge.Seam && edge.SupportIDs[0] == out.SphereID {
			out.EdgeID, out.CircleID = edge.EdgeID, edge.ModelCurveID
		}
	}
	sphere, e := simple(f, out.SphereID, "SPHERICAL_SURFACE", 3)
	if e != nil {
		return out, e
	}
	sid, e := ref(sphere.Args[1])
	if e != nil {
		return out, e
	}
	origin, axes, e := sphereLiftFrame(f, sid)
	if e != nil {
		return out, e
	}
	sr, e := liftSourceRat(sphere.Args[2])
	if e != nil || sr.Sign() <= 0 {
		return out, geom.ErrDomain
	}
	circle, e := simple(f, out.CircleID, "CIRCLE", 3)
	if e != nil {
		return out, e
	}
	cid, e := ref(circle.Args[1])
	if e != nil {
		return out, e
	}
	placement, e := simple(f, cid, "AXIS2_PLACEMENT_3D", 4)
	if e != nil {
		return out, e
	}
	cr, e := liftSourceRat(circle.Args[2])
	if e != nil || cr.Sign() <= 0 {
		return out, geom.ErrDomain
	}
	vector := func(v Value, kind string) (a [3]*big.Rat, err error) {
		id, err := ref(v)
		if err != nil {
			return a, err
		}
		p, err := simple(f, id, kind, 2)
		if err != nil || p.Args[1].Kind != Aggregate || len(p.Args[1].Items) != 3 {
			return a, geom.ErrDomain
		}
		for i := range a {
			a[i], err = liftSourceRat(p.Args[1].Items[i])
			if err != nil {
				return a, err
			}
		}
		return
	}
	dot := func(a, b [3]*big.Rat) *big.Rat {
		q := new(big.Rat)
		for i := range a {
			q.Add(q, new(big.Rat).Mul(a[i], b[i]))
		}
		return q
	}
	center, e := vector(placement.Args[1], "CARTESIAN_POINT")
	if e != nil {
		return out, e
	}
	for i := range center {
		if center[i].Cmp(origin[i]) != 0 {
			return out, geom.ErrDomain
		}
	}
	n, e := vector(placement.Args[2], "DIRECTION")
	if e != nil {
		return out, e
	}
	x, e := vector(placement.Args[3], "DIRECTION")
	if e != nil {
		return out, e
	}
	nn, xx := dot(n, n), dot(x, x)
	if nn.Sign() == 0 || xx.Sign() == 0 || dot(n, x).Sign() != 0 {
		return out, geom.ErrDomain
	}
	out.DirectionNormSquared, out.ReferenceNormSquared = nn, xx
	out.MeridianContainsSphereAxis = dot(n, axes[2]).Sign() == 0
	out.FullCircleSphereSquaredRadiusResidual = new(big.Rat).Sub(new(big.Rat).Mul(cr, cr), new(big.Rat).Mul(sr, sr))
	edge, e := simple(f, out.EdgeID, "EDGE_CURVE", 5)
	if e != nil {
		return out, e
	}
	for i := 0; i < 2; i++ {
		id, e := ref(edge.Args[i+1])
		if e != nil {
			return out, e
		}
		out.VertexIDs[i] = id
		v, e := simple(f, id, "VERTEX_POINT", 2)
		if e != nil {
			return out, e
		}
		p, e := vector(v.Args[1], "CARTESIAN_POINT")
		if e != nil {
			return out, e
		}
		for j := range p {
			p[j] = new(big.Rat).Sub(p[j], center[j])
		}
		pp := dot(p, p)
		pn := dot(p, n)
		pz := dot(p, axes[2])
		plane := new(big.Rat).Quo(new(big.Rat).Mul(pn, pn), nn)
		out.EndpointPlaneSquaredDistances[i] = plane
		out.EndpointSphereResiduals[i] = new(big.Rat).Sub(pp, new(big.Rat).Mul(sr, sr))
		out.EndpointCircleRadialResiduals[i] = new(big.Rat).Sub(new(big.Rat).Sub(pp, plane), new(big.Rat).Mul(cr, cr))
		out.EndpointAxisSquaredDistances[i] = new(big.Rat).Sub(pp, new(big.Rat).Mul(pz, pz))
	}
	return out, nil
}
