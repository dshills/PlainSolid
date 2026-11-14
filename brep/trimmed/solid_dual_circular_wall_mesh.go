package trimmed

import (
	"context"
	"plainsolid/geom"
)

// TessellateDualCircularWallCertified recovers the bounded two-circular-wall
// meshing family directly from stored geometry, including after native JSON
// reload. It requires exactly two independently proved rational cylindrical
// rectangle charts and affine remaining faces. It does not infer material from
// names, IDs, constructor recipes, or sampled fits. Complete edge identity,
// inward wall orientation, closed display embedding and the requested distance
// and normal bounds are re-proved by TessellateCertified. Unsupported surfaces
// or topology reject without partial output. Limits remain16faces/64edges.
func (s Solid) TessellateDualCircularWallCertified(ctx context.Context, o SideOpenDualDiskMeshOptions) (CertifiedSideOpenDualDiskMesh, error) {
	var empty CertifiedSideOpenDualDiskMesh
	if ctx == nil || len(s.faces) < 2 || len(s.faces) > 16 || len(s.edges) > 64 {
		return empty, geom.ErrInvalid
	}
	var walls []int
	for fi, f := range s.faces {
		if e := ctx.Err(); e != nil {
			return empty, e
		}
		p, e := exactFaceSupportPatch(f, geom.SurfacePatchOptions{})
		if e != nil {
			return empty, e
		}
		if _, e = geom.NewAffineSurfaceChart(ctx, p, geom.AffineSurfaceChartOptions{}); e == nil {
			continue
		}
		if _, e = proveDualDiskWall(ctx, f); e != nil {
			return empty, e
		}
		walls = append(walls, fi)
		if len(walls) > 2 {
			return empty, geom.ErrDomain
		}
	}
	if len(walls) != 2 {
		return empty, geom.ErrDomain
	}
	return (SideOpenDualDiskCut{Solid: s, WallFaces: [2][]int{{walls[0]}, {walls[1]}}, FloorFace: -1}).TessellateCertified(ctx, o)
}
