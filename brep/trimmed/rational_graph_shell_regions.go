package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

type RationalGraphShellRegionOptions struct {
	Topology trimvalidate.Options
}

// RationalGraphShellRegion indexes one outer wire and its immediate hole
// wires. Material islands inside a hole become separate regions.
type RationalGraphShellRegion struct {
	Outer int
	Holes []int
}

// RationalGraphShellRegions proves a planar material arrangement for the
// corresponding oriented shell section. Rings follows original wire order.
// ProjectionAxes are ordered so positive planar winding agrees with the
// oriented plane. Loops bound the exact material, independent of display.
// This result does not build or attach a cap.
type RationalGraphShellRegions struct {
	ProjectionAxes [2]int
	Rings          []trimvalidate.RingPlacement
	Material       []RationalGraphShellRegion
	certificate    *trimvalidate.CertifiedLoops
	plane          polynomialPlaneEquation
	maxBits        int
	wires          [][]projectedGraphSectionPiece
	material       []RationalGraphShellRegion
}

// RationalGraphSectionPiece identifies an exact retained interval on an
// original section piece. Generated edges have Source.Branch == -1. Its
// Homogeneous controls are expressed on the section's ProjectionAxes.
type RationalGraphSectionPiece struct {
	Source      RationalGraphCapEdgeSource
	Parameter   [2]*big.Rat
	Homogeneous [][3]*big.Rat
}

// SectionWires returns original owners, retained parameter intervals and
// exact projected controls in boundary order. Every rational is caller-owned.
func (r *RationalGraphShellRegions) SectionWires() [][]RationalGraphSectionPiece {
	if r == nil || r.certificate == nil {
		return nil
	}
	out := make([][]RationalGraphSectionPiece, len(r.wires))
	for i, wire := range r.wires {
		out[i] = make([]RationalGraphSectionPiece, len(wire))
		for j, p := range wire {
			out[i][j] = RationalGraphSectionPiece{p.source, [2]*big.Rat{
				graphCapOriginalParameter(p.span, new(big.Rat)), graphCapOriginalParameter(p.span, big.NewRat(1, 1)),
			}, copyHomogeneousUV(p.control)}
		}
	}
	return out
}

// RationalGraphSectionArea reports exact projected area bounds and the squared
// model-to-projection area scale. True area is projected area times the
// positive square root of ModelScaleSquared.
type RationalGraphSectionArea struct {
	Projection        trimvalidate.SignedAreaInterval
	ModelScaleSquared *big.Rat
}

// BoundArea measures the certified planar material without meshing. The
// requested positive width is in projected model units squared.
func (r *RationalGraphShellRegions) BoundArea(ctx context.Context, projectedWidth *big.Rat) (RationalGraphSectionArea, error) {
	if r == nil || r.certificate == nil {
		return RationalGraphSectionArea{}, geom.ErrInvalid
	}
	a, e := r.certificate.BoundSignedArea(ctx, projectedWidth)
	if e != nil {
		return RationalGraphSectionArea{}, e
	}
	m := new(big.Rat)
	for _, d := range r.plane.normal {
		m.Add(m, new(big.Rat).Mul(d, d))
	}
	return RationalGraphSectionArea{Projection: a, ModelScaleSquared: m}, nil
}

// LocateRational classifies an exact model-space point in the planar material
// region. A nonincident point returns ErrDomain. Boundary is proved against
// the exact curves; no projected display polygon is used.
func (r *RationalGraphShellRegions) LocateRational(ctx context.Context, point [3]*big.Rat) (trimvalidate.Location, trimvalidate.Report, error) {
	if ctx == nil || r == nil || r.certificate == nil || point[0] == nil || point[1] == nil || point[2] == nil {
		return trimvalidate.Unknown, trimvalidate.Report{}, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return trimvalidate.Unknown, trimvalidate.Report{}, e
	}
	for _, q := range point {
		if q.Num().BitLen() > r.maxBits || q.Denom().BitLen() > r.maxBits {
			return trimvalidate.Unknown, trimvalidate.Report{}, geom.ErrConvergence
		}
	}
	if r.plane.dot(point).Cmp(r.plane.offset) != 0 {
		return trimvalidate.Unknown, trimvalidate.Report{}, geom.ErrDomain
	}
	axes := orientedRationalGraphProjection(r.plane)
	return r.certificate.Locate(ctx, [2]*big.Rat{point[axes[0]], point[axes[1]]})
}

func orientedRationalGraphProjection(plane polynomialPlaneEquation) [2]int {
	axes := plane.axes
	if plane.orientation < 0 {
		axes[0], axes[1] = axes[1], axes[0]
	}
	return axes
}

