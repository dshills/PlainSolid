package offset

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"

	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// VerifiedSweepMeshOptions bounds the display of a single verified normal
// sweep. Zero selects .05 units of deflection, eight dyadic refinement
// levels, 100000 triangles, 200 million geometry work units and the
// default exact collision budget.
//
// Every new corner reserves MaxSourceOperations (default 4096) for its
// original source-jet evaluation. The geometry limit counts that reservation
// and the actual reported offset and cell work, including rejected grids.
type VerifiedSweepMeshOptions struct {
	Deflection                   float64
	MaxDepth, MaxTriangles       int
	MaxWork, MaxSourceOperations int
	MaxCollisionPairs            int
	// NormalAngle optionally limits facet deviation in radians from the
	// continuous outward normal over the whole source cell.
	NormalAngle float64 `json:",omitempty"`
}

// CertifiedVerifiedSweepMesh is an outward, embedded, closed boundary of
// one original verified sweep. VertexStations indexes the three shared
// station arrays in (U,V,displacement) order. U and V are exact represented
// binary parameters; displacement stations remain exact rationals.
// OriginalFace gives the analytic source side of each triangle.
type CertifiedVerifiedSweepMesh struct {
	Mesh                 mesh.Mesh
	UStations, VStations []float64
	DisplacementStations []*big.Rat
	VertexStations       [][3]int
	OriginalFace         []geom.NormalOffsetSweepSide
	DistanceUpperSquared *big.Rat
	SignedDisplayVolume  *big.Rat
	GeometryWork         int
	Collision            collision.Report
	// SquaredFacetNormalTangent is nil without the angular option.
	SquaredFacetNormalTangent *big.Rat `json:",omitempty"`
}

// TessellateVerifiedNormalOffsetSweep bounds both directed distances between
// the analytic boundary and the display using enclosing boxes shared by
// each continuous source cell and its rounded facets. Exact indexed
// collision and shell validation certify one positive boundary. Failure
// is atomic. A positive NormalAngle also checks the actual represented
// facet directions against continuous outward normals on all analytic sides.
func TessellateVerifiedNormalOffsetSweep(ctx context.Context, sweep *geom.VerifiedNormalOffsetSweep,
	o VerifiedSweepMeshOptions) (CertifiedVerifiedSweepMesh, error) {
	var empty CertifiedVerifiedSweepMesh
	if ctx == nil || sweep == nil {
		return empty, geom.ErrInvalid
	}
	domain, displacement := sweep.Domain()
	if !domain.Valid() || displacement.Lower == nil || displacement.Upper == nil ||
		displacement.Lower.Cmp(displacement.Upper) >= 0 {
		return empty, geom.ErrInvalid
	}
	o, err := checkedVerifiedSweepMeshOptions(o)
	if err != nil {
		return empty, err
	}
	limit := new(big.Rat).SetFloat64(o.Deflection)
	halfLimit := new(big.Rat).Quo(limit, big.NewRat(2, 1))
	limit.Mul(limit, limit)
	span := new(big.Rat).Sub(displacement.Upper, displacement.Lower)
	var faces [6]geom.VerifiedNormalOffsetSweepFace
	for i := range faces {
		face, err := sweep.Boundary(geom.NormalOffsetSweepSide(i))
		if err != nil {
			return empty, err
		}
		faces[i] = face
	}
	// In the displacement direction the original derivative has unit
	// length. Stop refining that direction once its trace is at most
	// half the requested physical distance. Every wall cell still has
	// to pass the complete rational box check.
	maxT := 1
	for maxT < 1<<o.MaxDepth && span.Cmp(halfLimit) > 0 {
		maxT *= 2
		span.Quo(span, big.NewRat(2, 1))
	}
	work := verifiedSweepMeshWork{options: o}
	for depth := 0; depth <= o.MaxDepth; depth++ {
		if err := ctx.Err(); err != nil {
			return empty, err
		}
		n := 1 << depth
		nT := min(n, maxT)
		if o.NormalAngle > 0 {
			// A wall's normal can change with displacement even
			// though its trace length is already below deflection.
			nT = n
		}
		triangles := 4*n*n + 8*n*nT
		if triangles > o.MaxTriangles || work.remaining() == 0 {
			return empty, ErrLimit
		}
		grid, err := makeVerifiedSweepGrid(domain, displacement, n, nT)
		if err != nil {
			return empty, err
		}
		out, err := work.sample(ctx, sweep, grid, faces, limit, triangles)
		if errors.Is(err, geom.ErrConvergence) {
			continue
		}
		if err != nil {
			return empty, err
		}
		locator, err := collision.NewMeshPointLocator(ctx, out.Mesh, nil, collision.LocationOptions{
			MaxTriangles: o.MaxTriangles, MaxCollisionPairs: o.MaxCollisionPairs,
		})
		if err != nil {
			return empty, fmt.Errorf("offset: verified sweep mesh embedding: %w", err)
		}
		shells := locator.Shells()
		if len(shells) != 1 || shells[0].Parent != -1 || shells[0].Depth != 0 ||
			len(shells[0].Triangles) != triangles || shells[0].SignedVolume.Sign() <= 0 {
			return empty, geom.ErrConvergence
		}
		out.Collision = locator.BoundaryCollision()
		out.SignedDisplayVolume = shells[0].SignedVolume
		out.GeometryWork = work.operations
		if err := ctx.Err(); err != nil {
			return empty, err
		}
		return out, nil
	}
	return empty, geom.ErrConvergence
}

