package step

import (
	"context"
	"io"
	"math/big"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// UnequalCenterSideOpenDualDiskProduct contains a certified external-center homogeneous-wall native
// body, never an equal-radius analytical result. The descriptor origin is
// normalized to the stock lower corner and disks are ordered by local X.
// SourceWallFaces refers to that normalized disk order; original call order
// is not uniquely encoded. SourceFloorFace is zero for a through cut.
type UnequalCenterSideOpenDualDiskProduct struct {
	// RadicalJoin is an owned exact proof value; both coordinates must also be
	// losslessly representable by the current native float-coordinate format.
	RadicalJoin                [2]*big.Rat
	Descriptor                 UnequalCenterSideOpenDualDiskDescriptor
	Native                     trimmed.UnequalCenterSideOpenDualDiskCut
	Body, Shell                int64
	Faces, Edges               []int64
	SourceWallFaces            [2]int64
	SourceFloorFace            int64
	ReversedModelLines         []int64
	RephasedModelCircles       []int64
	OppositeNormalModelCircles []int64
	OppositeNormalPlanes       []int64
}

// ReadUnequalCenterSideOpenDualDiskProduct re-proves the complete canonical rooted
// unequal-center model-only graph from exact source fields and its registered AP203
// product identifier. Common-ordinate categories, vendor pcurves and
// unsupported constructions reject.
// Caller tolerance is not source metadata.
func ReadUnequalCenterSideOpenDualDiskProduct(ctx context.Context, r io.Reader, l Limits, tolerance float64) (UnequalCenterSideOpenDualDiskProduct, error) {
	return ReadUnequalCenterSideOpenDualDiskProductWithOptions(ctx, r, l, tolerance, SideOpenDualDiskReadOptions{})
}

// ReadUnequalCenterSideOpenDualDiskProductWithOptions adds only the selected exact
// line/circle/plane encoding proofs. Each source circle retains its own radius,
// center and native owner; the complete reconstructed graph is still compared.
func ReadUnequalCenterSideOpenDualDiskProductWithOptions(ctx context.Context, r io.Reader, l Limits, tolerance float64, options SideOpenDualDiskReadOptions) (UnequalCenterSideOpenDualDiskProduct, error) {
	result, err := readSideOpenDualDiskProduct(ctx, r, l, tolerance, options, sideDiskUnequalCenter)
	if err != nil {
		return UnequalCenterSideOpenDualDiskProduct{}, err
	}
	out := UnequalCenterSideOpenDualDiskProduct{Descriptor: UnequalCenterSideOpenDualDiskDescriptor{SideOpenDualDiskDescriptor: result.Descriptor}, Native: result.unequalCenterNative, Body: result.Body, Shell: result.Shell, Faces: result.Faces, Edges: result.Edges, ReversedModelLines: result.ReversedModelLines, RephasedModelCircles: result.RephasedModelCircles, OppositeNormalModelCircles: result.OppositeNormalModelCircles, OppositeNormalPlanes: result.OppositeNormalPlanes}
	for i, ids := range out.Native.WallFaces {
		if len(ids) != 1 || ids[0] < 0 || ids[0] >= len(out.Faces) {
			return UnequalCenterSideOpenDualDiskProduct{}, geom.ErrDomain
		}
		out.SourceWallFaces[i] = out.Faces[ids[0]]
		if _, ok := out.Native.Solid.Faces()[ids[0]].Surface.(*geom.ExactBezierSurface); !ok {
			return UnequalCenterSideOpenDualDiskProduct{}, geom.ErrDomain
		}
	}
	if out.Native.FloorFace >= 0 {
		if out.Native.FloorFace >= len(out.Faces) {
			return UnequalCenterSideOpenDualDiskProduct{}, geom.ErrDomain
		}
		out.SourceFloorFace = out.Faces[out.Native.FloorFace]
	}
	for i, q := range result.unequalCenterJoin {
		if q == nil {
			return UnequalCenterSideOpenDualDiskProduct{}, geom.ErrDomain
		}
		out.RadicalJoin[i] = new(big.Rat).Set(q)
		out.Descriptor.Join[i] = new(big.Rat).Set(q)
	}
	return out, nil
}

// Recover the upper crossing only from the unique actual axial edge shared by
// the two cylindrical face loops. The later whole-graph proof binds all source
// senses, owners, analytic line geometry, endpoints and metadata.
func sideDiskReadOwnedJoin(f *File, faces []int64, d SideOpenDualDiskDescriptor) ([2]*big.Rat, error) {
	var zero [2]*big.Rat
	var walls []map[int64]bool
	for _, fid := range faces {
		face, e := simple(f, fid, "ADVANCED_FACE", 4)
		if e != nil {
			return zero, e
		}
		sid, e := ref(face.Args[2])
		if e != nil {
			return zero, e
		}
		en, ok := f.Entities[sid]
		if !ok || len(en.Records) != 1 {
			return zero, geom.ErrDomain
		}
		if en.Records[0].Type != "CYLINDRICAL_SURFACE" {
			continue
		}
		bounds, e := listRefs(face.Args[1], 1)
		if e != nil || len(bounds) != 1 {
			return zero, geom.ErrDomain
		}
		bound, e := simple(f, bounds[0], "FACE_OUTER_BOUND", 3)
		if e != nil {
			return zero, e
		}
		lid, e := ref(bound.Args[1])
		if e != nil {
			return zero, e
		}
		loop, e := simple(f, lid, "EDGE_LOOP", 2)
		if e != nil {
			return zero, e
		}
		uses, e := listRefs(loop.Args[1], 1)
		if e != nil || len(uses) != 4 {
			return zero, geom.ErrDomain
		}
		edges := map[int64]bool{}
		for _, uid := range uses {
			u, e := simple(f, uid, "ORIENTED_EDGE", 5)
			if e != nil {
				return zero, e
			}
			eid, e := ref(u.Args[3])
			if e != nil || edges[eid] {
				return zero, geom.ErrDomain
			}
			edges[eid] = true
		}
		walls = append(walls, edges)
	}
	if len(walls) != 2 {
		return zero, geom.ErrDomain
	}
	var shared int64
	for id := range walls[0] {
		if walls[1][id] {
			if shared != 0 {
				return zero, geom.ErrDomain
			}
			shared = id
		}
	}
	if shared == 0 {
		return zero, geom.ErrDomain
	}
	edge, e := simple(f, shared, "EDGE_CURVE", 5)
	if e != nil {
		return zero, e
	}
	cid, e := ref(edge.Args[3])
	if e != nil {
		return zero, e
	}
	if _, e = simple(f, cid, "LINE", 3); e != nil {
		return zero, e
	}
	var ends [2]geom.Vec3
	for i := range ends {
		vid, e := ref(edge.Args[i+1])
		if e != nil {
			return zero, e
		}
		v, e := simple(f, vid, "VERTEX_POINT", 2)
		if e != nil {
			return zero, e
		}
		pid, e := ref(v.Args[1])
		if e != nil {
			return zero, e
		}
		p, e := splitTriple(f, pid, "CARTESIAN_POINT")
		if e != nil {
			return zero, e
		}
		ends[i], e = sideDiskLocal(d.Frame, p)
		if e != nil {
			return zero, e
		}
	}
	a, b := ends[0], ends[1]
	if a.X != b.X || a.Y != b.Y || !((a.Z == 0 && b.Z == d.Depth) || (b.Z == 0 && a.Z == d.Depth)) || a.Y <= d.Min.Y {
		return zero, geom.ErrDomain
	}
	for _, disc := range d.Discs {
		if !sideDiskOnCircle(a, disc) {
			return zero, geom.ErrDomain
		}
	}
	return [2]*big.Rat{sideRat(a.X), sideRat(a.Y)}, nil
}
