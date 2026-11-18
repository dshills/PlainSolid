//go:build research

package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/brep"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// RationalCutFaceMeshOptions retains cumulative mesh and boundary limits.
// Partition bounds knot discovery independently for each source trim.
type RationalCutFaceMeshOptions struct {
	Face         MultiSpanCurvedTrimMeshOptions
	Partition    MultiSpanTrimSeamOptions
	MaxInputBits int
}

// CertifiedRationalCutFaceMesh stores exact source subtrim ancestry separately
// from the legacy Boundary proposal. Trims follows the original CCW outer loop;
// no binary64 source interval is fabricated in Boundary.Region.Loops.
// Source area fields are conservative [0, chart rectangle area] enclosures.
type CertifiedRationalCutFaceMesh struct {
	nativeTrims []CertifiedExactQuadraticTrimPolyline
	CertifiedCurvedQuadraticTrimMesh
	Trims []CertifiedRationalCutTrimPolyline
}

// TessellateRationalCutFaceCertified constructs a certified curved outer fill
// from fixed rational cuts on each original quadratic/linear UV trim. A nil
// cuts slice requests only automatic cuts. Holes are outside this initial API.
// Exact independent source/polygon topology proofs and boundary homotopy bound
// the UV fill difference; the global chart Lipschitz bound transports it.
// Exact rational triangle residuals include model rounding. No embedding or
// cross-face conformance is implied. All errors return no partial result.
func TessellateRationalCutFaceCertified(ctx context.Context, p *geom.SurfacePatch, outer []SplineUV, cuts [][]*big.Rat, o RationalCutFaceMeshOptions) (CertifiedRationalCutFaceMesh, error) {
	return tessellateRationalCutFace(ctx, p, outer, cuts, o, 1)
}
func tessellateRationalCutFace(ctx context.Context, p *geom.SurfacePatch, outer []SplineUV, cuts [][]*big.Rat, o RationalCutFaceMeshOptions, minimum int) (CertifiedRationalCutFaceMesh, error) {
	return buildRationalCutFace(ctx, p, outer, cuts, o, minimum, false)
}

func buildRationalCutFace(ctx context.Context, p *geom.SurfacePatch, outer []SplineUV, cuts [][]*big.Rat, o RationalCutFaceMeshOptions, minimum int, boundaryOnly bool) (CertifiedRationalCutFaceMesh, error) {
	return buildControlledCutFace(ctx, p, outer, nil, cuts, o, minimum, boundaryOnly, geom.SurfaceCurveCompositionOptions{}, false)
}

