package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// AffineQuadraticTrimRegionArea certifies source and polygon regions separately.
// Loops stores outer then holes, each normalized CCW for its individual area
// proof. For holes, OriginalSource maps each normalized segment Source back to
// its original trim index; swap Start/End to recover the original CW traversal.
// Loop index itself is unchanged. SourceUVArea and ModelAreaVector subtract
// holes. FillSquaredDistance bounds both directed distances between the closed
// source fill and the exact UV chord polygon fill mapped through the affine
// chart. It does not refer to a polygon formed from rounded display vertices.
// Equality of these fills is not claimed.
type AffineQuadraticTrimRegionArea struct {
	FillSquaredDistance       *big.Rat
	Loops                     []AffineQuadraticTrimLoopArea
	OriginalSource            [][]int
	SourceUVArea              RationalAreaInterval
	PolygonUVArea             *big.Rat
	ModelAreaVector           [3]RationalAreaInterval
	SourceProof, PolygonProof trimvalidate.Report
}

// BoundAffineQuadraticTrimRegionArea requires a CCW outer and CW holes with
// exact cyclic joins. Joint exact trim validation proves separation, orientation
// and containment of both original curved boundaries and chord polygons.
// Segment/check budgets apply globally; topology options bound each proof.
// Touching, overlapping, nested or exterior holes reject. Errors are atomic.
//
// Fill proof: the parameterwise straight boundary homotopy stays within epsilon
// of both corresponding endpoints. Away from that homotopy, winding numbers
// agree. A point in either closed fill but outside the other therefore lies on
// the homotopy (or on the first boundary), and is within epsilon of the other
// boundary, hence its closed fill. The exact topology proofs identify winding
// one with material and winding zero with exterior/holes. Boundary points obey
// the same distance bound directly. This proves both directed fill distances;
// the homotopy need not itself consist of embedded curves.
func BoundAffineQuadraticTrimRegionArea(ctx context.Context, patch *geom.SurfacePatch, outer []SplineUV, holes [][]SplineUV, o QuadraticTrimLoopOptions) (AffineQuadraticTrimRegionArea, error) {
	empty := AffineQuadraticTrimRegionArea{}
	if ctx == nil || patch == nil {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	sourceOptions, e := o.Source.WithDefaults()
	if e != nil {
		return empty, e
	}
	polygonOptions, e := o.Polygon.WithDefaults()
	if e != nil {
		return empty, e
	}
	if o.Tessellation.MaxSegments == 0 {
		o.Tessellation.MaxSegments = 4096
	}
	if o.Tessellation.MaxChecks == 0 {
		o.Tessellation.MaxChecks = 2*o.Tessellation.MaxSegments - 1
	}
	if o.Tessellation.MaxSegments < 1 || o.Tessellation.MaxSegments > 65536 || o.Tessellation.MaxChecks < 1 {
		return empty, geom.ErrInvalid
	}
	if len(holes) > sourceOptions.MaxEdges/2 {
		return empty, geom.ErrConvergence
	}
	count := 0
	inputs := append([][]SplineUV{outer}, holes...)
	source := make([]trimvalidate.Loop, len(inputs))
	for i, loop := range inputs {
		if len(loop) < 2 || len(loop) > sourceOptions.MaxEdges-count {
			return empty, geom.ErrConvergence
		}
		count += len(loop)
		source[i] = make(trimvalidate.Loop, len(loop))
		for j, t := range loop {
			source[i][j], e = quadraticTrimSourceSegment(t)
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
	result := AffineQuadraticTrimRegionArea{FillSquaredDistance: new(big.Rat), SourceProof: sourceProof, SourceUVArea: RationalAreaInterval{new(big.Rat), new(big.Rat)}, PolygonUVArea: new(big.Rat)}
	for i := range result.ModelAreaVector {
		result.ModelAreaVector[i] = RationalAreaInterval{new(big.Rat), new(big.Rat)}
	}
	segments, checks := 0, 0
	var polygons []trimvalidate.Loop
	for li, input := range inputs {
		normalized := make([]SplineUV, len(input))
		mapping := make([]int, len(input))
		for j := range input {
			index := j
			if li > 0 {
				index = len(input) - 1 - j
			}
			normalized[j] = input[index]
			mapping[j] = index
			if li > 0 {
				normalized[j].Reversed = !normalized[j].Reversed
			}
		}
		opt := o
		opt.Tessellation.MaxSegments -= segments
		opt.Tessellation.MaxChecks -= checks
		if opt.Tessellation.MaxSegments < 1 || opt.Tessellation.MaxChecks < 1 {
			return empty, geom.ErrConvergence
		}
		loop, e := BoundAffineQuadraticTrimLoopArea(ctx, patch, normalized, opt)
		if e != nil {
			return empty, e
		}
		segments += len(loop.Boundary.Segments)
		checks += loop.Boundary.Checks
		if segments > polygonOptions.MaxEdges {
			return empty, geom.ErrConvergence
		}
		// The boundary residual uses rounded display chord endpoints. Move
		// back to the exact affine chord using its maximum endpoint discrepancy.
		// (a+b)^2 <= 2(a^2+b^2), with no inflation for exact display endpoints.
		for j, bound := range loop.Boundary.SquaredErrors {
			rounding := new(big.Rat)
			for _, vertex := range []int{j, (j + 1) % len(loop.Boundary.Points)} {
				p := loop.Boundary.Points[vertex]
				values := []float64{p.X, p.Y, p.Z}
				delta := new(big.Rat)
				for k, v := range values {
					d := new(big.Rat).Sub(loop.Boundary.ExactPoints[vertex][k], ratInset(v))
					delta.Add(delta, new(big.Rat).Mul(d, d))
				}
				if delta.Cmp(rounding) > 0 {
					rounding = delta
				}
			}
			exactBound := new(big.Rat).Set(bound)
			if rounding.Sign() > 0 {
				exactBound.Add(exactBound, rounding)
				exactBound.Mul(exactBound, big.NewRat(2, 1))
			}
			if exactBound.Cmp(result.FillSquaredDistance) > 0 {
				result.FillSquaredDistance = exactBound
			}
		}
		result.Loops = append(result.Loops, loop)
		result.OriginalSource = append(result.OriginalSource, mapping)
		combine := func(sum *RationalAreaInterval, part RationalAreaInterval) {
			if li == 0 {
				sum.Lower.Add(sum.Lower, part.Lower)
				sum.Upper.Add(sum.Upper, part.Upper)
			} else {
				sum.Lower.Sub(sum.Lower, part.Upper)
				sum.Upper.Sub(sum.Upper, part.Lower)
			}
		}
		combine(&result.SourceUVArea, loop.SourceUVArea)
		for k := range result.ModelAreaVector {
			combine(&result.ModelAreaVector[k], loop.ModelAreaVector[k])
		}
		if li == 0 {
			result.PolygonUVArea.Add(result.PolygonUVArea, loop.PolygonUVArea)
		} else {
			result.PolygonUVArea.Sub(result.PolygonUVArea, loop.PolygonUVArea)
		}
		uv := append([][2]*big.Rat(nil), loop.Boundary.ExactUV...)
		if li > 0 {
			for i, j := 0, len(uv)-1; i < j; i, j = i+1, j-1 {
				uv[i], uv[j] = uv[j], uv[i]
			}
		}
		polygon := make(trimvalidate.Loop, len(uv))
		for i, p := range uv {
			q := uv[(i+1)%len(uv)]
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
		polygons = append(polygons, polygon)
	}
	result.PolygonProof, e = trimvalidate.ValidateLoops(ctx, polygons[0], polygons[1:], polygonOptions)
	if e != nil {
		return empty, e
	}
	if result.PolygonProof.Status != trimvalidate.Certified {
		return empty, geom.ErrConvergence
	}
	if result.SourceUVArea.Lower.Sign() < 0 {
		result.SourceUVArea.Lower.SetInt64(0)
	}
	return result, nil
}
