package trimmed

import (
	"context"
	"fmt"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
)

// ConvexAffinePolygonCapFuseOptions bounds fragment/inverse/selection arithmetic
// cumulatively (64M default, 128M maximum), and selected faces (512 default,
// 4096 maximum). Evaluation and Sew retain separately bounded proof stages.
type ConvexAffinePolygonCapFuseOptions struct {
	// RationalUVEndpoints retains nonbinary original UV in exact native
	// straight quadratic p-curves, with separate nearest display coordinates.
	// Model vertices remain lossless. Existing native 256-edge limits apply.
	RationalUVEndpoints bool

	MaxOperations, MaxFaces int
	Evaluation              geom.SurfacePatchEvaluationOptions
	Sew                     SewOptions
}

// ConvexAffinePolygonCapUnion retains the original supports and complete exact
// face partitions, including removed cap cells. Native model vertices must be
// lossless. Original UV endpoints are lossless by default, or retained as exact
// rational p-curves with separate display coordinates in the opt-in mode.
type ConvexAffinePolygonCapUnion struct {
	contact   *ConvexAffinePolygonCapContact
	solid     Solid
	sources   []AffineShellCapFaceSource
	sections  []AffineShellCapEdgeSource
	fragments [2][]*AffineImprintFaceFragments
	removed   [2]*big.Rat
	volume    *big.Rat
	work      int
}

func (r *ConvexAffinePolygonCapUnion) Solid() Solid {
	if r == nil {
		return Solid{}
	}
	return r.solid
}
func (r *ConvexAffinePolygonCapUnion) InterfaceEdges() []AffineShellCapEdgeSource {
	if r == nil {
		return nil
	}
	return append([]AffineShellCapEdgeSource(nil), r.sections...)
}
func (r *ConvexAffinePolygonCapUnion) FaceSources() []AffineShellCapFaceSource {
	if r == nil {
		return nil
	}
	return append([]AffineShellCapFaceSource(nil), r.sources...)
}
func (r *ConvexAffinePolygonCapUnion) OriginalContact() *ConvexAffinePolygonCapContact {
	if r == nil {
		return nil
	}
	return r.contact
}
func (r *ConvexAffinePolygonCapUnion) ExactVolume() *big.Rat {
	if r == nil {
		return nil
	}
	return new(big.Rat).Set(r.volume)
}
func (r *ConvexAffinePolygonCapUnion) ExactOperations() int {
	if r == nil {
		return 0
	}
	return r.work
}
func (r *ConvexAffinePolygonCapUnion) RemovedCapUVArea() [2]*big.Rat {
	if r == nil {
		return [2]*big.Rat{}
	}
	return [2]*big.Rat{new(big.Rat).Set(r.removed[0]), new(big.Rat).Set(r.removed[1])}
}

// SourceFaceFragments retains original coedge ranges. Its SectionEdge IDs
// index InterfaceEdges, rather than a transverse shell section wire.
func (r *ConvexAffinePolygonCapUnion) SourceFaceFragments(owner, face int) *AffineImprintFaceFragments {
	if r == nil || owner < 0 || owner > 1 || face < 0 || face >= len(r.fragments[owner]) {
		return nil
	}
	return r.fragments[owner][face]
}

