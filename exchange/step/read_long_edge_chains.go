package step

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// ReadLongEdgeTreatments accepts exactly the canonical origin-anchored native
// treatment graphs emitted by WriteLongEdgeTreatments whose recipe can be
// recovered from represented coordinates. Recovered dimensions
// must regenerate the entire represented topology and geometry without change.
// Equivalent alternate STEP frames, face partitions and constructions are not
// inferred by this recipe reader.
func ReadLongEdgeTreatments(r io.Reader, l Limits) (LongEdgeTreatmentModel, error) {
	f, rep, unit, ids, e := parseCanonicalCircularExtrusions(r, l)
	if e != nil {
		return LongEdgeTreatmentModel{}, e
	}
	if len(ids) != 1 {
		return LongEdgeTreatmentModel{}, geom.ErrDomain
	}
	b, _, e := recoverLongEdgeRecipe(f, ids[0], false)
	if e != nil {
		return LongEdgeTreatmentModel{}, e
	}
	m := LongEdgeTreatmentModel{LengthUnit: unit, Bodies: []LongEdgeTreatmentBody{b}}
	var out bytes.Buffer
	if e = WriteLongEdgeTreatments(&out, m); e != nil {
		return LongEdgeTreatmentModel{}, e
	}
	if e = verifyCanonicalCircularExtrusionGraph(f, rep, &out); e != nil {
		return LongEdgeTreatmentModel{}, e
	}
	return m, nil
}

