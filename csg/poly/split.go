package poly

import (
	"context"
	"fmt"
	"math/big"

	"plainsolid/brep"
	"plainsolid/geom"
)

// PlaneSplitSideHistory identifies original faces and retained edge intervals
// for one output. OperandA is the input boundary. OperandB, Face -1 identifies
// a generated cap; cutter edges are not included in Edges.
type PlaneSplitSideHistory struct {
	Faces []Source
	Edges []EdgeAncestry
}

// PlaneSplitHistory aligns with the negative and positive split boundaries.
type PlaneSplitHistory struct{ Negative, Positive PlaneSplitSideHistory }

// SplitPlane partitions a planar solid into negative and positive closed pieces.
// Plane classification uses the exact represented Origin and Normal. A rational
// tangent basis and enclosing cutter are constructed without float conversion;
// only final boundary vertices are rounded. The same validation and budgets as
// Combine apply, cumulatively across both pieces. Tangent planes return the
// unchanged solid on its occupied side. More complicated contacts are accepted
// only if both results pass embedded manifold validation. No partial result is
// returned when either piece fails.
func SplitPlane(ctx context.Context, s brep.Solid, p geom.Plane, o Options) (negative, positive brep.Solid, err error) {
	return splitPlaneDetailed(ctx, s, p.Origin(), p.Normal(), o, nil)
}

// SplitPlaneWithHistory is the ancestry-producing variant of SplitPlane. The
// represented origin and nonzero normal define the plane directly; the normal
// is not normalized. This preserves exact direction ratios of oblique planes.
func SplitPlaneWithHistory(ctx context.Context, s brep.Solid, origin, normal geom.Vec3, o Options) (brep.Solid, brep.Solid, PlaneSplitHistory, error) {
	var h PlaneSplitHistory
	a, b, e := splitPlaneDetailed(ctx, s, origin, normal, o, &h)
	if e != nil {
		h = PlaneSplitHistory{}
	}
	return a, b, h, e
}

func splitPlaneDetailed(ctx context.Context, s brep.Solid, rawOrigin, rawNormal geom.Vec3, o Options, h *PlaneSplitHistory) (negative, positive brep.Solid, err error) {
	empty := brep.Solid{}
	if ctx == nil || !rawOrigin.IsFinite() || !rawNormal.IsFinite() || rawNormal == (geom.Vec3{}) {
		return empty, empty, ErrInvalid
	}
	o, err = defaults(o)
	if err != nil {
		return empty, empty, err
	}
	b := &budget{ctx: ctx, o: o}
	if !b.tick(1) {
		return empty, empty, b.err
	}
	ps, err := input(s, b)
	if err != nil {
		return empty, empty, err
	}
	if len(ps) == 0 {
		return empty, empty, nil
	}
	origin, n := vec(rawOrigin), vec(rawNormal)
	cut := plane{n, dot(n, origin)}
	b.point(origin)
	b.point(n)
	b.check(cut.d)
	negSide, posSide := false, false
	for _, q := range ps {
		for _, x := range q.v {
			if !b.tick(1) {
				return empty, empty, b.err
			}
			d := cut.distance(x)
			b.check(d)
			negSide = negSide || d.Sign() < 0
			posSide = posSide || d.Sign() > 0
		}
	}
	if b.err != nil {
		return empty, empty, b.err
	}
	if !negSide {
		if h != nil {
			h.Positive, err = unchangedPlaneSplitHistory(s, b)
			if err != nil {
				return empty, empty, err
			}
		}
		return empty, s, nil
	}
	if !posSide {
		if h != nil {
			h.Negative, err = unchangedPlaneSplitHistory(s, b)
			if err != nil {
				return empty, empty, err
			}
		}
		return s, empty, nil
	}
	faceOf := s.TriangleFaces()
	for i := range ps {
		ps[i].source = Source{Operand: OperandA, Face: faceOf[i], Triangle: i}
	}
	tool := rationalCutter(ps, origin, n, b)
	if b.err != nil {
		return empty, empty, b.err
	}
	np := booleanPolygons(ps, tool, Intersection, b)
	if b.err != nil {
		return empty, empty, b.err
	}
	negative, err = assemble(np, b)
	if err != nil {
		return empty, empty, fmt.Errorf("poly negative split: %w", err)
	}
	if h != nil {
		h.Negative, err = finishPlaneSplitHistory(negative, s, cut, b)
		if err != nil {
			return empty, empty, err
		}
	}
	pp := booleanPolygons(ps, tool, Difference, b)
	if b.err != nil {
		return empty, empty, b.err
	}
	positive, err = assemble(pp, b)
	if err != nil {
		return empty, empty, fmt.Errorf("poly positive split: %w", err)
	}
	if h != nil {
		h.Positive, err = finishPlaneSplitHistory(positive, s, cut, b)
		if err != nil {
			return empty, empty, err
		}
	}
	return negative, positive, nil
}

