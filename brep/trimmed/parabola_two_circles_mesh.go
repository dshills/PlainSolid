package trimmed

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// ParabolaTwoCirclesMeshOptions controls the two-sided boundary distance,
// relative volume agreement, and the assembly's display work.
// MaxSubdivisions bounds the number of segments per original edge.
// Collision applies to each component, and MaxPairs also bounds the cross
// checks if represented component bounds overlap.
type ParabolaTwoCirclesMeshOptions struct {
	Deflection, RelativeVolumeTolerance float64
	MaxSubdivisions, MaxTriangles       int
	Collision                           collision.Options

	// NormalTurn optionally bounds the pairwise turn in radians of
	// original source normals over each display triangle's chart footprint,
	// enlarged by the boundary transport and rounding bounds.
	// It compares normals from one original face at a time. Zero selects
	// the distance and volume path without an angular certificate.
	NormalTurn float64

	// NormalAngle optionally bounds deviation in radians between each
	// delivered facet normal and the original normals on its own face.
	// A positive request is below pi/2 and enables shape-quality flips.
	NormalAngle float64
}

// ParabolaTwoCirclesMeshEdge records the original model edge and the exact
// original parameters of the shared displayed vertices, in model order.
type ParabolaTwoCirclesMeshEdge struct {
	Edge       int
	Vertices   []int
	Parameters []*big.Rat
}

// CertifiedParabolaTwoCirclesComponentMesh keeps component-local native
// vertex, edge, and triangle owner indices. DistanceUpper bounds the
// two-sided boundary distance. SourceVolume encloses the positive true
// volume. SignedDisplayVolume is exact for the delivered float vertices.
type CertifiedParabolaTwoCirclesComponentMesh struct {
	Mesh                 mesh.Mesh
	Region, Subdivisions int
	OriginalFace         []int
	OriginalVertices     []int
	OriginalEdges        []ParabolaTwoCirclesMeshEdge
	DistanceUpper        *big.Rat
	SourceVolume         [2]*big.Rat
	SignedDisplayVolume  *big.Rat
	Collision            collision.Report

	// SquaredSourceNormalTurn bounds the requested pairwise turn in
	// radians squared. Nil when NormalTurn is zero.
	SquaredSourceNormalTurn *big.Rat `json:",omitempty"`

	// SquaredFacetNormalTangent bounds tan(deviation)^2 over each facet's
	// padded source footprint. Nil when the facet option is zero.
	SquaredFacetNormalTangent *big.Rat `json:",omitempty"`
}

// TessellateCertified displays the sealed native Cut on its original
// single-span polynomial identity-XY roofs, of degree at most eight.
// Both owners use one exact polygon triangulation. The source deflection
// certificate includes boundary transport and whole-triangle interpolation,
// and the separately integrated source volume bounds include pi where
// needed. Global contact and vertex fan tests independently validate each
// display. Options default to .05 distance, .02 relative volume error, 64
// subdivisions, and 100000 triangles for all components.
func (r *ParabolaTwoCirclesGraphCut) TessellateCertified(ctx context.Context, o ParabolaTwoCirclesMeshOptions) ([]CertifiedParabolaTwoCirclesComponentMesh, error) {
	return r.tessellateCertified(ctx, o, nil)
}

