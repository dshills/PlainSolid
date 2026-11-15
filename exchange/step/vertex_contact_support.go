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
	r, err := simple(f, id, kind, 2)
	if err != nil {
		return q, err
	}
	v := r.Args[1]
	if v.Kind != Aggregate || len(v.Items) != 3 {
		return q, errors.New("step: expected three support coordinates")
	}
	for i, x := range v.Items {
		q[i], err = exactVertexContactScalar(x)
		if err != nil {
			return q, err
		}
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

func exactVertexContactLine(f *File, id int64, edge linearEdge) error {
	r, err := simple(f, id, "EDGE_CURVE", 5)
	if err != nil {
		return err
	}
	// The ordinary reader has already checked the reference types. A
	// SURFACE_CURVE would also require authenticating its planar owner charts.
	line, err := simple(f, r.Args[3].Ref, "LINE", 3)
	if err != nil {
		return err
	}
	origin, err := vertexContactSupportTriple(f, line.Args[1].Ref, "CARTESIAN_POINT")
	if err != nil {
		return err
	}
	vector, err := simple(f, line.Args[2].Ref, "VECTOR", 3)
	if err != nil {
		return err
	}
	d, err := vertexContactSupportTriple(f, vector.Args[1].Ref, "DIRECTION")
	if err != nil {
		return err
	}
	magnitude, err := exactVertexContactScalar(vector.Args[2])
	if err != nil || magnitude.Sign() <= 0 || vertexContactZero(d) {
		return errors.New("step: invalid exact line vector")
	}
	a, err := vertexContactSupportTriple(f, edge.startPoint, "CARTESIAN_POINT")
	if err != nil {
		return err
	}
	b, err := vertexContactSupportTriple(f, edge.endPoint, "CARTESIAN_POINT")
	if err != nil {
		return err
	}
	if !vertexContactZero(mappedRationalCross(vertexContactDifference(a, origin), d)) ||
		!vertexContactZero(mappedRationalCross(vertexContactDifference(b, origin), d)) {
		return errors.New("step: original endpoints do not lie on the declared line exactly")
	}
	sense, err := boolean(r.Args[4])
	dot := mappedRationalDot(vertexContactDifference(b, a), d).Sign()
	if err != nil || dot == 0 || sense != (dot > 0) {
		return errors.New("step: original edge sense does not match its declared line")
	}
	return nil
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

func exactVertexContactPlane(f *File, r Record, vertices []geom.Vec3, edges []brep.Edge, face brep.Face) error {
	surface, err := simple(f, r.Args[2].Ref, "PLANE", 2)
	if err != nil {
		return err
	}
	axis, err := simple(f, surface.Args[1].Ref, "AXIS2_PLACEMENT_3D", 4)
	if err != nil {
		return err
	}
	origin, err := vertexContactSupportTriple(f, axis.Args[1].Ref, "CARTESIAN_POINT")
	if err != nil {
		return err
	}
	n := mappedRatVec(geom.Vec3{Z: 1})
	if axis.Args[2].Kind != Omitted {
		n, err = vertexContactSupportTriple(f, axis.Args[2].Ref, "DIRECTION")
		if err != nil {
			return err
		}
	}
	if vertexContactZero(n) {
		return errors.New("step: zero exact plane normal")
	}
	if axis.Args[3].Kind != Omitted {
		x, err := vertexContactSupportTriple(f, axis.Args[3].Ref, "DIRECTION")
		if err != nil {
			return err
		}
		if vertexContactZero(mappedRationalCross(x, n)) {
			return errors.New("step: exact reference axis is parallel to the plane normal")
		}
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