func checkedVerifiedSweepMeshOptions(o VerifiedSweepMeshOptions) (VerifiedSweepMeshOptions, error) {
	if o.Deflection == 0 {
		o.Deflection = .05
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 8
	}
	if o.MaxTriangles == 0 {
		o.MaxTriangles = collision.MaxTriangles
	}
	if o.MaxWork == 0 {
		o.MaxWork = 200_000_000
	}
	if o.MaxSourceOperations == 0 {
		o.MaxSourceOperations = 4096
	}
	if o.MaxCollisionPairs == 0 {
		o.MaxCollisionPairs = collision.DefaultMaxPairs
	}
	if !finite(o.Deflection) || o.Deflection <= 0 ||
		o.MaxDepth < 1 || o.MaxDepth > 9 || o.MaxTriangles < 1 || o.MaxTriangles > collision.MaxTriangles ||
		o.MaxWork < 1 || o.MaxWork > 1_000_000_000 || o.MaxSourceOperations < 1 ||
		o.MaxSourceOperations > 1_000_000_000 || o.MaxCollisionPairs < 1 ||
		!finite(o.NormalAngle) || o.NormalAngle < 0 || o.NormalAngle >= math.Pi/2 {
		return o, geom.ErrInvalid
	}
	return o, nil
}

type verifiedSweepGrid struct {
	uv           [2][]float64
	stations     [3][]*big.Rat
	segments     [3]int
	interiorSide [2][2]float64
}

func makeVerifiedSweepGrid(d geom.ParameterRectangle, t geom.ExactParameterInterval, n, nT int) (g verifiedSweepGrid, err error) {
	g.segments = [3]int{n, n, nT}
	for i, interval := range [2][2]float64{{d.UMin, d.UMax}, {d.VMin, d.VMax}} {
		lo, hi := sweepRat(interval[0]), sweepRat(interval[1])
		original := sweepRationalStations(lo, hi, n)
		g.uv[i] = make([]float64, n+1)
		g.stations[i] = make([]*big.Rat, n+1)
		for j, q := range original {
			value, _ := q.Float64()
			if j > 0 && value <= g.uv[i][j-1] {
				return g, ErrPrecision
			}
			g.uv[i][j], g.stations[i][j] = value, sweepRat(value)
		}
		// The wall enclosure uses only a thin collar on the inside of
		// the fixed coordinate. Shrink it along with the UV grid.
		width := new(big.Rat).Sub(hi, lo)
		width.Quo(width, big.NewRat(int64(32*n), 1))
		left, _ := new(big.Rat).Add(lo, width).Float64()
		right, _ := new(big.Rat).Sub(hi, width).Float64()
		if left <= interval[0] {
			left = math.Nextafter(interval[0], interval[1])
		}
		if right >= interval[1] {
			right = math.Nextafter(interval[1], interval[0])
		}
		g.interiorSide[i] = [2]float64{left, right}
	}
	g.stations[2] = sweepRationalStations(t.Lower, t.Upper, nT)
	return g, nil
}

func sweepRationalStations(lo, hi *big.Rat, n int) []*big.Rat {
	step := new(big.Rat).Sub(hi, lo)
	step.Quo(step, big.NewRat(int64(n), 1))
	out := make([]*big.Rat, n+1)
	out[0] = new(big.Rat).Set(lo)
	for i := 1; i <= n; i++ {
		out[i] = new(big.Rat).Add(out[i-1], step)
	}
	return out
}