func classifyRationalGraphShellRegions(ctx context.Context, section RationalGraphShellSection, faces []splineEmbeddingFace, plane polynomialPlaneEquation, limits geom.SurfacePatchOptions, o RationalGraphShellRegionOptions) (*RationalGraphShellRegions, error) {
	opts, e := o.Topology.WithDefaults()
	if e != nil {
		return nil, e
	}
	axes := orientedRationalGraphProjection(plane)
	rings := make([]trimvalidate.Loop, len(section.Wires))
	wires := make([][]projectedGraphSectionPiece, len(section.Wires))
	count := 0
	for i, wire := range section.Wires {
		for _, id := range wire {
			b := section.Branches[id]
			for pieceID, piece := range b.Pieces {
				count++
				if count > opts.MaxEdges {
					return nil, geom.ErrConvergence
				}
				project := projectedRationalGraphSectionControls
				if b.compose {
					project = composeProjectedGraphSectionControls
				}
				h, e := project(ctx, faces[b.Face].patch, piece, axes, limits)
				if e != nil {
					return nil, e
				}
				q, e := trimvalidate.ExactBezier(h)
				if e != nil {
					return nil, e
				}
				rings[i] = append(rings[i], q)
				wires[i] = append(wires[i], projectedGraphSectionPiece{
					source:  RationalGraphCapEdgeSource{Wire: i, Branch: id, Piece: pieceID, Face: b.Face},
					control: h,
				})
			}
		}
	}
	return certifyProjectedRationalGraphRegions(ctx, wires, rings, plane, opts)
}

func certifyProjectedRationalGraphRegions(ctx context.Context, wires [][]projectedGraphSectionPiece, rings []trimvalidate.Loop, plane polynomialPlaneEquation, opts trimvalidate.Options) (*RationalGraphShellRegions, error) {
	certificate, placements, proof, e := trimvalidate.CertifyRingForest(ctx, rings, opts)
	if e != nil {
		return nil, e
	}
	if proof.Status != trimvalidate.Certified {
		return nil, geom.ErrConvergence
	}
	out := &RationalGraphShellRegions{ProjectionAxes: orientedRationalGraphProjection(plane), Rings: placements, certificate: certificate, plane: plane, maxBits: opts.MaxBits, wires: wires}
	byOuter := make(map[int]int, len(placements))
	for i, r := range placements {
		if r.Depth%2 == 0 {
			if r.Orientation != 1 {
				return nil, geom.ErrConvergence
			}
			byOuter[i] = len(out.Material)
			out.Material = append(out.Material, RationalGraphShellRegion{Outer: i})
		} else if r.Orientation != -1 {
			return nil, geom.ErrConvergence
		}
	}
	for i, r := range placements {
		if r.Depth%2 != 0 {
			k, ok := byOuter[r.Parent]
			if !ok {
				return nil, geom.ErrConvergence
			}
			out.Material[k].Holes = append(out.Material[k].Holes, i)
		}
	}
	for _, r := range out.Material {
		out.material = append(out.material, RationalGraphShellRegion{Outer: r.Outer, Holes: append([]int(nil), r.Holes...)})
	}
	return out, nil
}

// The originating proof guarantees the selected model projections are affine
// on this entire knot cell. Exact derivatives at an interior anchor recover
// their coefficients in the original UV units. Homogeneous affine transport
// then preserves weights, endpoints, traversal, and complete curve coverage.
func projectedRationalGraphSectionControls(ctx context.Context, patch *geom.SurfacePatch, c PCurve, axes [2]int, limits geom.SurfacePatchOptions) ([][3]*big.Rat, error) {
	h, e := polynomialPlaneControls(c)
	if e != nil {
		return nil, e
	}
	var mid [2]*big.Rat
	first, last := h[0], h[len(h)-1]
	for i := range mid {
		a, b := new(big.Rat).Quo(first[i], first[2]), new(big.Rat).Quo(last[i], last[2])
		mid[i] = new(big.Rat).Quo(new(big.Rat).Add(a, b), big.NewRat(2, 1))
	}
	j, e := patch.EvaluateRational(ctx, mid[0], mid[1], geom.SurfacePatchEvaluationOptions{
		MaxOperations: limits.MaxOperations, MaxCoefficientBits: limits.MaxCoefficientBits,
	})
	if e != nil {
		return nil, e
	}
	result := make([][3]*big.Rat, len(h))
	for i, p := range h {
		result[i][2] = p[2]
		for k, axis := range axes {
			result[i][k] = new(big.Rat).Mul(j.Point[axis], p[2])
			for a, derivative := range [2]*big.Rat{j.Du[axis], j.Dv[axis]} {
				delta := new(big.Rat).Sub(p[a], new(big.Rat).Mul(mid[a], p[2]))
				result[i][k].Add(result[i][k], new(big.Rat).Mul(derivative, delta))
			}
		}
	}
	return result, nil
}
