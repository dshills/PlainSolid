package trimmed

import (
	"context"
	"fmt"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// ConvexAffinePolygonMaterialOptions uses the same bounded native fragment,
// exact endpoint and sewing stages as polygon cap Fuse.
type ConvexAffinePolygonMaterialOptions ConvexAffinePolygonCapFuseOptions

// ConvexAffinePolygonMaterial is one closed connected original-support result,
// with full retained/discarded fragment occupancy and complete section ancestry.
type ConvexAffinePolygonMaterial struct {
	crossing *ConvexAffinePolygonCrossing
	material *AffineShellMaterial
}

func (r *ConvexAffinePolygonMaterial) Solid() Solid {
	if r == nil {
		return Solid{}
	}
	return r.material.Solid()
}
func (r *ConvexAffinePolygonMaterial) ExactVolume() *big.Rat {
	if r == nil {
		return nil
	}
	return r.material.ExactVolume()
}
func (r *ConvexAffinePolygonMaterial) ExactOperations() int {
	if r == nil {
		return 0
	}
	return r.material.ExactOperations()
}
func (r *ConvexAffinePolygonMaterial) OriginalCrossing() *ConvexAffinePolygonCrossing {
	if r == nil {
		return nil
	}
	return r.crossing
}
func (r *ConvexAffinePolygonMaterial) Fragments() []AffineShellMaterialFace {
	if r == nil {
		return nil
	}
	return r.material.Fragments()
}
func (r *ConvexAffinePolygonMaterial) FaceSources() []AffineShellMaterialFace {
	if r == nil {
		return nil
	}
	return r.material.FaceSources()
}
func (r *ConvexAffinePolygonMaterial) SourceFaceFragments(owner, face int) *AffineImprintFaceFragments {
	if r == nil {
		return nil
	}
	return r.material.SourceFaceFragments(owner, face)
}
func affinePolygonFragmentInside(b *nativePrismAffineBudget, part *AffineImprintFaceFragments, c AffineImprintFragmentCycle, native SewFace, planes []affineMaterialPlane) (bool, error) {
	poly := make([]affineMaterialPoint, 3)
	center := [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}
	for k, u := range c.Uses {
		poly[k] = affineMaterialPoint{uv: part.uv[u.Start], xyz: affineMaterialXYZ(native.Vertices[k])}
		for j := 0; j < 3; j++ {
			center[j] = b.op('+', center[j], b.op('/', poly[k].xyz[j], big.NewRat(3, 1)))
		}
	}
	insideCenter, outsideCenter := true, false
	for _, plane := range planes {
		sign := affineMaterialValue(b, plane, center).Sign()
		insideCenter = insideCenter && sign < 0
		outsideCenter = outsideCenter || sign > 0
		next := []affineMaterialPoint{}
		for i, x := range poly {
			y := poly[(i+1)%len(poly)]
			vx, vy := affineMaterialValue(b, plane, x.xyz), affineMaterialValue(b, plane, y.xyz)
			if vx.Sign() <= 0 {
				next = append(next, x)
			}
			if vx.Sign()*vy.Sign() < 0 {
				t := b.op('/', vx, b.op('-', vx, vy))
				z := affineMaterialPoint{}
				for k := 0; k < 2; k++ {
					z.uv[k] = b.op('+', x.uv[k], b.op('*', t, b.op('-', y.uv[k], x.uv[k])))
				}
				for k := 0; k < 3; k++ {
					z.xyz[k] = b.op('+', x.xyz[k], b.op('*', t, b.op('-', y.xyz[k], x.xyz[k])))
				}
				next = append(next, z)
			}
		}
		poly = next
	}
	area := new(big.Rat)
	for i, p := range poly {
		q := poly[(i+1)%len(poly)]
		area = b.op('+', area, b.op('-', b.op('*', p.uv[0], q.uv[1]), b.op('*', p.uv[1], q.uv[0])))
	}
	area = b.op('/', area, big.NewRat(2, 1))
	inside := area.Cmp(c.Area) == 0 && insideCenter
	outside := area.Sign() == 0 && outsideCenter
	if b.err != nil {
		return false, b.err
	}
	if !inside && !outside {
		return false, geom.ErrDomain
	}

	return inside, nil
}

// Combine classifies complete triangles against every original opposing
// supporting halfspace. Sample-only occupancy, coplanar area, disconnected
// results and unproved native endpoint representation are never accepted.
func (r *ConvexAffinePolygonCrossing) Combine(ctx context.Context, op intersect.AffineRingOperation, o ConvexAffinePolygonMaterialOptions) (*ConvexAffinePolygonMaterial, error) {
	if ctx == nil || r == nil || r.source == nil || r.shells[0] == nil || r.shells[1] == nil || (op != intersect.AffineRingIntersection && op != intersect.AffineRingUnion && op != intersect.AffineRingDifference) {
		return nil, geom.ErrInvalid
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 64000000
	}
	if o.MaxFaces == 0 {
		o.MaxFaces = 512
	}
	if o.MaxOperations < 1 || o.MaxOperations > 128000000 || o.MaxFaces < 1 || o.MaxFaces > 4096 {
		return nil, geom.ErrInvalid
	}
	b := &nativePrismAffineBudget{ctx: ctx, max: o.MaxOperations, bits: 16384}
	if e := r.validateSectionBinding(b); e != nil {
		return nil, e
	}
	out := &AffineShellMaterial{source: r.source, operation: op, volume: new(big.Rat)}
	planes := [2][]affineMaterialPlane{}
	for owner, shell := range r.shells {
		for _, q := range shell.queries {
			p := q.Plane()
			planes[owner] = append(planes[owner], affineMaterialPlane{n: [3]*big.Rat{p[0], p[1], p[2]}, d: new(big.Rat).Neg(p[3])})
		}
	}
	inverse := func(owner, face int, p [3]*big.Rat) (geom.AffineChartInverse, error) {
		if b.work >= b.max {
			return geom.AffineChartInverse{}, geom.ErrConvergence
		}
		q, e := r.shells[owner].queries[face].chart.InvertExactContext(ctx, p, geom.AffineChartInverseOptions{MaxOperations: b.max - b.work, MaxCoefficientBits: 16384})
		b.work += q.ExactOperations
		return q, e
	}
	selected := []SewFace{}
	for owner, source := range r.source.sources {
		originalVolume := new(big.Rat)
		out.fragments[owner] = make([]*AffineImprintFaceFragments, len(source.faces))
		for fi, f := range source.faces {
			points := [][2]*big.Rat{}
			polygon := affinePolygonFaceLoop(f)
			for _, p := range r.points {
				q, e := inverse(owner, fi, p)
				if e != nil {
					return nil, e
				}
				if !q.OnPlane || !q.Inside {
					continue
				}
				inside, _ := affinePolygonContains(b, polygon, [2]*big.Rat{q.U, q.V})
				if inside {
					points = append(points, [2]*big.Rat{q.U, q.V})
				}
			}
			if b.work >= b.max {
				return nil, geom.ErrConvergence
			}
			cycles := min(o.MaxFaces, 1024)
			part, e := r.source.fragmentFacePolygonPoints(ctx, owner, fi, AffineImprintFragmentOptions{AuxiliarySubdivision: true, MaxOperations: b.max - b.work, MaxCycles: cycles}, points, true)
			if e != nil {
				return nil, fmt.Errorf("polygon crossing fragment %d/%d: %w", owner, fi, e)
			}
			b.work += part.ExactOperations()
			out.fragments[owner][fi] = part
			var native []SewFace
			if o.RationalUVEndpoints {
				native, e = nativePolygonCapRationalFaces(ctx, part, o.Evaluation)
			} else {
				native, e = part.NativeFaces(ctx, o.Evaluation)
			}
			if e != nil {
				return nil, fmt.Errorf("polygon crossing native %d/%d: %w", owner, fi, e)
			}
			for ci, cycle := range part.cycles {
				f := native[ci]
				if len(f.Vertices) != 3 {
					return nil, geom.ErrDomain
				}
				originalVolume = b.op('+', originalVolume, affineCapTriangleVolume(b, f))
				inside, e := affinePolygonFragmentInside(b, part, cycle, f, planes[1-owner])
				if e != nil {
					return nil, e
				}
				keep := inside
				if op == intersect.AffineRingUnion || (op == intersect.AffineRingDifference && owner == 0) {
					keep = !inside
				}
				reverse := keep && op == intersect.AffineRingDifference && owner == 1
				out.ledger = append(out.ledger, AffineShellMaterialFace{Owner: owner, Face: fi, Fragment: ci, InsideOther: inside, Retained: keep, Reversed: reverse})
				if !keep {
					continue
				}
				if len(selected) >= o.MaxFaces {
					return nil, geom.ErrConvergence
				}
				if reverse {
					f.Face.Reversed = !f.Face.Reversed
				}
				selected = append(selected, f)
				out.volume = b.op('+', out.volume, affineCapTriangleVolume(b, f))
			}
		}
		if originalVolume.Cmp(r.shells[owner].volume) != 0 {
			return nil, geom.ErrDomain
		}
	}
	if b.err != nil {
		return nil, b.err
	}
	if len(selected) == 0 || out.volume.Sign() <= 0 {
		return nil, geom.ErrDomain
	}
	if o.Sew.Tolerance == 0 {
		o.Sew.Tolerance = max(r.source.sources[0].tolerance, r.source.sources[1].tolerance)
	}
	o.Sew.MaxFaces = o.MaxFaces
	solid, _, e := Sew(ctx, selected, o.Sew)
	if e != nil {
		return nil, e
	}
	if e = affineMaterialVertexLinks(ctx, solid); e != nil {
		return nil, e
	}
	if !affinePolygonConnectedFaces(solid) {
		return nil, geom.ErrDomain
	}
	sources := out.FaceSources()
	for fi, f := range solid.faces {
		ancestry := sources[fi]
		for _, u := range f.Outer {
			edge := solid.edges[u.Edge]
			a, c := edge.Start, edge.End
			if u.Reversed {
				a, c = c, a
			}
			for k, vi := range []int{a, c} {
				q, e := inverse(ancestry.Owner, ancestry.Face, affineMaterialXYZ(solid.vertices[vi]))
				if e != nil {
					return nil, e
				}
				uv := u.UVStart
				if k == 1 {
					uv = u.UVEnd
				}
				exact := [2]*big.Rat{ratInset(uv.X), ratInset(uv.Y)}
				if o.RationalUVEndpoints {
					curve, ok := u.Curve.(ExactQuadraticUV)
					if !ok || !curve.rationalEndpoints {
						return nil, geom.ErrDomain
					}
					ends, display := curve.Endpoints()
					if display[k] != uv {
						return nil, geom.ErrDomain
					}
					exact = ends[k]
				}
				if !q.OnPlane || !q.Inside || q.U.Cmp(exact[0]) != 0 || q.V.Cmp(exact[1]) != 0 {
					return nil, geom.ErrDomain
				}
			}
		}
	}
	report, e := solid.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if e != nil {
		return nil, e
	}
	if !report.Complete {
		return nil, geom.ErrDomain
	}
	for _, u := range report.Uses {
		if u.Report.SquaredUpperBound == nil || u.Report.SquaredUpperBound.Sign() != 0 {
			return nil, geom.ErrDomain
		}
	}
	if b.err != nil {
		return nil, b.err
	}
	if e = ctx.Err(); e != nil {
		return nil, e
	}
	out.solid = solid
	out.work = b.work
	return &ConvexAffinePolygonMaterial{crossing: r, material: out}, nil
}
func affinePolygonConnectedFaces(s Solid) bool {
	owners := make([][]int, len(s.edges))
	for fi, f := range s.faces {
		for _, u := range f.Outer {
			owners[u.Edge] = append(owners[u.Edge], fi)
		}
	}
	adj := make([][]int, len(s.faces))
	for _, pair := range owners {
		if len(pair) != 2 {
			return false
		}
		adj[pair[0]] = append(adj[pair[0]], pair[1])
		adj[pair[1]] = append(adj[pair[1]], pair[0])
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
	return len(seen) == len(s.faces)
}

// Check the immutable clipped-pair table against every source section atom
// before consuming it. This also prevents incomplete internal reconstructions.
func (r *ConvexAffinePolygonCrossing) validateSectionBinding(b *nativePrismAffineBudget) error {
	n, m := len(r.source.sources[0].faces), len(r.source.sources[1].faces)
	if n == 0 || m == 0 || len(r.contacts) != n*m || len(r.points) != len(r.nodes) {
		return geom.ErrInvalid
	}
	count := 0
	degree := make([]int, len(r.points))
	for index, pair := range r.contacts {
		if pair == nil {
			return geom.ErrInvalid
		}
		contact := pair.Contact()
		kind, e := contact.Kind()
		if e != nil {
			return e
		}
		if kind == intersect.AffinePatchContactEmpty {
			continue
		}
		if kind != intersect.AffinePatchContactSegment || count >= len(r.source.edges) {
			return geom.ErrDomain
		}
		edge := r.source.edges[count]
		count++
		if edge.Faces != [2]int{index / m, index % m} {
			return geom.ErrDomain
		}
		points, e := contact.Vertices()
		if e != nil {
			return e
		}
		if len(points) != 2 {
			return geom.ErrDomain
		}
		for k, p := range points {
			vi := edge.Vertices[k]
			if vi < 0 || vi >= len(r.points) {
				return geom.ErrDomain
			}
			degree[vi]++
			for axis, v := range p.Point {
				b.check(v)
				if b.err != nil {
					return b.err
				}
				if v.Cmp(r.points[vi][axis]) != 0 {
					return geom.ErrDomain
				}
			}
			for owner, uv := range [2][2]*big.Rat{p.A, p.B} {
				for axis, v := range uv {
					b.check(edge.UV[owner][k][axis])
					if b.err != nil {
						return b.err
					}
					if v.Cmp(edge.UV[owner][k][axis]) != 0 {
						return geom.ErrDomain
					}
				}
			}
		}
	}
	if count != len(r.source.edges) {
		return geom.ErrDomain
	}
	for _, d := range degree {
		if d != 2 {
			return geom.ErrDomain
		}
	}
	return b.err
}
