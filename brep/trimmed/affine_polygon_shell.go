package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// ConvexAffinePolygonShellOptions bounds this standalone authenticator. Face
// and Edges retain their own stage limits; MaxOperations cumulatively bounds
// exact inverse, halfspace, volume and complete pair-contact arithmetic.
// Defaults are 64 faces, 64 vertices per face, 1024 uses and 64M exact
// operations; hard maxima are 128, 128, 4096 and 128M respectively.
type ConvexAffinePolygonShellOptions struct {
	Face                                              AffineSplineFaceQueryOptions
	Edges                                             edgecheck.Options
	MaxFaces, MaxFaceVertices, MaxUses, MaxOperations int
}

// ConvexAffinePolygonShell proves a closed, outward convex embedded shell with
// original straight UV polygon trims. Holes and collinear polygon corners are
// outside this contract. No new topology or replacement support is inferred.
type ConvexAffinePolygonShell struct {
	source     Solid
	queries    []*AffineSplineFaceQuery
	pairs      []*intersect.AffinePolygonContact
	volume     *big.Rat
	work, uses int
}

func (r *ConvexAffinePolygonShell) ExactVolume() *big.Rat {
	if r == nil || r.volume == nil {
		return nil
	}
	return new(big.Rat).Set(r.volume)
}
func (r *ConvexAffinePolygonShell) Original() Solid {
	if r == nil {
		return Solid{}
	}
	return r.source
}
func (r *ConvexAffinePolygonShell) CompleteFacePairs() int {
	if r == nil {
		return 0
	}
	return len(r.pairs)
}
func (r *ConvexAffinePolygonShell) ExactOperations() int {
	if r == nil {
		return 0
	}
	return r.work
}
func (r *ConvexAffinePolygonShell) EdgeUses() int {
	if r == nil {
		return 0
	}
	return r.uses
}

// FaceQuery returns the original face's immutable affine trim proof.
func (r *ConvexAffinePolygonShell) FaceQuery(face int) (*AffineSplineFaceQuery, error) {
	if r == nil || face < 0 || face >= len(r.queries) {
		return nil, geom.ErrInvalid
	}
	return r.queries[face], nil
}

// PairContact returns the complete finite contact in ascending original face
// order. Its trim side indices refer to the original Outer use indices.
func (r *ConvexAffinePolygonShell) PairContact(a, b int) (*intersect.AffinePolygonContact, error) {
	if r == nil || a < 0 || b <= a || b >= len(r.queries) {
		return nil, geom.ErrInvalid
	}
	n := len(r.queries)
	return r.pairs[a*(2*n-a-1)/2+b-a-1], nil
}