func sweepRat(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }

type verifiedSweepMeshWork struct {
	options    VerifiedSweepMeshOptions
	operations int
}

func (w *verifiedSweepMeshWork) remaining() int { return w.options.MaxWork - w.operations }

func (w *verifiedSweepMeshWork) charge(work int) { w.operations += min(work, w.remaining()) }

func (w *verifiedSweepMeshWork) sample(ctx context.Context, sweep *geom.VerifiedNormalOffsetSweep, g verifiedSweepGrid,
	faces [6]geom.VerifiedNormalOffsetSweepFace, limit *big.Rat, triangles int) (CertifiedVerifiedSweepMesh, error) {
	var empty CertifiedVerifiedSweepMesh
	out := CertifiedVerifiedSweepMesh{
		UStations: g.uv[0], VStations: g.uv[1], DisplacementStations: g.stations[2],
		DistanceUpperSquared: new(big.Rat),
	}
	if w.options.NormalAngle > 0 {
		out.SquaredFacetNormalTangent = new(big.Rat)
	}
	vertexIDs := make(map[[3]int]int, triangles/2+2)
	represented := make([][3]*big.Rat, 0, triangles/2+2)
	out.Mesh.Triangles = make([][3]int, 0, triangles)
	out.Mesh.Vertices = make([]geom.Vec3, 0, triangles/2+2)
	out.VertexStations = make([][3]int, 0, triangles/2+2)
	out.OriginalFace = make([]geom.NormalOffsetSweepSide, 0, triangles)
	for side, face := range faces {
		reverse := face.ReverseWinding()
		fixed := side / 2
		free := [3][2]int{{1, 2}, {0, 2}, {0, 1}}[fixed]
		for i := 0; i < g.segments[free[0]]; i++ {
			for j := 0; j < g.segments[free[1]]; j++ {
				if err := ctx.Err(); err != nil {
					return empty, err
				}
				if w.remaining() == 0 {
					return empty, ErrLimit
				}
				lo := [3]int{}
				if side%2 != 0 {
					lo[fixed] = g.segments[fixed]
				}
				lo[free[0]], lo[free[1]] = i, j
				hi := lo
				hi[free[0]]++
				hi[free[1]]++
				rect := g.cellRectangle(lo, hi, side)
				b, err := sweep.BoundsRational(ctx, rect, g.stations[2][lo[2]], g.stations[2][hi[2]],
					geom.SurfacePatchOptions{MaxOperations: w.remaining()})
				w.charge(b.ExactOperations)
				if err != nil {
					return empty, err
				}
				if verifiedSweepBoxDiameterSquared(b, nil).Cmp(limit) > 0 {
					return empty, geom.ErrConvergence
				}
				corners := [4][3]int{lo, lo, hi, lo}
				corners[1][free[0]]++
				corners[3][free[1]]++
				var ids [4]int
				var points [4][3]*big.Rat
				for k, station := range corners {
					id, found := vertexIDs[station]
					if !found {
						q, exact, err := w.vertex(ctx, sweep, g, station)
						if err != nil {
							return empty, err
						}
						id = len(out.Mesh.Vertices)
						vertexIDs[station] = id
						out.VertexStations = append(out.VertexStations, station)
						out.Mesh.Vertices = append(out.Mesh.Vertices, q)
						represented = append(represented, exact)
					}
					ids[k], points[k] = id, represented[id]
				}
				bound := verifiedSweepBoxDiameterSquared(b, points[:])
				if bound.Cmp(limit) > 0 {
					return empty, geom.ErrConvergence
				}
				if bound.Cmp(out.DistanceUpperSquared) > 0 {
					out.DistanceUpperSquared = bound
				}
				triangles := verifiedSweepCellTriangles(ids, reverse, false)
				if w.options.NormalAngle > 0 {
					if w.remaining() == 0 {
						return empty, ErrLimit
					}
					box, err := face.NormalBoundsRational(ctx, rect, g.stations[2][lo[2]], g.stations[2][hi[2]],
						geom.SurfacePatchOptions{MaxOperations: w.remaining()})
					w.charge(box.ExactOperations)
					if err != nil {
						return empty, err
					}
					angle, err := verifiedSweepFacetBound(box.Bounds, triangles, out.Mesh.Vertices, w.options.NormalAngle)
					if errors.Is(err, geom.ErrConvergence) {
						triangles = verifiedSweepCellTriangles(ids, reverse, true)
						angle, err = verifiedSweepFacetBound(box.Bounds, triangles, out.Mesh.Vertices, w.options.NormalAngle)
					}
					if err != nil {
						return empty, err
					}
					if angle.Cmp(out.SquaredFacetNormalTangent) > 0 {
						out.SquaredFacetNormalTangent = angle
					}
				}
				for _, tri := range triangles {
					out.Mesh.Triangles = append(out.Mesh.Triangles, tri)
					out.OriginalFace = append(out.OriginalFace, geom.NormalOffsetSweepSide(side))
				}
			}
		}
	}
	if len(out.Mesh.Vertices) != triangles/2+2 || len(out.Mesh.Triangles) != triangles {
		return empty, geom.ErrConvergence
	}
	return out, nil
}

