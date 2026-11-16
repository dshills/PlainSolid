package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// AffineShellMaterialOptions bounds fragmentation/classification cumulatively.
// MaxOperations defaults to 64 million (maximum 256 million); MaxFaces defaults
// to 512 (maximum 4096). NativeEvaluation bounds each original face's exact
// endpoint evaluation separately, and Sew retains its own proof/use bounds.
type AffineShellMaterialOptions struct {
	MaxOperations, MaxFaces int
	NativeEvaluation        geom.SurfacePatchEvaluationOptions
	Sew                     SewOptions
}

// AffineShellMaterialFace records selection of one original chart fragment.
// Reversed is an additional Boolean reversal, not the original chart orientation.
type AffineShellMaterialFace struct {
	Owner, Face, Fragment           int
	InsideOther, Retained, Reversed bool
}

// AffineShellMaterial binds selected native boundary faces to immutable original
// chart fragments and exact occupancy. It stores no rounded volume estimate.
type AffineShellMaterial struct {
	source    *AffineShellPairedImprint
	operation intersect.AffineRingOperation
	solid     Solid
	ledger    []AffineShellMaterialFace
	fragments [2][]*AffineImprintFaceFragments
	volume    *big.Rat
	work      int
}

func (r *AffineShellMaterial) Solid() Solid {
	if r == nil {
		return Solid{}
	}
	return r.solid
}

// Fragments includes both retained and discarded original fragments.
func (r *AffineShellMaterial) Fragments() []AffineShellMaterialFace {
	if r == nil {
		return nil
	}
	return append([]AffineShellMaterialFace(nil), r.ledger...)
}

// FaceSources is indexed exactly like Solid().Faces().
func (r *AffineShellMaterial) FaceSources() []AffineShellMaterialFace {
	if r == nil {
		return nil
	}
	out := []AffineShellMaterialFace{}
	for _, f := range r.ledger {
		if f.Retained {
			out = append(out, f)
		}
	}
	return out
}

// SourceFaceFragments retains the exact original coedge/section/auxiliary proof.
func (r *AffineShellMaterial) SourceFaceFragments(owner, face int) *AffineImprintFaceFragments {
	if r == nil || owner < 0 || owner > 1 || face < 0 || face >= len(r.fragments[owner]) {
		return nil
	}
	return r.fragments[owner][face]
}
func (r *AffineShellMaterial) ExactVolume() *big.Rat {
	if r == nil {
		return nil
	}
	return new(big.Rat).Set(r.volume)
}

// ExactOperations counts fragment construction and rational classification/volume
// arithmetic. Native endpoint evaluation and Sew retain their separate bounds.
func (r *AffineShellMaterial) ExactOperations() int {
	if r == nil {
		return 0
	}
	return r.work
}

type affineMaterialPoint struct {
	uv  [2]*big.Rat
	xyz [3]*big.Rat
}
type affineMaterialPlane struct {
	n [3]*big.Rat
	d *big.Rat
}

func affineMaterialXYZ(p geom.Vec3) [3]*big.Rat {
	return [3]*big.Rat{ratInset(p.X), ratInset(p.Y), ratInset(p.Z)}
}
func affineMaterialValue(b *nativePrismAffineBudget, p affineMaterialPlane, x [3]*big.Rat) *big.Rat {
	v := new(big.Rat).Neg(p.d)
	for k := 0; k < 3; k++ {
		v = b.op('+', v, b.op('*', p.n[k], x[k]))
	}
	return v
}

// CombineConvex selects original fragments by exact convex occupancy. It is
// restricted to independently authenticated affine shells whose every face is
// an outward supporting plane. No coplanar contact, rounded classification or
// inferred pairing is introduced. Ambiguous, nonmanifold, empty or nonlosslessly
// representable native outputs reject; this method does not publish empty solids.
func (s *AffineShellPairedImprint) CombineConvex(ctx context.Context, op intersect.AffineRingOperation, o AffineShellMaterialOptions) (*AffineShellMaterial, error) {
	out, selected, o, e := s.selectConvexMaterial(ctx, op, o)
	if e != nil {
		return nil, e
	}
	solid, _, e := Sew(ctx, selected, o.Sew)
	if e != nil {
		return nil, e
	}
	// The selected occupancy sectors must form one circular link at each vertex.
	// Exact coincident coordinates alone cannot authorize a pinched material node.
	if e = affineMaterialVertexLinks(ctx, solid); e != nil {
		return nil, e
	}
	if e = ctx.Err(); e != nil {
		return nil, e
	}
	out.solid = solid
	return out, nil
}