func CertifyConvexAffinePolygonShell(ctx context.Context, s Solid, o ConvexAffinePolygonShellOptions) (*ConvexAffinePolygonShell, error) {
	if ctx == nil {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if o.MaxFaces == 0 {
		o.MaxFaces = 64
	}
	if o.MaxFaceVertices == 0 {
		o.MaxFaceVertices = 64
	}
	if o.MaxUses == 0 {
		o.MaxUses = 1024
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 64000000
	}
	if o.MaxFaces < 4 || o.MaxFaces > 128 || o.MaxFaceVertices < 3 || o.MaxFaceVertices > 128 || o.MaxUses < 12 || o.MaxUses > 4096 || o.MaxOperations < 1 || o.MaxOperations > 128000000 {
		return nil, geom.ErrInvalid
	}
	if len(s.faces) < 4 || len(s.faces) > o.MaxFaces || len(s.edges) > o.MaxUses/2 || len(s.vertices) > o.MaxUses {
		return nil, geom.ErrConvergence
	}
	uses := 0
	for _, f := range s.faces {
		if len(f.Outer) > o.MaxFaceVertices || len(f.Outer) > o.MaxUses-uses {
			return nil, geom.ErrConvergence
		}
		uses += len(f.Outer)
	}
	b := &nativePrismAffineBudget{ctx: ctx, max: o.MaxOperations, bits: 16384}
	r := &ConvexAffinePolygonShell{source: s, volume: new(big.Rat)}
	loops := make([][][2]*big.Rat, len(s.faces))
	vertices := make([]map[int]bool, len(s.faces))
	edgeFaces := make([][]int, len(s.edges))
	edgeSigns := make([]int, len(s.edges))
	for fi, f := range s.faces {
		if len(f.Holes) != 0 || len(f.Outer) < 3 || len(f.Outer) > o.MaxFaceVertices {
			return nil, geom.ErrDomain
		}
		q, e := NewAffineSplineFaceQuery(ctx, f, o.Face)
		if e != nil {
			return nil, e
		}
		r.queries = append(r.queries, q)
		vertices[fi] = map[int]bool{}
		model := make([][3]*big.Rat, len(f.Outer))
		for ui, u := range f.Outer {
			if u.Curve != nil || u.Edge < 0 || u.Edge >= len(s.edges) {
				return nil, geom.ErrDomain
			}
			ed := s.edges[u.Edge]
			a, c := ed.Start, ed.End
			if u.Reversed {
				a, c = c, a
			}
			if a < 0 || c < 0 || a >= len(s.vertices) || c >= len(s.vertices) || a == c || vertices[fi][a] {
				return nil, geom.ErrDomain
			}
			vertices[fi][a] = true
			next := f.Outer[(ui+1)%len(f.Outer)]
			if next.Edge < 0 || next.Edge >= len(s.edges) {
				return nil, geom.ErrDomain
			}
			ne := s.edges[next.Edge]
			ns := ne.Start
			if next.Reversed {
				ns = ne.End
			}
			if c != ns {
				return nil, geom.ErrDomain
			}
			for k, vi := range []int{a, c} {
				if b.work >= b.max {
					return nil, geom.ErrConvergence
				}
				inv, e := q.chart.InvertExactContext(ctx, affineMaterialXYZ(s.vertices[vi]), geom.AffineChartInverseOptions{MaxOperations: b.max - b.work, MaxCoefficientBits: 16384})
				if e != nil {
					return nil, e
				}
				b.work += inv.ExactOperations
				uv := u.UVStart
				if k == 1 {
					uv = u.UVEnd
				}
				if !inv.OnPlane || !inv.Inside || inv.U.Cmp(ratInset(uv.X)) != 0 || inv.V.Cmp(ratInset(uv.Y)) != 0 {
					return nil, geom.ErrDomain
				}
			}
			loops[fi] = append(loops[fi], [2]*big.Rat{ratInset(u.UVStart.X), ratInset(u.UVStart.Y)})
			model[ui] = affineMaterialXYZ(s.vertices[a])
			edgeFaces[u.Edge] = append(edgeFaces[u.Edge], fi)
			sg := 1
			if u.Reversed != f.Reversed {
				sg = -1
			}
			edgeSigns[u.Edge] += sg
			r.uses++
		}
		for i, a := range loops[fi] {
			c := loops[fi][(i+1)%len(loops[fi])]
			for j, p := range loops[fi] {
				if j == i || j == (i+1)%len(loops[fi]) {
					continue
				}
				d := b.op('-', b.op('*', b.op('-', c[0], a[0]), b.op('-', p[1], a[1])), b.op('*', b.op('-', c[1], a[1]), b.op('-', p[0], a[0])))
				if d.Sign() <= 0 {
					return nil, geom.ErrDomain
				}
			}
		}
		pl := q.Plane()
		plane := affineMaterialPlane{n: [3]*big.Rat{pl[0], pl[1], pl[2]}, d: new(big.Rat).Neg(pl[3])}
		strict := false
		for _, p := range s.vertices {
			sign := affineMaterialValue(b, plane, affineMaterialXYZ(p)).Sign()
			if sign > 0 {
				return nil, geom.ErrDomain
			}
			strict = strict || sign < 0
		}
		if !strict {
			return nil, geom.ErrDomain
		}
		for i := 1; i+1 < len(model); i++ {
			det := new(big.Rat)
			for k := 0; k < 3; k++ {
				j, l := (k+1)%3, (k+2)%3
				det = b.op('+', det, b.op('*', model[0][k], b.op('-', b.op('*', model[i][j], model[i+1][l]), b.op('*', model[i][l], model[i+1][j]))))
			}
			if f.Reversed {
				det.Neg(det)
			}
			r.volume = b.op('+', r.volume, b.op('/', det, big.NewRat(6, 1)))
		}
		if b.err != nil {
			return nil, b.err
		}
	}
	adj := make([][]int, len(s.faces))
	for ei, fs := range edgeFaces {
		if len(fs) != 2 || fs[0] == fs[1] || edgeSigns[ei] != 0 {
			return nil, geom.ErrDomain
		}
		adj[fs[0]] = append(adj[fs[0]], fs[1])
		adj[fs[1]] = append(adj[fs[1]], fs[0])
	}
	seen := map[int]bool{}
	queue := []int{0}
	for len(queue) > 0 {
		f := queue[0]
		queue = queue[1:]
		if seen[f] {
			continue
		}
		seen[f] = true
		queue = append(queue, adj[f]...)
	}
	if len(seen) != len(s.faces) {
		return nil, geom.ErrDomain
	}
	if e := affineMaterialVertexLinks(ctx, s); e != nil {
		return nil, e
	}
	report, e := s.ValidateEdgeAgreement(ctx, o.Edges)
	if e != nil {
		return nil, e
	}
	if !report.Complete || len(report.Uses) != r.uses {
		return nil, geom.ErrDomain
	}
	for _, u := range report.Uses {
		if u.Report.Status != edgecheck.Certified || u.Report.SquaredUpperBound == nil || u.Report.SquaredUpperBound.Sign() != 0 {
			return nil, geom.ErrDomain
		}
	}
	for i := range s.faces {
		for j := i + 1; j < len(s.faces); j++ {
			if b.work >= b.max {
				return nil, geom.ErrConvergence
			}
			a, e := algebraicShellTracePatch(s.faces[i], r.queries[i].chart.Domain())
			if e != nil {
				return nil, e
			}
			c, e := algebraicShellTracePatch(s.faces[j], r.queries[j].chart.Domain())
			if e != nil {
				return nil, e
			}
			pair, e := intersect.IntersectConvexAffinePatches(ctx, a, c, [2][][2]*big.Rat{loops[i], loops[j]}, o.Face.Patch, intersect.AffinePolygonContactOptions{MaxEdges: 2 * o.MaxFaceVertices, MaxOperations: b.max - b.work, MaxCoefficientBits: 16384})
			if e != nil {
				return nil, e
			}
			contact := pair.Contact()
			b.work += contact.ExactOperations()
			kind, e := contact.Kind()
			if e != nil {
				return nil, e
			}
			points, e := contact.Vertices()
			if e != nil {
				return nil, e
			}
			common := map[int]bool{}
			for v := range vertices[i] {
				if vertices[j][v] {
					common[v] = true
				}
			}
			shared := 0
			for _, fs := range edgeFaces {
				if (fs[0] == i && fs[1] == j) || (fs[0] == j && fs[1] == i) {
					shared++
				}
			}
			want := intersect.AffinePatchContactEmpty
			if shared == 1 && len(common) == 2 {
				want = intersect.AffinePatchContactSegment
			} else if shared == 0 && len(common) == 1 {
				want = intersect.AffinePatchContactPoint
			} else if shared != 0 || len(common) != 0 {
				return nil, geom.ErrDomain
			}
			if kind != want || len(points) != len(common) {
				return nil, geom.ErrDomain
			}
			for _, p := range points {
				found := -1
				for v := range common {
					xyz := affineMaterialXYZ(s.vertices[v])
					if p.Point[0].Cmp(xyz[0]) == 0 && p.Point[1].Cmp(xyz[1]) == 0 && p.Point[2].Cmp(xyz[2]) == 0 {
						found = v
						break
					}
				}
				if found < 0 {
					return nil, geom.ErrDomain
				}
				delete(common, found)
			}
			r.pairs = append(r.pairs, pair)
		}
	}
	if b.err != nil {
		return nil, b.err
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if r.volume.Sign() <= 0 {
		return nil, geom.ErrDomain
	}
	r.work = b.work
	return r, nil
}
