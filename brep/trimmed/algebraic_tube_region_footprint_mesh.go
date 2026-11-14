package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
	"plainsolid/mesh"
)

type AlgebraicTubeFootprintMeshOptions struct {
	Boundary                    AlgebraicTubeRegionsBoundaryOptions
	Polygon                     trimvalidate.Options
	MaxTriangles, MaxPredicates int
}
type CertifiedAlgebraicTubeFootprint struct {
	Predicates                             int
	Boundary                               CertifiedAlgebraicTubeRegionsBoundary
	Mesh                                   mesh.Mesh // Model XY projection, with Z=0.
	Region                                 int
	GlobalBoundaryVertices                 []int   // Mesh vertex -> original common model network index.
	BoundaryLoops                          [][]int // Outer CCW, holes CW.
	PolygonArea, SourceFillSquaredDistance *big.Rat
	PolygonProof                           trimvalidate.Report
}

// TessellateFootprintCertified fills the model-XY projection of one bounded
// region, including its immediate holes. Exact source containment comes from
// the sealed collection; exact displayed polygon topology is checked separately.
// The loopwise point-correspondence bound projects nonexpansively to XY. Winding
// through its straight boundary homotopy bounds both directed closed-fill
// distances, including hole and outer boundary points. This is a planar
// footprint certificate only: it is neither a curved face mesh nor a material
// volume certificate. It does not require biquadratic surface supports.
// Original float spline and exact homogeneous Bézier owner charts are retained
// without conversion of their control points.
func (r AlgebraicTubeRegions) TessellateFootprintCertified(ctx context.Context, region int, o AlgebraicTubeFootprintMeshOptions) (CertifiedAlgebraicTubeFootprint, error) {
	var empty CertifiedAlgebraicTubeFootprint
	if ctx == nil || r.section.runSource() == nil || region < 0 || region >= len(r.regions) {
		return empty, geom.ErrInvalid
	}
	network, e := r.TessellateBoundaryCertified(ctx, o.Boundary)
	if e != nil {
		return empty, e
	}
	loops := append([]int{r.regions[region].Outer}, r.regions[region].Holes...)
	out, e := triangulateTubeFootprintNetwork(ctx, network, loops, o)
	if e != nil {
		return empty, e
	}
	out.Region = region
	out.Boundary = network
	return out, nil
}

