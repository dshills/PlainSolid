package draft

import (
	"context"
	"math"
	"math/big"
	"plainsolid/brep"
)

// TaperedEdgeChamfer specifies in-face perpendicular setbacks at the original
// edge's stored Start and End vertices. Face selects one incident face; Other
// setbacks refer to its neighbor. All four values must be strictly positive.
// Widths vary affinely along the entire original edge, without reorientation
// by face winding. The four exact offset points must be coplanar.
type TaperedEdgeChamfer struct {
	Edge, Face                                                   int
	StartSetback, StartOtherSetback, EndSetback, EndOtherSetback float64
}

// TaperedChamferSection retains original vertex direction, the selected width
// field and exact endpoint offsets (Start/Face, Start/Other, End/Face, End/Other).
// Rationals are caller-owned; linear interpolation gives every intermediate
// in-face offset and lies exactly on the generated bevel plane.
type TaperedChamferSection struct {
	Selection    TaperedEdgeChamfer
	Vertices     [2]int
	OtherFace    int
	OffsetPoints [4][3]*big.Rat
}
type TaperedChamferResult struct {
	ChamferResult
	Sections []TaperedChamferSection
}

// ChamferTapered applies exact planar tapered bevels simultaneously. It retains
// original source supports, original edge directions and ChamferResult ancestry.
// A constant-width selection agrees with Chamfer. Euclidean/coordinate metrics
// have the same meanings and rational-length restrictions as Chamfer. Exact
// coplanarity and strictly removed original endpoints prove the whole linear
// width field; incompatible endpoint ratios fail without a partial result.
// Large or interacting selections can remove faces or empty the convex solid,
// just as in Chamfer. All reconstruction/conversion/embedding limits apply.
func ChamferTapered(ctx context.Context, s brep.Solid, cs []TaperedEdgeChamfer, coordinateSetbacks bool, o Options) (TaperedChamferResult, error) {
	var out TaperedChamferResult
	if ctx == nil {
		return out, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	var e error
	o, e = options(o)
	if e != nil {
		return out, e
	}
	if len(cs) > o.MaxPlanes {
		return out, ErrLimit
	}
	edges, faces, vertices := s.Edges(), s.Faces(), s.Vertices()
	if len(faces) > o.MaxFaces {
		return out, ErrLimit
	}
	adjacent := make([][]int, len(edges))
	supports := make([]plane, len(faces))
	for i, f := range faces {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		var ids []int
		for _, u := range f.Boundary {
			x := edges[u.Edge].Start
			if u.Reversed {
				x = edges[u.Edge].End
			}
			ids = append(ids, x)
			adjacent[u.Edge] = append(adjacent[u.Edge], i)
		}
		if len(ids) < 3 {
			return out, ErrInvalid
		}
		p, ok := through(exact(vertices[ids[0]]), exact(vertices[ids[1]]), exact(vertices[ids[2]]))
		if !ok {
			return out, ErrInvalid
		}
		supports[i] = p
	}
	seen := map[int]bool{}
	var extra []plane
	var sections []TaperedChamferSection
	b := &budget{ctx: ctx, o: o}
	for i, c := range cs {
		if !b.tick() {
			return out, b.err
		}
		if c.Edge < 0 || c.Edge >= len(edges) || seen[c.Edge] {
			return out, ErrInvalid
		}
		for _, v := range []float64{c.StartSetback, c.StartOtherSetback, c.EndSetback, c.EndOtherSetback} {
			if v <= 0 || math.IsNaN(v) || math.IsInf(v, 0) {
				return out, ErrInvalid
			}
		}
		seen[c.Edge] = true
		adj := adjacent[c.Edge]
		if len(adj) != 2 {
			return out, ErrInvalid
		}
		a, other := adj[0], adj[1]
		if other == c.Face {
			a, other = other, a
		}
		if a != c.Face {
			return out, ErrInvalid
		}
		p, q := supports[a], supports[other]
		edge := edges[c.Edge]
		x := exact(vertices[edge.Start])
		v := diff(exact(vertices[edge.End]), x)
		// Within face p, project q's normal into p's tangent plane. Its negative
		// is the inward perpendicular to the common edge.
		direction := func(a, c point) point { return diff(scale(c, dot(a, a)), scale(a, dot(a, c))) }
		da, db := direction(p.n, q.n), direction(q.n, p.n)
		if zero(da) || zero(db) || zero(v) {
			return out, ErrUnsupported
		}
		var ua, ub point
		if coordinateSetbacks {
			ua = scale(da, div(big.NewRat(1, 1), dot(q.n, da)))
			ub = scale(db, div(big.NewRat(1, 1), dot(p.n, db)))
		} else {
			la, ok := rationalSqrt(dot(da, da))
			if !ok {
				return out, ErrUnsupported
			}
			lb, ok := rationalSqrt(dot(db, db))
			if !ok {
				return out, ErrUnsupported
			}
			ua = scale(da, div(big.NewRat(1, 1), la))
			ub = scale(db, div(big.NewRat(1, 1), lb))
		}
		y := exact(vertices[edge.End])
		pa, pb := diff(x, scale(ua, num(c.StartSetback))), diff(x, scale(ub, num(c.StartOtherSetback)))
		qa, qb := diff(y, scale(ua, num(c.EndSetback))), diff(y, scale(ub, num(c.EndOtherSetback)))
		normal := cross(diff(pb, pa), diff(qa, pa))
		d := dot(normal, pa)
		if zero(normal) || dot(normal, qb).Cmp(d) != 0 {
			return out, ErrUnsupported
		}
		startSign := sub(dot(normal, x), d).Sign()
		endSign := sub(dot(normal, y), d).Sign()
		if startSign == 0 || startSign != endSign {
			return out, ErrUnsupported
		}
		if startSign < 0 {
			normal = scale(normal, big.NewRat(-1, 1))
			d = new(big.Rat).Neg(d)
		}
		// All computed points and the two endpoint support residuals are exact;
		// linear interpolation preserves each in-face setback and plane equation.
		for _, point := range []point{pa, pb, qa, qb} {
			b.point(point)
		}
		bevel, ok := normalize(normal, d)
		if !ok {
			return out, ErrInvalid
		}
		bevel.sources = []int{-i - 1}
		b.point(bevel.n)
		b.check(bevel.d)
		if b.err != nil {
			return out, b.err
		}
		extra = append(extra, bevel)
		sections = append(sections, TaperedChamferSection{Selection: c, Vertices: [2]int{edge.Start, edge.End}, OtherFace: other, OffsetPoints: [4][3]*big.Rat{pa, pb, qa, qb}})
	}
	r, e := applyExtra(ctx, s, nil, extra, o)
	if e != nil {
		return out, e
	}
	out.Sections = sections
	out.Result = r
	out.EdgeFaces = make([]int, len(cs))
	for i := range out.EdgeFaces {
		out.EdgeFaces[i] = -1
	}
	out.FaceEdges = make([][]int, len(r.FaceSources))
	for i, src := range r.FaceSources {
		var kept []int
		for _, id := range src {
			if id >= 0 {
				kept = append(kept, id)
			} else {
				selection := -id - 1
				out.EdgeFaces[selection] = i
				out.FaceEdges[i] = append(out.FaceEdges[i], cs[selection].Edge)
			}
		}
		out.FaceSources[i] = kept
	}
	return out, nil
}
