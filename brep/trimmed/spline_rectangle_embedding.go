package trimmed

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// SplineRectangleEmbeddingOptions controls a sufficient global embedding
// proof for one shell of rectangular, clamped rational spline faces.
// MaxFaces defaults to 64 (maximum 128). Other limits apply per proof.
type SplineRectangleEmbeddingOptions struct {
	Patches       geom.SurfacePatchOptions
	Injectivity   geom.SurfaceInjectivityOptions
	Plane         geom.SurfaceSupportingPlaneOptions
	Adjacency     geom.SurfaceAdjacencyOptions
	Vertex        geom.SurfaceVertexSeparationOptions
	Separation    geom.SurfaceSurfaceIsolationOptions
	EdgeAgreement edgecheck.Options
	MaxFaces      int
}

// SplineRectanglePairProof identifies either strict separation or contact
// confined to the pair's topologically shared boundary. SupportingFace is
// -1 without a planar supporting face. SharedEdge is -1 unless an independent
// separating plane proves that two full sides are their entire intersection.
// SharedVertex is -1 unless an independent plane isolates a shared corner.
type SplineRectanglePairProof struct {
	A, B, SupportingFace, SharedEdge, SharedVertex int
}

// SplineRectangleEmbedding reports exact certificates on the ideal rational
// geometry. Complete means the single shell is embedded and outward oriented.
// OutwardFace is -1 when exact signed volume proves the orientation instead.
// A missing certificate does not by itself prove that a shell is invalid.
type SplineRectangleEmbedding struct {
	Faces       []geom.SurfaceInjectivity
	Pairs       []SplineRectanglePairProof
	OutwardFace int
	Complete    bool
}

type splineEmbeddingFace struct {
	patch     *geom.SurfacePatch
	vertex    [4]int // counterclockwise from the lower-left UV corner
	edge      [4]int // geom.SurfaceEdge order
	plane     [3]geom.Vec3
	planeSign int // chart normal relative to the raw supporting plane normal
}

// ValidateSplineRectangleEmbedding proves embedding for a single outward
// spline shell whose face pairs either separate or meet only on supported
// planar contacts. Each face must cover a full rectangle of its chart,
// with four exact isoparametric boundary edges. This is useful for curved
// prisms and related shells, including some adjacent nonplanar faces.
// Coplanar face partitions, multiple shells, poles and periodic seams may be
// unresolved.
func (s Solid) ValidateSplineRectangleEmbedding(ctx context.Context, o SplineRectangleEmbeddingOptions) (r SplineRectangleEmbedding, err error) {
	return s.validateSplineRectangleEmbedding(ctx, o, nil)
}

