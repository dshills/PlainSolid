package step

import (
	"bytes"
	"context"
	"io"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// TripleCrossedBoreBody retains three equal-radius concurrent cardinal
// bores on rectangular stock. Crossing is in stock coordinates. BlindDepth
// optionally stops the Z bore beyond the full overlap; OppositeCap selects
// its upper entry. XBlindDepth and YBlindDepth independently stop the other
// axes beyond the full overlap. Zero depths retain through bores.
type TripleCrossedBoreBody struct {
	Name                       string
	Frame                      geom.Frame
	Dimensions, Crossing       geom.Vec3
	Radius                     float64
	BlindDepth                 float64
	OppositeCap                bool
	XBlindDepth, YBlindDepth   float64
	XOppositeCap, YOppositeCap bool
}
type TripleCrossedBoreModel struct {
	Body       TripleCrossedBoreBody
	LengthUnit float64
}

func WriteTripleCrossedBore(w io.Writer, m TripleCrossedBoreModel) error {
	if w == nil {
		return geom.ErrInvalid
	}
	var out bytes.Buffer
	if e := writeTripleCrossedBore(&out, m); e != nil {
		return e
	}
	if _, e := ReadTripleCrossedBore(bytes.NewReader(out.Bytes()), Limits{}); e != nil {
		return e
	}
	_, e := io.Copy(w, &out)
	return e
}
func writeTripleCrossedBore(w io.Writer, m TripleCrossedBoreModel) error {
	p := m.Body
	var r trimmed.BoxCrossedBoresResult
	var e error
	if p.XBlindDepth != 0 || p.YBlindDepth != 0 || p.XOppositeCap || p.YOppositeCap {
		r, e = trimmed.BoxFiniteTripleCrossedBores(context.Background(), p.Frame, p.Dimensions, p.Crossing, p.Radius, [3]float64{p.XBlindDepth, p.YBlindDepth, p.BlindDepth}, [3]bool{p.XOppositeCap, p.YOppositeCap, p.OppositeCap}, 1e-9)
	} else if p.BlindDepth != 0 || p.OppositeCap {
		r, e = trimmed.BoxBlindTripleCrossedBores(context.Background(), p.Frame, p.Dimensions, p.Crossing, p.Radius, p.BlindDepth, p.OppositeCap, 1e-9)
	} else {
		r, e = trimmed.BoxTripleCrossedBores(context.Background(), p.Frame, p.Dimensions, p.Crossing, p.Radius, 1e-9)
	}
	if e != nil {
		return e
	}
	return writeNativeCrossedBoreTopology(w, m.LengthUnit, p.Name, r.Solid)
}

// ReadTripleCrossedBore verifies the entire graph against exact regeneration.
func ReadTripleCrossedBore(r io.Reader, limits Limits) (TripleCrossedBoreModel, error) {
	f, rep, unit, ids, e := parseCanonicalCircularExtrusions(r, limits)
	if e != nil || len(ids) != 1 {
		return TripleCrossedBoreModel{}, circularSourceFailure()
	}
	p, e := recoverTripleCrossedBore(f, ids[0])
	if e != nil {
		return TripleCrossedBoreModel{}, e
	}
	m := TripleCrossedBoreModel{Body: p, LengthUnit: unit}
	var out bytes.Buffer
	if e = writeTripleCrossedBore(&out, m); e != nil {
		return TripleCrossedBoreModel{}, e
	}
	if e = verifyCanonicalCircularExtrusionGraph(f, rep, &out); e != nil {
		return TripleCrossedBoreModel{}, e
	}
	return m, nil
}
func recoverTripleCrossedBore(f *File, id int64) (TripleCrossedBoreBody, error) {
	fail := func() (TripleCrossedBoreBody, error) { return TripleCrossedBoreBody{}, circularSourceFailure() }
	body, e := simple(f, id, "MANIFOLD_SOLID_BREP", 2)
	if e != nil {
		return fail()
	}
	name, e := textValue(body.Args[0])
	if e != nil {
		return fail()
	}
	sid, e := ref(body.Args[1])
	if e != nil {
		return fail()
	}
	shell, e := simple(f, sid, "CLOSED_SHELL", 2)
	if e != nil {
		return fail()
	}
	faces, e := listRefs(shell.Args[1], 54)
	if e != nil || len(faces) < 54 || len(faces) > 57 {
		return fail()
	}
	lower, e := readCircularSourceFace(f, faces[0])
	if e != nil || lower.kind != "PLANE" {
		return fail()
	}
	var supports [3]circularSourceFace
	floorCount := len(faces) - 54
	starts := []int{6 + floorCount, 22 + floorCount, 38 + floorCount}
	for i, fi := range starts {
		supports[i], e = readCircularSourceFace(f, faces[fi])
		if e != nil || supports[i].kind != "CYLINDER" || supports[i].sense {
			return fail()
		}
	}
	z, x, y := supports[0], supports[1], supports[2]
	if z.radius != x.radius || z.radius != y.radius || z.frame.Origin() != x.frame.Origin() || z.frame.Origin() != y.frame.Origin() {
		return fail()
	}
	frame, e := geom.NewFrame(lower.frame.Origin(), x.frame.ZAxis(), z.frame.ZAxis())
	if e != nil || frame.YAxis() != y.frame.ZAxis() {
		return fail()
	}
	var vertices []geom.Vec3
	var capFaces [6]circularSourceFace
	for index, id := range faces[:6] {
		face, e := readCircularSourceFace(f, id)
		if e != nil || face.kind != "PLANE" || !face.sense || len(face.loops) < 1 || len(face.loops) > 2 || len(face.loops[0]) != 4 || len(face.loops) == 2 && len(face.loops[1]) != 8 {
			return fail()
		}
		capFaces[index] = face
		for _, use := range face.loops[0] {
			start, _, _, e := analyticEdgeGeometry(f, use.id, true)
			if e != nil {
				return fail()
			}
			p, e := analyticVertex(f, start)
			if e != nil {
				return fail()
			}
			vertices = append(vertices, frame.ToLocal(p))
		}
	}
	bounds, e := geom.Bounds(vertices)
	if e != nil || bounds.Min != (geom.Vec3{}) {
		return fail()
	}
	p := TripleCrossedBoreBody{Name: name, Frame: frame, Dimensions: bounds.Max, Crossing: frame.ToLocal(z.frame.Origin()), Radius: z.radius}

	// Floors precede cylinder groups in canonical Z,X,Y order. Each absent
	// exit opening identifies its axis and entry side without relying on position.
	nextFloor := 6
	for _, axis := range []struct {
		lo, hi    int
		normal    geom.Vec3
		size      float64
		depth     *float64
		opposite  *bool
		component int
	}{
		{0, 1, frame.ZAxis(), bounds.Max.Z, &p.BlindDepth, &p.OppositeCap, 2},
		{5, 3, frame.XAxis(), bounds.Max.X, &p.XBlindDepth, &p.XOppositeCap, 0},
		{2, 4, frame.YAxis(), bounds.Max.Y, &p.YBlindDepth, &p.YOppositeCap, 1},
	} {
		low, high := capFaces[axis.lo], capFaces[axis.hi]
		if len(low.loops) == 2 && len(high.loops) == 2 {
			continue
		}
		if len(low.loops)+len(high.loops) != 3 || nextFloor >= 6+floorCount {
			return fail()
		}
		floor, err := readCircularSourceFaceBounded(f, faces[nextFloor], 1, 8)
		if err != nil || floor.kind != "PLANE" || !floor.sense || len(floor.loops) != 1 || len(floor.loops[0]) != 8 {
			return fail()
		}
		nextFloor++
		point := frame.ToLocal(floor.frame.Origin())
		level := []float64{point.X, point.Y, point.Z}[axis.component]
		*axis.opposite = len(high.loops) == 2
		normal := axis.normal.Scale(-1)
		*axis.depth = level
		if *axis.opposite {
			normal = axis.normal
			*axis.depth = axis.size - level
		}
		if floor.frame.ZAxis() != normal || *axis.depth <= 0 || *axis.depth >= axis.size {
			return fail()
		}
	}
	if nextFloor != 6+floorCount {
		return fail()
	}

	return p, nil
}
