package step

import (
	"cmp"
	"context"
	"errors"
	"io"
	"math/big"
	"slices"

	"plainsolid/brep/trimmed"
	"plainsolid/exchange/internal/annularchamfer"
	"plainsolid/geom"
)

// EccentricAnnularChamferSTEP retains a model-only source and its verified
// native solid in the original length unit. A complete original rim owns
// four native quadrant edges. An added cardinal vertex has source ID zero.
type EccentricAnnularChamferSTEP struct {
	Name                                     string
	LengthUnit                               float64
	Solid                                    trimmed.Solid
	Source                                   *File
	SourceBodyID, SourceShellID              int64
	NativeFaceSourceIDs, NativeEdgeSourceIDs []int64
	NativeVertexSourceIDs                    []int64
}

// ReadEccentricAnnularChamfer strictly imports one four-rim-chamfered,
// eccentric tapered annulus with two +Z/-Z planes and six axis-parallel
// revolution supports. This model-only dialect uses separate complete
// circular rims, exact finite degree-one spline generators and paired
// source seam uses. No coordinate or support is repaired.
func ReadEccentricAnnularChamfer(ctx context.Context, r io.Reader, l Limits, o geom.SurfacePatchOptions) (EccentricAnnularChamferSTEP, error) {
	var out EccentricAnnularChamferSTEP
	if ctx == nil || r == nil {
		return out, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return out, err
	}
	l, err := l.defaults()
	if err != nil {
		return out, err
	}
	l.MaxBytes = min(l.MaxBytes, 128<<10)
	l.MaxEntities = min(l.MaxEntities, 512)
	l.MaxValues = min(l.MaxValues, 8192)
	l.MaxDepth = min(l.MaxDepth, 16)
	f, rep, unit, items, err := parseCanonicalCircularExtrusions(r, l)
	if err != nil {
		return out, err
	}
	representation, _ := simple(f, rep, "ADVANCED_BREP_SHAPE_REPRESENTATION", 3)
	if err := occtLoopContextMode(f, representation.Args[2].Ref, true); err != nil {
		return out, err
	}
	bodyID, err := modelOnlyCircularSTEPBody(f, rep, items, map[string]bool{
		"SURFACE_OF_REVOLUTION": true, "AXIS1_PLACEMENT": true, "B_SPLINE_CURVE_WITH_KNOTS": true,
	})
	if err != nil {
		return out, errEccentricAnnularChamferSTEP
	}
	body, _ := simple(f, bodyID, "MANIFOLD_SOLID_BREP", 2)
	name, err := textValue(body.Args[0])
	if err != nil {
		return out, err
	}
	shellID, err := ref(body.Args[1])
	if err != nil {
		return out, err
	}
	shell, err := simple(f, shellID, "CLOSED_SHELL", 2)
	if err != nil {
		return out, err
	}
	ids, err := listRefs(shell.Args[1], 8)
	if err != nil || len(ids) != 8 {
		return out, errEccentricAnnularChamferSTEP
	}
	faces, err := readAnnularChamferSTEPFaces(f, ids)
	if err != nil {
		return out, err
	}
	native, err := reconstructAnnularChamferSTEP(ctx, faces, o)
	if err != nil {
		return out, err
	}
	g, err := annularchamfer.Extract(ctx, native, o)
	if err != nil {
		return out, err
	}
	faceIDs, edgeIDs, vertexIDs, err := proveAnnularChamferSTEPIncidence(ctx, f, faces, g)
	if err != nil {
		return out, err
	}
	return EccentricAnnularChamferSTEP{Name: name, LengthUnit: unit, Solid: native, Source: f,
		SourceBodyID: bodyID, SourceShellID: shellID, NativeFaceSourceIDs: faceIDs, NativeEdgeSourceIDs: edgeIDs, NativeVertexSourceIDs: vertexIDs}, nil
}

var errEccentricAnnularChamferSTEP = errors.New("step: unsupported exact eccentric annular chamfer")