func (s Solid) validateSplineRectangleEmbedding(ctx context.Context, o SplineRectangleEmbeddingOptions, capture *[]splineEmbeddingFace) (r SplineRectangleEmbedding, err error) {
	r.OutwardFace = -1
	if o.MaxFaces == 0 {
		o.MaxFaces = 64
	}
	if ctx == nil || o.MaxFaces < 1 || o.MaxFaces > 128 || len(s.faces) == 0 {
		return r, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if len(s.faces) > o.MaxFaces {
		return r, parametric.ErrBudget
	}
	if len(s.Topology().Shells) != 1 {
		return r, fmt.Errorf("%w: expected one connected spline shell", geom.ErrConvergence)
	}
	faces := make([]splineEmbeddingFace, 0, len(s.faces))
	for i, f := range s.faces {
		if e := ctx.Err(); e != nil {
			return r, e
		}
		x, proof, e := s.splineEmbeddingFace(ctx, f, o)
		if e != nil {
			return r, fmt.Errorf("trimmed: spline embedding face %d: %w", i, e)
		}
		faces = append(faces, x)
		r.Faces = append(r.Faces, proof)
	}
	if !s.splineRectangleVertexLinks(faces) {
		return r, fmt.Errorf("%w: disconnected vertex link", geom.ErrDegenerate)
	}
	type planeResult struct {
		q    geom.SurfaceSupportingPlaneCertificate
		done bool
	}
	cache := make([]planeResult, len(faces)*len(faces))
	contact := func(i, j int) (geom.SurfaceSupportingPlaneCertificate, error) {
		x := &cache[i*len(faces)+j]
		if x.done || faces[i].planeSign == 0 {
			return x.q, nil
		}
		q, e := geom.CertifySurfaceSupportingPlane(ctx, faces[j].patch, faces[i].plane[0], faces[i].plane[1], faces[i].plane[2], o.Plane)
		x.q, x.done = q, true
		if e != nil && !errors.Is(e, geom.ErrConvergence) {
			return q, e
		}
		return q, nil
	}
	planar := make([]bool, len(faces))
	for i := range faces {
		q, e := contact(i, i)
		if e != nil {
			return r, e
		}
		planar[i] = q.Complete && q.Coincident
	}
	for i := range faces {
		for j := i + 1; j < len(faces); j++ {
			if e := ctx.Err(); e != nil {
				return r, e
			}
			a, b := faces[i], faces[j]
			if !splineRectanglesShareVertex(a, b) {
				x, _ := a.patch.Bounds()
				y, _ := b.patch.Bounds()
				if _, overlap := x.Intersect(y); !overlap {
					r.Pairs = append(r.Pairs, SplineRectanglePairProof{i, j, -1, -1, -1})
					continue
				}
			}
			support := -1
			for _, pair := range [2][2]int{{i, j}, {j, i}} {
				if !planar[pair[0]] {
					continue
				}
				q, e := contact(pair[0], pair[1])
				if e != nil {
					return r, e
				}
				if q.Complete && !q.Coincident && rectangleContactsMatch(faces[pair[0]], faces[pair[1]], q) {
					support = pair[0]
					break
				}
			}
			if support >= 0 {
				r.Pairs = append(r.Pairs, SplineRectanglePairProof{i, j, support, -1, -1})
				continue
			}
			if ai, bi, ok := splineRectangleSingleSharedEdge(a, b); ok {
				q, e := geom.CertifyProjectedAdjacentSurfaceSeparation(ctx, a.patch, b.patch, ai, bi, o.Adjacency)
				if errors.Is(e, geom.ErrConvergence) || errors.Is(e, geom.ErrUnrepresentable) {
					q, e = geom.CertifyAdjacentSurfaceSeparation(ctx, a.patch, b.patch, ai, bi, o.Adjacency)
				}
				if errors.Is(e, geom.ErrConvergence) || errors.Is(e, geom.ErrUnrepresentable) {
					z, err := geom.CertifyQuadricAdjacentSurfaceSeparation(ctx, a.patch, b.patch, ai, bi, o.Adjacency)
					e = err
					if z.Complete && z.Separated {
						r.Pairs = append(r.Pairs, SplineRectanglePairProof{i, j, -1, a.edge[ai], -1})
						continue
					}
					if errors.Is(e, geom.ErrConvergence) || errors.Is(e, geom.ErrUnrepresentable) {
						separated, alternateErr := proveSplineCyclicQuadricAdjacency(ctx, a.patch, b.patch, ai, bi, o.Adjacency)
						e = alternateErr
						if separated {
							r.Pairs = append(r.Pairs, SplineRectanglePairProof{i, j, -1, a.edge[ai], -1})
							continue
						}
					}
				}
				if e != nil && !errors.Is(e, geom.ErrConvergence) && !errors.Is(e, geom.ErrUnrepresentable) {
					return r, e
				}
				if q.Complete && q.Separated {
					r.Pairs = append(r.Pairs, SplineRectanglePairProof{i, j, -1, a.edge[ai], -1})
					continue
				}
			}
			if ai, bi, ok := splineRectangleSingleSharedVertex(a, b); ok {
				q, e := geom.CertifySurfaceVertexSeparation(ctx, a.patch, b.patch, ai, bi, o.Vertex)
				if e != nil && !errors.Is(e, geom.ErrConvergence) && !errors.Is(e, geom.ErrUnrepresentable) {
					return r, e
				}
				if q.Complete && q.Separated {
					r.Pairs = append(r.Pairs, SplineRectanglePairProof{i, j, -1, -1, a.vertex[ai]})
					continue
				}
			}
			if !splineRectanglesShareVertex(a, b) {
				isolation := o.Separation
				isolation.SeparatingProjections = true
				q, e := geom.BoundSurfaceSurfaceContext(ctx, a.patch, b.patch, isolation)
				if e != nil && !errors.Is(e, geom.ErrConvergence) {
					return r, e
				}
				if q.Complete && len(q.Candidates) == 0 && len(q.Coincident) == 0 {
					r.Pairs = append(r.Pairs, SplineRectanglePairProof{i, j, -1, -1, -1})
					continue
				}
			}
			return r, fmt.Errorf("trimmed: cannot prove spline faces %d and %d meet only along shared topology: %w", i, j, geom.ErrConvergence)
		}
	}
	// At a globally supporting face the bounded material is on the same
	// side as the rest of the boundary. Compare that side with its oriented
	// normal to distinguish an outward shell from its reversed copy.
	for i := range faces {
		if !planar[i] {
			continue
		}
		inside, valid := 0, true
		for j := range faces {
			q, e := contact(i, j)
			if e != nil {
				return r, e
			}
			if !q.Complete || q.HalfspaceSign != 0 && inside != 0 && q.HalfspaceSign != inside {
				valid = false
				break
			}
			if q.HalfspaceSign != 0 {
				inside = q.HalfspaceSign
			}
		}
		if !valid || inside == 0 {
			continue
		}
		if faces[i].planeSign == inside {
			return r, fmt.Errorf("%w: spline shell is inward oriented", geom.ErrDegenerate)
		}
		r.OutwardFace, r.Complete = i, true
		if capture != nil {
			*capture = faces
		}
		return r, nil
	}
	// Once incidence and pair separation prove an embedded oriented shell,
	// a positive exact signed volume enclosure also proves outward winding.
	for _, depth := range []int{0, 2, 4, 6} {
		lo, hi := new(big.Rat), new(big.Rat)
		for i, f := range faces {
			a, b, e := geom.BoundSurfaceOrientedVolumeFlux(ctx, f.patch, s.vertices[0], depth, geom.SurfacePatchEvaluationOptions{
				MaxOperations: o.Patches.MaxOperations, MaxCoefficientBits: o.Patches.MaxCoefficientBits,
			})
			if e != nil {
				return r, e
			}
			if s.faces[i].Reversed {
				a, b = b.Neg(b), a.Neg(a)
			}
			lo.Add(lo, a)
			hi.Add(hi, b)
		}
		if hi.Sign() < 0 {
			return r, fmt.Errorf("%w: spline shell is inward oriented", geom.ErrDegenerate)
		}
		if lo.Sign() > 0 {
			r.Complete = true
			if capture != nil {
				*capture = faces
			}
			return r, nil
		}
	}
	return r, fmt.Errorf("%w: no global supporting face proved shell orientation", geom.ErrConvergence)
}

// The quadric search uses Z as its dependent coordinate. Exact proper axis
// permutations provide two other search directions while leaving every UV
// coordinate, curve parameter and incidence unchanged. The proof depends on
// exact equivalence under the permutation, not on a candidate face normal.
func proveSplineCyclicQuadricAdjacency(ctx context.Context, a, b *geom.SurfacePatch, ai, bi geom.SurfaceEdge, o geom.SurfaceAdjacencyOptions) (bool, error) {
	for _, m := range []geom.Affine{{{0, 1, 0, 0}, {0, 0, 1, 0}, {1, 0, 0, 0}}, {{0, 0, 1, 0}, {1, 0, 0, 0}, {0, 1, 0, 0}}} {
		if e := ctx.Err(); e != nil {
			return false, e
		}
		limits := geom.SurfacePatchOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits}
		x, e := a.WithLimits(limits)
		if e != nil {
			return false, e
		}
		x, e = x.Transformed(m)
		if e != nil {
			return false, e
		}
		y, e := b.WithLimits(limits)
		if e != nil {
			return false, e
		}
		y, e = y.Transformed(m)
		if e != nil {
			return false, e
		}
		r, e := geom.CertifyQuadricAdjacentSurfaceSeparation(ctx, x, y, ai, bi, o)
		if e != nil && !errors.Is(e, geom.ErrConvergence) && !errors.Is(e, geom.ErrUnrepresentable) {
			return false, e
		}
		if r.Complete && r.Separated {
			return true, nil
		}
	}
	return false, geom.ErrConvergence
}