func affinePolygonFaceLoop(f Face) [][2]*big.Rat {
	p := [][2]*big.Rat{}
	for _, u := range f.Outer {
		p = append(p, [2]*big.Rat{ratInset(u.UVStart.X), ratInset(u.UVStart.Y)})
	}
	return p
}
func affinePolygonSide(b *nativePrismAffineBudget, a, c, p [2]*big.Rat) *big.Rat {
	return b.op('-', b.op('*', b.op('-', c[0], a[0]), b.op('-', p[1], a[1])), b.op('*', b.op('-', c[1], a[1]), b.op('-', p[0], a[0])))
}
func affinePolygonContains(b *nativePrismAffineBudget, polygon [][2]*big.Rat, p [2]*big.Rat) (bool, bool) {
	boundary := false
	for i, a := range polygon {
		sign := affinePolygonSide(b, a, polygon[(i+1)%len(polygon)], p).Sign()
		if sign < 0 {
			return false, false
		}
		boundary = boundary || sign == 0
	}
	return true, boundary
}
func affinePolygonClip(b *nativePrismAffineBudget, p, polygon [][2]*big.Rat) [][2]*big.Rat {
	for k, a := range polygon {
		c := polygon[(k+1)%len(polygon)]
		out := [][2]*big.Rat{}
		for i, x := range p {
			y := p[(i+1)%len(p)]
			vx, vy := affinePolygonSide(b, a, c, x), affinePolygonSide(b, a, c, y)
			if vx.Sign() >= 0 {
				out = append(out, x)
			}
			if vx.Sign()*vy.Sign() < 0 {
				t := b.op('/', vx, b.op('-', vx, vy))
				z := [2]*big.Rat{}
				for j := range z {
					z[j] = b.op('+', x[j], b.op('*', t, b.op('-', y[j], x[j])))
				}
				out = append(out, z)
			}
		}
		p = out
	}
	return p
}