type annularChamferSTEPFace struct {
	boreRimSTEPFace
	station [2]trimmed.WrappedLoftStation // lower and upper for a wall
}

func readAnnularChamferSTEPFaces(f *File, ids []int64) ([]annularChamferSTEPFace, error) {
	owned := map[int64]bool{}
	claim := func(id int64) bool {
		if owned[id] {
			return false
		}
		owned[id] = true
		return true
	}
	faces := make([]annularChamferSTEPFace, 0, len(ids))
	for _, id := range ids {
		r, err := simple(f, id, "ADVANCED_FACE", 4)
		if err != nil || !claim(id) {
			return nil, errEccentricAnnularChamferSTEP
		}
		p := annularChamferSTEPFace{boreRimSTEPFace: boreRimSTEPFace{id: id}}
		p.sense, err = boolean(r.Args[3])
		if err != nil {
			return nil, err
		}
		support, err := ref(r.Args[2])
		if err != nil || !claim(support) {
			return nil, errEccentricAnnularChamferSTEP
		}
		bounds, err := listRefs(r.Args[1], 1)
		if err != nil || len(bounds) > 2 {
			return nil, errEccentricAnnularChamferSTEP
		}
		if err := readModelOnlyCircularSTEPWires(f, bounds, &p.boreRimSTEPFace, claim); err != nil {
			return nil, err
		}
		if plane, err := simple(f, support, "PLANE", 2); err == nil {
			pose, err := ref(plane.Args[1])
			if err != nil {
				return nil, err
			}
			p.center, p.axis, err = boreRimSTEPFrame(f, pose)
			if err != nil || p.axis != (geom.Vec3{Z: 1}) && p.axis != (geom.Vec3{Z: -1}) ||
				len(p.loops) != 2 || len(p.loops[0]) != 1 || len(p.loops[1]) != 1 {
				return nil, errEccentricAnnularChamferSTEP
			}
		} else if len(p.loops) != 1 || len(p.loops[0]) != 4 || !readAnnularChamferSTEPRevolution(f, support, &p) {
			return nil, errEccentricAnnularChamferSTEP
		}
		faces = append(faces, p)
	}
	return faces, nil
}

// A positive-radial meridian whose finite parameter increases along the
// revolution axis has an outward underlying normal for either signed axis.
func readAnnularChamferSTEPRevolution(f *File, id int64, p *annularChamferSTEPFace) bool {
	r, err := simple(f, id, "SURFACE_OF_REVOLUTION", 3)
	if err != nil {
		return false
	}
	refs, err := namedReferences(r, 1, 2)
	if err != nil {
		return false
	}
	axis, err := simple(f, refs[1], "AXIS1_PLACEMENT", 3)
	if err != nil {
		return false
	}
	a, err := namedReferences(axis, 1, 2)
	if err != nil {
		return false
	}
	p.center, err = mappedExactCoordinate(f, a[0], "CARTESIAN_POINT", 3)
	if err != nil {
		return false
	}
	p.axis, err = mappedExactCoordinate(f, a[1], "DIRECTION", 3)
	if err != nil || p.axis != (geom.Vec3{Z: 1}) && p.axis != (geom.Vec3{Z: -1}) {
		return false
	}
	poles, err := annularChamferSTEPLineSegment(f, refs[0])
	if err != nil || poles[0].Z == poles[1].Z || (poles[1].Z > poles[0].Z) != (p.axis.Z > 0) {
		return false
	}
	if poles[0].Z > poles[1].Z {
		poles[0], poles[1] = poles[1], poles[0]
	}
	for i, pole := range poles {
		if pole.Y != p.center.Y || pole.X <= p.center.X {
			return false
		}
		r := new(big.Rat).Sub(sideRat(pole.X), sideRat(p.center.X))
		radius, exact := r.Float64()
		if !exact {
			return false
		}
		p.station[i] = trimmed.WrappedLoftStation{Z: pole.Z, Radius: radius}
	}
	return true
}

