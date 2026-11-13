package draft

import (
	"context"
	"math"
	"math/big"
	"plainsolid/brep"
)

// EdgeChamfer names an original edge and one of its incident faces. Setback is
// measured into Face; OtherSetback is measured into the other incident face.
// Both distances must be strictly positive. Selections apply simultaneously.
type EdgeChamfer struct {
	Edge, Face            int
	Setback, OtherSetback float64
}

// ChamferResult adds selected-edge ancestry to the exact reconstruction result.
// FaceEdges aligns with output faces; entries are original edge IDs. EdgeFaces
// aligns with the selection list and contains -1 for a vanished bevel.
type ChamferResult struct {
	Result
	FaceEdges [][]int
	EdgeFaces []int
}

// Chamfer constructs exact two-distance planar bevels. Euclidean in-face
// perpendicular directions must have rational lengths; otherwise ErrUnsupported
// is returned. CoordinateSetbacks removes that restriction by measuring each
// setback in the opposite support plane's dominant-component-normalized linear
// coordinate (not Euclidean distance). Input supports and edge IDs stay fixed.
func Chamfer(ctx context.Context, s brep.Solid, cs []EdgeChamfer, coordinateSetbacks bool, o Options) (ChamferResult, error) {
	var out ChamferResult
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
	b := &budget{ctx: ctx, o: o}
	for i, c := range cs {
		if !b.tick() {
			return out, b.err
		}
		if c.Edge < 0 || c.Edge >= len(edges) || seen[c.Edge] || c.Setback <= 0 || c.OtherSetback <= 0 || math.IsNaN(c.Setback) || math.IsNaN(c.OtherSetback) || math.IsInf(c.Setback, 0) || math.IsInf(c.OtherSetback, 0) {
			return out, ErrInvalid
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
		var pa, pb point
		if coordinateSetbacks {
			pa = diff(x, scale(da, div(num(c.Setback), dot(q.n, da))))
			pb = diff(x, scale(db, div(num(c.OtherSetback), dot(p.n, db))))
		} else {
			la, ok := rationalSqrt(dot(da, da))
			if !ok {
				return out, ErrUnsupported
			}
			lb, ok := rationalSqrt(dot(db, db))
			if !ok {
				return out, ErrUnsupported
			}
			pa = diff(x, scale(da, div(num(c.Setback), la)))
			pb = diff(x, scale(db, div(num(c.OtherSetback), lb)))
		}
		normal := cross(v, diff(pb, pa))
		d := dot(normal, pa)
		// The removed original edge must be strictly outside the new halfspace.
		if sub(dot(normal, x), d).Sign() < 0 {
			normal = scale(normal, big.NewRat(-1, 1))
			d = new(big.Rat).Neg(d)
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
	}
	r, e := applyExtra(ctx, s, nil, extra, o)
	if e != nil {
		return out, e
	}
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
func rationalSqrt(x q) (q, bool) {
	if x.Sign() < 0 {
		return nil, false
	}
	n := new(big.Int).Sqrt(x.Num())
	d := new(big.Int).Sqrt(x.Denom())
	if new(big.Int).Mul(n, n).Cmp(x.Num()) != 0 || new(big.Int).Mul(d, d).Cmp(x.Denom()) != 0 {
		return nil, false
	}
	return new(big.Rat).SetFrac(n, d), true
}
