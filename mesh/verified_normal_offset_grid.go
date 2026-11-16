package mesh

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"

	"plainsolid/geom"
)

// NormalOffsetGridOptions bounds a tensor grid for a verified normal offset.
// Zero selects defaults of .05 model units, eight dyadic refinement levels,
// 131072 triangles and 100 million geometry work units. MinDepth can start
// at a finer dyadic level to match the grid of an adjacent face.
type NormalOffsetGridOptions struct {
	Deflection                       float64
	MinDepth, MaxDepth, MaxTriangles int
	MaxWork                          int

	// MaxSourceOperations defaults to 4096. Each new corner reserves this
	// many work units and applies the same limit to its source-jet evaluation.
	// MaxWork also counts the reported offset-point and cell-bound work,
	// including work on unsuccessful grids. Reused corners incur no charge.
	MaxSourceOperations int

	// NormalAngle optionally bounds the deviation in radians between
	// each facet and the continuous offset normal anywhere in its cell.
	// A positive request must be below pi/2.
	NormalAngle float64 `json:",omitempty"`
}

// CertifiedNormalOffsetGrid is an embedded open mesh on the full offset
// parameter rectangle. Vertices are ordered U-major with V varying fastest.
// Every cell emits two consecutive triangles oriented in the parameter axes.
type CertifiedNormalOffsetGrid struct {
	Mesh                 Mesh
	UStations, VStations []float64

	// DistanceUpperSquared bounds the squared Hausdorff distance in model units.
	DistanceUpperSquared *big.Rat
	// ProjectedMonotonicityLower bounds each symmetric Jacobian's smaller
	// eigenvalue in the original parameter units, uniformly across the grid.
	ProjectedMonotonicityLower *big.Rat
	// GeometryWork includes cell bounds, offset-point evaluations and the
	// fixed source-jet reservations for all distinct sampled corners. Optional
	// unit-normal bounds also count toward this limit.
	GeometryWork int

	// SquaredFacetNormalTangent bounds the squared tangent of the
	// requested facet deviation. It is nil when NormalAngle is zero.
	SquaredFacetNormalTangent *big.Rat `json:",omitempty"`
}

// TessellateVerifiedNormalOffsetGrid encloses each continuous offset cell and
// every represented corner in the same exact rational box. Both directed
// distances between that offset and its delivered mesh are then bounded by
// the largest box diameter. A positive symmetric projected Jacobian on
// every triangle supplies a single positive monotonicity bound. Integrate
// that bound over segments in the convex domain to prove global injectivity.
// A positive NormalAngle also checks the represented facets against the
// continuous source normal over each full cell. An unresolved proof or
// exhausted budget returns an empty result.
func TessellateVerifiedNormalOffsetGrid(ctx context.Context, p *geom.VerifiedNormalOffsetPatch,
	o NormalOffsetGridOptions) (CertifiedNormalOffsetGrid, error) {
	var empty CertifiedNormalOffsetGrid
	if ctx == nil || p == nil || !p.Domain().Valid() {
		return empty, geom.ErrInvalid
	}
	o, err := checkedNormalOffsetGridOptions(o)
	if err != nil {
		return empty, err
	}
	distance := new(big.Rat).SetFloat64(o.Deflection)
	w := normalOffsetGridWork{options: o, points: make(map[[2]float64]normalOffsetGridPoint)}
	distance.Mul(distance, distance)
	if o.NormalAngle > 0 {
		w.facetLimit = normalOffsetFacetTangentLimit(o.NormalAngle)
	}
	for depth := o.MinDepth; depth <= o.MaxDepth; depth++ {
		if err := ctx.Err(); err != nil {
			return empty, err
		}
		n := 1 << depth
		if 2*n*n > o.MaxTriangles || w.operations == o.MaxWork {
			break
		}
		out, err := w.grid(ctx, p, n, distance)
		if errors.Is(err, geom.ErrConvergence) {
			continue
		}
		if err != nil {
			return empty, err
		}
		if err := out.Mesh.Validate(); err != nil {
			return empty, fmt.Errorf("mesh: rounded normal offset grid cannot be represented: %w", err)
		}
		if err := ctx.Err(); err != nil {
			return empty, err
		}
		out.GeometryWork = w.operations
		return out, nil
	}
	return empty, geom.ErrConvergence
}

