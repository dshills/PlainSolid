package draft

import (
	"context"
	"math"
	"math/big"
	"plainsolid/brep"
)

// FaceHinge rotates one original supporting face about an incident original
// edge, oriented from its stored Start to End. HalfAngleTangent is tan(angle/2),
// interpreted exactly as represented. Reversing the axis and the sign gives
// the same rotation. Finite values exclude exactly pi rotations.
type FaceHinge struct {
	Face, Edge       int
	HalfAngleTangent float64
}

// HingeProvenance retains the original axis, exact fixed endpoints and exact
// rotation coefficients. OutputFace is -1 when the moved support disappears.
// Intersections with other supports may trim away either or both hinge ends;
// this provenance states the transformation's fixed line, not retained edges.
type HingeProvenance struct {
	Face, Edge, StartVertex, EndVertex, OutputFace int
	Endpoints                                      [2][3]*big.Rat
	Rotation                                       [3][3]*big.Rat
}
type HingeResult struct {
	Result
	Hinges []HingeProvenance
}

// HingeDraft applies independently authored proper rotations simultaneously.
// Axis lengths must have rational square roots; unsupported irrational axes
// fail explicitly. All exact reconstruction and checked-display limits apply.
// The preparation and reconstruction each have a separate Options work budget.
// No input geometry or selection is mutated; returned rationals are owned.
func HingeDraft(ctx context.Context, s brep.Solid, changes []FaceHinge, o Options) (HingeResult, error) {
	empty := HingeResult{}
	if ctx == nil {
		return empty, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	var e error
	o, e = options(o)
	if e != nil {
		return empty, e
	}
	if len(changes) > o.MaxFaces {
		return empty, ErrLimit
	}
	fs, es, vs := s.Faces(), s.Edges(), s.Vertices()
	if len(fs) > o.MaxFaces {
		return empty, ErrLimit
	}
	transforms := map[int]matrix{}
	proofs := make([]HingeProvenance, len(changes))
	work := &budget{ctx: ctx, o: o}
	for k, c := range changes {
		if !work.tick() {
			return empty, work.err
		}
		if c.Face < 0 || c.Face >= len(fs) || c.Edge < 0 || c.Edge >= len(es) || math.IsNaN(c.HalfAngleTangent) || math.IsInf(c.HalfAngleTangent, 0) {
			return empty, ErrInvalid
		}
		if _, ok := transforms[c.Face]; ok {
			return empty, ErrInvalid
		}
		found := false
		for _, loop := range append([][]brep.EdgeUse{fs[c.Face].Boundary}, fs[c.Face].Holes...) {
			for _, u := range loop {
				if !work.tick() {
					return empty, work.err
				}
				if u.Edge == c.Edge {
					found = true
				}
			}
		}
		if !found {
			return empty, ErrInvalid
		}
		edge := es[c.Edge]
		a, b := exact(vs[edge.Start]), exact(vs[edge.End])
		axis := diff(b, a)
		length, ok := rationalSqrt(dot(axis, axis))
		if !ok {
			return empty, ErrUnsupported
		}
		if length.Sign() == 0 {
			return empty, ErrInvalid
		}
		axis = scale(axis, div(big.NewRat(1, 1), length))
		t := num(c.HalfAngleTangent)
		t2 := mul(t, t)
		den := add(big.NewRat(1, 1), t2)
		cos := div(sub(big.NewRat(1, 1), t2), den)
		sin := div(mul(big.NewRat(2, 1), t), den)
		oneMinus := sub(big.NewRat(1, 1), cos)
		var m matrix
		var rot [3][3]*big.Rat
		for i := 0; i < 3; i++ {
			for j := 0; j < 3; j++ {
				v := mul(oneMinus, mul(axis[i], axis[j]))
				if i == j {
					v = add(v, cos)
				}
				basis := point{new(big.Rat), new(big.Rat), new(big.Rat)}
				basis[j].SetInt64(1)
				v = add(v, mul(sin, cross(axis, basis)[i]))
				m[i][j] = v
				rot[i][j] = new(big.Rat).Set(v)
				work.check(v)
			}
			m[i][3] = sub(a[i], dot(point{m[i][0], m[i][1], m[i][2]}, a))
			work.check(m[i][3])
		}
		if work.err != nil {
			return empty, work.err
		}
		transforms[c.Face] = m
		proofs[k] = HingeProvenance{Face: c.Face, Edge: c.Edge, StartVertex: edge.Start, EndVertex: edge.End, Endpoints: [2][3]*big.Rat{a, b}, Rotation: rot}
	}
	r, e := apply(ctx, s, transforms, o)
	if e != nil {
		return empty, e
	}
	for i := range proofs {
		proofs[i].OutputFace = r.OldToNew[proofs[i].Face]
	}
	return HingeResult{r, proofs}, nil
}