func verifiedSweepCellTriangles(ids [4]int, reverse, alternate bool) [2][3]int {
	triangles := [2][3]int{{ids[0], ids[1], ids[2]}, {ids[0], ids[2], ids[3]}}
	if alternate {
		triangles = [2][3]int{{ids[0], ids[1], ids[3]}, {ids[1], ids[2], ids[3]}}
	}
	if reverse {
		for i := range triangles {
			triangles[i][1], triangles[i][2] = triangles[i][2], triangles[i][1]
		}
	}
	return triangles
}

func verifiedSweepFacetBound(box geom.Box, triangles [2][3]int, vertices []geom.Vec3, angle float64) (*big.Rat, error) {
	var facets [2][3]geom.Vec3
	for i, tri := range triangles {
		for j, id := range tri {
			facets[i][j] = vertices[id]
		}
	}
	return mesh.CertifyFacetNormalBox(box, facets[:], angle)
}

func (g verifiedSweepGrid) cellRectangle(lo, hi [3]int, side int) geom.ParameterRectangle {
	limits := [2][2]float64{{g.uv[0][lo[0]], g.uv[0][hi[0]]}, {g.uv[1][lo[1]], g.uv[1][hi[1]]}}
	if fixed := side / 2; fixed < 2 {
		limits[fixed][1-side%2] = g.interiorSide[fixed][side%2]
	}
	return geom.ParameterRectangle{UMin: limits[0][0], UMax: limits[0][1], VMin: limits[1][0], VMax: limits[1][1]}
}

func (w *verifiedSweepMeshWork) vertex(ctx context.Context, sweep *geom.VerifiedNormalOffsetSweep, g verifiedSweepGrid,
	station [3]int) (geom.Vec3, [3]*big.Rat, error) {
	var exact [3]*big.Rat
	source := w.options.MaxSourceOperations
	if w.remaining() <= source {
		return geom.Vec3{}, exact, ErrLimit
	}
	w.charge(source)
	p, err := sweep.EvaluateRational(ctx, g.stations[0][station[0]], g.stations[1][station[1]], g.stations[2][station[2]],
		geom.SurfacePatchEvaluationOptions{MaxOperations: source},
		geom.SurfaceNormalOffsetEvaluationOptions{MaxOperations: w.remaining()})
	w.charge(p.ExactOperations)
	if err != nil {
		return geom.Vec3{}, exact, err
	}
	var rounded [3]float64
	for axis := range rounded {
		mid := new(big.Rat).Add(p.Lower[axis], p.Upper[axis])
		rounded[axis], _ = mid.Quo(mid, big.NewRat(2, 1)).Float64()
		exact[axis] = sweepRat(rounded[axis])
		if exact[axis] == nil {
			return geom.Vec3{}, exact, ErrPrecision
		}
	}
	return geom.Vec3{X: rounded[0], Y: rounded[1], Z: rounded[2]}, exact, nil
}

func verifiedSweepBoxDiameterSquared(b geom.RationalSurfaceOffsetPoint, represented [][3]*big.Rat) *big.Rat {
	result := new(big.Rat)
	for axis := range b.Lower {
		lo, hi := b.Lower[axis], b.Upper[axis]
		for _, p := range represented {
			if p[axis].Cmp(lo) < 0 {
				lo = p[axis]
			}
			if p[axis].Cmp(hi) > 0 {
				hi = p[axis]
			}
		}
		width := new(big.Rat).Sub(hi, lo)
		result.Add(result, width.Mul(width, width))
	}
	return result
}
