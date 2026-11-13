package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// CertifiedExactBezierGraphChain retains every piece proof and maps its curve
// samples into one global face mesh. JunctionVertices has len(Pieces)+1 entries;
// neighboring pieces share the same exact UV/model point and display index.
// The embedded Source/Curve fields are empty: Pieces are the source authority.
type CertifiedExactBezierGraphChain struct {
	// ClosingVertices gives the directed baseline, right, and left closing
	// chains, including both endpoints; curve/junction indices stay explicit.
	ClosingVertices [3][]int
	CertifiedExactBezierGraphFace
	Pieces           []CertifiedExactBezierGraphFace
	PieceVertices    [][]int
	JunctionVertices []int
}

// TessellateExactBezierGraphChainCertified accepts 1..64 increasing graph pieces
// on the same exact affine support and baseline. Exact adjacent endpoints must
// match in both coordinates. Gaps, overlap, reversed pieces and touching baseline
// reject atomically. C0 joins need not have matching tangent directions.
// Display MaxSegments and MaxOperations and composition MaxOperations are total
// budgets divided equally (integer quotient) among pieces before work. Defaults
// are4096,8M,1M respectively. Chart MaxOperations defaults8M and is divided among
// len(pieces)+1 chart proofs; exact point evaluation retains its per-point limit.
// Closing subdivisions add at most381 exact evaluations. Each lies on the same
// original UV segment, so the source region and its homotopy bound are unchanged.
// MaxTriangles limits each proposal and the final global mesh. Endpoint/Junction
// display overrides are not accepted: exact junction points are rounded once.
func TessellateExactBezierGraphChainCertified(ctx context.Context, p *geom.SurfacePatch, pieces []ExactBezierUV, baseline *big.Rat, o ExactBezierGraphFaceOptions) (CertifiedExactBezierGraphChain, error) {
	return tessellateExactBezierGraphChainCuts(ctx, p, pieces, baseline, nil, o)
}

