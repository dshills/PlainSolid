//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// ConvexAffinePolygonMaterialCollectionOptions retains the material proof limits and
// bounds component pairs (MaxComponents defaults to 8, maximum 32).
type ConvexAffinePolygonMaterialCollectionOptions struct {
	Material      ConvexAffinePolygonMaterialOptions
	MaxComponents int
}

// ConvexAffinePolygonComponentSeparation proves opposite closed halfspaces of an
// original source plane. Point is nil for strict separation. Otherwise one
// component's entire plane zero set is exactly Point, a vertex of both solids.
type ConvexAffinePolygonComponentSeparation struct {
	Components            [2]int
	PlaneOwner, PlaneFace int
	Normal                [3]*big.Rat
	Offset                *big.Rat
	Sides                 [2]int
	Point                 [3]*big.Rat
	PointSide             int // index into Components; -1 for strict separation
	SectionVertex         int // index into OriginalCrossing().SectionVertices(); -1 if separated
}
type ConvexAffinePolygonMaterialCollection struct {
	crossing   *ConvexAffinePolygonCrossing
	selection  *AffineShellMaterial
	components []Solid
	sources    [][]AffineShellMaterialFace
	volumes    []*big.Rat
	pairs      []ConvexAffinePolygonComponentSeparation
	work       int
}

func (r *ConvexAffinePolygonMaterialCollection) OriginalCrossing() *ConvexAffinePolygonCrossing {
	if r == nil {
		return nil
	}
	return r.crossing
}
func (r *ConvexAffinePolygonMaterialCollection) Fragments() []AffineShellMaterialFace {
	if r == nil {
		return nil
	}
	return r.selection.Fragments()
}
func (r *ConvexAffinePolygonMaterialCollection) Components() []Solid {
	if r == nil {
		return nil
	}
	return append([]Solid(nil), r.components...)
}
func (r *ConvexAffinePolygonMaterialCollection) FaceSources(component int) []AffineShellMaterialFace {
	if r == nil || component < 0 || component >= len(r.sources) {
		return nil
	}
	return append([]AffineShellMaterialFace(nil), r.sources[component]...)
}
func (r *ConvexAffinePolygonMaterialCollection) ComponentVolumes() []*big.Rat {
	if r == nil {
		return nil
	}
	out := make([]*big.Rat, len(r.volumes))
	for i, v := range r.volumes {
		out[i] = new(big.Rat).Set(v)
	}
	return out
}
func (r *ConvexAffinePolygonMaterialCollection) ExactVolume() *big.Rat {
	if r == nil {
		return nil
	}
	return r.selection.ExactVolume()
}
func (r *ConvexAffinePolygonMaterialCollection) ExactOperations() int {
	if r == nil {
		return 0
	}
	return r.work
}
func (r *ConvexAffinePolygonMaterialCollection) SourceFaceFragments(owner, face int) *AffineImprintFaceFragments {
	if r == nil {
		return nil
	}
	return r.selection.SourceFaceFragments(owner, face)
}
func (r *ConvexAffinePolygonMaterialCollection) Separations() []ConvexAffinePolygonComponentSeparation {
	if r == nil {
		return nil
	}
	out := append([]ConvexAffinePolygonComponentSeparation(nil), r.pairs...)
	for i := range out {
		out[i].Offset = new(big.Rat).Set(out[i].Offset)
		for k := 0; k < 3; k++ {
			out[i].Normal[k] = new(big.Rat).Set(out[i].Normal[k])
			if out[i].Point[k] != nil {
				out[i].Point[k] = new(big.Rat).Set(out[i].Point[k])
			}
		}
	}
	return out
}