func (r *ParabolaTwoCirclesGraphCut) tessellateCertified(ctx context.Context, o ParabolaTwoCirclesMeshOptions,
	world *PlacedParabolaTwoCirclesGraphCut) ([]CertifiedParabolaTwoCirclesComponentMesh, error) {
	if ctx == nil || r == nil || len(r.components) == 0 || len(r.components) != len(r.history) {
		return nil, geom.ErrInvalid
	}
	if o.Deflection == 0 {
		o.Deflection = .05
	}
	if o.RelativeVolumeTolerance == 0 {
		o.RelativeVolumeTolerance = .02
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 64
	}
	if o.MaxTriangles == 0 {
		o.MaxTriangles = 100000
	}
	if !finite(o.Deflection) || o.Deflection <= 0 || !finite(o.RelativeVolumeTolerance) ||
		o.RelativeVolumeTolerance <= 0 || o.RelativeVolumeTolerance >= 1 ||
		o.MaxSubdivisions < 1 || o.MaxSubdivisions > 128 || o.MaxTriangles < 1 || o.MaxTriangles > collision.MaxTriangles ||
		o.Collision.MaxPairs < 0 || o.Collision.MaxContacts < 0 ||
		!finite(o.NormalTurn) || o.NormalTurn < 0 || o.NormalTurn > math.Pi ||
		!finite(o.NormalAngle) || o.NormalAngle < 0 || o.NormalAngle >= math.Pi/2 {
		return nil, geom.ErrInvalid
	}
	var height [2]parabolaCircleMeshHeight
	var patches [2]*geom.SurfacePatch
	for i := range height {
		var err error
		height[i], patches[i], err = parabolaCircleDisplayHeight(ctx, r.sources[i+1])
		if err != nil {
			return nil, err
		}
		if o.NormalTurn > 0 || o.NormalAngle > 0 {
			height[i].normalHessian = parabolaCircleMeshHessian(r.sources[i+1])
		}
	}
	source, err := geom.DiscoverSplineGraphParabolaTwoCircles(ctx, patches[0], patches[1], r.axis, geom.GraphPointContactOptions{})
	if err != nil {
		return nil, err
	}
	for i, point := range source.ParabolaControls {
		if point[r.axis].Cmp(big.NewRat(int64(i), 2)) != 0 {
			return nil, geom.ErrConvergence
		}
	}
	volume := parabolaCircleMeshSourceVolumes(height, source)

	// This auxiliary plane is only the exact UV polygon certificate, never
	// a replacement for either original support or any native edge.
	var controls [][4]*big.Rat
	for u := range 2 {
		for v := range 2 {
			controls = append(controls, [4]*big.Rat{big.NewRat(int64(u), 1), big.NewRat(int64(v), 1), new(big.Rat), big.NewRat(1, 1)})
		}
	}
	plane, err := geom.NewExactHomogeneousBezierSurface(ctx, 1, 1, controls, geom.SurfacePatchOptions{})
	if err != nil {
		return nil, err
	}
	maximumTriangles := o.MaxTriangles
	result := make([]CertifiedParabolaTwoCirclesComponentMesh, len(r.components))
	meshes := make([]mesh.Mesh, len(result))
	for i, part := range r.components {
		h := r.history[i]
		if h.Region < 0 || h.Region >= len(volume) || volume[h.Region][0].Sign() <= 0 || source.RegionSides[h.Region] <= 0 {
			return nil, geom.ErrConvergence
		}
		if o.MaxTriangles < len(part.faces) {
			return nil, parametric.ErrBudget
		}
		var placement *parabolaCircleDisplayPlacement
		if world != nil {
			placement = &parabolaCircleDisplayPlacement{chart: world.chart, part: world.world[i]}
		}
		result[i], err = parabolaTwoCirclesComponentDisplay(ctx, part, h, height, plane, volume[h.Region], o, placement)
		if err != nil {
			return nil, fmt.Errorf("trimmed: parabola and circles display region %d: %w", h.Region, err)
		}
		meshes[i] = result[i].Mesh
		o.MaxTriangles -= len(meshes[i].Triangles)
	}
	if world != nil && !world.chart.signedAxisUnit {
		separated, err := world.chart.separatesDisplayedComponents(ctx, meshes)
		if err != nil {
			return nil, err
		}
		if separated {
			return result, nil
		}
	}
	if err := proveParabolaCircleDisplaySeparation(ctx, meshes, maximumTriangles, o.Collision.MaxPairs); err != nil {
		return nil, fmt.Errorf("trimmed: parabola and circles display separation: %w", err)
	}
	return result, nil
}

