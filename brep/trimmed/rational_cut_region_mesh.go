//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// CertifiedRationalCutRegionMesh covers a CCW outer loop and CW holes.
// LoopTrims is outer then holes, each normalized CCW for its boundary proof.
// Boundary.Region.OriginalSource maps normalized trim indices to the original
// input; hole traversal is reversed. Exact source parameter values are retained.
// BoundaryLoops and the triangulated mesh retain outer CCW and hole CW winding.
type CertifiedRationalCutRegionMesh struct {
	nativeLoopTrims [][]CertifiedExactQuadraticTrimPolyline
	CertifiedCurvedQuadraticTrimMesh
	LoopTrims [][]CertifiedRationalCutTrimPolyline
}

// TessellateRationalCutRegionCertified extends exact rational-cut filling to
// separated curved holes. cuts, when nonnil, indexes outer then holes and their
// original directed trims; it need not contain 0 or1. Joint exact source and
// polygon validation proves orientation, separation and hole containment.
// The boundary homotopy theorem applies to the closed holed region; it does not
// assert that source holes and inscribed polygon holes have identical fills.
// Global boundary and span counters include every loop and final refinement.
// No partial mesh is returned on unresolved topology, knot cuts, or work limits.
func TessellateRationalCutRegionCertified(ctx context.Context, p *geom.SurfacePatch, outer []SplineUV, holes [][]SplineUV, cuts [][][]*big.Rat, o RationalCutFaceMeshOptions) (CertifiedRationalCutRegionMesh, error) {
	return tessellateControlledCutRegion(ctx, p, outer, holes, nil, cuts, o, geom.SurfaceCurveCompositionOptions{}, false)
}
func tessellateControlledCutRegion(ctx context.Context, p *geom.SurfacePatch, outer []SplineUV, holes [][]SplineUV, native [][]ExactQuadraticUV, cuts [][][]*big.Rat, o RationalCutFaceMeshOptions, composition geom.SurfaceCurveCompositionOptions, fixedBoundary bool) (CertifiedRationalCutRegionMesh, error) {
	empty := CertifiedRationalCutRegionMesh{}
	if ctx == nil || p == nil || len(holes) > 64 || (cuts != nil && len(cuts) != 1+len(holes)) {
		return empty, geom.ErrInvalid
	}
	if o.MaxInputBits == 0 {
		o.MaxInputBits = 16384
	}
	if o.MaxInputBits < 1 || o.MaxInputBits > 65536 {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	options := o.Face
	mo := &options.Mesh
	if mo.MaxSubdivisions == 0 {
		mo.MaxSubdivisions = 32
	}
	if mo.MaxTriangleChecks == 0 {
		mo.MaxTriangleChecks = 100000
	}
	if mo.MaxTriangles == 0 {
		mo.MaxTriangles = 100000
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
	if mo.Boundary.Tessellation.MaxSegments == 0 {
		mo.Boundary.Tessellation.MaxSegments = 4096
	}
	if mo.Boundary.Tessellation.MaxChecks == 0 {
		mo.Boundary.Tessellation.MaxChecks = 2*mo.Boundary.Tessellation.MaxSegments - 1
	}
	sourceOptions, e := mo.Boundary.Source.WithDefaults()
	if e != nil {
		return empty, e
	}
	polygonOptions, e := mo.Boundary.Polygon.WithDefaults()
	if e != nil {
		return empty, e
	}
	inputs := append([][]SplineUV{outer}, holes...)
	source := make([]trimvalidate.Loop, len(inputs))
	count := 0
	for li, loop := range inputs {
		if len(loop) < 2 || len(loop) > brep.MaxHoledFaceVertices-count || len(loop) > sourceOptions.MaxEdges-count {
			return empty, geom.ErrConvergence
		}
		count += len(loop)
		if cuts != nil && cuts[li] != nil && len(cuts[li]) != len(loop) {
			return empty, geom.ErrInvalid
		}
		source[li] = make(trimvalidate.Loop, len(loop))
		for j, t := range loop {
			if native == nil {
				source[li][j], e = quadraticTrimSourceSegment(t)
			} else {
				source[li][j], e = trimvalidate.ExactQuadratic(native[li][j].HomogeneousControls())
			}
			if e != nil {
				return empty, e
			}
		}
	}
	sourceProof, e := trimvalidate.ValidateLoops(ctx, source[0], source[1:], sourceOptions)
	if e != nil {
		return empty, e
	}
	if sourceProof.Status != trimvalidate.Certified {
		if sourceProof.Status == trimvalidate.Invalid {
			return empty, geom.ErrInvalid
		}
		return empty, geom.ErrConvergence
	}
	result := CertifiedRationalCutRegionMesh{}
	region := AffineQuadraticTrimRegionArea{SourceProof: sourceProof, FillSquaredDistance: new(big.Rat), PolygonUVArea: new(big.Rat)}
	cells, pieces, segments, checks := 0, 0, 0, 0
	polygons := make([]trimvalidate.Loop, len(inputs))
	var l2 *big.Rat
	for li, input := range inputs {
		normalized := make([]SplineUV, len(input))
		var nativeNormalized []ExactQuadraticUV
		if native != nil {
			nativeNormalized = make([]ExactQuadraticUV, len(input))
		}
		mapping := make([]int, len(input))
		mandatory := make([][]*big.Rat, len(input))
		for j := range input {
			index := j
			if li > 0 {
				index = len(input) - 1 - j
			}
			normalized[j] = input[index]
			mapping[j] = index
			if native != nil {
				nativeNormalized[j] = native[li][index]
				if li > 0 {
					nativeNormalized[j] = nativeNormalized[j].reversed()
				}
			}
			if li > 0 {
				normalized[j].Reversed = !normalized[j].Reversed
			}
			if cuts != nil && cuts[li] != nil {
				if len(cuts[li][index]) > 4096 {
					return empty, geom.ErrInvalid
				}
				for _, q := range cuts[li][index] {
					if q == nil || q.Num().BitLen() > o.MaxInputBits || q.Denom().BitLen() > o.MaxInputBits {
						return empty, geom.ErrInvalid
					}
					value := new(big.Rat).Set(q)
					if li > 0 {
						value.Sub(big.NewRat(1, 1), value)
					}
					mandatory[j] = append(mandatory[j], value)
				}
			}
		}
		perLoop := o
		perLoop.Face = options
		perLoop.Face.MaxSpanCells -= cells
		perLoop.Face.MaxSpanPieces -= pieces
		perLoop.Face.Mesh.Boundary.Tessellation.MaxSegments -= segments
		perLoop.Face.Mesh.Boundary.Tessellation.MaxChecks -= checks
		if perLoop.Face.MaxSpanCells < 1 || perLoop.Face.MaxSpanPieces < 1 || perLoop.Face.Mesh.Boundary.Tessellation.MaxSegments < 1 || perLoop.Face.Mesh.Boundary.Tessellation.MaxChecks < 1 {
			return empty, geom.ErrConvergence
		}
		built, e := buildControlledCutFace(ctx, p, normalized, nativeNormalized, mandatory, perLoop, 1, true, composition, false)
		if e != nil {
			return empty, e
		}
		cells += built.SpanCellChecks
		pieces += built.SpanPieceChecks
		for _, trim := range built.Trims {
			segments += len(trim.SquaredErrors)
			checks += trim.Checks
		}
		if segments > brep.MaxHoledFaceVertices || segments > polygonOptions.MaxEdges {
			return empty, geom.ErrConvergence
		}
		if l2 == nil {
			l2 = built.LipschitzSquared
		} else if l2.Cmp(built.LipschitzSquared) != 0 {
			return empty, geom.ErrConvergence
		}
		loop := built.Boundary.Region.Loops[0]
		region.Loops = append(region.Loops, loop)
		region.OriginalSource = append(region.OriginalSource, mapping)
		if built.Boundary.Region.FillSquaredDistance.Cmp(region.FillSquaredDistance) > 0 {
			region.FillSquaredDistance.Set(built.Boundary.Region.FillSquaredDistance)
		}
		if li == 0 {
			region.PolygonUVArea.Add(region.PolygonUVArea, loop.PolygonUVArea)
			region.SourceUVArea = built.Boundary.Region.SourceUVArea
			region.ModelAreaVector = built.Boundary.Region.ModelAreaVector
		} else {
			region.PolygonUVArea.Sub(region.PolygonUVArea, loop.PolygonUVArea)
		}
		result.LoopTrims = append(result.LoopTrims, built.Trims)
		if native != nil {
			result.nativeLoopTrims = append(result.nativeLoopTrims, built.nativeTrims)
		}
		uv := loop.Boundary.ExactUV
		polygons[li] = make(trimvalidate.Loop, len(uv))
		for j := range uv {
			a, b := uv[j], uv[(j+1)%len(uv)]
			if li > 0 {
				a, b = uv[len(uv)-1-j], uv[(len(uv)-2-j+len(uv))%len(uv)]
			}
			var h [3][3]*big.Rat
			for k := 0; k < 2; k++ {
				h[0][k] = a[k]
				h[2][k] = b[k]
				h[1][k] = new(big.Rat).Quo(new(big.Rat).Add(a[k], b[k]), big.NewRat(2, 1))
			}
			for k := 0; k < 3; k++ {
				h[k][2] = big.NewRat(1, 1)
			}
			polygons[li][j], e = trimvalidate.ExactQuadratic(h)
			if e != nil {
				return empty, e
			}
		}
	}
	region.PolygonProof, e = trimvalidate.ValidateLoops(ctx, polygons[0], polygons[1:], polygonOptions)
	if e != nil {
		return empty, e
	}
	if region.PolygonProof.Status != trimvalidate.Certified {
		return empty, geom.ErrConvergence
	}
	boundary, e := triangulateCertifiedAffineTrimRegion(ctx, region, len(holes), mo.MaxTriangles)
	if e != nil {
		return empty, e
	}
	work := multiSpanMeshWork{options: options, cells: cells, pieces: pieces}
	result.CertifiedCurvedQuadraticTrimMesh, e = tessellateCertifiedTrimBoundaryMode(ctx, p, boundary, *mo, 1, &work, l2, fixedBoundary)
	if e != nil {
		return empty, e
	}
	return result, nil
}
