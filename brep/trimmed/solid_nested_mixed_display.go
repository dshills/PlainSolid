//go:build research

package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/placement"
	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// NestedMixedDisplayOptions retains the existing exact native nesting proof.
// Meshing.Faces uses original global face IDs; other meshing limits apply per
// shell, bounded by MaxShells (default 8, max 16). Combined vertices/triangles are
// separately capped (default 100000); original topology is capped at 128 faces
// and 2048 edges/vertices; MaxFacePairs defaults 4096, max 16384 and is
// cumulative across all original inter-shell face pairs. Validation separately
// bounds the final global represented contact/nesting proof.
type NestedMixedDisplayOptions struct {
	Source                                             SplineShellBooleanOptions
	Meshing                                            StoredMixedClosedMeshOptions
	Validation                                         collision.LocationOptions
	MaxShells, MaxVertices, MaxTriangles, MaxFacePairs int
}

// NativeDisplayNesting identifies a source shell's parent, depth and face IDs.
type NativeDisplayNesting struct {
	Parent, Depth int
	Faces         []int
}

// NativeDisplayClearance is a conservative squared SOURCE boundary separation
// from original full-support boxes, strictly exceeding the combined display
// error allowance 2*(squaredErrorA+squaredErrorB).
type NativeDisplayClearance struct {
	ShellA, ShellB, FacePairs           int
	SquaredGapLower, RequiredSquaredGap *big.Rat
}

// PreparedNestedMixedDisplay authenticates disconnected boundary shells of ONE
// oriented native material, including inward cavities and islands. This first
// slice accepts rectangular spline components only, and rejects any unresolved
// full-support-box clearance. Placement (including repeated placement) also
// requires four times the aggregate squared error below every original squared
// gap, so added world rounding cannot consume the source separation.
// It is not a Boolean or a new source construction.
type PreparedNestedMixedDisplay struct {
	prepared  *PreparedStoredMixedDisplay
	nesting   []NativeDisplayNesting
	clearance []NativeDisplayClearance
}

func (p *PreparedNestedMixedDisplay) Source() Solid {
	if p == nil {
		return Solid{}
	}
	return p.prepared.Source()
}
func (p *PreparedNestedMixedDisplay) Ancestry() StoredDisplayAncestry {
	if p == nil {
		return StoredDisplayAncestry{}
	}
	return p.prepared.Ancestry()
}
func (p *PreparedNestedMixedDisplay) SquaredDistance() *big.Rat {
	if p == nil {
		return nil
	}
	return p.prepared.SourceSquaredDistance()
}
func (p *PreparedNestedMixedDisplay) Nesting() []NativeDisplayNesting {
	if p == nil {
		return nil
	}
	out := append([]NativeDisplayNesting(nil), p.nesting...)
	for i := range out {
		out[i].Faces = append([]int(nil), out[i].Faces...)
	}
	return out
}
func (p *PreparedNestedMixedDisplay) Clearances() []NativeDisplayClearance {
	if p == nil {
		return nil
	}
	out := append([]NativeDisplayClearance(nil), p.clearance...)
	for i := range out {
		out[i].SquaredGapLower = new(big.Rat).Set(out[i].SquaredGapLower)
		out[i].RequiredSquaredGap = new(big.Rat).Set(out[i].RequiredSquaredGap)
	}
	return out
}
func (p *PreparedNestedMixedDisplay) Place(ctx context.Context, pose placement.RationalRigid, o placement.ClosedDisplayOptions) (*PlacedStoredMixedDisplay, error) {
	if p == nil || p.prepared == nil {
		return nil, geom.ErrInvalid
	}
	return p.prepared.Place(ctx, pose, o)
}