func tessellateExactBezierGraphChainCuts(ctx context.Context, p *geom.SurfacePatch, pieces []ExactBezierUV, baseline *big.Rat, cuts [][]*big.Rat, o ExactBezierGraphFaceOptions) (CertifiedExactBezierGraphChain, error) {
	var empty CertifiedExactBezierGraphChain
	n := len(pieces)
	if ctx == nil || p == nil || baseline == nil || n < 1 || n > 64 || o.Display.EndpointPoints != nil || o.Display.JunctionPoints != nil {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	divisions := o.ClosingSubdivisions
	if divisions == 0 {
		divisions = 1
	}
	if divisions < 1 || divisions > 128 || divisions&(divisions-1) != 0 {
		return empty, geom.ErrInvalid
	}
	if o.Display.MaxSegments == 0 {
		o.Display.MaxSegments = 4096
	}
	if o.Display.MaxOperations == 0 {
		o.Display.MaxOperations = 8000000
	}
	if o.Composition.MaxOperations == 0 {
		o.Composition.MaxOperations = 1000000
	}
	if o.Chart.MaxOperations == 0 {
		o.Chart.MaxOperations = 8000000
	}
	if o.MaxTriangles == 0 {
		o.MaxTriangles = 100000
	}
	if o.Display.MaxSegments < n || o.Display.MaxOperations < n || o.Composition.MaxOperations < n || o.Chart.MaxOperations < n+1 {
		return empty, geom.ErrConvergence
	}
	if o.MaxTriangles < 1 || o.MaxTriangles > 1000000 {
		return empty, geom.ErrInvalid
	}
	if o.MaxTriangles < 3*divisions+n-2 {
		return empty, geom.ErrConvergence
	}
	local := o
	local.Display.MaxSegments /= n
	local.Display.MaxOperations /= n
	local.Composition.MaxOperations /= n
	local.Chart.MaxOperations /= n + 1
	chart, e := geom.NewAffineSurfaceChart(ctx, p, local.Chart)
	if e != nil {
		return empty, e
	}
	endpoints := make([][2]*big.Rat, n+1)
	display := make([]geom.Vec3, n+1)
	for i, q := range pieces {
		a, _, e := q.EvaluateRational(new(big.Rat))
		if e != nil {
			return empty, e
		}
		b, _, e := q.EvaluateRational(big.NewRat(1, 1))
		if e != nil {
			return empty, e
		}
		if a[0].Cmp(b[0]) >= 0 {
			return empty, geom.ErrInvalid
		}
		if i > 0 && (a[0].Cmp(endpoints[i][0]) != 0 || a[1].Cmp(endpoints[i][1]) != 0) {
			return empty, geom.ErrInvalid
		}
		endpoints[i], endpoints[i+1] = a, b
	}
	for i, uv := range endpoints {
		point, e := chart.EvalExact(uv[0], uv[1])
		if e != nil {
			return empty, e
		}
		display[i].X, _ = point[0].Float64()
		display[i].Y, _ = point[1].Float64()
		display[i].Z, _ = point[2].Float64()
		if !display[i].IsFinite() {
			return empty, geom.ErrConvergence
		}
	}
	r := CertifiedExactBezierGraphChain{PieceVertices: make([][]int, n), JunctionVertices: make([]int, n+1)}
	for i, q := range pieces {
		local.Display.EndpointPoints = &[2]geom.Vec3{display[i], display[i+1]}
		var fixed []*big.Rat
		if cuts != nil {
			fixed = cuts[i]
		}
		piece, e := tessellateExactBezierGraphFaceCuts(ctx, p, q, baseline, fixed, local)
		if e != nil {
			return empty, e
		}
		r.Pieces = append(r.Pieces, piece)
		r.PieceVertices[i] = make([]int, len(piece.CurveVertices))
	}
	boundary := CertifiedQuadraticTrimLoop{}
	rounding := new(big.Rat)
	maximumError := 0.
	add := func(uv [2]*big.Rat, point [3]*big.Rat, display geom.Vec3) int {
		index := len(boundary.Points)
		boundary.Points = append(boundary.Points, display)
		boundary.ExactUV = append(boundary.ExactUV, uv)
		boundary.ExactPoints = append(boundary.ExactPoints, point)
		sq := new(big.Rat)
		for i, f := range []float64{display.X, display.Y, display.Z} {
			z := new(big.Rat).Sub(point[i], ratInset(f))
			sq.Add(sq, new(big.Rat).Mul(z, z))
		}
		if sq.Cmp(rounding) > 0 {
			rounding.Set(sq)
		}
		return index
	}
	interior := func(a, b [2]*big.Rat) ([]int, error) {
		var indices []int
		for j := 1; j < divisions; j++ {
			if e := ctx.Err(); e != nil {
				return nil, e
			}
			uv := [2]*big.Rat{}
			for k := range uv {
				uv[k] = new(big.Rat).Add(a[k], new(big.Rat).Mul(new(big.Rat).Sub(b[k], a[k]), big.NewRat(int64(j), int64(divisions))))
			}
			point, e := chart.EvalExact(uv[0], uv[1])
			if e != nil {
				return nil, e
			}
			v := geom.Vec3{}
			for k, f := range []*float64{&v.X, &v.Y, &v.Z} {
				*f, _ = point[k].Float64()
			}
			if !v.IsFinite() {
				return nil, geom.ErrConvergence
			}
			indices = append(indices, add(uv, point, v))
		}
		return indices, nil
	}
	first, last := r.Pieces[0], r.Pieces[n-1]
	leftBottom := add(first.ExactUV[0], first.ExactPoints[0], first.Mesh.Vertices[0])
	baseIndices, e := interior(first.ExactUV[0], last.ExactUV[1])
	if e != nil {
		return empty, e
	}
	rightBottom := add(last.ExactUV[1], last.ExactPoints[1], last.Mesh.Vertices[1])
	r.ClosingVertices[0] = append(append([]int{leftBottom}, baseIndices...), rightBottom)
	rightTopUV := last.ExactUV[last.CurveVertices[len(last.CurveVertices)-1]]
	rightIndices, e := interior(last.ExactUV[1], rightTopUV)
	if e != nil {
		return empty, e
	}
	r.ClosingVertices[1] = append([]int{rightBottom}, rightIndices...)
	for i := n - 1; i >= 0; i-- {
		piece := r.Pieces[i]
		maximumError = max(maximumError, piece.Curve.MaximumError)
		for j := len(piece.CurveVertices) - 1; j >= 0; j-- {
			if i < n-1 && j == len(piece.CurveVertices)-1 {
				r.PieceVertices[i][j] = r.PieceVertices[i+1][0]
				continue
			}
			k := piece.CurveVertices[j]
			r.PieceVertices[i][j] = add(piece.ExactUV[k], piece.ExactPoints[k], piece.Mesh.Vertices[k])
		}
	}
	for i := range pieces {
		r.JunctionVertices[i] = r.PieceVertices[i][0]
	}
	r.JunctionVertices[n] = r.PieceVertices[n-1][len(r.PieceVertices[n-1])-1]
	r.ClosingVertices[1] = append(r.ClosingVertices[1], r.JunctionVertices[n])
	leftTop := r.JunctionVertices[0]
	leftIndices, e := interior(boundary.ExactUV[leftTop], first.ExactUV[0])
	if e != nil {
		return empty, e
	}
	r.ClosingVertices[2] = append(append([]int{leftTop}, leftIndices...), leftBottom)
	area := new(big.Rat)
	for i, a := range boundary.ExactUV {
		b := boundary.ExactUV[(i+1)%len(boundary.ExactUV)]
		area.Add(area, new(big.Rat).Sub(new(big.Rat).Mul(a[0], b[1]), new(big.Rat).Mul(a[1], b[0])))
	}
	area.Quo(area, big.NewRat(2, 1))
	errRat := ratInset(maximumError)
	bound := new(big.Rat).Mul(errRat, errRat)
	bound.Add(bound, rounding)
	bound.Mul(bound, big.NewRat(2, 1))
	region := AffineQuadraticTrimRegionArea{Loops: []AffineQuadraticTrimLoopArea{{Boundary: boundary}}, PolygonUVArea: area, FillSquaredDistance: bound}
	mesh, e := triangulateCertifiedAffineTrimRegion(ctx, region, 0, o.MaxTriangles)
	if e != nil {
		return empty, e
	}
	r.CertifiedExactBezierGraphFace = CertifiedExactBezierGraphFace{Baseline: new(big.Rat).Set(baseline), Mesh: mesh.Mesh, ExactUV: mesh.ExactUV, ExactPoints: mesh.ExactPoints, Boundary: mesh.BoundaryLoops[0], PolygonUVArea: area, FillSquaredDistance: mesh.FillSquaredDistance}
	return r, nil
}
