package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// AffineShellCapFuseOptions bounds fragment/inverse/selection arithmetic
// cumulatively (64M default, 128M maximum), and selected faces (512 default,
// 4096 maximum). Evaluation and Sew retain separately bounded proof stages.
type AffineShellCapFuseOptions struct {
	MaxOperations, MaxFaces int
	Evaluation              geom.SurfacePatchEvaluationOptions
	Sew                     SewOptions
}
type AffineShellCapFaceSource struct{ Owner, Face, Fragment int }

// AffineShellCapEdgeSource maps a fragment SectionEdge to the original exact
// coplanar contact edge and owning source chart.
type AffineShellCapEdgeSource struct{ Owner, ContactEdge int }
type AffineShellCapUnion struct {
	contact   *AffineShellCapContact
	solid     Solid
	sources   []AffineShellCapFaceSource
	sections  []AffineShellCapEdgeSource
	fragments [2][]*AffineImprintFaceFragments
	removed   [2]*big.Rat
	volume    *big.Rat
	work      int
}

func (r *AffineShellCapUnion) Solid() Solid {
	if r == nil {
		return Solid{}
	}
	return r.solid
}
func (r *AffineShellCapUnion) InterfaceEdges() []AffineShellCapEdgeSource {
	if r == nil {
		return nil
	}
	return append([]AffineShellCapEdgeSource(nil), r.sections...)
}
func (r *AffineShellCapUnion) FaceSources() []AffineShellCapFaceSource {
	if r == nil {
		return nil
	}
	return append([]AffineShellCapFaceSource(nil), r.sources...)
}
func (r *AffineShellCapUnion) OriginalContact() *AffineShellCapContact {
	if r == nil {
		return nil
	}
	return r.contact
}
func (r *AffineShellCapUnion) ExactVolume() *big.Rat {
	if r == nil {
		return nil
	}
	return new(big.Rat).Set(r.volume)
}
func (r *AffineShellCapUnion) ExactOperations() int {
	if r == nil {
		return 0
	}
	return r.work
}
func (r *AffineShellCapUnion) RemovedCapUVArea() [2]*big.Rat {
	if r == nil {
		return [2]*big.Rat{}
	}
	return [2]*big.Rat{new(big.Rat).Set(r.removed[0]), new(big.Rat).Set(r.removed[1])}
}

// SourceFaceFragments retains original coedge ranges. Its SectionEdge IDs
// index InterfaceEdges, rather than a transverse shell section wire.
func (r *AffineShellCapUnion) SourceFaceFragments(owner, face int) *AffineImprintFaceFragments {
	if r == nil || owner < 0 || owner > 1 || face < 0 || face >= len(r.fragments[owner]) {
		return nil
	}
	return r.fragments[owner][face]
}
func affineCapPolygonArea(b *nativePrismAffineBudget, p [][2]*big.Rat) *big.Rat {
	v := new(big.Rat)
	for i, x := range p {
		y := p[(i+1)%len(p)]
		v = b.op('+', v, b.op('-', b.op('*', x[0], y[1]), b.op('*', x[1], y[0])))
	}
	v = b.op('/', v, big.NewRat(2, 1))
	return v.Abs(v)
}
func affineCapClip(b *nativePrismAffineBudget, p [][2]*big.Rat, d geom.ParameterRectangle) [][2]*big.Rat {
	for axis := 0; axis < 2; axis++ {
		lo, hi := d.UMin, d.UMax
		if axis == 1 {
			lo, hi = d.VMin, d.VMax
		}
		for side, bound := range []*big.Rat{ratInset(lo), ratInset(hi)} {
			out := [][2]*big.Rat{}
			for i, x := range p {
				y := p[(i+1)%len(p)]
				vx, vy := b.op('-', x[axis], bound), b.op('-', y[axis], bound)
				if side == 0 {
					vx.Neg(vx)
					vy.Neg(vy)
				}
				if vx.Sign() <= 0 {
					out = append(out, x)
				}
				if vx.Sign()*vy.Sign() < 0 {
					t := b.op('/', vx, b.op('-', vx, vy))
					z := [2]*big.Rat{}
					for k := 0; k < 2; k++ {
						z[k] = b.op('+', x[k], b.op('*', t, b.op('-', y[k], x[k])))
					}
					out = append(out, z)
				}
			}
			p = out
		}
	}
	return p
}
func affineCapTriangleVolume(b *nativePrismAffineBudget, f SewFace) *big.Rat {
	x, y, z := affineMaterialXYZ(f.Vertices[0]), affineMaterialXYZ(f.Vertices[1]), affineMaterialXYZ(f.Vertices[2])
	v := new(big.Rat)
	for k := 0; k < 3; k++ {
		j, l := (k+1)%3, (k+2)%3
		v = b.op('+', v, b.op('*', x[k], b.op('-', b.op('*', y[j], z[l]), b.op('*', y[l], z[j]))))
	}
	if f.Face.Reversed {
		v.Neg(v)
	}
	return b.op('/', v, big.NewRat(6, 1))
}