func (s Solid) splineEmbeddingFace(ctx context.Context, f Face, o SplineRectangleEmbeddingOptions) (x splineEmbeddingFace, proof geom.SurfaceInjectivity, err error) {
	if len(f.Outer) != 4 || len(f.Holes) != 0 {
		return x, proof, geom.ErrConvergence
	}
	d, e := faceUVBounds(f)
	if e != nil || !d.Valid() {
		return x, proof, geom.ErrInvalid
	}
	corners := [4]geom.Vec2{{X: d.UMin, Y: d.VMin}, {X: d.UMax, Y: d.VMin},
		{X: d.UMax, Y: d.VMax}, {X: d.UMin, Y: d.VMax}}
	base, m := parametric.SurfaceMapping(f.Surface)
	p, e := exactSplineSupportPatch(base, o.Patches)
	if e != nil {
		return x, proof, e
	}
	p, e = p.Trim(d)
	if e != nil {
		return x, proof, e
	}
	if m != geom.Identity() {
		p, e = p.Transformed(m)
		if e != nil {
			return x, proof, e
		}
	}
	x.patch = p
	var point [4]geom.Vec3
	var seen uint8
	for _, u := range f.Outer {
		i := -1
		for j, a := range corners {
			if a == u.UVStart && corners[(j+1)%4] == u.UVEnd {
				i = j
				break
			}
		}
		if i < 0 || seen&(1<<i) != 0 || !exactAffineNativeSourceUse(u) || u.Edge < 0 || u.Edge >= len(s.edges) {
			return x, proof, geom.ErrConvergence
		}
		seen |= 1 << i
		edge := s.edges[u.Edge]
		if edge.Start == edge.End {
			return x, proof, geom.ErrConvergence
		}
		x.vertex[i] = edge.Start
		if u.Reversed {
			x.vertex[i] = edge.End
		}
		x.edge[[4]geom.SurfaceEdge{geom.SurfaceEdgeVMin, geom.SurfaceEdgeUMax, geom.SurfaceEdgeVMax, geom.SurfaceEdgeUMin}[i]] = u.Edge
		if !sameSplineTrim(edge, f.Surface, u) {
			match, e := compareSplineEdgeUse(ctx, edge, f.Surface, u, o.EdgeAgreement)
			if e != nil {
				return x, proof, e
			}
			if !match {
				return x, proof, fmt.Errorf("%w: edge agreement is not exact", geom.ErrConvergence)
			}
		}
		c, e := p.IsoCurve(0, u.UVStart.X, o.Patches)
		if e != nil {
			return x, proof, e
		}
		point[i], e = c.ExactPoint(u.UVStart.Y)
		if e != nil {
			return x, proof, e
		}
		if point[i] != s.vertices[x.vertex[i]] {
			return x, proof, fmt.Errorf("%w: exact patch corner differs from its shared vertex", geom.ErrConvergence)
		}
	}
	projections := [][2]int{o.Injectivity.ProjectionAxes}
	if o.Injectivity.ProjectionAxes == ([2]int{}) {
		projections = [][2]int{{0, 1}, {0, 2}, {1, 2}}
	}
	for _, axes := range projections {
		if e := ctx.Err(); e != nil {
			return x, proof, e
		}
		options := o.Injectivity
		options.ProjectionAxes = axes
		q, e := geom.CertifySurfaceInjectivity(p, options)
		if e != nil && !errors.Is(e, geom.ErrConvergence) {
			return x, proof, e
		}
		proof = q
		if proof.ProvenInjective {
			break
		}
	}
	if !proof.ProvenInjective || proof.ProjectionOrientation == 0 {
		return x, proof, fmt.Errorf("%w: no projection injectivity proof", geom.ErrConvergence)
	}
	project := func(p geom.Vec3) geom.Vec2 {
		coords := [3]float64{p.X, p.Y, p.Z}
		return geom.Vec2{X: coords[proof.ProjectionAxes[0]], Y: coords[proof.ProjectionAxes[1]]}
	}
	for _, t := range [][3]int{{0, 1, 3}, {0, 1, 2}, {0, 2, 3}, {1, 2, 3}} {
		a, b, c := point[t[0]], point[t[1]], point[t[2]]
		sign, e := geom.Orient2D(project(a), project(b), project(c))
		if e != nil {
			return x, proof, e
		}
		if sign != 0 {
			x.plane, x.planeSign = [3]geom.Vec3{a, b, c}, sign*proof.ProjectionOrientation
			if f.Reversed {
				x.planeSign = -x.planeSign
			}
			break
		}
	}
	return x, proof, nil
}