func checkedNormalOffsetGridOptions(o NormalOffsetGridOptions) (NormalOffsetGridOptions, error) {
	if o.Deflection == 0 {
		o.Deflection = .05
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 8
	}
	if o.MaxTriangles == 0 {
		o.MaxTriangles = 131072
	}
	if o.MaxWork == 0 {
		o.MaxWork = 100_000_000
	}
	if o.MaxSourceOperations == 0 {
		o.MaxSourceOperations = 4096
	}
	if !finite(o.Deflection) || o.Deflection <= 0 || o.MinDepth < 0 || o.MinDepth > o.MaxDepth || o.MaxDepth < 0 || o.MaxDepth > 9 ||
		o.MaxTriangles < 1 || o.MaxTriangles > 524288 ||
		o.MaxWork < 1 || o.MaxWork > 1_000_000_000 ||
		o.MaxSourceOperations < 1 || o.MaxSourceOperations > 1_000_000_000 ||
		!finite(o.NormalAngle) || o.NormalAngle < 0 || o.NormalAngle >= math.Pi/2 {
		return o, geom.ErrInvalid
	}
	return o, nil
}

type normalOffsetGridPoint struct {
	world geom.Vec3
	exact [3]*big.Rat
}

type normalOffsetGridWork struct {
	options    NormalOffsetGridOptions
	operations int
	points     map[[2]float64]normalOffsetGridPoint
	facetLimit *big.Rat
}

func (w *normalOffsetGridWork) remaining() int { return w.options.MaxWork - w.operations }

func (w *normalOffsetGridWork) charge(reported int) {
	// The exact geometry counters can include the first rejected operation.
	w.operations += min(reported, w.remaining())
}

func (w *normalOffsetGridWork) vertex(ctx context.Context, p *geom.VerifiedNormalOffsetPatch,
	uv [2]float64) (normalOffsetGridPoint, error) {
	var empty normalOffsetGridPoint
	if out, ok := w.points[uv]; ok {
		return out, nil
	}
	source := w.options.MaxSourceOperations
	if w.remaining() <= source {
		w.operations = w.options.MaxWork
		return empty, geom.ErrConvergence
	}
	w.charge(source)
	b, err := p.EvaluateRational(ctx, new(big.Rat).SetFloat64(uv[0]), new(big.Rat).SetFloat64(uv[1]),
		geom.SurfacePatchEvaluationOptions{MaxOperations: source},
		geom.SurfaceNormalOffsetEvaluationOptions{MaxOperations: w.remaining()})
	w.charge(b.ExactOperations)
	if err != nil {
		return empty, err
	}
	out := normalOffsetGridPoint{}
	var world [3]float64
	for axis := range world {
		mid := new(big.Rat).Add(b.Lower[axis], b.Upper[axis])
		world[axis], _ = mid.Quo(mid, big.NewRat(2, 1)).Float64()
		out.exact[axis] = new(big.Rat).SetFloat64(world[axis])
		if out.exact[axis] == nil {
			return empty, geom.ErrInvalid
		}
	}
	out.world = geom.Vec3{X: world[0], Y: world[1], Z: world[2]}
	w.points[uv] = out
	return out, nil
}

