package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// AffineTriangleShellOptions bounds independent projection and native-use
// proofs. MaxWork defaults to 64M. Each use conservatively reserves its full
// configured Proof.MaxWork (default 1M) from the cumulative allowance.
type AffineTriangleShellOptions struct {
	MaxWork    int
	Projection [2]geom.SurfaceInjectivityOptions
	Proof      edgecheck.Options
}

// AffineTriangleMateShell is an embedded indexed open disk, never a Solid.
// Original triangle indices are retained in ancestry: edge i joins original
// vertices i and (i+1)%3. Native outer uses may reverse that authored order.
type AffineTriangleMateShell struct {
	contact    *geom.SurfaceAffineTriangleMate
	graph      *WrappedAlgebraicOpenShell
	operations int
}

func (s *AffineTriangleMateShell) Contact() *geom.SurfaceAffineTriangleMate {
	if s == nil {
		return nil
	}
	return s.contact
}
func (s *AffineTriangleMateShell) Vertices() []geom.Vec3 {
	if s == nil {
		return nil
	}
	return s.graph.Vertices()
}
func (s *AffineTriangleMateShell) Edges() []parametric.Edge {
	if s == nil {
		return nil
	}
	return s.graph.Edges()
}
func (s *AffineTriangleMateShell) Faces() []Face {
	if s == nil {
		return nil
	}
	return s.graph.Faces()
}
func (s *AffineTriangleMateShell) Boundary() []WrappedOpenBoundaryUse {
	if s == nil {
		return nil
	}
	return s.graph.Boundary()
}
func (s *AffineTriangleMateShell) VertexSources(i int) ([]WrappedOpenVertexSource, error) {
	if s == nil {
		return nil, geom.ErrInvalid
	}
	return s.graph.VertexSources(i)
}
func (s *AffineTriangleMateShell) EdgeSources(i int) ([]WrappedOpenEdgeSource, error) {
	if s == nil {
		return nil, geom.ErrInvalid
	}
	return s.graph.EdgeSources(i)
}
func (s *AffineTriangleMateShell) ExactOperations() int {
	if s == nil {
		return 0
	}
	return s.operations
}

