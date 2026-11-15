package step

import (
	"errors"
	"math/big"
	"strings"

	"plainsolid/brep"
	"plainsolid/geom"
)

// Direction ratios and support origins are read from their original decimal
// tokens. Only topological vertex coordinates must fit in binary64 exactly.
func vertexContactSupportTriple(f *File, id int64, kind string) (q [3]*big.Rat, err error) {
	return vertexContactSupportCoordinates(f, id, kind, 3)
}

func vertexContactSupportCoordinates(f *File, id int64, kind string, dim int) (q [3]*big.Rat, err error) {
	r, err := simple(f, id, kind, 2)
	if err != nil {
		return q, err
	}
	v := r.Args[1]
	if v.Kind != Aggregate || len(v.Items) != dim {
		return q, errors.New("step: wrong support coordinate dimension")
	}
	for i, x := range v.Items {
		q[i], err = exactVertexContactScalar(x)
		if err != nil {
			return q, err
		}
	}
	if dim == 2 {
		q[2] = new(big.Rat)
	}
	return q, nil
}

func vertexContactDifference(a, b [3]*big.Rat) (c [3]*big.Rat) {
	for i := range c {
		c[i] = new(big.Rat).Sub(a[i], b[i])
	}
	return c
}

func vertexContactZero(v [3]*big.Rat) bool {
	return v[0].Sign() == 0 && v[1].Sign() == 0 && v[2].Sign() == 0
}

func vertexContactLineData(f *File, id int64, dim int) (p, d [3]*big.Rat, err error) {
	line, err := simple(f, id, "LINE", 3)
	if err != nil {
		return p, d, err
	}
	p, err = vertexContactSupportCoordinates(f, line.Args[1].Ref, "CARTESIAN_POINT", dim)
	if err != nil {
		return p, d, err
	}
	vector, err := simple(f, line.Args[2].Ref, "VECTOR", 3)
	if err != nil {
		return p, d, err
	}
	d, err = vertexContactSupportCoordinates(f, vector.Args[1].Ref, "DIRECTION", dim)
	if err != nil {
		return p, d, err
	}
	magnitude, err := exactVertexContactScalar(vector.Args[2])
	if err != nil || magnitude.Sign() <= 0 || vertexContactZero(d) {
		return p, d, errors.New("step: invalid exact line vector")
	}
	return p, d, nil
}

func vertexContactOnLine(a, b, origin, d [3]*big.Rat) bool {
	return vertexContactZero(mappedRationalCross(vertexContactDifference(a, origin), d)) &&
		vertexContactZero(mappedRationalCross(vertexContactDifference(b, origin), d))
}

func exactVertexContactLine(f *File, id int64, edge linearEdge) (owners [2]int64, err error) {
	r, err := simple(f, id, "EDGE_CURVE", 5)
	if err != nil {
		return owners, err
	}
	lineID := r.Args[3].Ref
	var pcurveIDs []int64
	if kind := f.Entities[lineID].Records; len(kind) == 1 && kind[0].Type == "SURFACE_CURVE" {
		rec, err := simple(f, lineID, "SURFACE_CURVE", 4)
		if err != nil {
			return owners, err
		}
		pcurveIDs, err = listRefs(rec.Args[2], 2)
		if err != nil || len(pcurveIDs) != 2 {
			return owners, errors.New("step: exact surface curve requires both owner pcurves")
		}
		lineID = rec.Args[1].Ref
	}
	origin, d, err := vertexContactLineData(f, lineID, 3)
	if err != nil {
		return owners, err
	}
	a, err := vertexContactSupportTriple(f, edge.startPoint, "CARTESIAN_POINT")
	if err != nil {
		return owners, err
	}
	b, err := vertexContactSupportTriple(f, edge.endPoint, "CARTESIAN_POINT")
	if err != nil {
		return owners, err
	}
	if !vertexContactOnLine(a, b, origin, d) {
		return owners, errors.New("step: original endpoints do not lie on the declared line exactly")
	}
	sense, err := boolean(r.Args[4])
	dot := mappedRationalDot(vertexContactDifference(b, a), d).Sign()
	if err != nil || dot == 0 || sense != (dot > 0) {
		return owners, errors.New("step: original edge sense does not match its declared line")
	}
	for i, pc := range pcurveIDs {
		owners[i], err = exactVertexContactPCurve(f, pc, a, b, d)
		if err != nil {
			return owners, err
		}
	}
	if len(pcurveIDs) != 0 && owners[0] == owners[1] {
		return owners, errors.New("step: exact surface curve repeats its owner plane")
	}
	return owners, nil
}

func vertexContactLoopVertex(u brep.EdgeUse, edges []brep.Edge) int {
	edge := edges[u.Edge]
	if u.Reversed {
		return edge.End
	}
	return edge.Start
}

func vertexContactOrientedNormal(vertices []geom.Vec3, edges []brep.Edge, loop []brep.EdgeUse) [3]*big.Rat {
	origin := mappedRatVec(vertices[vertexContactLoopVertex(loop[0], edges)])
	n := [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}
	for i := 1; i+1 < len(loop); i++ {
		a := mappedRatVec(vertices[vertexContactLoopVertex(loop[i], edges)])
		b := mappedRatVec(vertices[vertexContactLoopVertex(loop[i+1], edges)])
		c := mappedRationalCross(vertexContactDifference(a, origin), vertexContactDifference(b, origin))
		for j := range n {
			n[j].Add(n[j], c[j])
		}
	}
	return n
}