// PrepareNestedMixedDisplay builds and verifies a display for the original
// nested components of this native rectangular spline material.
func (s Solid) PrepareNestedMixedDisplay(ctx context.Context, o NestedMixedDisplayOptions) (*PreparedNestedMixedDisplay, error) {
	if ctx == nil {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if o.MaxShells == 0 {
		o.MaxShells = 8
	}
	if o.MaxVertices == 0 {
		o.MaxVertices = 100000
	}
	if o.MaxTriangles == 0 {
		o.MaxTriangles = collision.MaxTriangles
	}
	if o.MaxFacePairs == 0 {
		o.MaxFacePairs = 4096
	}
	if o.MaxShells < 1 || o.MaxShells > 16 || o.MaxVertices < 1 || o.MaxVertices > 100000 || o.MaxTriangles < 1 || o.MaxTriangles > collision.MaxTriangles || o.MaxFacePairs < 1 || o.MaxFacePairs > 16384 || len(s.faces) > 128 || len(s.edges) > 2048 || len(s.vertices) > 2048 || len(o.Meshing.Faces) != len(s.faces) {
		return nil, geom.ErrInvalid
	}
	topology := s.Topology()
	if len(topology.Shells) < 2 || len(topology.Shells) > o.MaxShells {
		return nil, geom.ErrConvergence
	}
	pairs := 0
	for i, a := range topology.Shells {
		for _, b := range topology.Shells[i+1:] {
			pairs += len(a.Faces) * len(b.Faces)
		}
	}
	if pairs > o.MaxFacePairs {
		return nil, geom.ErrConvergence
	}
	shells, e := gatherCertifiedSplineMaterial(ctx, s, o.Source)
	if e != nil {
		return nil, e
	}
	for _, shell := range shells {
		if shell.Rectangle == nil {
			return nil, geom.ErrConvergence
		}
	}
	containment, e := splineMaterialContainment(ctx, shells, o.Source)
	if e != nil {
		return nil, e
	}
	if e = assignSplineMaterialDepths(shells, containment); e != nil {
		return nil, e
	}
	boxes := make([]geom.Box, len(s.faces))
	for i, f := range s.faces {
		patch, e := exactFaceSupportPatch(f, o.Meshing.Faces[i].Face.Face.Boundary.Composition.SurfacePatchOptions)
		if e != nil {
			return nil, e
		}
		boxes[i], e = patch.Bounds()
		if e == nil && (!boxes[i].Min.IsFinite() || !boxes[i].Max.IsFinite()) {
			return nil, geom.ErrConvergence
		}
		if e != nil {
			return nil, e
		}
	}
	var combined mesh.Mesh
	var owners []int
	ancestry := StoredDisplayAncestry{FaceVertices: make([][]int, len(s.faces)), EdgeVertices: make([][]int, len(s.edges)), EdgeParameters: make([][]*big.Rat, len(s.edges)), CornerVertices: map[int]int{}}
	out := &PreparedNestedMixedDisplay{}
	errorsSquared := make([]*big.Rat, len(shells))
	maximum := new(big.Rat)
	for i, shell := range shells {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		options := o.Meshing
		options.Faces = make([]SolidMixedFaceMeshOptions, len(shell.original.Faces))
		for j, id := range shell.original.Faces {
			options.Faces[j] = o.Meshing.Faces[id]
		}
		report, e := shell.solid.TessellateMixedClosedCertified(ctx, options)
		if e != nil {
			return nil, e
		}
		if len(report.Mesh.Vertices) > o.MaxVertices-len(combined.Vertices) || len(report.Mesh.Triangles) > o.MaxTriangles-len(combined.Triangles) {
			return nil, geom.ErrConvergence
		}
		squared := report.SquaredDistance
		if squared == nil || squared.Sign() < 0 || squared.Num().BitLen() > 65536 || squared.Denom().BitLen() > 65536 {
			return nil, geom.ErrConvergence
		}
		errorsSquared[i] = new(big.Rat).Set(squared)
		if squared.Cmp(maximum) > 0 {
			maximum.Set(squared)
		}
		offset := len(combined.Vertices)
		combined.Vertices = append(combined.Vertices, report.Mesh.Vertices...)
		for j, tr := range report.Mesh.Triangles {
			if shell.inward {
				tr[1], tr[2] = tr[2], tr[1]
			}
			combined.Triangles = append(combined.Triangles, [3]int{offset + tr[0], offset + tr[1], offset + tr[2]})
			owners = append(owners, shell.original.Faces[report.OriginalFace[j]])
		}
		for j, row := range report.FaceVertices {
			global := shell.original.Faces[j]
			ancestry.FaceVertices[global] = append([]int(nil), row...)
			for k := range row {
				ancestry.FaceVertices[global][k] += offset
			}
		}
		for j, row := range report.Network.EdgeVertices {
			global := shell.original.Edges[j]
			ancestry.EdgeVertices[global] = append([]int(nil), row...)
			for k := range row {
				ancestry.EdgeVertices[global][k] += offset
			}
			ancestry.EdgeParameters[global] = report.Network.EdgeParameters[j]
		}
		for corner, id := range report.Network.CornerVertices {
			ancestry.CornerVertices[shell.original.Vertices[corner]] = id + offset
		}
		out.nesting = append(out.nesting, NativeDisplayNesting{shell.Parent, shell.Depth, append([]int(nil), shell.original.Faces...)})
	}
	for i, a := range shells {
		for j := i + 1; j < len(shells); j++ {
			b := shells[j]
			var minimum *big.Rat
			count := 0
			for _, af := range a.original.Faces {
				for _, bf := range b.original.Faces {
					if e := ctx.Err(); e != nil {
						return nil, e
					}
					count++
					x, y := boxes[af], boxes[bf]
					lower := new(big.Rat)
					for axis := 0; axis < 3; axis++ {
						amin := []float64{x.Min.X, x.Min.Y, x.Min.Z}[axis]
						amax := []float64{x.Max.X, x.Max.Y, x.Max.Z}[axis]
						bmin := []float64{y.Min.X, y.Min.Y, y.Min.Z}[axis]
						bmax := []float64{y.Max.X, y.Max.Y, y.Max.Z}[axis]
						gap := new(big.Rat).Sub(new(big.Rat).SetFloat64(bmin), new(big.Rat).SetFloat64(amax))
						if gap.Sign() <= 0 {
							gap.Sub(new(big.Rat).SetFloat64(amin), new(big.Rat).SetFloat64(bmax))
						}
						if gap.Sign() > 0 {
							v := new(big.Rat).Mul(gap, gap)
							if v.Cmp(lower) > 0 {
								lower = v
							}
						}
					}
					if lower.Sign() == 0 {
						return nil, geom.ErrConvergence
					}
					if minimum == nil || lower.Cmp(minimum) < 0 {
						minimum = lower
					}
				}
			}
			required := new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Add(errorsSquared[i], errorsSquared[j]))
			if minimum == nil || minimum.Cmp(required) <= 0 {
				return nil, geom.ErrConvergence
			}
			out.clearance = append(out.clearance, NativeDisplayClearance{i, j, count, minimum, required})
		}
	}
	display, e := placement.PrepareClosedDisplay(ctx, combined, owners, placement.ClosedDisplayOptions{Rounding: placement.MeshRoundingOptions{MaxVertices: o.MaxVertices, MaxTriangles: o.MaxTriangles}, Validation: o.Validation})
	if e != nil {
		return nil, e
	}
	result := display.Shells()
	if len(result) != len(shells) {
		return nil, geom.ErrDomain
	}
	for i, shell := range result {
		if shell.Parent != shells[i].Parent || shell.Depth != shells[i].Depth {
			return nil, geom.ErrDomain
		}
		allowed := map[int]bool{}
		for _, id := range shells[i].original.Faces {
			allowed[id] = true
		}
		for _, tri := range shell.Triangles {
			if !allowed[owners[tri]] {
				return nil, geom.ErrDomain
			}
		}
	}
	var minimumGap *big.Rat
	for _, c := range out.clearance {
		if minimumGap == nil || c.SquaredGapLower.Cmp(minimumGap) < 0 {
			minimumGap = new(big.Rat).Set(c.SquaredGapLower)
		}
	}
	if minimumGap == nil || new(big.Rat).Mul(big.NewRat(4, 1), maximum).Cmp(minimumGap) >= 0 {
		return nil, geom.ErrConvergence
	}
	out.prepared = &PreparedStoredMixedDisplay{source: s, display: display, squared: maximum, ancestry: ancestry, sourceSeparationSquared: minimumGap}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	return out, nil
}
