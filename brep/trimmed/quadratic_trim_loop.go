package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// QuadraticTrimLoopOptions applies Tessellation's segment/check limits to the
// entire loop. Source and Polygon independently bound exact UV topology proofs.
type QuadraticTrimLoopOptions struct {
	Tessellation    QuadraticTrimTessellationOptions
	Source, Polygon trimvalidate.Options
}

// QuadraticTrimLoopSegment identifies one directed original-parameter interval
// and its shared cyclic vertex indices. Source indexes the caller's trim slice.
type QuadraticTrimLoopSegment struct {
	Source     int
	Start, End float64
	Vertices   [2]int
}

// CertifiedQuadraticTrimLoop has exact cyclic joins, a certified simple CCW
// source UV loop, and an independently certified simple CCW exact UV chord
// polygon. Each model chord has a two-sided correspondence bound. These facts
// do NOT assert equality of the enclosed regions, a fill homotopy, model-space
// embeddedness, or an interior face tessellation certificate.
type CertifiedQuadraticTrimLoop struct {
	Points                    []geom.Vec3
	ExactPoints               [][3]*big.Rat
	ExactUV                   [][2]*big.Rat
	Segments                  []QuadraticTrimLoopSegment
	SquaredErrors             []*big.Rat
	SourceProof, PolygonProof trimvalidate.Report
	Checks                    int
}

// TessellateQuadraticSplineTrimLoopCertified builds one closed CCW UV loop.
// Linear and rational quadratic source spans may have nonunit, restricted, or
// reversed ranges. All endpoint joins are exact; no tolerance welding occurs.
// Points do not repeat the closing vertex. Failure returns no partial loop.
func TessellateQuadraticSplineTrimLoopCertified(ctx context.Context, patch *geom.SurfacePatch, trims []SplineUV, o QuadraticTrimLoopOptions) (CertifiedQuadraticTrimLoop, error) {
	empty := CertifiedQuadraticTrimLoop{}
	if ctx == nil || patch == nil || len(trims) < 2 {
		return empty, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return empty, err
	}
	if o.Tessellation.MaxSegments == 0 {
		o.Tessellation.MaxSegments = 4096
	}
	if o.Tessellation.MaxChecks == 0 {
		o.Tessellation.MaxChecks = 2*o.Tessellation.MaxSegments - 1
	}
	if o.Tessellation.MaxSegments < 1 || o.Tessellation.MaxSegments > 65536 || o.Tessellation.MaxChecks < 1 || len(trims) > o.Tessellation.MaxSegments {
		return empty, geom.ErrInvalid
	}
	sourceOptions, e := o.Source.WithDefaults()
	if e != nil {
		return empty, e
	}
	if len(trims) > sourceOptions.MaxEdges {
		return empty, geom.ErrConvergence
	}
	source := make(trimvalidate.Loop, len(trims))
	for i, t := range trims {
		source[i], e = quadraticTrimSourceSegment(t)
		if e != nil {
			return empty, e
		}
	}
	proof, e := trimvalidate.ValidateLoops(ctx, source, nil, sourceOptions)
	if e != nil {
		return empty, e
	}
	if proof.Status != trimvalidate.Certified {
		if proof.Status == trimvalidate.Invalid {
			return empty, geom.ErrInvalid
		}
		return empty, geom.ErrConvergence
	}
	result := CertifiedQuadraticTrimLoop{SourceProof: proof}
	var lastUV [2]*big.Rat
	var lastPoint [3]*big.Rat
	equal := func(a, b []*big.Rat) bool {
		for i := range a {
			if a[i] == nil || b[i] == nil || a[i].Cmp(b[i]) != 0 {
				return false
			}
		}
		return true
	}
	for i, t := range trims {
		options := o.Tessellation
		options.MaxSegments -= len(result.Segments)
		options.MaxChecks -= result.Checks
		if options.MaxSegments < 1 || options.MaxChecks < 1 {
			return empty, geom.ErrConvergence
		}
		piece, e := TessellateQuadraticSplineTrimCertified(ctx, patch, t, options)
		if e != nil {
			return empty, e
		}
		if i > 0 && (!equal(lastUV[:], piece.ExactUV[0][:]) || !equal(lastPoint[:], piece.ExactPoints[0][:])) {
			return empty, geom.ErrInvalid
		}
		for j, b := range piece.SquaredErrors {
			id := len(result.Points)
			result.Points = append(result.Points, piece.Points[j])
			result.ExactPoints = append(result.ExactPoints, piece.ExactPoints[j])
			result.ExactUV = append(result.ExactUV, piece.ExactUV[j])
			result.SquaredErrors = append(result.SquaredErrors, b)
			result.Segments = append(result.Segments, QuadraticTrimLoopSegment{Source: i, Start: piece.Parameters[j], End: piece.Parameters[j+1], Vertices: [2]int{id, id + 1}})
		}
		result.Checks += piece.Checks
		lastUV = piece.ExactUV[len(piece.ExactUV)-1]
		lastPoint = piece.ExactPoints[len(piece.ExactPoints)-1]
	}
	if !equal(lastUV[:], result.ExactUV[0][:]) || !equal(lastPoint[:], result.ExactPoints[0][:]) {
		return empty, geom.ErrInvalid
	}
	result.Segments[len(result.Segments)-1].Vertices[1] = 0
	polygonOptions, e := o.Polygon.WithDefaults()
	if e != nil {
		return empty, e
	}
	if len(result.Segments) > polygonOptions.MaxEdges {
		return empty, geom.ErrConvergence
	}
	polygon := make(trimvalidate.Loop, len(result.Segments))
	for i, p := range result.ExactUV {
		q := result.ExactUV[(i+1)%len(result.ExactUV)]
		var h [3][3]*big.Rat
		for k := 0; k < 2; k++ {
			h[0][k] = p[k]
			h[2][k] = q[k]
			h[1][k] = new(big.Rat).Quo(new(big.Rat).Add(p[k], q[k]), big.NewRat(2, 1))
		}
		for j := 0; j < 3; j++ {
			h[j][2] = big.NewRat(1, 1)
		}
		polygon[i], e = trimvalidate.ExactQuadratic(h)
		if e != nil {
			return empty, e
		}
	}
	result.PolygonProof, e = trimvalidate.ValidateLoops(ctx, polygon, nil, polygonOptions)
	if e != nil {
		return empty, e
	}
	if result.PolygonProof.Status != trimvalidate.Certified {
		return empty, geom.ErrConvergence
	}
	return result, nil
}

