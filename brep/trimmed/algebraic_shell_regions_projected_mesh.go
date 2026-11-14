package trimmed

import (
	"context"
	"plainsolid/geom"
)

// Only even YZ permutation is accepted here. Every original branch and its
// parameter data remain unchanged; the private canonical graph/network view
// exists only during the existing component and global contact proofs.
func tessellateProjectedDisjointBooleanRegions(ctx context.Context, r AlgebraicTubeRegions, o AlgebraicTubeRegionsMeshOptions) (CertifiedAlgebraicTubeRegionsMesh, error) {
	var empty CertifiedAlgebraicTubeRegionsMesh
	if r.section.runSource() == nil || r.section.runSource().projection != [2]int{1, 2} {
		return empty, geom.ErrConvergence
	}
	if o.Boundary.Loop.Run.Display.Point.Tolerance == 0 {
		o.Boundary.Loop.Run.Display.Point.Tolerance = o.Boundary.Loop.Run.Display.Deflection * 1e-6
	}
	original, e := r.TessellateBoundaryCertified(ctx, o.Boundary)
	if e != nil {
		return empty, e
	}
	projected := original
	projected.Vertices = append([]geom.Vec3(nil), original.Vertices...)
	forward := func(p geom.Vec3) geom.Vec3 { return geom.Vec3{X: p.Y, Y: p.Z, Z: p.X} }
	inverse := func(p geom.Vec3) geom.Vec3 { return geom.Vec3{X: p.Z, Y: p.X, Z: p.Y} }
	for i, p := range projected.Vertices {
		projected.Vertices[i] = forward(p)
	}
	projected.Loops = append([]CertifiedAlgebraicTubeLensBoundary(nil), original.Loops...)
	for i := range projected.Loops {
		projected.Loops[i].Run.Vertices = append([]geom.Vec3(nil), original.Loops[i].Run.Vertices...)
		for j, p := range projected.Loops[i].Run.Vertices {
			projected.Loops[i].Run.Vertices[j] = forward(p)
		}
	}
	o.AffineCharts = true
	out, e := r.tessellateDisjointClosedCertified(ctx, o, &projected)
	if e != nil {
		return empty, e
	}
	out.Boundary = original
	for i, p := range out.Mesh.Vertices {
		out.Mesh.Vertices[i] = inverse(p)
	}
	for i := range out.Components {
		c := &out.Components[i]
		for j, p := range c.Mesh.Vertices {
			c.Mesh.Vertices[j] = inverse(p)
		}
		c.Boundary = original.Loops[r.regions[i].Outer]
	}
	// Exact proper permutation transports all recorded triangle contacts and
	// distance bounds. Check returned incidence without re-spending pair budgets.
	if e := out.Mesh.ValidateSolid(); e != nil {
		return empty, e
	}
	return out, nil
}
