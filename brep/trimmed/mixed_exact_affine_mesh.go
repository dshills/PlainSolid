package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// MixedExactAffineMeshOptions bounds source/polygon topology separately. Display
// operations/segments and composition operations are divided equally among all
// input trims; chart evaluation limits apply per point. At most256 trims and64
// holes are accepted. Display segments default4096 and are capped at4096.
type MixedExactAffineMeshOptions struct {
	// DomainSubdivisions/DomainDepth opt into exact homogeneous subdivision
	// for controls outside the support but curves wholly inside it. Work is
	// cumulative over all face trims; zero values retain the old hull-only gate.
	DomainSubdivisions, DomainDepth int
	// MaxBoundaryVertices opts into bounded exact triangulation above the
	// legacy256-vertex proposal limit (maximum2048). Zero preserves that limit.
	// MaxTriangulationWork bounds exact orientation/bridge work, default2000000.
	MaxBoundaryVertices, MaxTriangulationWork int
	Chart                                     geom.AffineSurfaceChartOptions
	Composition                               geom.SurfaceCurveAgreementOptions
	Display                                   geom.ComposedCollectionPolylineOptions
	Topology                                  trimvalidate.Options
	MaxTriangles                              int
}

// MixedExactTrimSamples retains original loop/trim ownership and normalized
// parameter values. Holes are traversed backwards in the internal CCW proof;
// Parameters always use the original input trim direction. Vertices are final
// mesh IDs, including both endpoints. No duplicate endpoint vertex is inserted.
type MixedExactTrimSamples struct {
	Loop, Trim int
	Parameters []*big.Rat
	Vertices   []int
	Polyline   geom.CertifiedComposedCollectionPolyline
}

// CertifiedMixedExactAffineMesh certifies both directed source-fill/display
// distances. SourceProof/PolygonProof establish independent Jordan regions;
// exact triangulation certifies the chord polygon. Embedded Region is only an
// internal polygon/fill proof carrier: its quadratic source/area fields are not
// populated. Source and Samples retain the actual mixed native trim authority.
// This does not prove cross-face sewing or rounded display embedding.
type CertifiedMixedExactAffineMesh struct {
	DomainSubdivisions int
	CertifiedAffineQuadraticTrimMesh
	Source                    [][]PCurve
	Samples                   []MixedExactTrimSamples
	SourceProof, PolygonProof trimvalidate.Report
}

// TessellateMixedExactAffineRegionCertified accepts CCW outer/CW separated holes
// consisting of native ExactQuadraticUV and ExactBezierUV values. It never
// converts exact coefficients to sampled splines. Source/chord winding agrees
// away from the bounded straight boundary homotopy; therefore each closed fill
// is within the maximum exact model chord error of the other. Final triangle
// display rounding is added independently. Unsupported topology fails atomically.
func TessellateMixedExactAffineRegionCertified(ctx context.Context, p *geom.SurfacePatch, outer []PCurve, holes [][]PCurve, o MixedExactAffineMeshOptions) (CertifiedMixedExactAffineMesh, error) {
	return tessellateMixedExactAffineRegionCuts(ctx, p, outer, holes, nil, o)
}

