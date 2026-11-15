package trimmed

import (
	"context"
	"fmt"
	"math/big"
	"slices"

	"plainsolid/brep/edgecheck"
	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// MappedChainClosedUse identifies a closing trim on original face 0 or 1, or
// an outer trim on a planar face starting at index 2. The mapped chain is
// paired implicitly between the two original spline owners.
type MappedChainClosedUse struct{ Face, Trim int }

// MappedChainPlanarFace supplies an original affine spline chart and its CCW
// outer ring of exact quadratic or Bézier trims. Reversed flips the face in
// the assembled shell.
type MappedChainPlanarFace struct {
	Surface  geom.Surface
	Outer    []PCurve
	Reversed bool
	Mesh     MixedExactAffineMeshOptions
}

// MappedChainClosedMeshOptions bounds the original per-face fills and the
// final assembly. Boundary supplies the common chain ledger. ClosingProof
// applies independently to each declared closing seam. MaxRounds defaults to
// two (maximum eight), with independent face limits per fill. MaxTriangles
// defaults to 100000 and is capped at collision.MaxTriangles. At most 2048
// sample occurrences across the shared seam partitions are accepted.
type MappedChainClosedMeshOptions struct {
	Boundary     MappedChainPolylineOptions
	Curved       [2]MixedExactMultiSpanMeshOptions
	ClosingProof edgecheck.Options
	MaxTriangles int
	MaxRounds    int
	Collision    collision.Options
}

// CertifiedMappedChainClosedMesh covers one connected, oriented manifold
// display shell with positive exact represented volume and certified global
// triangle contacts. SquaredDistance bounds both directed source/display
// distances. FaceVertices maps face-local IDs to the shell. ChainSamples
// records every shared chain vertex with both original authored parameters,
// including refinement beyond the initial Boundary ledger. A joint vertex
// retains the incoming original coedge parameters in ChainSamples and both
// original occurrences in ChainJoints. ClosingVertices follows the first declared use of each closing seam. This is a display
// certificate; ideal source embedding and Solid admission require separate proofs.
type CertifiedMappedChainClosedMesh struct {
	Mesh         mesh.Mesh
	OriginalFace []int
	FaceVertices [][]int
	ChainSamples []MappedChainFaceSample
	// ChainJoints contains incoming and outgoing samples at an original
	// coedge boundary with one shared display vertex.
	ChainJoints      [][2]MappedChainFaceSample
	ClosingVertices  [][]int
	Boundary         CertifiedMappedChainPolyline
	Curved           [2]CertifiedMixedExactCurvedMesh
	Planar           []CertifiedMixedExactAffineMesh
	SquaredDistance  *big.Rat
	DisplayVolume    *big.Rat
	DisplayCollision collision.Report
	RefinementRounds int
}

type mappedChainClosedFaceMesh struct {
	mesh     mesh.Mesh
	points   [][3]*big.Rat
	samples  []MixedExactTrimSamples
	reversed bool
}

// TessellateClosedCertified covers two original chain owners and up to 14
// affine faces. Curved owners have opposite display orientation after applying
// reversed. Every other coedge must occur in exactly one declared seam between
// distinct faces, with complete exact model-curve agreement in opposite shell
// directions. Different adaptive partitions are aligned by exact parameter
// union and both sides are recertified. The assembly shares vertices only by
// certified seam incidence and returns no partial shell on any failure.
func (r MappedSplineChainAgreement) TessellateClosedCertified(ctx context.Context, closing [2][]PCurve, reversed [2]bool, planar []MappedChainPlanarFace, seams [][2]MappedChainClosedUse, o MappedChainClosedMeshOptions) (CertifiedMappedChainClosedMesh, error) {
	var zero CertifiedMappedChainClosedMesh
	if ctx == nil || o.Collision.MaxPairs < 0 || o.Collision.MaxContacts < 0 {
		return zero, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	if o.MaxTriangles == 0 {
		o.MaxTriangles = 100000
	}
	if o.MaxRounds == 0 {
		o.MaxRounds = 2
	}
	if o.MaxTriangles < 1 || o.MaxTriangles > collision.MaxTriangles || o.MaxRounds < 1 || o.MaxRounds > 8 {
		return zero, geom.ErrInvalid
	}
	source, err := r.mappedChainClosingSource(closing, reversed, planar, seams)
	if err != nil {
		return zero, err
	}
	faceCount := len(source.trims)
	if o.MaxTriangles < faceCount {
		return zero, geom.ErrConvergence
	}
	for _, face := range o.Curved {
		if face.Face.Cuts != nil {
			return zero, geom.ErrInvalid
		}
	}
	if err := source.proveSeams(ctx, seams, o.ClosingProof); err != nil {
		return zero, err
	}
	faces := make([]mappedChainClosedFaceMesh, faceCount)
	for i, reversed := range source.reversed {
		faces[i].reversed = reversed
	}

	boundary, err := r.TessellateCertified(ctx, o.Boundary)
	if err != nil {
		return zero, err
	}
	out := CertifiedMappedChainClosedMesh{
		Boundary: boundary, Planar: make([]CertifiedMixedExactAffineMesh, len(planar)),
		SquaredDistance: new(big.Rat), FaceVertices: make([][]int, faceCount),
		ClosingVertices: make([][]int, len(seams)),
	}
	var curved [2]CertifiedMappedChainFace
	build := func(cuts [][][]*big.Rat) error {
		out.SquaredDistance.SetInt64(0)
		for owner := range curved {
			x, err := r.tessellateFaceWithBoundary(ctx, owner, closing[owner], boundary, cuts[owner], o.Curved[owner])
			if err != nil {
				return fmt.Errorf("chain owner %d: %w", owner, err)
			}
			curved[owner], out.Curved[owner] = x, x.Face
			faces[owner].mesh, faces[owner].points, faces[owner].samples = x.Face.Mesh, x.Face.ExactPoints, x.Face.UVRegion.Samples
			if bound := x.Face.FillSquaredDistance; bound.Cmp(out.SquaredDistance) > 0 {
				out.SquaredDistance.Set(bound)
			}
		}
		for i, p := range planar {
			f := i + 2
			patch, err := exactSplineSupportPatch(p.Surface, p.Mesh.Composition.SurfacePatchOptions)
			if err != nil {
				return fmt.Errorf("planar face %d: %w", f, err)
			}
			var fixed [][][]*big.Rat
			if cuts[f] != nil {
				fixed = [][][]*big.Rat{cuts[f]}
			}
			x, err := tessellateMixedExactAffineRegionCuts(ctx, patch, p.Outer, nil, fixed, p.Mesh)
			if err != nil {
				return fmt.Errorf("planar face %d: %w", f, err)
			}
			out.Planar[i] = x
			faces[f].mesh, faces[f].points, faces[f].samples = x.Mesh, x.ExactPoints, x.Samples
			if bound := x.FillSquaredDistance; bound.Cmp(out.SquaredDistance) > 0 {
				out.SquaredDistance.Set(bound)
			}
		}
		return nil
	}

	// Translate the original seam uses once into the face mesh's trim indexes.
	// Each original chain cell has one paired use in each roof chart.
	nCells := len(r.cuts) - 1
	pairs := mappedChainCellPairs(nCells, seams)
	cuts := make([][][]*big.Rat, faceCount)
	if err := build(cuts); err != nil {
		return zero, err
	}
	for {
		aligned, err := alignMappedChainClosedCuts(faces, pairs, cuts)
		if err != nil {
			return zero, err
		}
		if aligned {
			break
		}
		if out.RefinementRounds == o.MaxRounds {
			return zero, fmt.Errorf("mapped chain shell partitions: %w", geom.ErrConvergence)
		}
		if err := build(cuts); err != nil {
			return zero, err
		}
		out.RefinementRounds++
	}

	return stitchMappedChainClosed(ctx, out, faces, curved, pairs[nCells:], o)
}

func stitchMappedChainClosed(ctx context.Context, out CertifiedMappedChainClosedMesh, faces []mappedChainClosedFaceMesh, curved [2]CertifiedMappedChainFace, seams [][2]MappedChainClosedUse, o MappedChainClosedMeshOptions) (CertifiedMappedChainClosedMesh, error) {
	var zero CertifiedMappedChainClosedMesh
	offsets := make([]int, len(faces)+1)
	triangles := 0
	for i, f := range faces {
		if len(f.mesh.Triangles) > o.MaxTriangles-triangles {
			return zero, geom.ErrConvergence
		}
		triangles += len(f.mesh.Triangles)
		offsets[i+1] = offsets[i] + len(f.mesh.Vertices)
	}
	weld := newExactVertexWeld(offsets[len(faces)])
	share := func(a, idA, b, idB int) error {
		if faces[a].mesh.Vertices[idA] != faces[b].mesh.Vertices[idB] {
			return geom.ErrUnrepresentable
		}
		for k, c := range faces[a].points[idA] {
			if c.Cmp(faces[b].points[idB][k]) != 0 {
				return geom.ErrConvergence
			}
		}
		weld.join(offsets[a]+idA, offsets[b]+idB)
		return nil
	}

	a, b := curved[0].Samples, curved[1].Samples
	if len(a) != len(b) {
		return zero, fmt.Errorf("chain sample partition: %w", geom.ErrConvergence)
	}
	for i, s := range a {
		if s.ModelParameter.Cmp(b[i].ModelParameter) != 0 {
			return zero, fmt.Errorf("chain sample %d parameter: %w", i, geom.ErrConvergence)
		}
		if err := share(0, s.Vertex, 1, b[i].Vertex); err != nil {
			return zero, fmt.Errorf("chain sample %d point: %w", i, err)
		}
		if i == 0 || a[i-1].Vertex != s.Vertex {
			out.ChainSamples = append(out.ChainSamples, s)
		} else if prior := a[i-1]; prior.AuthoredParts != s.AuthoredParts && prior.ModelParameter.Cmp(s.ModelParameter) == 0 {
			out.ChainJoints = append(out.ChainJoints, [2]MappedChainFaceSample{prior, s})
		}
	}
	for i, seam := range seams {
		a, b := seam[0].Face, seam[1].Face
		sa, sb := faces[a].samples[seam[0].Trim], faces[b].samples[seam[1].Trim]
		for j, id := range sa.Vertices {
			k := j
			if faces[a].reversed == faces[b].reversed {
				k = len(sb.Vertices) - 1 - j
			}
			if err := share(a, id, b, sb.Vertices[k]); err != nil {
				return zero, fmt.Errorf("closing seam %d sample %d point: %w", i, j, err)
			}
			out.ClosingVertices[i] = append(out.ClosingVertices[i], id)
		}
	}

	global := make(map[int]int)
	positions := make(map[geom.Vec3]int)
	for i, f := range faces {
		if err := ctx.Err(); err != nil {
			return zero, err
		}
		ids := make([]int, len(f.mesh.Vertices))
		for j, p := range f.mesh.Vertices {
			representative := weld.root(offsets[i] + j)
			id, ok := global[representative]
			if !ok {
				if _, exists := positions[p]; exists {
					return zero, fmt.Errorf("unpaired display vertices coincide: %w", geom.ErrConvergence)
				}
				id = len(out.Mesh.Vertices)
				global[representative], positions[p] = id, id
				out.Mesh.Vertices = append(out.Mesh.Vertices, p)
			}
			ids[j] = id
		}
		out.FaceVertices[i] = ids
		for _, tri := range f.mesh.Triangles {
			if f.reversed {
				tri[1], tri[2] = tri[2], tri[1]
			}
			out.Mesh.Triangles = append(out.Mesh.Triangles, [3]int{ids[tri[0]], ids[tri[1]], ids[tri[2]]})
			out.OriginalFace = append(out.OriginalFace, i)
		}
	}
	for i, s := range out.ChainSamples {
		out.ChainSamples[i].Vertex = out.FaceVertices[0][s.Vertex]
	}
	for i, pair := range out.ChainJoints {
		for j, sample := range pair {
			out.ChainJoints[i][j].Vertex = out.FaceVertices[0][sample.Vertex]
		}
	}
	for i, ids := range out.ClosingVertices {
		for j, id := range ids {
			ids[j] = out.FaceVertices[seams[i][0].Face][id]
		}
	}
	volume, report, err := checkClosedPeriodicMesh(ctx, out.Mesh, o.Collision)
	if err != nil {
		return zero, fmt.Errorf("closed mapped chain display: %w", err)
	}
	out.DisplayVolume, out.DisplayCollision = volume, report
	return out, nil
}

// Copy both ordered sample sets to the first use's direction before taking the
// union. Each prescribed list is then restored to its original chart direction.
func alignMappedChainClosedCuts(faces []mappedChainClosedFaceMesh, pairs [][2]MappedChainClosedUse, cuts [][][]*big.Rat) (bool, error) {
	aligned, samples := true, 0
	one := big.NewRat(1, 1)
	for i, f := range faces {
		cuts[i] = make([][]*big.Rat, len(f.samples))
	}
	for _, pair := range pairs {
		a, b := pair[0], pair[1]
		x := faces[a.Face].samples[a.Trim].Parameters
		y := faces[b.Face].samples[b.Trim].Parameters
		reverse := faces[a.Face].reversed == faces[b.Face].reversed
		if reverse {
			turned := make([]*big.Rat, len(y))
			for i, t := range y {
				turned[len(y)-1-i] = new(big.Rat).Sub(one, t)
			}
			y = turned
		}
		if slices.EqualFunc(x, y, func(a, b *big.Rat) bool { return a.Cmp(b) == 0 }) {
			samples += len(x)
			cuts[a.Face][a.Trim] = x
			cuts[b.Face][b.Trim] = faces[b.Face].samples[b.Trim].Parameters
			continue
		}
		aligned = false
		union := make([]*big.Rat, 0, len(x)+len(y))
		union = append(union, x...)
		union = append(union, y...)
		slices.SortFunc(union, (*big.Rat).Cmp)
		union = slices.CompactFunc(union, func(a, b *big.Rat) bool { return a.Cmp(b) == 0 })
		samples += len(union)
		cuts[a.Face][a.Trim], cuts[b.Face][b.Trim] = union, union
		if reverse {
			turned := make([]*big.Rat, len(union))
			for i, t := range union {
				turned[len(union)-1-i] = new(big.Rat).Sub(one, t)
			}
			cuts[b.Face][b.Trim] = turned
		}
	}
	if samples > 2048 {
		return false, geom.ErrConvergence
	}
	return aligned, nil
}