func (w *normalOffsetGridWork) grid(ctx context.Context, p *geom.VerifiedNormalOffsetPatch,
	n int, distance *big.Rat) (CertifiedNormalOffsetGrid, error) {
	var empty CertifiedNormalOffsetGrid
	domain := p.Domain()
	u, err := normalOffsetGridStations(domain.UMin, domain.UMax, n)
	if err != nil {
		return empty, err
	}
	v, err := normalOffsetGridStations(domain.VMin, domain.VMax, n)
	if err != nil {
		return empty, err
	}
	out := CertifiedNormalOffsetGrid{UStations: u, VStations: v, DistanceUpperSquared: new(big.Rat)}
	if w.facetLimit != nil {
		out.SquaredFacetNormalTangent = new(big.Rat)
	}
	out.Mesh.Vertices = make([]geom.Vec3, (n+1)*(n+1))
	out.Mesh.Triangles = make([][3]int, 0, 2*n*n)
	projection := p.Projection()
	for i := 0; i < n; i++ {
		du := normalOffsetGridSpan(u[i], u[i+1])
		for j := 0; j < n; j++ {
			if err := ctx.Err(); err != nil {
				return empty, err
			}
			if w.remaining() == 0 {
				return empty, geom.ErrConvergence
			}
			rect := geom.ParameterRectangle{UMin: u[i], UMax: u[i+1], VMin: v[j], VMax: v[j+1]}
			bounds, err := p.BoundsRational(ctx, rect, geom.SurfacePatchOptions{MaxOperations: w.remaining()})
			w.charge(bounds.ExactOperations)
			if err != nil {
				return empty, err
			}
			if normalOffsetGridDiameterSquared(bounds, nil).Cmp(distance) > 0 {
				return empty, geom.ErrConvergence
			}
			dv := normalOffsetGridSpan(v[j], v[j+1])
			a, d := i*(n+1)+j, i*(n+1)+j+1
			b, c := a+n+1, d+n+1
			ids := [4]int{a, b, c, d}
			stations := [4][2]float64{{u[i], v[j]}, {u[i+1], v[j]}, {u[i+1], v[j+1]}, {u[i], v[j+1]}}
			var corners [4]normalOffsetGridPoint
			for k, parameter := range stations {
				pt, err := w.vertex(ctx, p, parameter)
				if err != nil {
					return empty, err
				}
				corners[k] = pt
				out.Mesh.Vertices[ids[k]] = pt.world
			}
			bound := normalOffsetGridDiameterSquared(bounds, corners[:])
			if bound.Cmp(distance) > 0 {
				return empty, geom.ErrConvergence
			}
			if bound.Cmp(out.DistanceUpperSquared) > 0 {
				out.DistanceUpperSquared = bound
			}
			var normals *normalOffsetFacetBox
			if w.facetLimit != nil {
				if w.remaining() == 0 {
					return empty, geom.ErrConvergence
				}
				box, err := p.NormalBoundsRational(ctx, rect, geom.SurfacePatchOptions{MaxOperations: w.remaining()})
				w.charge(box.ExactOperations)
				if err != nil {
					return empty, err
				}
				normals = newNormalOffsetFacetBox(box.Bounds)
			}
			diagonal, margin, facetBound := certifyNormalOffsetGridCell(corners, projection, du, dv, normals, w.facetLimit)
			if margin == nil {
				return empty, geom.ErrConvergence
			}
			if out.ProjectedMonotonicityLower == nil || margin.Cmp(out.ProjectedMonotonicityLower) < 0 {
				out.ProjectedMonotonicityLower = margin
			}
			if facetBound != nil && facetBound.Cmp(out.SquaredFacetNormalTangent) > 0 {
				out.SquaredFacetNormalTangent = facetBound
			}
			for _, tri := range normalOffsetGridCellTriangles(diagonal) {
				out.Mesh.Triangles = append(out.Mesh.Triangles, [3]int{ids[tri[0]], ids[tri[1]], ids[tri[2]]})
			}
		}
	}
	return out, nil
}