// The helper proves only polygon topology/coverage. Source fill correspondence
// is asserted by the public method only after obtaining the private sealed
// region and freshly rederived common network.
func triangulateTubeFootprintNetwork(ctx context.Context, network CertifiedAlgebraicTubeRegionsBoundary, loops []int, o AlgebraicTubeFootprintMeshOptions) (CertifiedAlgebraicTubeFootprint, error) {
	var empty CertifiedAlgebraicTubeFootprint
	if ctx == nil || len(loops) == 0 {
		return empty, geom.ErrInvalid
	}
	if o.MaxPredicates == 0 {
		o.MaxPredicates = 10000000
	}
	if o.MaxPredicates < 1 || o.MaxPredicates > 100000000 {
		return empty, geom.ErrInvalid
	}
	if o.MaxTriangles == 0 {
		o.MaxTriangles = 100000
	}
	if o.MaxTriangles < 1 || o.MaxTriangles > 1000000 {
		return empty, geom.ErrInvalid
	}
	options, e := o.Polygon.WithDefaults()
	if e != nil {
		return empty, e
	}
	region := AffineQuadraticTrimRegionArea{PolygonUVArea: new(big.Rat), FillSquaredDistance: new(big.Rat)}
	var polygons []trimvalidate.Loop
	var globalIDs [][]int
	count := 0
	for li, loop := range loops {
		if e := ctx.Err(); e != nil {
			return empty, e
		}
		if loop < 0 || loop >= len(network.Loops) || loop >= len(network.LoopVertices) {
			return empty, geom.ErrInvalid
		}
		var ring []int
		for _, path := range network.Loops[loop].Run.PieceVertices {
			if len(path) < 2 {
				return empty, geom.ErrInvalid
			}
			if len(ring) > 0 && ring[len(ring)-1] != path[0] {
				return empty, geom.ErrInvalid
			}
			if len(ring) == 0 {
				ring = append(ring, path[0])
			}
			ring = append(ring, path[1:]...)
		}
		if len(ring) < 4 || ring[0] != ring[len(ring)-1] {
			return empty, geom.ErrInvalid
		}
		ring = ring[:len(ring)-1]
		count += len(ring)
		if count > options.MaxEdges {
			return empty, geom.ErrConvergence
		}
		ids := make([]int, len(ring))
		boundary := CertifiedQuadraticTrimLoop{}
		for i, id := range ring {
			if id < 0 || id >= len(network.LoopVertices[loop]) {
				return empty, geom.ErrInvalid
			}
			global := network.LoopVertices[loop][id]
			if global < 0 || global >= len(network.Vertices) {
				return empty, geom.ErrInvalid
			}
			ids[i] = global
			p := network.Vertices[global]
			if !p.IsFinite() {
				return empty, geom.ErrInvalid
			}
			boundary.Points = append(boundary.Points, geom.Vec3{X: p.X, Y: p.Y})
			boundary.ExactUV = append(boundary.ExactUV, [2]*big.Rat{ratInset(p.X), ratInset(p.Y)})
			boundary.ExactPoints = append(boundary.ExactPoints, [3]*big.Rat{ratInset(p.X), ratInset(p.Y), new(big.Rat)})
		}
		area := new(big.Rat)
		for i, p := range boundary.ExactUV {
			q := boundary.ExactUV[(i+1)%len(ring)]
			area.Add(area, new(big.Rat).Sub(new(big.Rat).Mul(p[0], q[1]), new(big.Rat).Mul(p[1], q[0])))
		}
		area.Quo(area, big.NewRat(2, 1))
		if area.Sign() == 0 {
			return empty, geom.ErrDegenerate
		}
		if area.Sign() < 0 {
			area.Neg(area)
			for i, j := 0, len(ring)-1; i < j; i, j = i+1, j-1 {
				ids[i], ids[j] = ids[j], ids[i]
				boundary.Points[i], boundary.Points[j] = boundary.Points[j], boundary.Points[i]
				boundary.ExactUV[i], boundary.ExactUV[j] = boundary.ExactUV[j], boundary.ExactUV[i]
				boundary.ExactPoints[i], boundary.ExactPoints[j] = boundary.ExactPoints[j], boundary.ExactPoints[i]
			}
		}
		polygon := make(trimvalidate.Loop, len(ring))
		for i := range polygon {
			a, b := i, (i+1)%len(ring)
			if li > 0 {
				a, b = len(ring)-1-i, (len(ring)-2-i+len(ring))%len(ring)
			}
			p, q := boundary.Points[a], boundary.Points[b]
			polygon[i] = trimvalidate.Segment{Start: geom.Vec2{X: p.X, Y: p.Y}, End: geom.Vec2{X: q.X, Y: q.Y}}
		}
		polygons = append(polygons, polygon)
		globalIDs = append(globalIDs, ids)
		region.Loops = append(region.Loops, AffineQuadraticTrimLoopArea{Boundary: boundary, PolygonUVArea: area})
		if li == 0 {
			region.PolygonUVArea.Add(region.PolygonUVArea, area)
		} else {
			region.PolygonUVArea.Sub(region.PolygonUVArea, area)
		}
		for _, bounds := range network.Loops[loop].Run.SquaredErrors {
			for _, q := range bounds {
				if q == nil || q.Sign() < 0 {
					return empty, geom.ErrInvalid
				}
				if q.Cmp(region.FillSquaredDistance) > 0 {
					region.FillSquaredDistance.Set(q)
				}
			}
		}
	}
	proof, e := trimvalidate.ValidateLoops(ctx, polygons[0], polygons[1:], options)
	if e != nil {
		return empty, e
	}
	if proof.Status != trimvalidate.Certified {
		return empty, geom.ErrConvergence
	}
	region.PolygonProof = proof

	out := CertifiedAlgebraicTubeFootprint{PolygonArea: new(big.Rat).Set(region.PolygonUVArea), SourceFillSquaredDistance: new(big.Rat).Set(region.FillSquaredDistance), PolygonProof: proof}
	var points [][2]*big.Rat
	for li, loop := range region.Loops {
		var ring []int
		for j := range loop.Boundary.Points {
			index := j
			if li > 0 {
				index = len(loop.Boundary.Points) - 1 - j
			}
			ring = append(ring, len(points))
			points = append(points, loop.Boundary.ExactUV[index])
			out.Mesh.Vertices = append(out.Mesh.Vertices, loop.Boundary.Points[index])
			out.GlobalBoundaryVertices = append(out.GlobalBoundaryVertices, globalIDs[li][index])
		}
		out.BoundaryLoops = append(out.BoundaryLoops, ring)
	}
	if len(points)+2*(len(loops)-1)-2 > o.MaxTriangles {
		return empty, geom.ErrConvergence
	}
	out.Mesh.Triangles, out.Predicates, e = exactHoledFootprintTriangulation(ctx, points, out.BoundaryLoops, o.MaxPredicates)
	if e != nil {
		return empty, e
	}
	return out, nil
}
