//go:build research

package trimmed

import (
	"context"
	"plainsolid/geom"
)

// Every original branch and its
// parameter data remain unchanged; the private canonical graph/network view
// exists only during the existing component and global contact proofs.
func tessellateProjectedDisjointBooleanRegions(ctx context.Context, r AlgebraicTubeRegions, o AlgebraicTubeRegionsMeshOptions) (CertifiedAlgebraicTubeRegionsMesh, error) {
	var empty CertifiedAlgebraicTubeRegionsMesh
	if r.section.runSource() == nil || (r.section.runSource().projection != [2]int{1, 2} && r.section.runSource().projection != [2]int{0, 2}) {
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
	axes := r.section.runSource().projection
	order := [3]int{axes[0], axes[1], 3 - axes[0] - axes[1]}
	forward := func(p geom.Vec3) geom.Vec3 {
		v := [3]float64{p.X, p.Y, p.Z}
		return geom.Vec3{X: v[order[0]], Y: v[order[1]], Z: v[order[2]]}
	}
	inverse := func(p geom.Vec3) geom.Vec3 {
		var v [3]float64
		v[order[0]], v[order[1]], v[order[2]] = p.X, p.Y, p.Z
		return geom.Vec3{X: v[0], Y: v[1], Z: v[2]}
	}
	odd := projectedAxisParity(axes) < 0
	if odd {
		reverseProjectedBoundaryOwners(&original)
	}
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
		if odd {
			for j := range c.Mesh.Triangles {
				t := &c.Mesh.Triangles[j]
				t[1], t[2] = t[2], t[1]
			}
		}
	}
	if odd {
		for i := range out.Mesh.Triangles {
			t := &out.Mesh.Triangles[i]
			t[1], t[2] = t[2], t[1]
		}
	}
	// Exact permutation transports all recorded triangle contacts and
	// distance bounds. Check returned incidence without re-spending pair budgets.
	if e := out.Mesh.ValidateSolid(); e != nil {
		return empty, e
	}
	return out, nil
}
