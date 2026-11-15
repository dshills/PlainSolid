package trimmed

import (
	"context"
	"fmt"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// MappedChainRectangularSolid is an opt-in material construction. ChainEdges
// follows the common original model-station order, and ClosingEdges follows the
// declared closing seams. The native solid's face order preserves source IDs.
type MappedChainRectangularSolid struct {
	Solid        Solid
	ChainEdges   []int
	ClosingEdges []int
	Embedding    MappedChainRectangularEmbedding
}

// BuildRectangularSolid proves continuous source embedding before creating a
// native trimmed Solid. Each common chain cell keeps both original authored
// splines, restricted to the exact represented raw parameter endpoints with
// its own positive speed map. The ordinary Solid and native edge validation
// also run. Model corner points must be exactly representable in float64.
func (r MappedSplineChainShell) BuildRectangularSolid(ctx context.Context, tolerance float64, o geom.SurfacePatchOptions) (MappedChainRectangularSolid, error) {
	var zero MappedChainRectangularSolid
	if ctx == nil || !finite(tolerance) || tolerance <= 0 {
		return zero, geom.ErrInvalid
	}
	embedding, err := r.CertifyRectangularEmbedding(ctx, o)
	if err != nil {
		return zero, err
	}
	source, err := r.boundary.mappedChainClosingSource(r.closing, r.reversed, r.planar, r.seams)
	if err != nil {
		return zero, err
	}
	nCells := len(r.boundary.cuts) - 1
	pairs := mappedChainCellPairs(nCells, r.seams)
	faces := make([]Face, len(source.trims))
	patches := make([]*geom.SurfacePatch, len(faces))
	for i, p := range source.surfaces {
		patches[i], err = exactSplineSupportPatch(p, o)
		if err != nil {
			return zero, err
		}
		trims := source.trims[i]
		if i < 2 {
			c := r.boundary.sources[i]
			cells := make([]PCurve, 0, nCells+len(trims))
			for j := 0; j < nCells; j++ {
				part, lo, hi, err := c.cellParameters(r.boundary.cuts[j], r.boundary.cuts[j+1])
				if err != nil {
					return zero, err
				}
				p, err := restrictMappedSource(ctx, c.parts[part], lo, hi, o)
				if err != nil {
					return zero, fmt.Errorf("native roof %d chain cell %d: %w", i, j, err)
				}
				cells = append(cells, p)
			}
			trims = append(cells, trims...)
			source.trims[i] = trims
		}
		faces[i] = Face{Surface: p, Reversed: source.reversed[i], Outer: make([]Use, len(trims))}
	}
	var vertices []geom.Vec3
	edges := make([]parametric.Edge, len(pairs))
	vertexIDs := make(map[geom.Vec3]int)
	eval := geom.SurfacePatchEvaluationOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits}
	for edge, pair := range pairs {
		reverse := source.reversed[pair[0].Face] == source.reversed[pair[1].Face]
		var model [2]geom.Vec3
		for k, u := range pair {
			trim := source.trims[u.Face][u.Trim]
			use, points, err := mappedChainNativeUse(ctx, patches[u.Face], trim, eval)
			if err != nil {
				return zero, fmt.Errorf("native face %d trim %d: %w", u.Face, u.Trim, err)
			}
			use.Edge = edge
			use.Reversed = k == 1 && reverse
			if use.Reversed {
				points[0], points[1] = points[1], points[0]
			}
			if k == 0 {
				model = points
				edges[edge].Curve = surfaceTrimCurve{source.surfaces[u.Face], use.UVStart, use.UVEnd, trim}
			} else if points != model {
				return zero, geom.ErrConvergence
			}
			faces[u.Face].Outer[u.Trim] = use
		}
		var ids [2]int
		for i, p := range model {
			id, ok := vertexIDs[p]
			if !ok {
				id = len(vertices)
				vertexIDs[p] = id
				vertices = append(vertices, p)
			}
			ids[i] = id
		}
		edges[edge].Start, edges[edge].End = ids[0], ids[1]
		edges[edge].Range = parametric.Interval{Max: 1}
	}
	if len(vertices) != r.topology.Vertices+nCells-1 {
		return zero, geom.ErrConvergence
	}
	solid, err := NewSolidContext(ctx, vertices, edges, faces, tolerance)
	if err != nil {
		return zero, err
	}
	if err = validateNativeExactQuadratics(ctx, solid); err != nil {
		return zero, fmt.Errorf("native mapped chain solid: %w", err)
	}
	out := MappedChainRectangularSolid{Solid: solid, Embedding: embedding}
	for i := range edges {
		if i < nCells {
			out.ChainEdges = append(out.ChainEdges, i)
		} else {
			out.ClosingEdges = append(out.ClosingEdges, i)
		}
	}
	return out, nil
}

func mappedChainNativeUse(ctx context.Context, p *geom.SurfacePatch, curve PCurve, o geom.SurfacePatchEvaluationOptions) (Use, [2]geom.Vec3, error) {
	var points [2]geom.Vec3
	exact := curve
	if c, ok := curve.(MappedSplineUV); ok {
		exact = c.curve
	}
	var ends [2]geom.Vec2
	for i, t := range [2]*big.Rat{new(big.Rat), big.NewRat(1, 1)} {
		uv, err := mixedExactEvaluate(exact, t)
		if err != nil {
			return Use{}, points, err
		}
		ends[i].X, _ = uv[0].Float64()
		ends[i].Y, _ = uv[1].Float64()
		jet, err := p.EvaluateRational(ctx, uv[0], uv[1], o)
		if err != nil {
			return Use{}, points, err
		}
		points[i], err = geom.RepresentedExactPoint(jet.Point)
		if err != nil {
			return Use{}, points, err
		}
	}
	return Use{UVStart: ends[0], UVEnd: ends[1], Curve: curve}, points, nil
}