func vertexContactPlaneAxes(f *File, id int64) (origin, n, x [3]*big.Rat, err error) {
	surface, err := simple(f, id, "PLANE", 2)
	if err != nil {
		return origin, n, x, err
	}
	axis, err := simple(f, surface.Args[1].Ref, "AXIS2_PLACEMENT_3D", 4)
	if err != nil {
		return origin, n, x, err
	}
	origin, err = vertexContactSupportTriple(f, axis.Args[1].Ref, "CARTESIAN_POINT")
	if err != nil {
		return origin, n, x, err
	}
	n = mappedRatVec(geom.Vec3{Z: 1})
	if axis.Args[2].Kind != Omitted {
		n, err = vertexContactSupportTriple(f, axis.Args[2].Ref, "DIRECTION")
		if err != nil {
			return origin, n, x, err
		}
	}
	if vertexContactZero(n) {
		return origin, n, x, errors.New("step: zero exact plane normal")
	}
	if axis.Args[3].Kind != Omitted {
		x, err = vertexContactSupportTriple(f, axis.Args[3].Ref, "DIRECTION")
		if err != nil {
			return origin, n, x, err
		}
		if vertexContactZero(mappedRationalCross(x, n)) {
			return origin, n, x, errors.New("step: exact reference axis is parallel to the plane normal")
		}
	}
	return origin, n, x, nil
}

func exactVertexContactPlane(f *File, r Record, vertices []geom.Vec3, edges []brep.Edge, face brep.Face) error {
	origin, n, _, err := vertexContactPlaneAxes(f, r.Args[2].Ref)
	if err != nil {
		return err
	}
	for _, loop := range append([][]brep.EdgeUse{face.Boundary}, face.Holes...) {
		for _, u := range loop {
			p := mappedRatVec(vertices[vertexContactLoopVertex(u, edges)])
			if mappedRationalDot(vertexContactDifference(p, origin), n).Sign() != 0 {
				return errors.New("step: original face vertex does not lie on the declared plane exactly")
			}
		}
	}
	sense, err := boolean(r.Args[3])
	dot := mappedRationalDot(vertexContactOrientedNormal(vertices, edges, face.Boundary), n).Sign()
	if err != nil || dot == 0 || sense != (dot > 0) {
		return errors.New("step: original face orientation does not match its declared plane")
	}
	return nil
}

func exactVertexContactPCurve(f *File, id int64, a, b, modelDirection [3]*big.Rat) (int64, error) {
	r, err := simple(f, id, "PCURVE", 3)
	if err != nil {
		return 0, err
	}
	plane := r.Args[1].Ref
	origin, n, x, err := vertexContactPlaneAxes(f, plane)
	if err != nil {
		return 0, err
	}
	if x[0] == nil {
		return 0, errors.New("step: exact pcurve plane requires an explicit reference direction")
	}
	n, err = mappedRationalUnit(n)
	if err != nil {
		return 0, errors.New("step: exact pcurve plane requires a rational unit normal")
	}
	dot := mappedRationalDot(n, x)
	for i := range x {
		x[i].Sub(x[i], new(big.Rat).Mul(dot, n[i]))
	}
	x, err = mappedRationalUnit(x)
	if err != nil {
		return 0, errors.New("step: exact pcurve plane requires a rational unit reference direction")
	}
	y := mappedRationalCross(n, x)
	rep, err := simple(f, r.Args[2].Ref, "DEFINITIONAL_REPRESENTATION", 3)
	if err != nil {
		return 0, err
	}
	// The ordinary pcurve preflight has already checked this representation
	// and ensures that it contains a single line in a 2D context.
	p, d, err := vertexContactLineData(f, rep.Args[1].Items[0].Ref, 2)
	if err != nil {
		return 0, err
	}
	lift := func(q [3]*big.Rat) (r [3]*big.Rat) {
		for i := range r {
			r[i] = new(big.Rat).Add(new(big.Rat).Mul(q[0], x[i]), new(big.Rat).Mul(q[1], y[i]))
		}
		return r
	}
	p, d = lift(p), lift(d)
	for i := range p {
		p[i].Add(p[i], origin[i])
	}
	if !vertexContactOnLine(a, b, p, d) || mappedRationalDot(modelDirection, d).Sign() <= 0 {
		return 0, errors.New("step: original owner pcurve does not lift to its directed model edge exactly")
	}
	return plane, nil
}

// STEP DIRECTION encodes ratios. Scale by a common power of two instead of
// normalizing with an irrational length or rounding the component ratios.
func vertexContactDirectionText(v [3]*big.Rat) (string, error) {
	if vertexContactZero(v) {
		return "", geom.ErrDegenerate
	}
	var scale int
	haveScale := false
	for _, x := range v {
		e := x.Num().BitLen() - x.Denom().BitLen()
		if x.Sign() != 0 && (!haveScale || e > scale) {
			scale, haveScale = e, true
		}
	}
	factor := big.NewRat(1, 1)
	if scale >= 0 {
		factor.SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), uint(scale)))
	} else {
		factor.SetInt(new(big.Int).Lsh(big.NewInt(1), uint(-scale)))
	}
	var out [3]string
	for i, x := range v {
		out[i] = exactVertexContactDyadicText(new(big.Rat).Mul(x, factor))
	}
	return strings.Join(out[:], ","), nil
}