// CombineComponents accepts independently closed manifold components that
// may meet at explicitly proved isolated vertices. It never merges those vertex
// identities across components. Interval/area contacts and a pinched connected
// component remain unsupported. The strict Combine API is unchanged.
func (s *ConvexAffinePolygonCrossing) CombineComponents(ctx context.Context, op intersect.AffineRingOperation, o ConvexAffinePolygonMaterialCollectionOptions) (*ConvexAffinePolygonMaterialCollection, error) {
	if o.MaxComponents == 0 {
		o.MaxComponents = 8
	}
	if o.MaxComponents < 1 || o.MaxComponents > 32 {
		return nil, geom.ErrInvalid
	}
	selection, faces, options, e := s.selectPolygonMaterial(ctx, op, o.Material)
	if e != nil {
		return nil, e
	}
	b := &nativePrismAffineBudget{ctx: ctx, max: options.MaxOperations, bits: 16384, work: selection.work}
	// Identity keys use only lossless represented endpoints already proved by
	// NativeFaces. Continuous equality is re-proved by Sew inside each component.
	type edgeKey struct{ a, c geom.Vec3 }
	type use struct {
		face    int
		forward bool
	}
	less := func(a, c geom.Vec3) bool {
		if a.X != c.X {
			return a.X < c.X
		}
		if a.Y != c.Y {
			return a.Y < c.Y
		}
		return a.Z < c.Z
	}
	edges := map[edgeKey][]use{}
	adj := make([][]int, len(faces))
	for fi, f := range faces {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		for _, u := range f.Face.Outer {
			edge := f.Edges[u.Edge]
			a, c := f.Vertices[edge.Start], f.Vertices[edge.End]
			if u.Reversed != f.Face.Reversed {
				a, c = c, a
			}
			forward := less(a, c)
			if a == c {
				return nil, geom.ErrDomain
			}
			if !forward {
				a, c = c, a
			}
			key := edgeKey{a, c}
			edges[key] = append(edges[key], use{fi, forward})
			if len(edges[key]) > 2 {
				return nil, geom.ErrDomain
			}
		}
	}
	for _, uses := range edges {
		if len(uses) != 2 || uses[0].face == uses[1].face || uses[0].forward == uses[1].forward {
			return nil, geom.ErrDomain
		}
		a, c := uses[0].face, uses[1].face
		adj[a] = append(adj[a], c)
		adj[c] = append(adj[c], a)
	}
	labels := make([]int, len(faces))
	for i := range labels {
		labels[i] = -1
	}
	groups := [][]int{}
	for start := range faces {
		if labels[start] >= 0 {
			continue
		}
		if len(groups) >= o.MaxComponents {
			return nil, geom.ErrConvergence
		}
		id := len(groups)
		queue := []int{start}
		labels[start] = id
		for at := 0; at < len(queue); at++ {
			for _, next := range adj[queue[at]] {
				if labels[next] < 0 {
					labels[next] = id
					queue = append(queue, next)
				}
			}
		}
		groups = append(groups, queue)
	}
	out := &ConvexAffinePolygonMaterialCollection{selection: selection, crossing: s}
	history := selection.FaceSources()
	sum := new(big.Rat)
	for _, group := range groups {
		// Stable output face order follows original selection, not hash adjacency.
		selected := []SewFace{}
		source := []AffineShellMaterialFace{}
		volume := new(big.Rat)
		id := labels[group[0]]
		for fi, f := range faces {
			if labels[fi] != id {
				continue
			}
			selected = append(selected, f)
			source = append(source, history[fi])
			x, y, z := affineMaterialXYZ(f.Vertices[0]), affineMaterialXYZ(f.Vertices[1]), affineMaterialXYZ(f.Vertices[2])
			det := new(big.Rat)
			for k := 0; k < 3; k++ {
				j, l := (k+1)%3, (k+2)%3
				det = b.op('+', det, b.op('*', x[k], b.op('-', b.op('*', y[j], z[l]), b.op('*', y[l], z[j]))))
			}
			if f.Face.Reversed {
				det.Neg(det)
			}
			volume = b.op('+', volume, b.op('/', det, big.NewRat(6, 1)))
		}
		if b.err != nil {
			return nil, b.err
		}
		if volume.Sign() <= 0 {
			return nil, geom.ErrDomain
		}
		if e := polygonComponentLinks(b, s, selected, id); e != nil {
			return nil, e
		}
		solid, _, e := Sew(ctx, selected, options.Sew)
		if e != nil {
			return nil, e
		}
		if e = affineMaterialVertexLinks(ctx, solid); e != nil {
			return nil, e
		}
		if !affinePolygonConnectedFaces(solid) {
			return nil, geom.ErrDomain
		}
		if e = s.verifyPolygonMaterialSolid(ctx, b, solid, source, options); e != nil {
			return nil, e
		}
		out.components = append(out.components, solid)
		out.sources = append(out.sources, source)
		out.volumes = append(out.volumes, volume)
		sum = b.op('+', sum, volume)
	}
	if sum.Cmp(selection.volume) != 0 {
		return nil, geom.ErrInvalid
	}
	for a := range out.components {
		for c := a + 1; c < len(out.components); c++ {
			proof, ok := polygonComponentSeparator(b, s, out.components[a], out.components[c])
			if b.err != nil {
				return nil, b.err
			}
			if !ok {
				return nil, geom.ErrDomain
			}
			proof.Components = [2]int{a, c}
			out.pairs = append(out.pairs, proof)
		}
	}
	if b.err != nil {
		return nil, b.err
	}
	if e = ctx.Err(); e != nil {
		return nil, e
	}
	out.work = b.work
	return out, nil
}