func buildControlledCutFace(ctx context.Context, p *geom.SurfacePatch, outer []SplineUV, native []ExactQuadraticUV, cuts [][]*big.Rat, o RationalCutFaceMeshOptions, minimum int, boundaryOnly bool, composition geom.SurfaceCurveCompositionOptions, fixedBoundary bool) (CertifiedRationalCutFaceMesh, error) {
	empty := CertifiedRationalCutFaceMesh{}
	if ctx == nil || p == nil || len(outer) < 2 || len(outer) > brep.MaxHoledFaceVertices || (cuts != nil && len(cuts) != len(outer)) {
		return empty, geom.ErrInvalid
	}
	for _, list := range cuts {
		if len(list) > 4096 {
			return empty, geom.ErrInvalid
		}
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	options := o.Face
	mo := &options.Mesh
	if !finite(mo.Deflection) || mo.Deflection <= 0 {
		return empty, geom.ErrInvalid
	}
	if mo.MaxSubdivisions == 0 {
		mo.MaxSubdivisions = 32
	}
	if mo.MaxTriangleChecks == 0 {
		mo.MaxTriangleChecks = 100000
	}
	if mo.MaxTriangles == 0 {
		mo.MaxTriangles = 100000
	}
	if mo.MaxSubdivisions < 1 || mo.MaxSubdivisions > 128 || mo.MaxSubdivisions&(mo.MaxSubdivisions-1) != 0 || mo.MaxTriangleChecks < 1 || mo.MaxTriangles < 1 || mo.MaxTriangles > 1000000 {
		return empty, geom.ErrInvalid
	}
	if options.MaxSpanCells == 0 {
		options.MaxSpanCells = 100000
	}
	if options.MaxSpanPieces == 0 {
		options.MaxSpanPieces = 100000
	}
	if options.Span.MaxCells == 0 {
		options.Span.MaxCells = 1024
	}
	if options.Span.MaxPieces == 0 {
		options.Span.MaxPieces = 8192
	}
	if options.Span.MaxInputBits == 0 {
		options.Span.MaxInputBits = 16384
	}
	if options.MaxSpanCells < 1 || options.MaxSpanCells > 1000000 || options.MaxSpanPieces < 1 || options.MaxSpanPieces > 1000000 || options.Span.MaxCells < 1 || options.Span.MaxCells > 65536 || options.Span.MaxPieces < 1 || options.Span.MaxPieces > 1000000 || options.Span.MaxInputBits < 1 || options.Span.MaxInputBits > 65536 {
		return empty, geom.ErrInvalid
	}
	sourceOptions, e := mo.Boundary.Source.WithDefaults()
	if e != nil {
		return empty, e
	}
	polygonOptions, e := mo.Boundary.Polygon.WithDefaults()
	if e != nil {
		return empty, e
	}
	if len(outer) > sourceOptions.MaxEdges {
		return empty, geom.ErrConvergence
	}
	source := make(trimvalidate.Loop, len(outer))
	for i, t := range outer {
		if native == nil {
			source[i], e = quadraticTrimSourceSegment(t)
		} else {
			source[i], e = trimvalidate.ExactQuadratic(native[i].HomogeneousControls())
		}
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
	work := multiSpanMeshWork{options: options}
	l2, l1, e := work.lipschitz(ctx, p)
	if e != nil {
		return empty, e
	}
	if l1.Sign() == 0 {
		return empty, geom.ErrConvergence
	}
	uvTolerance := new(big.Rat).Quo(ratInset(mo.Deflection), new(big.Rat).Mul(big.NewRat(4, 1), l1))
	tolerance, _ := uvTolerance.Float64()
	if ratInset(tolerance) != nil && ratInset(tolerance).Cmp(uvTolerance) > 0 {
		tolerance = math.Nextafter(tolerance, 0)
	}
	if !finite(tolerance) || tolerance <= 0 {
		return empty, geom.ErrConvergence
	}
	d := p.Domain()
	identity, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{X: d.UMin, Y: d.VMin}, {X: d.UMin, Y: d.VMax}, {X: d.UMax, Y: d.VMin}, {X: d.UMax, Y: d.VMax}}, []float64{d.UMin, d.UMin, d.UMax, d.UMax}, []float64{d.VMin, d.VMin, d.VMax, d.VMax}, nil)
	if e != nil {
		return empty, e
	}
	uvPatch, e := geom.NewSurfacePatch(identity, geom.SurfacePatchOptions{})
	if e != nil {
		return empty, e
	}
	to := mo.Boundary.Tessellation
	to.Deflection = tolerance
	if to.MaxSegments == 0 {
		to.MaxSegments = 4096
	}
	if to.MaxChecks == 0 {
		to.MaxChecks = 2*to.MaxSegments - 1
	}
	segments, checks := 0, 0
	result := CertifiedRationalCutFaceMesh{}
	loop := CertifiedQuadraticTrimLoop{SourceProof: proof}
	fill := new(big.Rat)
	equal := func(a, b [2]*big.Rat) bool { return a[0].Cmp(b[0]) == 0 && a[1].Cmp(b[1]) == 0 }
	var first, last [2]*big.Rat
	for i, t := range outer {
		var mandatory []*big.Rat
		if cuts != nil {
			mandatory = append(mandatory, cuts[i]...)
		}
		// Discover support crossings, then request that same exact partition on
		// the identity UV chart for the fill homotopy certificate.
		if native == nil {
			partition, err := CertifyMultiSpanTrimSeam(ctx, p, p, t, t, o.Partition)
			if err != nil {
				return empty, err
			}
			for _, piece := range partition.Pieces {
				mandatory = append(mandatory, piece.Min, piece.Max)
			}
		} else {
			h := native[i].HomogeneousControls()
			controls := make([][4]*big.Rat, 3)
			for j, control := range h {
				controls[j] = [4]*big.Rat{control[0], control[1], new(big.Rat), control[2]}
			}
			curve, err := geom.NewExactHomogeneousBezier(ctx, controls, composition.SurfacePatchOptions)
			if err != nil {
				return empty, err
			}
			partition, err := p.ComposeParameterCurvePieces(ctx, curve, composition)
			if err != nil {
				return empty, err
			}
			if !partition.Complete || !partition.Continuous {
				return empty, geom.ErrConvergence
			}
			for _, piece := range partition.Pieces {
				mandatory = append(mandatory, piece.Start, piece.End)
			}
		}
		trimOptions := to
		trimOptions.MaxSegments -= segments
		trimOptions.MaxChecks -= checks
		if trimOptions.MaxSegments < 1 || trimOptions.MaxChecks < 1 {
			return empty, geom.ErrConvergence
		}
		var line CertifiedRationalCutTrimPolyline
		if native == nil {
			line, e = TessellateRationalCutSplineTrimCertified(ctx, uvPatch, t, mandatory, RationalCutTrimOptions{Tessellation: trimOptions, Partition: o.Partition, MaxInputBits: o.MaxInputBits})
		} else {
			var exact CertifiedExactQuadraticTrimPolyline
			exact, e = TessellateExactQuadraticTrimCertified(ctx, uvPatch, native[i], mandatory, ExactQuadraticTrimMeshOptions{Tessellation: trimOptions, Composition: composition, MaxInputBits: o.MaxInputBits})
			line = exact.CertifiedRationalCutTrimPolyline
			result.nativeTrims = append(result.nativeTrims, exact)
		}
		if e != nil {
			return empty, e
		}
		if i == 0 {
			first = line.ExactUV[0]
		} else if !equal(last, line.ExactUV[0]) {
			return empty, geom.ErrInvalid
		}
		last = line.ExactUV[len(line.ExactUV)-1]
		segments += len(line.SquaredErrors)
		checks += line.Checks
		if segments > polygonOptions.MaxEdges || segments > brep.MaxHoledFaceVertices {
			return empty, geom.ErrConvergence
		}
		for j, bound := range line.SquaredErrors {
			rounding := new(big.Rat)
			for _, id := range []int{j, j + 1} {
				delta := new(big.Rat)
				point := exactBoundPoint(line.Points[id])
				for k := range point {
					v := new(big.Rat).Sub(point[k], line.ExactPoints[id][k])
					delta.Add(delta, new(big.Rat).Mul(v, v))
				}
				if delta.Cmp(rounding) > 0 {
					rounding = delta
				}
			}
			exactBound := new(big.Rat).Set(bound)
			if rounding.Sign() > 0 {
				exactBound.Mul(big.NewRat(2, 1), new(big.Rat).Add(exactBound, rounding))
			}
			if exactBound.Cmp(fill) > 0 {
				fill = exactBound
			}
			loop.Points = append(loop.Points, line.Points[j])
			loop.ExactPoints = append(loop.ExactPoints, line.ExactPoints[j])
			loop.ExactUV = append(loop.ExactUV, line.ExactUV[j])
			loop.SquaredErrors = append(loop.SquaredErrors, bound)
		}
		result.Trims = append(result.Trims, line)
	}
	if !equal(last, first) {
		return empty, geom.ErrInvalid
	}
	polygon := make(trimvalidate.Loop, len(loop.ExactUV))
	area := new(big.Rat)
	for i, a := range loop.ExactUV {
		b := loop.ExactUV[(i+1)%len(loop.ExactUV)]
		var h [3][3]*big.Rat
		for k := 0; k < 2; k++ {
			h[0][k] = a[k]
			h[2][k] = b[k]
			h[1][k] = new(big.Rat).Quo(new(big.Rat).Add(a[k], b[k]), big.NewRat(2, 1))
		}
		for j := 0; j < 3; j++ {
			h[j][2] = big.NewRat(1, 1)
		}
		polygon[i], e = trimvalidate.ExactQuadratic(h)
		if e != nil {
			return empty, e
		}
		area.Add(area, new(big.Rat).Sub(new(big.Rat).Mul(a[0], b[1]), new(big.Rat).Mul(a[1], b[0])))
	}
	area.Quo(area, big.NewRat(2, 1))
	polygonProof, e := trimvalidate.ValidateLoops(ctx, polygon, nil, polygonOptions)
	if e != nil {
		return empty, e
	}
	if polygonProof.Status != trimvalidate.Certified {
		return empty, geom.ErrConvergence
	}
	loop.PolygonProof = polygonProof
	loop.Checks = checks
	rectangleArea := new(big.Rat).Mul(new(big.Rat).Sub(ratInset(d.UMax), ratInset(d.UMin)), new(big.Rat).Sub(ratInset(d.VMax), ratInset(d.VMin)))
	interval := RationalAreaInterval{Lower: new(big.Rat), Upper: rectangleArea}
	vectors := [3]RationalAreaInterval{{Lower: new(big.Rat), Upper: new(big.Rat)}, {Lower: new(big.Rat), Upper: new(big.Rat)}, interval}
	mapping := make([]int, len(outer))
	for i := range mapping {
		mapping[i] = i
	}
	region := AffineQuadraticTrimRegionArea{FillSquaredDistance: fill, Loops: []AffineQuadraticTrimLoopArea{{Boundary: loop, SourceUVArea: interval, PolygonUVArea: area, ModelAreaVector: vectors}}, OriginalSource: [][]int{mapping}, SourceUVArea: interval, PolygonUVArea: area, ModelAreaVector: vectors, SourceProof: proof, PolygonProof: polygonProof}
	if boundaryOnly {
		result.Boundary.Region = region
		result.LipschitzSquared = l2
		result.SpanCellChecks, result.SpanPieceChecks = work.cells, work.pieces
		return result, nil
	}
	boundary, e := triangulateCertifiedAffineTrimRegion(ctx, region, 0, mo.MaxTriangles)
	if e != nil {
		return empty, e
	}
	result.CertifiedCurvedQuadraticTrimMesh, e = tessellateCertifiedTrimBoundaryMode(ctx, p, boundary, *mo, minimum, &work, l2, fixedBoundary)
	if e != nil {
		return empty, e
	}
	return result, nil
}