func splineRectanglesShareVertex(a, b splineEmbeddingFace) bool {
	for _, i := range a.vertex {
		for _, j := range b.vertex {
			if i == j {
				return true
			}
		}
	}
	return false
}

func splineRectangleSingleSharedVertex(a, b splineEmbeddingFace) (int, int, bool) {
	i, j := -1, -1
	for x, id := range a.vertex {
		for y, other := range b.vertex {
			if id == other {
				if i >= 0 {
					return 0, 0, false
				}
				i, j = x, y
			}
		}
	}
	return i, j, i >= 0
}

func splineRectangleSingleSharedEdge(a, b splineEmbeddingFace) (geom.SurfaceEdge, geom.SurfaceEdge, bool) {
	i, j := -1, -1
	for x, id := range a.edge {
		for y, other := range b.edge {
			if id == other {
				if i >= 0 {
					return 0, 0, false
				}
				i, j = x, y
			}
		}
	}
	if i < 0 {
		return 0, 0, false
	}
	n := 0
	for _, x := range a.vertex {
		for _, y := range b.vertex {
			if x == y {
				n++
			}
		}
	}
	return geom.SurfaceEdge(i), geom.SurfaceEdge(j), n == 2
}

func rectangleContactsMatch(a, b splineEmbeddingFace, q geom.SurfaceSupportingPlaneCertificate) bool {
	covered := q.CornerMask
	sideCorners := [4]uint8{0b1001, 0b0110, 0b0011, 0b1100}
	for i, id := range b.edge {
		shared := false
		for _, e := range a.edge {
			shared = shared || e == id
		}
		onPlane := q.SideMask&(1<<i) != 0
		if shared != onPlane {
			return false
		}
		if onPlane {
			covered |= sideCorners[i]
		}
	}
	for i, id := range b.vertex {
		shared := false
		for _, v := range a.vertex {
			shared = shared || v == id
		}
		if shared != (covered&(1<<i) != 0) {
			return false
		}
	}
	return true
}