func polygonComponentSeparator(b *nativePrismAffineBudget, s *ConvexAffinePolygonCrossing, a, c Solid) (ConvexAffinePolygonComponentSeparation, bool) {
	for owner := 0; owner < 2; owner++ {
		for fi, query := range s.shells[owner].queries {
			p := query.Plane()
			plane := affineMaterialPlane{n: [3]*big.Rat{p[0], p[1], p[2]}, d: new(big.Rat).Neg(p[3])}
			for _, direction := range []int{-1, 1} {
				proof := ConvexAffinePolygonComponentSeparation{PlaneOwner: owner, PlaneFace: fi, Normal: plane.n, Offset: plane.d, Sides: [2]int{direction, -direction}, PointSide: -1, SectionVertex: -1}
				zeros := [2][]geom.Vec3{}
				valid := true
				for side, solid := range []Solid{a, c} {
					for _, v := range solid.vertices {
						value := affineMaterialValue(b, plane, affineMaterialXYZ(v))
						sign := value.Sign()
						if sign*proof.Sides[side] < 0 {
							valid = false
							break
						}
						if sign == 0 {
							zeros[side] = append(zeros[side], v)
						}
					}
					if !valid {
						break
					}
				}
				if b.err != nil {
					return ConvexAffinePolygonComponentSeparation{}, false
				}
				if !valid {
					continue
				}
				if len(zeros[0]) == 0 || len(zeros[1]) == 0 {
					return proof, true
				}
				for side := 0; side < 2; side++ {
					if len(zeros[side]) != 1 {
						continue
					}
					point := zeros[side][0]
					for _, other := range zeros[1-side] {
						if point == other {
							proof.Point = affineMaterialXYZ(point)
							for ni, node := range s.nodes {
								if len(node.Owners) != 2 {
									continue
								}
								q := node.Point
								same := true
								for k := 0; k < 3; k++ {
									same = same && q[k].Cmp(proof.Point[k]) == 0
								}
								if same {
									proof.SectionVertex = ni
									break
								}
							}
							if proof.SectionVertex < 0 {
								continue
							}
							proof.PointSide = side
							return proof, true
						}
					}
				}
			}
		}
	}
	return ConvexAffinePolygonComponentSeparation{}, false
}

// ConvexAffinePolygonShellCollectionOptions bounds original crossing discovery
// and component reconstruction independently. Paired junctions remain opt-in.
type ConvexAffinePolygonShellCollectionOptions struct {
	Crossing ConvexAffinePolygonCrossingOptions
	Material ConvexAffinePolygonMaterialCollectionOptions
}

// CombineConvexAffinePolygonShellComponents reauthenticates both native sources.
// It accepts only individually closed components with certified strict or
// isolated original-vertex separation; it does not weld separate components.
func CombineConvexAffinePolygonShellComponents(ctx context.Context, a, c Solid, op parametric.BooleanOp, o ConvexAffinePolygonShellCollectionOptions) (*ConvexAffinePolygonMaterialCollection, error) {
	var operation intersect.AffineRingOperation
	switch op {
	case parametric.Intersection:
		operation = intersect.AffineRingIntersection
	case parametric.Union:
		operation = intersect.AffineRingUnion
	case parametric.Difference:
		operation = intersect.AffineRingDifference
	default:
		return nil, geom.ErrInvalid
	}
	crossing, e := CertifyConvexAffinePolygonCrossing(ctx, a, c, o.Crossing)
	if e != nil {
		return nil, e
	}
	return crossing.CombineComponents(ctx, operation, o.Material)
}