// ReadLongEdgeChains verifies a canonical actual horizontal fillet/chamfer,
// Z-through circular bore, rectangular through pocket and upper boss recipe.
// Verification includes exact decimal coefficients, native edge direction,
// all owner pcurves, face loops, sharing and units after native reconstruction.
// A recipe whose original setbacks or boss height cannot be recovered without
// changing represented geometry is rejected. This is not a generic analytic
// STEP importer.
func ReadLongEdgeChains(r io.Reader, l Limits) (LongEdgeChainModel, error) {
	f, rep, unit, ids, e := parseCanonicalCircularExtrusions(r, l)
	if e != nil {
		return LongEdgeChainModel{}, e
	}
	if len(ids) != 1 {
		return LongEdgeChainModel{}, geom.ErrDomain
	}
	b, features, e := recoverLongEdgeRecipe(f, ids[0], true)
	if e != nil {
		return LongEdgeChainModel{}, e
	}
	features.LongEdgeTreatmentBody = b
	m := LongEdgeChainModel{LengthUnit: unit, Bodies: []LongEdgeChainBody{features}}
	var out bytes.Buffer
	if e = WriteLongEdgeChains(&out, m); e != nil {
		return LongEdgeChainModel{}, e
	}
	if e = verifyCanonicalCircularExtrusionGraph(f, rep, &out); e != nil {
		return LongEdgeChainModel{}, e
	}
	return m, nil
}
func recoverLongEdgeRecipe(f *File, id int64, full bool) (LongEdgeTreatmentBody, LongEdgeChainBody, error) {
	fail := func() (LongEdgeTreatmentBody, LongEdgeChainBody, error) {
		return LongEdgeTreatmentBody{}, LongEdgeChainBody{}, fmt.Errorf("step: unsupported canonical long-edge recipe graph")
	}
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
	ids, e := listRefs(shell.Args[1], 8)
	count := 8
	if full {
		count = 21
	}
	if e != nil || len(ids) != count {
		return fail()
	}
	faces := make([]circularSourceFace, len(ids))
	for i, id := range ids {
		faces[i], e = readCircularSourceFace(f, id)
		if e != nil {
			return fail()
		}
	}
	points := func(indices []int) ([]geom.Vec3, error) {
		seen := map[int64]bool{}
		out := []geom.Vec3{}
		for _, i := range indices {
			for _, loop := range faces[i].loops {
				for _, u := range loop {
					edge, e := simple(f, u.id, "EDGE_CURVE", 5)
					if e != nil {
						return nil, e
					}
					for _, v := range edge.Args[1:3] {
						vid, e := ref(v)
						if e != nil {
							return nil, e
						}
						if seen[vid] {
							continue
						}
						seen[vid] = true
						p, e := analyticVertex(f, vid)
						if e != nil {
							return nil, e
						}
						out = append(out, p)
					}
				}
			}
		}
		return out, nil
	}
	stockPoints, e := points([]int{0, 1, 2, 3, 4, 5, 6, 7})
	if e != nil || len(stockPoints) < 8 {
		return fail()
	}
	b := LongEdgeTreatmentBody{Name: name}
	for _, p := range stockPoints {
		b.Length = math.Max(b.Length, p.X)
		b.Width = math.Max(b.Width, p.Y)
		b.Height = math.Max(b.Height, p.Z)
	}
	filletFace, chamferFace := -1, -1
	for i, face := range faces[:8] {
		switch face.kind {
		case "CYLINDER":
			if filletFace >= 0 || face.frame.ZAxis() != (geom.Vec3{X: 1}) {
				return fail()
			}
			filletFace = i
			b.Radius = face.radius
		case "PLANE":
			// Native canonical order retains the chamfer as stock face 4.
			// Exact re-authoring below verifies this role and every source ID.
			if i == 4 {
				chamferFace = i
			}

		default:
			return fail()
		}
	}
	if filletFace < 0 || chamferFace < 0 || !finiteNumber(b.Height) {
		return fail()
	}
	chamfer, e := points([]int{chamferFace})
	if e != nil || len(chamfer) != 4 {
		return fail()
	}
	minY, minZ := math.Inf(1), math.Inf(1)
	for _, p := range chamfer {
		minY = math.Min(minY, p.Y)
		minZ = math.Min(minZ, p.Z)
	}
	b.TopSetback, b.SideSetback = b.Width-minY, b.Height-minZ
	var features LongEdgeChainBody
	if !full {
		return b, features, nil
	}
	bbox := func(indices []int) (geom.Vec3, geom.Vec3, error) {
		p, e := points(indices)
		if e != nil || len(p) == 0 {
			return geom.Vec3{}, geom.Vec3{}, geom.ErrDomain
		}
		lo, hi := p[0], p[0]
		for _, v := range p[1:] {
			lo.X = math.Min(lo.X, v.X)
			lo.Y = math.Min(lo.Y, v.Y)
			lo.Z = math.Min(lo.Z, v.Z)
			hi.X = math.Max(hi.X, v.X)
			hi.Y = math.Max(hi.Y, v.Y)
			hi.Z = math.Max(hi.Z, v.Z)
		}
		return lo, hi, nil
	}
	lo, hi, e := bbox([]int{8, 9, 10, 11})
	if e != nil || lo.Z != 0 || hi.Z != b.Height {
		return fail()
	}
	features.Pocket = trimmed.ExtrusionRectangle{Min: geom.Vec2{X: lo.X, Y: lo.Y}, Max: geom.Vec2{X: hi.X, Y: hi.Y}}
	for i := 12; i < 16; i++ {
		face := faces[i]
		if face.kind != "CYLINDER" {
			return fail()
		}
		circle := trimmed.ExtrusionCircle{Center: geom.Vec2{X: face.frame.Origin().X, Y: face.frame.Origin().Y}, Radius: face.radius}
		if i == 12 {
			features.Bore = circle
		} else if features.Bore != circle {
			return fail()
		}
	}
	lo, hi, e = bbox([]int{16, 17, 18, 19, 20})
	if e != nil || lo.Z != b.Height {
		return fail()
	}
	features.Boss = trimmed.ExtrusionBoss{Footprint: trimmed.ExtrusionRectangle{Min: geom.Vec2{X: lo.X, Y: lo.Y}, Max: geom.Vec2{X: hi.X, Y: hi.Y}}, Height: hi.Z - lo.Z, Upper: true}
	return b, features, nil
}