func normalOffsetGridStations(low, high float64, n int) ([]float64, error) {
	// Take rational subdivisions before rounding. The resulting binary
	// stations are the exact chart coordinates used in all subsequent proofs.
	start := new(big.Rat).SetFloat64(low)
	step := normalOffsetGridSpan(low, high)
	step.Quo(step, big.NewRat(int64(n), 1))
	out := make([]float64, n+1)
	x := new(big.Rat).Set(start)
	for i := range out {
		out[i], _ = x.Float64()
		if i > 0 && out[i-1] >= out[i] {
			return nil, geom.ErrConvergence
		}
		x.Add(x, step)
	}
	return out, nil
}

func normalOffsetGridSpan(low, high float64) *big.Rat {
	return new(big.Rat).Sub(new(big.Rat).SetFloat64(high), new(big.Rat).SetFloat64(low))
}

func normalOffsetGridDiameterSquared(b geom.RationalSurfaceOffsetPoint, corners []normalOffsetGridPoint) *big.Rat {
	squared := new(big.Rat)
	for axis := range b.Lower {
		lower, upper := b.Lower[axis], b.Upper[axis]
		for _, corner := range corners {
			q := corner.exact[axis]
			if q.Cmp(lower) < 0 {
				lower = q
			}
			if q.Cmp(upper) > 0 {
				upper = q
			}
		}
		width := new(big.Rat).Sub(upper, lower)
		squared.Add(squared, width.Mul(width, width))
	}
	return squared
}

func normalOffsetGridCellTriangles(diagonal int) [2][3]int {
	if diagonal == 0 {
		return [2][3]int{{0, 1, 2}, {0, 2, 3}}
	}
	return [2][3]int{{0, 1, 3}, {1, 2, 3}}
}

func certifyNormalOffsetGridCell(corners [4]normalOffsetGridPoint, projection geom.NormalOffsetProjection, du, dv *big.Rat,
	normals *normalOffsetFacetBox, limit *big.Rat) (diagonal int, margin, facetBound *big.Rat) {
	difference := func(from, to int, scale *big.Rat) (out [2]*big.Rat) {
		for axis, coordinate := range projection.Axes {
			q := new(big.Rat).Sub(corners[to].exact[coordinate], corners[from].exact[coordinate])
			if projection.Reversed[axis] {
				q.Neg(q)
			}
			out[axis] = q.Mul(q, scale)
		}
		return out
	}
	// Scale every Jacobian by the positive cell area. In either diagonal
	// choice each derivative uses the corresponding rectangle side.
	bottom, top := difference(0, 1, dv), difference(3, 2, dv)
	left, right := difference(0, 3, du), difference(1, 2, du)
	scale := new(big.Rat).Mul(du, dv)
	for i, columns := range [2][2][2][2]*big.Rat{
		{{bottom, right}, {top, left}},
		{{bottom, left}, {top, right}},
	} {
		a := normalOffsetGridJacobianMargin(columns[0][0], columns[0][1], scale)
		b := normalOffsetGridJacobianMargin(columns[1][0], columns[1][1], scale)
		if a != nil && b != nil {
			if b.Cmp(a) < 0 {
				a = b
			}
			var bound *big.Rat
			if normals != nil {
				bound = normals.certifyCell(corners, normalOffsetGridCellTriangles(i), limit)
				if bound == nil {
					continue
				}
			}
			return i, a, bound
		}
	}
	return 0, nil, nil
}

func normalOffsetGridJacobianMargin(u, v [2]*big.Rat, scale *big.Rat) *big.Rat {
	if u[0].Sign() <= 0 || v[1].Sign() <= 0 {
		return nil
	}
	off := new(big.Rat).Add(u[1], v[0])
	det := new(big.Rat).Mul(u[0], v[1])
	four := big.NewRat(4, 1)
	det.Mul(det, four).Sub(det, off.Mul(off, off))
	if det.Sign() <= 0 {
		return nil
	}
	// For a positive symmetric 2x2 matrix, det/trace is at most its
	// smaller eigenvalue. Undo the common positive scale in that ratio.
	trace := new(big.Rat).Add(u[0], v[1])
	trace.Mul(trace, scale).Mul(trace, four)
	return det.Quo(det, trace)
}