func parabolaTwoCirclesComponentDisplay(ctx context.Context, s Solid, h ParabolaCircleCutComponentHistory,
	height [2]parabolaCircleMeshHeight, plane *geom.SurfacePatch, volume [2]*big.Rat,
	o ParabolaTwoCirclesMeshOptions, placement *parabolaCircleDisplayPlacement) (CertifiedParabolaTwoCirclesComponentMesh, error) {
	var empty CertifiedParabolaTwoCirclesComponentMesh
	roof, below := s.faces[h.RoofFaces[0]], s.faces[h.RoofFaces[1]]
	topUses := append([][]Use{roof.Outer}, roof.Holes...)
	bottomUses := append([][]Use{below.Outer}, below.Holes...)
	if len(topUses) != len(bottomUses) || roof.Reversed || !below.Reversed {
		return empty, geom.ErrConvergence
	}
	curves := make([][]PCurve, len(topUses))
	for i, loop := range topUses {
		if len(loop) != len(bottomUses[i]) {
			return empty, geom.ErrConvergence
		}
		for j, use := range loop {
			a, err := mixedStoredUseCurveContext(ctx, use, geom.SurfacePatchOptions{})
			if err != nil {
				return empty, err
			}
			b, err := mixedStoredUseCurveContext(ctx, bottomUses[i][j], geom.SurfacePatchOptions{})
			if err != nil {
				return empty, err
			}
			x, err := mixedExactControls(a)
			if err != nil {
				return empty, err
			}
			y, err := mixedExactControls(b)
			if err != nil || len(x) != len(y) {
				return empty, geom.ErrConvergence
			}
			for k := range x {
				for axis, q := range x[k] {
					if q.Cmp(y[k][axis]) != 0 {
						return empty, geom.ErrConvergence
					}
				}
			}
			curves[i] = append(curves[i], a)
		}
	}
	physicalDeflection := ratInset(o.Deflection)
	if placement.scaled() {
		var err error
		o.Deflection, err = placement.chart.localDistance(o.Deflection)
		if err != nil {
			return empty, err
		}
	}
	tolerance := ratInset(o.RelativeVolumeTolerance)
	minimum := new(big.Rat).Mul(volume[1], new(big.Rat).Sub(big.NewRat(1, 1), tolerance))
	maximum := new(big.Rat).Mul(volume[0], new(big.Rat).Add(big.NewRat(1, 1), tolerance))
	var turnLimit *big.Rat
	if o.NormalTurn > 0 {
		turnLimit = ratInset(o.NormalTurn)
		turnLimit.Mul(turnLimit, turnLimit)
	}
	for n := 1; n <= o.MaxSubdivisions; n *= 2 {
		cuts := make([]*big.Rat, n+1)
		for i := range cuts {
			cuts[i] = big.NewRat(int64(i), int64(n))
		}
		fixed := make([][][]*big.Rat, len(curves))
		for i, loop := range curves {
			fixed[i] = make([][]*big.Rat, len(loop))
			for j := range loop {
				fixed[i][j] = cuts
			}
		}
		foot, err := tessellateMixedExactAffineRegionCuts(ctx, plane, curves[0], curves[1:], fixed, MixedExactAffineMeshOptions{
			MaxBoundaryVertices: 2048, MaxTriangulationWork: 10000000, MaxTriangles: o.MaxTriangles,
			Topology: trimvalidate.Options{MaxEdges: 2048, MaxWork: 10000000},
			Display:  geom.ComposedCollectionPolylineOptions{ComposedCurvePolylineOptions: geom.ComposedCurvePolylineOptions{Deflection: o.Deflection / 8}},
		})
		if err != nil {
			if errors.Is(err, geom.ErrConvergence) {
				continue
			}
			return empty, err
		}
		display, err := parabolaTwoCirclesLiftFootprint(ctx, s, h, height, foot, topUses, bottomUses, n, o, placement)
		if err != nil {
			if errors.Is(err, errRefine) {
				continue
			}
			return empty, err
		}
		if display.DistanceUpper.Cmp(ratInset(o.Deflection)) > 0 ||
			display.SignedDisplayVolume.Cmp(minimum) < 0 || display.SignedDisplayVolume.Cmp(maximum) > 0 {
			continue
		}
		if turnLimit != nil && (display.SquaredSourceNormalTurn == nil || display.SquaredSourceNormalTurn.Cmp(turnLimit) > 0) {
			continue
		}
		if o.NormalAngle > 0 && display.SquaredFacetNormalTangent == nil {
			continue
		}
		native := s
		if placement != nil {
			native = placement.part
		}
		if err := auditParabolaTwoCirclesMesh(ctx, native, display); err != nil {
			return empty, err
		}
		actual, report, err := checkClosedPeriodicMesh(ctx, display.Mesh, o.Collision)
		if err != nil {
			return empty, err
		}
		if placement.scaled() {
			scale := placement.chart.scale
			display.DistanceUpper.Mul(display.DistanceUpper, scale)
			if display.DistanceUpper.Cmp(physicalDeflection) > 0 {
				continue
			}
			factor := placement.chart.volume
			localVolume := new(big.Rat).Quo(actual, factor)
			if localVolume.Cmp(minimum) < 0 || localVolume.Cmp(maximum) > 0 {
				continue
			}
			display.SignedDisplayVolume = actual
			volume = [2]*big.Rat{new(big.Rat).Mul(volume[0], factor), new(big.Rat).Mul(volume[1], factor)}
		} else if actual.Cmp(display.SignedDisplayVolume) != 0 {
			return empty, geom.ErrConvergence
		}
		display.Collision, display.SourceVolume, display.Subdivisions = report, volume, n
		return display, nil
	}
	return empty, parametric.ErrBudget
}

func proveParabolaCircleDisplaySeparation(ctx context.Context, meshes []mesh.Mesh, maxTriangles, maxPairs int) error {
	boxes := make([]geom.Box, len(meshes))
	for i, m := range meshes {
		if err := ctx.Err(); err != nil {
			return err
		}
		b, err := m.Bounds()
		if err != nil {
			return err
		}
		boxes[i] = b
	}
	for i, a := range boxes {
		for _, b := range boxes[:i] {
			// Strict separation of the represented coordinate bounds
			// excludes both boundary contact and hidden containment.
			if a.Max.X < b.Min.X || b.Max.X < a.Min.X || a.Max.Y < b.Min.Y ||
				b.Max.Y < a.Min.Y || a.Max.Z < b.Min.Z || b.Max.Z < a.Min.Z {
				continue
			}
			_, err := collision.ValidateMeshVertexContactAssembly(ctx, meshes, nil, collision.VertexContactAssemblyOptions{
				MaxCrossPairs: maxPairs, Validation: collision.LocationOptions{MaxTriangles: maxTriangles},
			})
			return err
		}
	}
	return nil
}