// Exact quadratic blossom restriction, including degree-elevated lines.
func quadraticTrimSourceSegment(t SplineUV) (trimvalidate.Segment, error) {
	h, e := quadraticTrimSourceControls(t)
	if e != nil {
		return trimvalidate.Segment{}, e
	}
	return trimvalidate.ExactQuadratic(h)
}

func quadraticTrimSourceControls(t SplineUV) ([3][3]*big.Rat, error) {
	empty := [3][3]*big.Rat{}
	if t.Curve == nil || !finite(t.Range.Min) || !finite(t.Range.Max) || t.Range.Min >= t.Range.Max {
		return empty, geom.ErrInvalid
	}
	degree := t.Curve.Degree()
	p, k, w := t.Curve.Poles(), t.Curve.Knots(), t.Curve.Weights()
	if degree < 1 || degree > 2 || len(p) != degree+1 || len(k) != 2*(degree+1) {
		return empty, geom.ErrConvergence
	}
	lo, hi := k[0], k[len(k)-1]
	for i := 0; i <= degree; i++ {
		if k[i] != lo || k[i+degree+1] != hi {
			return empty, geom.ErrConvergence
		}
	}
	if t.Range.Min < lo || t.Range.Max > hi {
		return empty, geom.ErrDomain
	}
	var h [3][3]*big.Rat
	for i, point := range p {
		if point.Z != 0 {
			return empty, geom.ErrDomain
		}
		weight := big.NewRat(1, 1)
		if len(w) > 0 {
			weight = ratInset(w[i])
		}
		h[i] = [3]*big.Rat{new(big.Rat).Mul(ratInset(point.X), weight), new(big.Rat).Mul(ratInset(point.Y), weight), weight}
	}
	if degree == 1 {
		h[2] = h[1]
		for j := 0; j < 3; j++ {
			h[1][j] = new(big.Rat).Quo(new(big.Rat).Add(h[0][j], h[2][j]), big.NewRat(2, 1))
		}
	}
	width := new(big.Rat).Sub(ratInset(hi), ratInset(lo))
	a := new(big.Rat).Quo(new(big.Rat).Sub(ratInset(t.Range.Min), ratInset(lo)), width)
	b := new(big.Rat).Quo(new(big.Rat).Sub(ratInset(t.Range.Max), ratInset(lo)), width)
	if t.Reversed {
		a, b = b, a
	}
	blossom := func(a, b *big.Rat) [3]*big.Rat {
		oneA := new(big.Rat).Sub(big.NewRat(1, 1), a)
		oneB := new(big.Rat).Sub(big.NewRat(1, 1), b)
		weights := [3]*big.Rat{new(big.Rat).Mul(oneA, oneB), new(big.Rat).Add(new(big.Rat).Mul(a, oneB), new(big.Rat).Mul(b, oneA)), new(big.Rat).Mul(a, b)}
		var v [3]*big.Rat
		for j := range v {
			v[j] = new(big.Rat)
			for i := range h {
				v[j].Add(v[j], new(big.Rat).Mul(weights[i], h[i][j]))
			}
		}
		return v
	}
	return [3][3]*big.Rat{blossom(a, a), blossom(a, b), blossom(b, b)}, nil
}