func finishPlaneSplitHistory(out, source brep.Solid, cut plane, b *budget) (PlaneSplitSideHistory, error) {
	if len(out.Faces()) == 0 {
		return PlaneSplitSideHistory{}, nil
	}
	h := PlaneSplitSideHistory{Faces: append([]Source(nil), b.sources...)}
	edges := out.Edges()
	for i, f := range out.Faces() {
		if h.Faces[i].Operand != OperandB {
			continue
		}
		for _, u := range f.Boundary {
			if !b.tick(1) {
				return PlaneSplitSideHistory{}, b.err
			}
			if cut.distance(b.exactVertices[edges[u.Edge].Start]).Sign() != 0 ||
				cut.distance(b.exactVertices[edges[u.Edge].End]).Sign() != 0 {
				return PlaneSplitSideHistory{}, ErrInvalid
			}
		}
	}
	var e error
	h.Edges, e = edgeAncestry(out, b.exactVertices, h.Faces, [2]brep.Solid{source, {}}, b)
	return h, e
}

func unchangedPlaneSplitHistory(s brep.Solid, b *budget) (PlaneSplitSideHistory, error) {
	h := PlaneSplitSideHistory{Faces: make([]Source, len(s.Faces())), Edges: make([]EdgeAncestry, len(s.Edges()))}
	for i, f := range s.Faces() {
		if !b.tick(1) {
			return PlaneSplitSideHistory{}, b.err
		}
		h.Faces[i] = Source{Operand: OperandA, Face: i, Triangle: -1}
		for _, loop := range append([][]brep.EdgeUse{f.Boundary}, f.Holes...) {
			for _, use := range loop {
				a := &h.Edges[use.Edge]
				a.AdjacentFaces = append(a.AdjacentFaces, h.Faces[i])
			}
		}
	}
	for i := range h.Edges {
		if !b.tick(1) {
			return PlaneSplitSideHistory{}, b.err
		}
		h.Edges[i].Overlaps = []EdgeOverlap{{Operand: OperandA, Edge: i,
			OutputFrom: new(big.Rat), OutputTo: big.NewRat(1, 1), InputFrom: new(big.Rat), InputTo: big.NewRat(1, 1)}}
	}
	return h, nil
}

// The finite cutter strictly contains the entire operand in both tangent
// coordinates and below its minimum normal coordinate. Its sole relevant face
// is exactly dot(n,x-origin)=0. All operations here are rational.
func rationalCutter(ps []polygon, origin, n point, b *budget) []polygon {
	axis := point{big.NewRat(1, 1), new(big.Rat), new(big.Rat)}
	if n[1].Sign() == 0 && n[2].Sign() == 0 {
		axis = point{new(big.Rat), big.NewRat(1, 1), new(big.Rat)}
	}
	v := cross(n, axis)
	u := cross(v, n)
	basis := [3]point{u, v, n}
	var norms [3]*big.Rat
	for i, a := range basis {
		norms[i] = dot(a, a)
		b.point(a)
		b.check(norms[i])
	}
	if b.err != nil {
		return nil
	}
	var lo, hi point
	first := true
	for _, q := range ps {
		for _, x := range q.v {
			if !b.tick(1) {
				return nil
			}
			d := diff(x, origin)
			for k, a := range basis {
				c := div(dot(d, a), norms[k])
				b.check(c)
				if first || c.Cmp(lo[k]) < 0 {
					lo[k] = c
				}
				if first || c.Cmp(hi[k]) > 0 {
					hi[k] = c
				}
			}
			first = false
		}
	}
	one := big.NewRat(1, 1)
	for k := 0; k < 3; k++ {
		lo[k] = sub(lo[k], one)
		hi[k] = add(hi[k], one)
	}
	hi[2] = new(big.Rat)
	corners := [8][3]int{{0, 0, 0}, {1, 0, 0}, {1, 1, 0}, {0, 1, 0}, {0, 0, 1}, {1, 0, 1}, {1, 1, 1}, {0, 1, 1}}
	var vertices [8]point
	for i, c := range corners {
		var x point
		for k := range x {
			x[k] = new(big.Rat).Set(origin[k])
			for j, a := range basis {
				t := lo[j]
				if c[j] == 1 {
					t = hi[j]
				}
				x[k] = add(x[k], mul(t, a[k]))
			}
		}
		b.point(x)
		vertices[i] = x
	}
	loops := [][4]int{{0, 3, 2, 1}, {4, 5, 6, 7}, {0, 1, 5, 4}, {1, 2, 6, 5}, {2, 3, 7, 6}, {3, 0, 4, 7}}
	out := make([]polygon, 0, 6)
	for i, loop := range loops {
		xs := []point{vertices[loop[0]], vertices[loop[1]], vertices[loop[2]], vertices[loop[3]]}
		p := makePlane(xs[0], xs[1], xs[2])
		b.point(p.n)
		b.check(p.d)
		out = append(out, polygon{v: xs, p: p, source: Source{Operand: OperandB, Face: -1, Triangle: -1 - i}})
	}
	return out
}