// Fuse removes exactly the positive-area common cap footprint and conforms all
// original side coedges to its exact endpoints. The complete supporting-plane
// proof excludes any other cross-shell contact. All retained native surfaces
// are original. Nonlossless model endpoints or unresolved topology reject
// atomically; RationalUVEndpoints explicitly enables exact nonbinary UV recipes.
func (r *ConvexAffinePolygonCapContact) Fuse(ctx context.Context, o ConvexAffinePolygonCapFuseOptions) (*ConvexAffinePolygonCapUnion, error) {
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
	out := &ConvexAffinePolygonCapUnion{contact: r, volume: new(big.Rat), removed: [2]*big.Rat{new(big.Rat), new(big.Rat)}}
	vs, e := r.contact.Contact().Vertices()
	if e != nil {
		return nil, e
	}
	edges, e := r.contact.Contact().Edges()
	if e != nil {
		return nil, e
	}
	// Private reuse of the exact source-face arrangement engine, without claiming
	// a transverse paired-edge authorization for this independently proved cap.
	source := &AffineShellPairedImprint{sources: [2]Solid{r.shells[0].source, r.shells[1].source}}
	for owner, shell := range r.shells {
		for _, f := range shell.source.faces {
			patch, e := exactFaceSupportPatch(f, geom.SurfacePatchOptions{})
			if e != nil {
				return nil, e
			}
			source.charts[owner] = append(source.charts[owner], splineEmbeddingFace{patch: patch})
		}
	}
	sides := r.contact.EdgeTrimSides()
	uv := func(i, owner int) [2]*big.Rat {
		if owner == 0 {
			return vs[i].A
		}
		return vs[i].B
	}
	for owner := 0; owner < 2; owner++ {
		for ei, edge := range edges {
			if len(sides[ei][owner]) != 0 {
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
		q, e := r.shells[owner].queries[face].chart.InvertExactContext(ctx, p, geom.AffineChartInverseOptions{MaxOperations: b.max - b.work, MaxCoefficientBits: 16384})
		b.work += q.ExactOperations
		return q, e
	}
	selected := []SewFace{}
	sourceVolume := new(big.Rat)
	for owner, s := range source.sources {
		out.fragments[owner] = make([]*AffineImprintFaceFragments, len(s.faces))
		for face := range s.faces {
			points := [][2]*big.Rat{}
			polygon := affinePolygonFaceLoop(s.faces[face])
			for _, v := range vs {
				q, e := inverse(owner, face, v.Point)
				if e != nil {
					return nil, e
				}
				if !q.OnPlane || !q.Inside {
					continue
				}
				inside, boundary := affinePolygonContains(b, polygon, [2]*big.Rat{q.U, q.V})
				if !inside {
					continue
				}
				if face != r.faces[owner] && !boundary {
					return nil, fmt.Errorf("polygon cap fuse invariant: %w", geom.ErrDomain)
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
			part, e := source.fragmentFacePolygonPoints(ctx, owner, face, AffineImprintFragmentOptions{AuxiliarySubdivision: true, MaxOperations: b.max - b.work, MaxCycles: cycles}, points, true)
			if e != nil {
				return nil, fmt.Errorf("polygon cap fragments owner %d face %d: %w", owner, face, e)
			}
			b.work += part.ExactOperations()
			out.fragments[owner][face] = part
			var native []SewFace
			if o.RationalUVEndpoints {
				native, e = nativePolygonCapRationalFaces(ctx, part, o.Evaluation)
			} else {
				native, e = part.NativeFaces(ctx, o.Evaluation)
			}
			if e != nil {
				return nil, fmt.Errorf("polygon cap native owner %d face %d: %w", owner, face, e)
			}
			for ci, f := range native {
				if len(f.Vertices) != 3 {
					return nil, fmt.Errorf("polygon cap fuse invariant: %w", geom.ErrDomain)
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
							return nil, fmt.Errorf("polygon cap fuse invariant: %w", geom.ErrDomain)
						}
						poly = append(poly, [2]*big.Rat{q.U, q.V})
					}
					area := affineCapPolygonArea(b, poly)
					clipped := affineCapPolygonArea(b, affinePolygonClip(b, poly, affinePolygonFaceLoop(source.sources[other].faces[r.faces[other]])))
					if clipped.Cmp(area) == 0 {
						keep = false
						out.removed[owner] = b.op('+', out.removed[owner], part.cycles[ci].Area)
					} else if clipped.Sign() != 0 {
						return nil, fmt.Errorf("polygon cap fuse invariant: %w", geom.ErrDomain)
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
			return nil, fmt.Errorf("polygon cap fuse invariant: %w", geom.ErrDomain)
		}
	}
	if b.err != nil {
		return nil, b.err
	}
	expected := b.op('+', r.shells[0].volume, r.shells[1].volume)
	if out.volume.Sign() <= 0 || out.volume.Cmp(sourceVolume) != 0 || sourceVolume.Cmp(expected) != 0 {
		return nil, fmt.Errorf("polygon cap fuse invariant: %w", geom.ErrDomain)
	}
	if o.Sew.Tolerance == 0 {
		o.Sew.Tolerance = max(source.sources[0].tolerance, source.sources[1].tolerance)
	}
	o.Sew.MaxFaces = o.MaxFaces
	solid, _, e := Sew(ctx, selected, o.Sew)
	if e != nil {
		return nil, e
	}
	if e = affineMaterialVertexLinks(ctx, solid); e != nil {
		return nil, e
	}
	// Sew's tolerance must not silently merge distinct exact source vertices.
	for fi, f := range solid.faces {
		ancestry := out.sources[fi]
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
				exactUV := [2]*big.Rat{ratInset(uv.X), ratInset(uv.Y)}
				if o.RationalUVEndpoints {
					curve, ok := u.Curve.(ExactQuadraticUV)
					if !ok || !curve.rationalEndpoints {
						return nil, geom.ErrDomain
					}
					endpoints, display := curve.Endpoints()
					if display[k] != uv {
						return nil, geom.ErrDomain
					}
					exactUV = endpoints[k]
				}
				if !q.OnPlane || !q.Inside || q.U.Cmp(exactUV[0]) != 0 || q.V.Cmp(exactUV[1]) != 0 {
					return nil, fmt.Errorf("polygon cap sewn source endpoint: %w", geom.ErrDomain)
				}
			}
		}
	}
	report, e := solid.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if e != nil {
		return nil, e
	}
	if !report.Complete {
		return nil, fmt.Errorf("polygon cap fuse invariant: %w", geom.ErrDomain)
	}
	for _, u := range report.Uses {
		if u.Report.Status != edgecheck.Certified || u.Report.SquaredUpperBound == nil || u.Report.SquaredUpperBound.Sign() != 0 {
			return nil, fmt.Errorf("polygon cap fuse invariant: %w", geom.ErrDomain)
		}
	}
	if e = ctx.Err(); e != nil {
		return nil, e
	}
	out.solid = solid
	out.work = b.work
	return out, nil
}
