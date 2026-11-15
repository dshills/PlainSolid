package step

import (
	"bytes"
	"context"
	"io"
	"math/big"

	rigid "plainsolid/brep/placement"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

const localExactCircularLoopThroughHoleProduct = "exact-circular-loop-through-hole-extrusion"
const rationalPosedExactCircularLoopThroughHoleProduct = "rational-posed-exact-circular-loop-through-hole-extrusion"

// RationalPosedExactCircularLoopThroughHoleProduct is a strictly authenticated
// canonical millimetre extrusion with one complete circular through-hole. The
// exact world placement acts on both the exterior recipe and the native body.
type RationalPosedExactCircularLoopThroughHoleProduct struct {
	Source   *File
	Exterior ExactCircularLoopExtrusion
	Hole     trimmed.ExtrusionCircle
	Native   trimmed.Solid
	Pose     rigid.RationalRigid
}

// WriteRationalPosedExactCircularLoopThroughHoleExtrusion writes a closed
// analytic B-rep with four inward cylindrical patches and two shared cap
// interior wires. The local frame is identity and height is positive.
func WriteRationalPosedExactCircularLoopThroughHoleExtrusion(ctx context.Context, w io.Writer, exterior ExactCircularLoopExtrusion, hole trimmed.ExtrusionCircle, pose rigid.RationalRigid) error {
	if exterior.LengthUnit != .001 || exterior.Height <= 0 || len(exterior.Segments) > 60 {
		return geom.ErrDomain
	}
	return writeRationalPosedExactCircularLoopExtrusion(ctx, w, exterior, &hole, pose, nil)
}

// WriteCanonicalSTEP emits the proved through-hole recipe and its original
// exact world pose as a strictly reopenable analytic product.
func (m RationalPosedConvexArcThroughBore) WriteCanonicalSTEP(ctx context.Context, w io.Writer) error {
	d := m.outer.LocalDescriptor()
	frame, err := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if err != nil {
		return err
	}
	d.Frame = frame
	return WriteRationalPosedExactCircularLoopThroughHoleExtrusion(ctx, w, d, m.hole, m.Pose())
}

// ReadRationalPosedExactCircularLoopThroughHoleExtrusion checks the entire
// original representation against a regenerated exact holed solid. Both cap
// wires, the inward bore patches, shared model edges and occurrence pose must
// agree. The canonical transfer uses no repair budget.
func ReadRationalPosedExactCircularLoopThroughHoleExtrusion(ctx context.Context, r io.Reader, l Limits) (RationalPosedExactCircularLoopThroughHoleProduct, error) {
	var zero RationalPosedExactCircularLoopThroughHoleProduct
	if ctx == nil || r == nil {
		return zero, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	l, err := occurrenceColorSourceLimits(l)
	if err != nil {
		return zero, err
	}
	f, rep, unit, bodies, err := parseCanonicalCircularExtrusions(r, l)
	if err != nil {
		return zero, err
	}
	if len(bodies) != 1 || unit != .001 {
		return zero, geom.ErrDomain
	}
	bodyID := bodies[0]
	pose, err := readRationalExactCircularLoopPose(f, bodyID)
	if err != nil {
		return zero, err
	}
	body, err := nauoClosure(f, bodies, 16)
	if err != nil {
		return zero, err
	}
	local, err := rationalConvexArcView(ctx, f, body, nil, pose, true)
	if err != nil {
		return zero, err
	}
	if err = setExactCircularLoopProductIdentifier(local, rationalPosedExactCircularLoopThroughHoleProduct, localExactCircularLoopProduct); err != nil {
		return zero, err
	}
	hole, err := isolateCanonicalCircularLoopExterior(local, bodyID, body, l)
	if err != nil {
		return zero, err
	}
	exteriorRaw, err := mappedPlanarSourceBytes(local)
	if err != nil {
		return zero, err
	}
	exterior, err := ReadExactCircularLoopExtrusion(ctx, bytes.NewReader(exteriorRaw), l)
	if err != nil {
		return zero, err
	}
	d := exterior.Descriptor
	native, err := proveExactCircularLoopThroughHoleGraph(ctx, f, rep, d, hole, pose)
	if err != nil {
		return zero, err
	}
	return RationalPosedExactCircularLoopThroughHoleProduct{Source: f, Exterior: d, Hole: hole, Native: native, Pose: pose}, nil
}

func isolateCanonicalCircularLoopExterior(f *File, bodyID int64, body map[int64]bool, l Limits) (trimmed.ExtrusionCircle, error) {
	var zero trimmed.ExtrusionCircle
	const borePatches = 4
	b, err := simple(f, bodyID, "MANIFOLD_SOLID_BREP", 2)
	if err != nil {
		return zero, err
	}
	shellID := b.Args[1].Ref
	shell, err := simple(f, shellID, "CLOSED_SHELL", 2)
	if err != nil {
		return zero, err
	}
	faces, err := listRefs(shell.Args[1], 2+3+borePatches)
	if err != nil || len(faces) > l.MaxFaces || len(faces) > 66 {
		return zero, geom.ErrDomain
	}
	exterior := len(faces) - borePatches
	segments := len(faces) - 2
	if 2*segments > l.MaxVertices || 6*segments > l.MaxEdgeUses {
		return zero, geom.ErrConvergence
	}
	planeCaps := faces[:2]
	for _, cap := range planeCaps {
		r, err := simple(f, cap, "ADVANCED_FACE", 4)
		if err != nil {
			return zero, err
		}
		bounds, err := listRefs(r.Args[1], 2)
		if err != nil || len(bounds) != 2 {
			return zero, geom.ErrDomain
		}
		if _, err = simple(f, bounds[0], "FACE_OUTER_BOUND", 3); err != nil {
			return zero, err
		}
		if _, err = simple(f, bounds[1], "FACE_BOUND", 3); err != nil {
			return zero, err
		}
		setConvexArcBoreRefs(f, cap, r.Args[1].Items[:1])
	}
	wall, err := simple(f, faces[exterior], "ADVANCED_FACE", 4)
	if err != nil {
		return zero, err
	}
	c, err := simple(f, wall.Args[2].Ref, "CYLINDRICAL_SURFACE", 3)
	if err != nil {
		return zero, err
	}
	o, _, _, err := occtLoopFrame(f, c.Args[1].Ref)
	if err != nil {
		return zero, err
	}
	radius, err := liftSourceRat(c.Args[2])
	if err != nil || radius.Sign() <= 0 || o[2].Sign() != 0 {
		return zero, geom.ErrDomain
	}
	center, err := circularRepairVector([3]*big.Rat{o[0], o[1], radius})
	if err != nil {
		return zero, err
	}
	setConvexArcBoreRefs(f, shellID, shell.Args[1].Items[:exterior])
	outer, err := nauoClosure(f, []int64{bodyID}, 16)
	if err != nil {
		return zero, err
	}
	order := f.Order[:0]
	for _, id := range f.Order {
		if body[id] && !outer[id] {
			delete(f.Entities, id)
		} else {
			order = append(order, id)
		}
	}
	f.Order = order
	return trimmed.ExtrusionCircle{Center: geom.Vec2{X: center.X, Y: center.Y}, Radius: center.Z}, nil
}

func proveExactCircularLoopThroughHoleGraph(ctx context.Context, source *File, rep int64, exterior ExactCircularLoopExtrusion, hole trimmed.ExtrusionCircle, pose rigid.RationalRigid) (trimmed.Solid, error) {
	var replay bytes.Buffer
	var material trimmed.Solid
	if exterior.LengthUnit != .001 || exterior.Height <= 0 || len(exterior.Segments) > 60 {
		return trimmed.Solid{}, geom.ErrDomain
	}
	if err := writeRationalPosedExactCircularLoopExtrusion(ctx, &replay, exterior, &hole, pose, &material); err != nil {
		return trimmed.Solid{}, err
	}
	expected, expectedRep, _, _, err := parseCanonicalCircularExtrusions(&replay, Limits{})
	if err != nil {
		return trimmed.Solid{}, err
	}
	if _, err = equalCircularExtrusionGraph(source, rep, expected, expectedRep); err != nil {
		return trimmed.Solid{}, err
	}
	return material, nil
}