func (s *AffineShellPairedImprint) selectConvexMaterial(ctx context.Context, op intersect.AffineRingOperation, o AffineShellMaterialOptions) (*AffineShellMaterial, []SewFace, AffineShellMaterialOptions, error) {
	if ctx == nil || s == nil || (op != intersect.AffineRingIntersection && op != intersect.AffineRingUnion && op != intersect.AffineRingDifference) {
		return nil, nil, o, geom.ErrInvalid
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 64000000
	}
	if o.MaxFaces == 0 {
		o.MaxFaces = 512
	}
	if o.MaxOperations < 1 || o.MaxOperations > 256000000 || o.MaxFaces < 1 || o.MaxFaces > 4096 {
		return nil, nil, o, geom.ErrInvalid
	}
	if o.NativeEvaluation.MaxOperations == 0 {
		o.NativeEvaluation.MaxOperations = 1000000
	}
	if o.Sew.Tolerance == 0 {
		o.Sew.Tolerance = 1e-10
	}
	o.Sew.MaxFaces = o.MaxFaces
	b := &nativePrismAffineBudget{ctx: ctx, max: o.MaxOperations, bits: 16384}
	planes := [2][]affineMaterialPlane{}
	for owner := 0; owner < 2; owner++ {
		for fi, ch := range s.charts[owner] {
			p := affineMaterialXYZ(s.sources[owner].vertices[ch.vertex[0]])
			q := affineMaterialXYZ(s.sources[owner].vertices[ch.vertex[1]])
			t := affineMaterialXYZ(s.sources[owner].vertices[ch.vertex[3]])
			n := [3]*big.Rat{}
			for k := 0; k < 3; k++ {
				j, l := (k+1)%3, (k+2)%3
				n[k] = b.op('-', b.op('*', b.op('-', q[j], p[j]), b.op('-', t[l], p[l])), b.op('*', b.op('-', q[l], p[l]), b.op('-', t[j], p[j])))
				if s.sources[owner].faces[fi].Reversed {
					n[k].Neg(n[k])
				}
			}
			plane := affineMaterialPlane{n: n, d: new(big.Rat)}
			for k := 0; k < 3; k++ {
				plane.d = b.op('+', plane.d, b.op('*', n[k], p[k]))
			}
			if b.err != nil {
				return nil, nil, o, b.err
			}
			if n[0].Sign() == 0 && n[1].Sign() == 0 && n[2].Sign() == 0 {
				return nil, nil, o, geom.ErrDomain
			}
			strict := false
			for _, v := range s.sources[owner].vertices {
				sign := affineMaterialValue(b, plane, affineMaterialXYZ(v)).Sign()
				if sign > 0 {
					return nil, nil, o, geom.ErrDomain
				}
				strict = strict || sign < 0
			}
			for _, id := range ch.vertex {
				if affineMaterialValue(b, plane, affineMaterialXYZ(s.sources[owner].vertices[id])).Sign() != 0 {
					return nil, nil, o, geom.ErrDomain
				}
			}
			if b.err != nil {
				return nil, nil, o, b.err
			}
			if !strict {
				return nil, nil, o, geom.ErrDomain
			}
			planes[owner] = append(planes[owner], plane)
		}
	}
	if b.err != nil {
		return nil, nil, o, b.err
	}
	out := &AffineShellMaterial{source: s, operation: op, volume: new(big.Rat)}
	for owner := 0; owner < 2; owner++ {
		out.fragments[owner] = make([]*AffineImprintFaceFragments, len(s.sources[owner].faces))
	}
	selected := []SewFace{}
	for owner := 0; owner < 2; owner++ {
		for face := range s.sources[owner].faces {
			remaining := b.max - b.work
			if remaining < 1 {
				return nil, nil, o, geom.ErrConvergence
			}
			if remaining > 128000000 {
				remaining = 128000000
			}
			cycles := o.MaxFaces
			if cycles > 1024 {
				cycles = 1024
			}
			parts, e := s.FragmentFace(ctx, owner, face, AffineImprintFragmentOptions{AuxiliarySubdivision: true, MaxOperations: remaining, MaxCycles: cycles})
			if e != nil {
				return nil, nil, o, e
			}
			b.work += parts.ExactOperations()
			out.fragments[owner][face] = parts
			fs, e := parts.NativeFaces(ctx, o.NativeEvaluation)
			if e != nil {
				return nil, nil, o, e
			}
			for ci, c := range parts.cycles {
				if len(c.Uses) != 3 {
					return nil, nil, o, geom.ErrDomain
				}
				poly := make([]affineMaterialPoint, 3)
				center := [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}
				for k, u := range c.Uses {
					poly[k] = affineMaterialPoint{uv: parts.uv[u.Start], xyz: affineMaterialXYZ(fs[ci].Vertices[k])}
					for j := 0; j < 3; j++ {
						center[j] = b.op('+', center[j], b.op('/', poly[k].xyz[j], big.NewRat(3, 1)))
					}
				}
				insideCenter, outsideCenter := true, false
				for _, plane := range planes[1-owner] {
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
					return nil, nil, o, b.err
				}
				if !inside && !outside {
					return nil, nil, o, geom.ErrDomain
				}
				keep := inside
				if op == intersect.AffineRingUnion {
					keep = outside
				}
				if op == intersect.AffineRingDifference && owner == 0 {
					keep = outside
				}
				reverse := keep && op == intersect.AffineRingDifference && owner == 1
				out.ledger = append(out.ledger, AffineShellMaterialFace{owner, face, ci, inside, keep, reverse})
				if !keep {
					continue
				}
				if len(selected) >= o.MaxFaces {
					return nil, nil, o, geom.ErrConvergence
				}
				f := fs[ci]
				if reverse {
					f.Face.Reversed = !f.Face.Reversed
				}
				selected = append(selected, f)
				x, y, z := affineMaterialXYZ(f.Vertices[0]), affineMaterialXYZ(f.Vertices[1]), affineMaterialXYZ(f.Vertices[2])
				det := new(big.Rat)
				for k := 0; k < 3; k++ {
					j, l := (k+1)%3, (k+2)%3
					det = b.op('+', det, b.op('*', x[k], b.op('-', b.op('*', y[j], z[l]), b.op('*', y[l], z[j]))))
				}
				if f.Face.Reversed {
					det.Neg(det)
				}
				out.volume = b.op('+', out.volume, b.op('/', det, big.NewRat(6, 1)))
			}
		}
	}
	if b.err != nil {
		return nil, nil, o, b.err
	}
	if len(selected) == 0 || out.volume.Sign() <= 0 {
		return nil, nil, o, geom.ErrDomain
	}
	out.work = b.work
	return out, selected, o, nil
}