func tessellateMixedExactAffineRegionCuts(ctx context.Context, p *geom.SurfacePatch, outer []PCurve, holes [][]PCurve, cuts [][][]*big.Rat, o MixedExactAffineMeshOptions) (CertifiedMixedExactAffineMesh, error) {
	var empty CertifiedMixedExactAffineMesh
	if ctx == nil || p == nil || len(holes) > 64 || o.Display.EndpointPoints != nil || o.Display.JunctionPoints != nil {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	if o.MaxTriangles == 0 {
		o.MaxTriangles = 100000
	}
	if o.MaxTriangles < 1 || o.MaxTriangles > 1000000 {
		return empty, geom.ErrInvalid
	}
	if o.Display.MaxSegments == 0 {
		o.Display.MaxSegments = 4096
	}
	if o.Display.MaxSegments < 1 || o.Display.MaxSegments > 4096 {
		return empty, geom.ErrInvalid
	}
	if o.Display.MaxOperations == 0 {
		o.Display.MaxOperations = 8000000
	}
	if o.Composition.MaxOperations == 0 {
		o.Composition.MaxOperations = 1000000
	}
	input := append([][]PCurve{outer}, holes...)
	count := 0
	for _, loop := range input {
		if len(loop) < 2 || len(loop) > 256-count {
			return empty, geom.ErrInvalid
		}
		count += len(loop)
	}
	if o.Display.MaxSegments < count || o.Display.MaxOperations < count || o.Composition.MaxOperations < count {
		return empty, geom.ErrConvergence
	}
	if cuts != nil {
		if len(cuts) != len(input) {
			return empty, geom.ErrInvalid
		}
		total := 0
		for i, loop := range cuts {
			if len(loop) != len(input[i]) {
				return empty, geom.ErrInvalid
			}
			for _, q := range loop {
				total += len(q)
				if total > 2048 {
					return empty, geom.ErrConvergence
				}
				if q != nil {
					if err := validateGraphCuts(q); err != nil {
						return empty, err
					}
				}
			}
		}
	}
	chart, e := geom.NewAffineSurfaceChart(ctx, p, o.Chart)
	if e != nil {
		return empty, e
	}
	proof, domainSubdivisions, e := proveMixedExactTrimLoops(ctx, p.Domain(), input, o.DomainSubdivisions, o.DomainDepth, o.Topology)
	if e != nil {
		return empty, e
	}
	owned := make([][]PCurve, len(input))
	for li, loop := range input {
		owned[li] = append([]PCurve(nil), loop...)
	}
	region := AffineQuadraticTrimRegionArea{FillSquaredDistance: new(big.Rat), PolygonUVArea: new(big.Rat), SourceProof: proof}
	var samples []MixedExactTrimSamples
	var polygons []trimvalidate.Loop
	local := o.Display
	local.MaxSegments /= count
	local.MaxOperations /= count
	comp := o.Composition
	comp.MaxOperations /= count
	for li, loop := range input {
		boundary := CertifiedQuadraticTrimLoop{}
		loopStart := len(samples)
		for j := range loop {
			index := j
			curve := loop[j]
			if li > 0 {
				index = len(loop) - 1 - j
				curve = mixedExactReverse(loop[index])
			}
			evaluate := func(t *big.Rat) ([2]*big.Rat, error) { return mixedExactEvaluate(curve, t) }
			ends := [2]geom.Vec3{}
			for k, t := range []*big.Rat{new(big.Rat), big.NewRat(1, 1)} {
				uv, e := evaluate(t)
				if e != nil {
					return empty, e
				}
				xyz, e := chart.EvalExact(uv[0], uv[1])
				if e != nil {
					return empty, e
				}
				for axis, f := range []*float64{&ends[k].X, &ends[k].Y, &ends[k].Z} {
					*f, _ = xyz[axis].Float64()
				}
				if !ends[k].IsFinite() {
					return empty, geom.ErrConvergence
				}
			}
			local.EndpointPoints = &ends
			cells, e := mixedExactCells(ctx, p, curve, comp)
			if e != nil {
				return empty, e
			}
			var fixed []*big.Rat
			if cuts != nil {
				fixed = cuts[li][index]
				if fixed != nil && li > 0 {
					reversed := make([]*big.Rat, len(fixed))
					for k, t := range fixed {
						reversed[len(fixed)-1-k] = new(big.Rat).Sub(big.NewRat(1, 1), t)
					}
					fixed = reversed
				}
			}
			line, e := tessellateGraphPrescribedCuts(ctx, cells, fixed, local)
			if e != nil {
				return empty, e
			}
			if !line.Complete || line.DisplayDegenerate {
				return empty, geom.ErrConvergence
			}
			sample := MixedExactTrimSamples{Loop: li, Trim: index, Polyline: line}
			rounding := new(big.Rat)
			for k, v := range line.Vertices {
				if v.Parameter.Lower.Cmp(v.Parameter.Upper) != 0 {
					return empty, geom.ErrConvergence
				}
				t := new(big.Rat).Set(v.Parameter.Lower)
				uv, e := evaluate(t)
				if e != nil {
					return empty, e
				}
				xyz, e := chart.EvalExact(uv[0], uv[1])
				if e != nil {
					return empty, e
				}
				if li > 0 {
					t.Sub(big.NewRat(1, 1), t)
				}
				sample.Parameters = append(sample.Parameters, t)
				sample.Vertices = append(sample.Vertices, len(boundary.Points))
				sq := new(big.Rat)
				for axis, f := range []float64{v.Point.X, v.Point.Y, v.Point.Z} {
					delta := new(big.Rat).Sub(xyz[axis], ratInset(f))
					sq.Add(sq, new(big.Rat).Mul(delta, delta))
				}
				if sq.Cmp(rounding) > 0 {
					rounding.Set(sq)
				}
				if k < len(line.Vertices)-1 {
					boundary.Points = append(boundary.Points, v.Point)
					boundary.ExactUV = append(boundary.ExactUV, uv)
					boundary.ExactPoints = append(boundary.ExactPoints, xyz)
				}
			}
			bound := new(big.Rat).Mul(ratInset(line.MaximumError), ratInset(line.MaximumError))
			bound.Add(bound, rounding)
			bound.Mul(bound, big.NewRat(2, 1))
			if bound.Cmp(region.FillSquaredDistance) > 0 {
				region.FillSquaredDistance.Set(bound)
			}
			samples = append(samples, sample)
		}
		if len(boundary.Points) > 4096 {
			return empty, geom.ErrConvergence
		}
		for i := loopStart; i < len(samples); i++ {
			for j, id := range samples[i].Vertices {
				samples[i].Vertices[j] = id % len(boundary.Points)
			}
		}
		area := new(big.Rat)
		for j, a := range boundary.ExactUV {
			b := boundary.ExactUV[(j+1)%len(boundary.ExactUV)]
			area.Add(area, new(big.Rat).Sub(new(big.Rat).Mul(a[0], b[1]), new(big.Rat).Mul(a[1], b[0])))
		}
		area.Quo(area, big.NewRat(2, 1))
		if area.Sign() <= 0 {
			return empty, geom.ErrConvergence
		}
		if li == 0 {
			region.PolygonUVArea.Add(region.PolygonUVArea, area)
		} else {
			region.PolygonUVArea.Sub(region.PolygonUVArea, area)
		}
		region.Loops = append(region.Loops, AffineQuadraticTrimLoopArea{Boundary: boundary, PolygonUVArea: area})
		uv := append([][2]*big.Rat(nil), boundary.ExactUV...)
		if li > 0 {
			for j, k := 0, len(uv)-1; j < k; j, k = j+1, k-1 {
				uv[j], uv[k] = uv[k], uv[j]
			}
		}
		poly := make(trimvalidate.Loop, len(uv))
		for j, a := range uv {
			b := uv[(j+1)%len(uv)]
			h := [3][3]*big.Rat{}
			for axis := 0; axis < 2; axis++ {
				h[0][axis] = a[axis]
				h[2][axis] = b[axis]
				h[1][axis] = new(big.Rat).Quo(new(big.Rat).Add(a[axis], b[axis]), big.NewRat(2, 1))
			}
			for k := 0; k < 3; k++ {
				h[k][2] = big.NewRat(1, 1)
			}
			poly[j], e = trimvalidate.ExactQuadratic(h)
			if e != nil {
				return empty, e
			}
		}
		polygons = append(polygons, poly)
	}
	polygonProof, e := trimvalidate.ValidateLoops(ctx, polygons[0], polygons[1:], o.Topology)
	if e != nil {
		return empty, e
	}
	if polygonProof.Status != trimvalidate.Certified {
		return empty, geom.ErrConvergence
	}
	region.PolygonProof = polygonProof
	filled, e := triangulateCertifiedAffineTrimRegionBounded(ctx, region, len(holes), o.MaxTriangles, o.MaxBoundaryVertices, o.MaxTriangulationWork)
	if e != nil {
		return empty, e
	}
	for i := range samples {
		li := samples[i].Loop
		ids := filled.BoundaryLoops[li]
		for j, id := range samples[i].Vertices {
			if li > 0 {
				id = len(ids) - 1 - id
			}
			samples[i].Vertices[j] = ids[id]
		}
	}
	return CertifiedMixedExactAffineMesh{DomainSubdivisions: domainSubdivisions, CertifiedAffineQuadraticTrimMesh: filled, Source: owned, Samples: samples, SourceProof: proof, PolygonProof: polygonProof}, nil
}
func mixedExactControls(c PCurve) ([][3]*big.Rat, error) {
	switch q := c.(type) {
	case ExactQuadraticUV:
		h := q.HomogeneousControls()
		return h[:], nil
	case ExactBezierUV:
		return q.HomogeneousControls(), nil
	}
	return nil, geom.ErrInvalid
}
func mixedExactReverse(c PCurve) PCurve {
	switch q := c.(type) {
	case ExactQuadraticUV:
		return q.reversed()
	case ExactBezierUV:
		return q.reversed()
	}
	return nil
}
func mixedExactEvaluate(c PCurve, t *big.Rat) ([2]*big.Rat, error) {
	switch q := c.(type) {
	case ExactQuadraticUV:
		p, _, e := q.EvaluateRational(t)
		return p, e
	case ExactBezierUV:
		p, _, e := q.EvaluateRational(t)
		return p, e
	}
	return [2]*big.Rat{}, geom.ErrInvalid
}
func mixedExactCells(ctx context.Context, p *geom.SurfacePatch, c PCurve, o geom.SurfaceCurveAgreementOptions) (geom.SurfaceCurveCompositionCells, error) {
	switch q := c.(type) {
	case ExactQuadraticUV:
		return q.ModelCurveCells(ctx, p, o)
	case ExactBezierUV:
		return q.ModelCurveCells(ctx, p, o)
	}
	return geom.SurfaceCurveCompositionCells{}, geom.ErrInvalid
}

// proveMixedExactTrimLoops validates the original chart domain and Jordan loops
// without constructing a chord polygon or evaluating any display samples.
func proveMixedExactTrimLoops(ctx context.Context, d geom.ParameterRectangle, input [][]PCurve, maxSubdivisions, maxDepth int, topology trimvalidate.Options) (trimvalidate.Report, int, error) {
	domain, e := newFaceDomainBudget(maxSubdivisions, maxDepth)
	if e != nil {
		return trimvalidate.Report{}, 0, e
	}
	bounds := [2][2]*big.Rat{{ratInset(d.UMin), ratInset(d.UMax)}, {ratInset(d.VMin), ratInset(d.VMax)}}
	source := make([]trimvalidate.Loop, len(input))
	for li, loop := range input {
		source[li] = make(trimvalidate.Loop, len(loop))
		for j, c := range loop {
			h, e := mixedExactControls(c)
			if e != nil {
				return trimvalidate.Report{}, 0, e
			}
			outside := false
			for _, q := range h {
				if q[0] == nil || q[1] == nil || q[2] == nil || q[2].Sign() <= 0 {
					return trimvalidate.Report{}, 0, geom.ErrInvalid
				}
				u, v := new(big.Rat).Quo(q[0], q[2]), new(big.Rat).Quo(q[1], q[2])
				if u.Cmp(bounds[0][0]) < 0 || u.Cmp(bounds[0][1]) > 0 || v.Cmp(bounds[1][0]) < 0 || v.Cmp(bounds[1][1]) > 0 {
					outside = true
				}
			}
			if outside {
				if e := domain.prove(ctx, h, bounds, 16384); e != nil {
					return trimvalidate.Report{}, 0, e
				}
			}
			source[li][j], e = trimvalidate.ExactBezier(h)
			if e != nil {
				return trimvalidate.Report{}, 0, e
			}
		}
	}
	proof, e := trimvalidate.ValidateLoops(ctx, source[0], source[1:], topology)
	if e != nil {
		return trimvalidate.Report{}, 0, e
	}
	if proof.Status != trimvalidate.Certified {
		return trimvalidate.Report{}, 0, geom.ErrConvergence
	}
	return proof, domain.used, nil
}