// An edge manifold can still identify two disconnected rings at one
// vertex. Prove one connected link per vertex from face corner incidences.
func (s Solid) splineRectangleVertexLinks(faces []splineEmbeddingFace) bool {
	adj := make([]map[int][]int, len(s.vertices))
	// Indices are the two rectangle sides incident to each UV corner.
	links := [4][2]geom.SurfaceEdge{
		{geom.SurfaceEdgeUMin, geom.SurfaceEdgeVMin}, {geom.SurfaceEdgeVMin, geom.SurfaceEdgeUMax},
		{geom.SurfaceEdgeUMax, geom.SurfaceEdgeVMax}, {geom.SurfaceEdgeVMax, geom.SurfaceEdgeUMin},
	}
	for _, f := range faces {
		for i, id := range f.vertex {
			if adj[id] == nil {
				adj[id] = make(map[int][]int)
			}
			a, b := f.edge[links[i][0]], f.edge[links[i][1]]
			adj[id][a] = append(adj[id][a], b)
			adj[id][b] = append(adj[id][b], a)
		}
	}
	for _, graph := range adj {
		if len(graph) < 2 {
			return false
		}
		var stack []int
		for id := range graph {
			stack = append(stack, id)
			break
		}
		seen := map[int]bool{}
		for len(stack) > 0 {
			id := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if seen[id] {
				continue
			}
			if len(graph[id]) != 2 {
				return false
			}
			seen[id] = true
			stack = append(stack, graph[id]...)
		}
		if len(seen) != len(graph) {
			return false
		}
	}
	return true
}