func affineMaterialVertexLinks(ctx context.Context, s Solid) error {
	return materialVertexLinks(ctx, s, false)
}

// materialVertexLinks also admits proved inner-wire incidence when the
// source-specific caller has authenticated the original hole loops.
func materialVertexLinks(ctx context.Context, s Solid, includeInnerWires bool) error {
	uses := make([][]int, len(s.edges))
	incident := make([]map[int]bool, len(s.vertices))
	for i := range incident {
		incident[i] = map[int]bool{}
	}
	for fi, f := range s.faces {
		loops := [][]Use{f.Outer}
		if includeInnerWires {
			loops = append(loops, f.Holes...)
		}
		for _, loop := range loops {
			for _, u := range loop {
				uses[u.Edge] = append(uses[u.Edge], fi)
				e := s.edges[u.Edge]
				incident[e.Start][fi] = true
				incident[e.End][fi] = true
			}
		}
	}
	for vi, faces := range incident {
		if err := ctx.Err(); err != nil {
			return err
		}
		adjacency := map[int][]int{}
		for ei, e := range s.edges {
			if e.Start != vi && e.End != vi {
				continue
			}
			if len(uses[ei]) != 2 || uses[ei][0] == uses[ei][1] {
				return geom.ErrDomain
			}
			a, c := uses[ei][0], uses[ei][1]
			adjacency[a] = append(adjacency[a], c)
			adjacency[c] = append(adjacency[c], a)
		}
		start := -1
		for f := range faces {
			if len(adjacency[f]) != 2 {
				return geom.ErrDomain
			}
			start = f
		}
		if start < 0 {
			return geom.ErrDomain
		}
		seen := map[int]bool{}
		queue := []int{start}
		for len(queue) > 0 {
			f := queue[0]
			queue = queue[1:]
			if seen[f] {
				continue
			}
			seen[f] = true
			queue = append(queue, adjacency[f]...)
		}
		if len(seen) != len(faces) {
			return geom.ErrDomain
		}
	}
	return nil
}