// Fuse removes exactly the positive-area common cap footprint and conforms all
// original side coedges to its exact endpoints. The complete supporting-plane
// proof excludes any other cross-shell contact. All retained native surfaces
// are original; nonlossless endpoints or unresolved topology reject atomically.
func (r *AffineShellCapContact) Fuse(ctx context.Context, o AffineShellCapFuseOptions) (*AffineShellCapUnion, error) {
	if r == nil || ctx == nil {
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
	out := &AffineShellCapUnion{contact: r, volume: new(big.Rat), removed: [2]*big.Rat{new(big.Rat), new(big.Rat)}}
	vs, e := r.contact.Vertices()
	if e != nil {
		return nil, e
	}
	edges, e := r.contact.Edges()
	if e != nil {
		return nil, e
	}
	// Private reuse of the exact source-face arrangement engine, without claiming
	// a transverse paired-edge authorization for this independently proved cap.
	source := &AffineShellPairedImprint{sources: r.sources, charts: r.charts}
	uv := func(i, owner int) [2]*big.Rat {
		if owner == 0 {
			return vs[i].A
		}
		return vs[i].B
	}
	for owner := 0; owner < 2; owner++ {
		for ei, edge := range edges {
			if edge.Boundary[owner] != 0 {
				continue
			}
			f := [2]int{-1, -1}
			f[owner] = r.faces[owner]
			mapped := [2][2][2]*big.Rat{}
			mapped[owner] = [2][2]*big.Rat{uv(edge.Start, owner), uv(edge.End, owner)}
			source.edges = append(source.edges, AffineShellImprintEdge{Faces: f, UV: mapped})
			out.sections = append(out.sections, AffineShellCapEdgeSource{owner, ei})
		}
	}
	inverse := func(owner, face int, p [3]*big.Rat) (geom.AffineChartInverse, error) {
		if b.work >= b.max {
			return geom.AffineChartInverse{}, geom.ErrConvergence
		}
		q, e := r.affine[owner][face].InvertExactContext(ctx, p, geom.AffineChartInverseOptions{MaxOperations: b.max - b.work, MaxCoefficientBits: 16384})
		b.work += q.ExactOperations
		return q, e
	}
	selected := []SewFace{}
	sourceVolume := new(big.Rat)
	for owner, s := range r.sources {
		out.fragments[owner] = make([]*AffineImprintFaceFragments, len(s.faces))
		for face := range s.faces {
			points := [][2]*big.Rat{}
			domain := r.charts[owner][face].patch.Domain()
			for _, v := range vs {
				q, e := inverse(owner, face, v.Point)
				if e != nil {
					return nil, e
				}
				if !q.OnPlane || !q.Inside {
					continue
				}
				if face != r.faces[owner] && q.U.Cmp(ratInset(domain.UMin)) != 0 && q.U.Cmp(ratInset(domain.UMax)) != 0 && q.V.Cmp(ratInset(domain.VMin)) != 0 && q.V.Cmp(ratInset(domain.VMax)) != 0 {
					return nil, geom.ErrDomain
				}
				points = append(points, [2]*big.Rat{q.U, q.V})
			}
			if b.work >= b.max {
				return nil, geom.ErrConvergence
			}
			cycles := o.MaxFaces
			if cycles > 1024 {
				cycles = 1024
			}
			part, e := source.fragmentFacePoints(ctx, owner, face, AffineImprintFragmentOptions{AuxiliarySubdivision: true, MaxOperations: b.max - b.work, MaxCycles: cycles}, points)
			if e != nil {
				return nil, e
			}
			b.work += part.ExactOperations()
			out.fragments[owner][face] = part
			native, e := part.NativeFaces(ctx, o.Evaluation)
			if e != nil {
				return nil, e
			}
			for ci, f := range native {
				if len(f.Vertices) != 3 {
					return nil, geom.ErrDomain
				}
				volume := affineCapTriangleVolume(b, f)
				sourceVolume = b.op('+', sourceVolume, volume)
				keep := true
				if face == r.faces[owner] {
					poly := [][2]*big.Rat{}
					other := 1 - owner
					for _, v := range f.Vertices {
						q, e := inverse(other, r.faces[other], affineMaterialXYZ(v))
						if e != nil {
							return nil, e
						}
						if !q.OnPlane {
							return nil, geom.ErrDomain
						}
						poly = append(poly, [2]*big.Rat{q.U, q.V})
					}
					area := affineCapPolygonArea(b, poly)
					clipped := affineCapPolygonArea(b, affineCapClip(b, poly, r.charts[other][r.faces[other]].patch.Domain()))
					if clipped.Cmp(area) == 0 {
						keep = false
						out.removed[owner] = b.op('+', out.removed[owner], part.cycles[ci].Area)
					} else if clipped.Sign() != 0 {
						return nil, geom.ErrDomain
					}
				}
				if b.err != nil {
					return nil, b.err
				}
				if keep {
					if len(selected) >= o.MaxFaces {
						return nil, geom.ErrConvergence
					}
					selected = append(selected, f)
					out.sources = append(out.sources, AffineShellCapFaceSource{owner, face, ci})
					out.volume = b.op('+', out.volume, volume)
				}
			}
		}
	}
	for owner := 0; owner < 2; owner++ {
		area := new(big.Rat)
		for _, edge := range edges {
			p, q := uv(edge.Start, owner), uv(edge.End, owner)
			area = b.op('+', area, b.op('-', b.op('*', p[0], q[1]), b.op('*', p[1], q[0])))
		}
		area = b.op('/', area, big.NewRat(2, 1))
		area.Abs(area)
		if out.removed[owner].Cmp(area) != 0 {
			return nil, geom.ErrDomain
		}
	}
	if b.err != nil {
		return nil, b.err
	}
	if out.volume.Sign() <= 0 || out.volume.Cmp(sourceVolume) != 0 {
		return nil, geom.ErrDomain
	}
	if o.Sew.Tolerance == 0 {
		o.Sew.Tolerance = max(r.sources[0].tolerance, r.sources[1].tolerance)
	}
	o.Sew.MaxFaces = o.MaxFaces
	solid, _, e := Sew(ctx, selected, o.Sew)
	if e != nil {
		return nil, e
	}
	if e = affineMaterialVertexLinks(ctx, solid); e != nil {
		return nil, e
	}
	if e = ctx.Err(); e != nil {
		return nil, e
	}
	out.solid = solid
	out.work = b.work
	return out, nil
}