// NewAffineTriangleMateShell accepts only exact single-cell original supports
// and exactly representable authored UV/model vertices. Internal model curves
// retain exact supports; no poles or endpoints are snapped. Each entire source
// chart is independently proved projected-injective. Contact's strict affine
// triangle signs exclude all additional inter-face contact. Both outer loops
// must induce opposed physical uses on the proved mate; otherwise reject.
func NewAffineTriangleMateShell(ctx context.Context, contact *geom.SurfaceAffineTriangleMate, o AffineTriangleShellOptions) (*AffineTriangleMateShell, error) {
	if ctx == nil || contact == nil {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if o.MaxWork == 0 {
		o.MaxWork = 64000000
	}
	if o.Proof.MaxWork == 0 {
		o.Proof.MaxWork = 1000000
	}
	if o.Proof.MaxBits == 0 {
		o.Proof.MaxBits = 16384
	}
	if o.MaxWork < 1 || o.Proof.MaxWork < 1 || o.Proof.MaxBits < 64 {
		return nil, geom.ErrInvalid
	}
	graph := &WrappedAlgebraicOpenShell{}
	out := &AffineTriangleMateShell{contact: contact, graph: graph}
	var uv [2][3]geom.Vec2
	var points [2][3][3]*big.Rat
	var surfaces [2]*geom.ExactBezierSurface
	var positive [2]bool
	for owner := 0; owner < 2; owner++ {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		p, t, e := contact.Source(owner)
		if e != nil {
			return nil, e
		}
		surfaces[owner], e = geom.NewExactBezierSurface(p)
		if e != nil {
			return nil, e
		}
		projection := o.Projection[owner]
		if projection.MaxOperations == 0 {
			projection.MaxOperations = 2000000
		}
		if projection.MaxCoefficientBits == 0 {
			projection.MaxCoefficientBits = o.Proof.MaxBits
		}
		if projection.MaxOperations < 1 || projection.MaxCoefficientBits > o.Proof.MaxBits {
			return nil, geom.ErrInvalid
		}
		if out.operations >= o.MaxWork {
			return nil, geom.ErrConvergence
		}
		projection.MaxOperations = min(projection.MaxOperations, o.MaxWork-out.operations)
		proof, e := geom.CertifySurfaceInjectivity(p, projection)
		if e != nil {
			return nil, e
		}
		out.operations += proof.ExactOperations
		if !proof.ProvenInjective {
			return nil, geom.ErrConvergence
		}
		for _, point := range t {
			for _, v := range point {
				out.operations++
				if out.operations > o.MaxWork || v.Num().BitLen() > o.Proof.MaxBits || v.Denom().BitLen() > o.Proof.MaxBits {
					return nil, geom.ErrConvergence
				}
			}
		}
		// Determinant is exact; the immutable contact already proves nondegeneracy.
		det := new(big.Rat).Sub(new(big.Rat).Mul(new(big.Rat).Sub(t[1][0], t[0][0]), new(big.Rat).Sub(t[2][1], t[0][1])), new(big.Rat).Mul(new(big.Rat).Sub(t[1][1], t[0][1]), new(big.Rat).Sub(t[2][0], t[0][0])))
		out.operations++
		if out.operations > o.MaxWork || det.Num().BitLen() > o.Proof.MaxBits || det.Denom().BitLen() > o.Proof.MaxBits {
			return nil, geom.ErrConvergence
		}
		if det.Sign() == 0 {
			return nil, geom.ErrDomain
		}
		positive[owner] = det.Sign() > 0
		for i, v := range t {
			x, xok := v[0].Float64()
			y, yok := v[1].Float64()
			if !xok || !yok {
				return nil, geom.ErrUnrepresentable
			}
			uv[owner][i] = geom.Vec2{X: x, Y: y}
			if out.operations >= o.MaxWork {
				return nil, geom.ErrConvergence
			}
			value, e := p.EvaluateRational(ctx, v[0], v[1], geom.SurfacePatchEvaluationOptions{MaxOperations: o.MaxWork - out.operations, MaxCoefficientBits: o.Proof.MaxBits})
			if e != nil {
				return nil, e
			}
			out.operations += value.ExactOperations
			points[owner][i] = value.Point
		}
	}
	if positive[0] == positive[1] {
		return nil, geom.ErrDomain
	}
	for i := 0; i < 2; i++ {
		for axis := 0; axis < 3; axis++ {
			if points[0][i][axis].Cmp(points[1][i][axis]) != 0 {
				return nil, geom.ErrDomain
			}
		}
	}
	for _, source := range [][2]int{{0, 0}, {0, 1}, {0, 2}, {1, 2}} {
		p := points[source[0]][source[1]]
		var xyz [3]float64
		for j := range p {
			var exact bool
			xyz[j], exact = p[j].Float64()
			if !exact {
				return nil, geom.ErrUnrepresentable
			}
		}
		graph.vertices = append(graph.vertices, geom.Vec3{X: xyz[0], Y: xyz[1], Z: xyz[2]})
		graph.vertexSources = append(graph.vertexSources, []WrappedOpenVertexSource{{Face: source[0], Vertex: source[1]}})
	}
	graph.vertexSources[0] = append(graph.vertexSources[0], WrappedOpenVertexSource{Face: 1, Vertex: 0})
	graph.vertexSources[1] = append(graph.vertexSources[1], WrappedOpenVertexSource{Face: 1, Vertex: 1})
	for owner := 0; owner < 2; owner++ {
		vertices := [3]int{0, 1, 2 + owner}
		var indices [3]int
		for i := 0; i < 3; i++ {
			if owner == 1 && i == 0 {
				indices[i] = 0
				graph.edgeSources[0] = append(graph.edgeSources[0], WrappedOpenEdgeSource{Face: 1, Edge: 0, Loop: 0, LocalEdge: 0})
				continue
			}
			indices[i] = len(graph.edges)
			a, b := uv[owner][i], uv[owner][(i+1)%3]
			graph.edges = append(graph.edges, parametric.Edge{Start: vertices[i], End: vertices[(i+1)%3], Curve: surfaceTrimCurve{surface: surfaces[owner], a: a, b: b}, Range: parametric.Interval{Max: 1}})
			graph.edgeSources = append(graph.edgeSources, []WrappedOpenEdgeSource{{Face: owner, Edge: i, Loop: 0, LocalEdge: i}})
		}
		face := Face{Surface: surfaces[owner]}
		order := []int{0, 1, 2}
		if !positive[owner] {
			order = []int{2, 1, 0}
		}
		for _, i := range order {
			a, b := uv[owner][i], uv[owner][(i+1)%3]
			if !positive[owner] {
				a, b = b, a
			}
			face.Outer = append(face.Outer, Use{Edge: indices[i], Reversed: !positive[owner], UVStart: a, UVEnd: b})
		}
		graph.faces = append(graph.faces, face)
	}
	for _, face := range graph.faces {
		for _, use := range face.Outer {
			if o.Proof.MaxWork > o.MaxWork-out.operations {
				return nil, geom.ErrConvergence
			}
			same, e := compareTriangleNativeUse(ctx, graph.edges[use.Edge], face.Surface, use, o.Proof)
			if e != nil {
				return nil, e
			}
			if !same {
				return nil, geom.ErrConvergence
			}
			out.operations += o.Proof.MaxWork
		}
	}
	boundary, e := validateWrappedAlgebraicPairIncidence(ctx, 4, graph.edges, graph.faces, 0)
	if e != nil {
		return nil, e
	}
	graph.boundary = boundary
	if e = ctx.Err(); e != nil {
		return nil, e
	}
	return out, nil
}

// Each stage consumes the remaining per-use allowance rather than resetting it.
func compareTriangleNativeUse(ctx context.Context, edge parametric.Edge, surface geom.Surface, use Use, o edgecheck.Options) (bool, error) {
	model, ok := edge.Curve.(surfaceTrimCurve)
	if !ok || model.curve != nil || use.Curve != nil || edge.Range.Min != 0 || edge.Range.Max != 1 {
		return false, geom.ErrInvalid
	}
	a, ok := model.surface.(*geom.ExactBezierSurface)
	if !ok {
		return false, geom.ErrInvalid
	}
	b, ok := surface.(*geom.ExactBezierSurface)
	if !ok {
		return false, geom.ErrInvalid
	}
	rat := func(v geom.Vec2) [2]*big.Rat {
		return [2]*big.Rat{new(big.Rat).SetFloat64(v.X), new(big.Rat).SetFloat64(v.Y)}
	}
	remaining := o.MaxWork
	opts := geom.SurfaceAffineCurveOptions{MaxPieces: 1, TubeAffineLineOptions: geom.TubeAffineLineOptions{MaxOperations: remaining, MaxCoefficientBits: o.MaxBits}}
	x, e := a.ExactPatch().AffineParameterCurvePieces(ctx, rat(model.a), rat(model.b), opts)
	if e != nil {
		return false, e
	}
	remaining -= x.ExactOperations
	if remaining < 1 {
		return false, geom.ErrConvergence
	}
	opts.MaxOperations = remaining
	y, e := b.ExactPatch().AffineParameterCurvePieces(ctx, rat(use.UVStart), rat(use.UVEnd), opts)
	if e != nil {
		return false, e
	}
	remaining -= y.ExactOperations
	if remaining < 1 {
		return false, geom.ErrConvergence
	}
	r, e := geom.CompareAffineParameterCurvePieces(ctx, x, y, geom.ExactCurveComparisonOptions{ReverseB: use.Reversed, MaxOperations: remaining, MaxCoefficientBits: o.MaxBits})
	return r.Complete && r.Equal, e
}