func annularChamferSTEPLineSegment(f *File, id int64) ([2]geom.Vec3, error) {
	var poles [2]geom.Vec3
	r, err := simple(f, id, "B_SPLINE_CURVE_WITH_KNOTS", 9)
	if err != nil {
		return poles, err
	}
	isEnum := func(v Value, s string) bool { return v.Kind == Enumeration && v.Text == s }
	if !isEnum(r.Args[3], "UNSPECIFIED") || !isEnum(r.Args[4], "F") || !isEnum(r.Args[5], "F") ||
		!isEnum(r.Args[8], "PIECEWISE_BEZIER_KNOTS") {
		return poles, errEccentricAnnularChamferSTEP
	}
	degree, err := integer(r.Args[1])
	if err != nil || degree != 1 {
		return poles, errEccentricAnnularChamferSTEP
	}
	multiplicities, knots := r.Args[6], r.Args[7]
	if multiplicities.Kind != Aggregate || len(multiplicities.Items) != 2 ||
		knots.Kind != Aggregate || len(knots.Items) != 2 {
		return poles, errEccentricAnnularChamferSTEP
	}
	for i, v := range multiplicities.Items {
		m, mErr := integer(v)
		k, kErr := cylinderSupportNumber(knots.Items[i])
		if mErr != nil || m != 2 || kErr != nil || k != float64(i) {
			return poles, errEccentricAnnularChamferSTEP
		}
	}
	points, err := listRefs(r.Args[2], 2)
	if err != nil || len(points) != 2 {
		return poles, errEccentricAnnularChamferSTEP
	}
	for i, id := range points {
		poles[i], err = mappedExactCoordinate(f, id, "CARTESIAN_POINT", 3)
		if err != nil {
			return poles, err
		}
	}
	return poles, nil
}

func reconstructAnnularChamferSTEP(ctx context.Context, faces []annularChamferSTEPFace, o geom.SurfacePatchOptions) (trimmed.Solid, error) {
	var caps [2]*annularChamferSTEPFace
	var walls [2][]annularChamferSTEPFace
	for i := range faces {
		f := &faces[i]
		if f.station[0].Radius == 0 {
			end := 0
			if f.sense == (f.axis.Z > 0) {
				end = 1
			}
			if caps[end] != nil {
				return trimmed.Solid{}, errEccentricAnnularChamferSTEP
			}
			caps[end] = f
			continue
		}
		side := 0
		if !f.sense {
			side = 1
		}
		walls[side] = append(walls[side], *f)
	}
	if caps[0] == nil || caps[1] == nil || caps[0].center.Z >= caps[1].center.Z {
		return trimmed.Solid{}, errEccentricAnnularChamferSTEP
	}
	var lofts [2]trimmed.Solid
	for side, wall := range walls {
		if len(wall) != 3 {
			return trimmed.Solid{}, errEccentricAnnularChamferSTEP
		}
		slices.SortFunc(wall, func(a, b annularChamferSTEPFace) int {
			return cmp.Compare(a.station[0].Z, b.station[0].Z)
		})
		center := geom.Vec2{X: wall[0].center.X, Y: wall[0].center.Y}
		stations := []trimmed.WrappedLoftStation{wall[0].station[0]}
		for i, face := range wall {
			if face.center.X != center.X || face.center.Y != center.Y || face.station[0] != stations[i] {
				return trimmed.Solid{}, errEccentricAnnularChamferSTEP
			}
			stations = append(stations, face.station[1])
		}
		if stations[0].Z != caps[0].center.Z || stations[3].Z != caps[1].center.Z {
			return trimmed.Solid{}, errEccentricAnnularChamferSTEP
		}
		var err error
		lofts[side], _, err = trimmed.LoftWrappedCircularStations(ctx, center, stations, 1e-9, o)
		if err != nil {
			return trimmed.Solid{}, err
		}
	}
	native, _, err := trimmed.CutWrappedEccentricCircularStationLofts(ctx, lofts[0], lofts[1], o)
	return native, err
}
