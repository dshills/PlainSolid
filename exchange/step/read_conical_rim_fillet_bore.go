package step

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/big"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// ConicalRimFilletBoreSTEP is a proved model-only cone with both external
// rolling rims and one eccentric through-bore. The original file geometry
// remains in its source units. A source full circle may own four native
// quadrant edges. New cardinal vertices have source ID zero.
type ConicalRimFilletBoreSTEP struct {
	Name                                     string
	LengthUnit                               float64
	Solid                                    trimmed.Solid
	Source                                   *File
	SourceBodyID, SourceShellID              int64
	NativeFaceSourceIDs, NativeEdgeSourceIDs []int64
	NativeVertexSourceIDs                    []int64
}

var errConicalRimFilletBoreSTEP = errors.New("step: unsupported exact conical rim fillet with eccentric bore")

type conicalRimFilletBoreFaces struct {
	cone   annularChamferSTEPFace
	bore   boreRimSTEPFace
	caps   [2]boreRimSTEPFace // lower, upper
	toroid [2]boreRimSTEPFace // lower, upper
}

// ReadConicalRimFilletBore strictly imports a vertically extruded bore in
// an ascending conical stock with two independent rolling rim fillets.
// It accepts a finite two-pole cone meridian, original full circles,
// analytic toroidal meridian arcs, and exact directed source incidence.
func ReadConicalRimFilletBore(ctx context.Context, r io.Reader, l Limits, o geom.SurfacePatchOptions) (ConicalRimFilletBoreSTEP, error) {
	var empty ConicalRimFilletBoreSTEP
	if ctx == nil || r == nil {
		return empty, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return empty, err
	}
	l, err := l.defaults()
	if err != nil {
		return empty, err
	}
	l.MaxBytes = min(l.MaxBytes, 128<<10)
	l.MaxEntities = min(l.MaxEntities, 384)
	l.MaxValues = min(l.MaxValues, 8192)
	l.MaxDepth = min(l.MaxDepth, 16)
	f, rep, unit, items, err := parseCanonicalCircularExtrusions(r, l)
	if err != nil {
		return empty, err
	}
	representation, _ := simple(f, rep, "ADVANCED_BREP_SHAPE_REPRESENTATION", 3)
	if err := occtLoopContextMode(f, representation.Args[2].Ref, true); err != nil {
		return empty, err
	}
	bodyID, err := modelOnlyCircularSTEPBody(f, rep, items, map[string]bool{
		"SURFACE_OF_REVOLUTION": true, "AXIS1_PLACEMENT": true, "B_SPLINE_CURVE_WITH_KNOTS": true,
	})
	if err != nil {
		return empty, errConicalRimFilletBoreSTEP
	}
	body, _ := simple(f, bodyID, "MANIFOLD_SOLID_BREP", 2)
	name, err := textValue(body.Args[0])
	if err != nil {
		return empty, err
	}
	shellID, err := ref(body.Args[1])
	if err != nil {
		return empty, err
	}
	shell, err := simple(f, shellID, "CLOSED_SHELL", 2)
	if err != nil {
		return empty, err
	}
	ids, err := listRefs(shell.Args[1], 6)
	if err != nil || len(ids) != 6 {
		return empty, errConicalRimFilletBoreSTEP
	}
	faces, err := readConicalRimFilletBoreSTEPFaces(f, ids)
	if err != nil {
		return empty, err
	}
	native, err := buildConicalRimFilletBoreSTEP(ctx, faces, o)
	if err != nil {
		return empty, err
	}
	g, owners, err := bindConicalRimFilletBoreFaces(faces, native)
	if err != nil {
		return empty, fmt.Errorf("step: bind conical rim supports: %w", err)
	}
	faceIDs, edgeIDs, vertexIDs, err := proveConicalRimFilletBoreSTEP(ctx, f, g, owners)
	if err != nil {
		return empty, fmt.Errorf("step: verify conical rim incidence: %w", err)
	}
	return ConicalRimFilletBoreSTEP{
		Name: name, LengthUnit: unit, Solid: native.Solid, Source: f, SourceBodyID: bodyID, SourceShellID: shellID,
		NativeFaceSourceIDs: faceIDs, NativeEdgeSourceIDs: edgeIDs, NativeVertexSourceIDs: vertexIDs,
	}, nil
}

func readConicalRimFilletBoreSTEPFaces(f *File, ids []int64) (conicalRimFilletBoreFaces, error) {
	var out conicalRimFilletBoreFaces
	owned := map[int64]bool{}
	claim := func(id int64) bool {
		if owned[id] {
			return false
		}
		owned[id] = true
		return true
	}
	for _, id := range ids {
		rec, err := simple(f, id, "ADVANCED_FACE", 4)
		if err != nil || !claim(id) {
			return out, errConicalRimFilletBoreSTEP
		}
		face := boreRimSTEPFace{id: id}
		face.sense, err = boolean(rec.Args[3])
		if err != nil {
			return out, err
		}
		support, err := ref(rec.Args[2])
		if err != nil || !claim(support) {
			return out, errConicalRimFilletBoreSTEP
		}
		bounds, err := listRefs(rec.Args[1], 1)
		if err != nil || len(bounds) > 2 {
			return out, errConicalRimFilletBoreSTEP
		}
		if err := readModelOnlyCircularSTEPWires(f, bounds, &face, claim); err != nil {
			return out, err
		}
		if len(f.Entities[support].Records) != 1 {
			return out, errConicalRimFilletBoreSTEP
		}
		s := f.Entities[support].Records[0]
		if s.Type == "SURFACE_OF_REVOLUTION" {
			cone := annularChamferSTEPFace{boreRimSTEPFace: face}
			if out.cone.id != 0 || !readAnnularChamferSTEPRevolution(f, support, &cone) || !face.sense ||
				len(face.loops) != 1 || len(face.loops[0]) != 4 || cone.station[0].Radius >= cone.station[1].Radius {
				return out, errConicalRimFilletBoreSTEP
			}
			out.cone = cone
			continue
		}
		if err := addConicalRimFilletBoreAnalyticFace(f, s, face, &out); err != nil {
			return out, err
		}
	}
	if out.cone.id == 0 || out.bore.id == 0 || out.caps[0].id == 0 || out.caps[1].id == 0 ||
		out.toroid[0].id == 0 || out.toroid[1].id == 0 {
		return out, errConicalRimFilletBoreSTEP
	}
	return out, nil
}

func addConicalRimFilletBoreAnalyticFace(f *File, r Record, face boreRimSTEPFace, out *conicalRimFilletBoreFaces) error {
	arity := 0
	switch r.Type {
	case "PLANE":
		arity = 2
	case "CYLINDRICAL_SURFACE":
		arity = 3
	case "TOROIDAL_SURFACE":
		arity = 4
	}
	if len(r.Args) != arity || arity == 0 {
		return errConicalRimFilletBoreSTEP
	}
	frame, err := ref(r.Args[1])
	if err != nil {
		return err
	}
	face.center, face.axis, err = boreRimSTEPFrame(f, frame)
	if err != nil || face.axis.X != 0 || face.axis.Y != 0 || (face.axis.Z != 1 && face.axis.Z != -1) {
		return errConicalRimFilletBoreSTEP
	}
	if arity == 2 {
		end := 0
		if face.sense == (face.axis.Z > 0) {
			end = 1
		}
		if out.caps[end].id != 0 || len(face.loops) != 2 || len(face.loops[0]) != 1 || len(face.loops[1]) != 1 {
			return errConicalRimFilletBoreSTEP
		}
		out.caps[end] = face
		return nil
	}
	face.major, err = cylinderSupportNumber(r.Args[2])
	if err != nil || face.major <= 0 || len(face.loops) != 1 || len(face.loops[0]) != 4 {
		return errConicalRimFilletBoreSTEP
	}
	if arity == 3 {
		if out.bore.id != 0 || face.sense || face.axis.Z != 1 {
			return errConicalRimFilletBoreSTEP
		}
		out.bore = face
		return nil
	}
	end := 0
	if face.axis.Z > 0 {
		end = 1
	}
	face.minor, err = cylinderSupportNumber(r.Args[3])
	if err != nil || !face.sense || out.toroid[end].id != 0 || face.minor <= 0 || face.minor >= face.major {
		return errConicalRimFilletBoreSTEP
	}
	out.toroid[end] = face
	return nil
}

func buildConicalRimFilletBoreSTEP(ctx context.Context, f conicalRimFilletBoreFaces,
	o geom.SurfacePatchOptions) (trimmed.WrappedFilletedFrustumBoreSet, error) {
	var out trimmed.WrappedFilletedFrustumBoreSet
	stock := f.cone.center
	if stock.X == f.bore.center.X && stock.Y == f.bore.center.Y {
		return out, errConicalRimFilletBoreSTEP
	}
	lower, upper := f.cone.station[0], f.cone.station[1]
	z0, z1 := f.caps[0].center.Z, f.caps[1].center.Z
	if z0 >= lower.Z || z1 <= upper.Z {
		return out, errConicalRimFilletBoreSTEP
	}
	diff := func(a, b float64) *big.Rat { return new(big.Rat).Sub(sideRat(a), sideRat(b)) }
	slope := new(big.Rat).Quo(diff(upper.Radius, lower.Radius), diff(upper.Z, lower.Z))
	extension := func(z float64) *big.Rat {
		return new(big.Rat).Add(sideRat(lower.Radius), new(big.Rat).Mul(slope, diff(z, lower.Z)))
	}
	r0, err := geom.RepresentedExactScalar(extension(z0))
	if err != nil || r0 <= 0 {
		return out, errConicalRimFilletBoreSTEP
	}
	height, err := geom.RepresentedExactScalar(diff(z1, z0))
	if err != nil {
		return out, err
	}
	base, err := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{X: stock.X, Y: stock.Y, Z: z0}, r0, height, o)
	if err != nil {
		return out, err
	}
	wall, _, err := geom.DraftWrappedFrustumExact(ctx, base, new(big.Rat), new(big.Rat).Sub(extension(z1), sideRat(r0)), o)
	if err != nil {
		return out, err
	}
	frustum, _, err := trimmed.NewWrappedFrustum(ctx, wall, 1e-9, o)
	if err != nil {
		return out, err
	}
	center := geom.Vec2{X: f.bore.center.X, Y: f.bore.center.Y}
	return trimmed.FilletAndBoreWrappedFrustumSet(ctx, frustum, f.toroid[0].minor, f.toroid[1].minor,
		[]trimmed.FilletedFrustumBoreCut{{Center: center, Radius: f.bore.major}}, o)
}
