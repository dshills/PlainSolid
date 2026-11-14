package step

import (
	"bytes"
	"context"
	"io"
	"math"
	"sort"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// SideOpeningPairBody retains two overlapping finite cylindrical openings on
// the front side of a rectangular stock. Both cuts enter the lower cap;
// Depth=Length opens both caps. Dimensions and centers are in Frame coordinates.
type SideOpeningPairBody struct {
	Name                string
	Frame               geom.Frame
	Width, Back, Length float64
	Cuts                [2]trimmed.SideOpeningCut
}

type SideOpeningPairModel struct {
	Body       SideOpeningPairBody
	LengthUnit float64
}

func (p SideOpeningPairBody) Solid(ctx context.Context) (trimmed.Solid, error) {
	result, err := trimmed.BoxSideOpeningPair(ctx, p.Frame, p.Width, p.Back, p.Length, p.Cuts, 1e-9)
	return result.Solid, err
}

// WriteSideOpeningPair publishes only a graph accepted by the canonical reader.
func WriteSideOpeningPair(w io.Writer, m SideOpeningPairModel) error {
	if w == nil {
		return geom.ErrInvalid
	}
	var out bytes.Buffer
	if err := writeSideOpeningPairBody(&out, m); err != nil {
		return err
	}
	if _, err := ReadSideOpeningPair(bytes.NewReader(out.Bytes()), Limits{}); err != nil {
		return err
	}
	_, err := io.Copy(w, &out)
	return err
}
func writeSideOpeningPairBody(w io.Writer, m SideOpeningPairModel) error {
	s, err := m.Body.Solid(context.Background())
	if err != nil {
		return err
	}
	return writeNativeLongEdgeTopology(w, m.LengthUnit, m.Body.Name, s)
}

// ReadSideOpeningPair recovers dimensions and cutter spans from the actual
// model supports, then verifies every entity and edge use by regeneration.
func ReadSideOpeningPair(r io.Reader, limits Limits) (SideOpeningPairModel, error) {
	f, rep, unit, ids, err := parseCanonicalCircularExtrusions(r, limits)
	if err != nil || len(ids) != 1 {
		return SideOpeningPairModel{}, circularSourceFailure()
	}
	p, err := recoverSideOpeningPair(f, ids[0])
	if err != nil {
		return SideOpeningPairModel{}, err
	}
	result := SideOpeningPairModel{Body: p, LengthUnit: unit}
	var out bytes.Buffer
	if err := writeSideOpeningPairBody(&out, result); err != nil {
		return SideOpeningPairModel{}, err
	}
	if err := verifyCanonicalCircularExtrusionGraph(f, rep, &out); err != nil {
		return SideOpeningPairModel{}, err
	}
	return result, nil
}
func recoverSideOpeningPair(f *File, id int64) (SideOpeningPairBody, error) {
	fail := func() (SideOpeningPairBody, error) { return SideOpeningPairBody{}, circularSourceFailure() }
	g, err := readCircularGraph(f, id, true)
	if err != nil || len(g.faces) < 8 || len(g.faces) > 128 || g.faces[0].kind != "PLANE" || g.faces[1].kind != "PLANE" {
		return fail()
	}
	frame := g.faces[0].frame
	end := frame.ToLocal(g.faces[1].frame.Origin())
	if end.X != 0 || end.Y != 0 || end.Z <= 0 {
		return fail()
	}
	var vertices []geom.Vec3
	for _, edge := range g.edges {
		for _, vertex := range []int64{edge.a, edge.b} {
			p, err := analyticVertex(f, vertex)
			if err != nil {
				return fail()
			}
			vertices = append(vertices, frame.ToLocal(p))
		}
	}
	bounds, err := geom.Bounds(vertices)
	if err != nil || bounds.Min != (geom.Vec3{}) || bounds.Max.X <= 0 || bounds.Max.Y <= 0 || bounds.Max.Z != end.Z {
		return fail()
	}
	body, err := simple(f, id, "MANIFOLD_SOLID_BREP", 2)
	if err != nil {
		return fail()
	}
	name, err := textValue(body.Args[0])
	if err != nil {
		return fail()
	}
	result := SideOpeningPairBody{Name: name, Frame: frame, Width: bounds.Max.X, Back: bounds.Max.Y, Length: bounds.Max.Z}
	index := map[trimmed.ExtrusionCircle]int{}
	lows := [2]float64{math.Inf(1), math.Inf(1)}
	for _, face := range g.faces {
		if face.kind == "PLANE" {
			continue
		}
		if face.kind != "CYLINDER" || face.sense || face.frame.XAxis() != frame.XAxis() || face.frame.ZAxis() != frame.ZAxis() {
			return fail()
		}
		center := frame.ToLocal(face.frame.Origin())
		circle := trimmed.ExtrusionCircle{Center: geom.Vec2{X: center.X, Y: center.Y}, Radius: face.radius}
		i, ok := index[circle]
		if !ok {
			if len(index) == 2 {
				return fail()
			}
			i = len(index)
			index[circle] = i
			result.Cuts[i].ExtrusionCircle = circle
		}
		low, high := math.Inf(1), math.Inf(-1)
		for _, loop := range face.loops {
			for _, use := range loop {
				edge := g.edges[use.id]
				for _, v := range []int64{edge.a, edge.b} {
					p, err := analyticVertex(f, v)
					if err != nil {
						return fail()
					}
					z := frame.ToLocal(p).Z
					low = min(low, z)
					high = max(high, z)
				}
			}
		}
		if low < 0 || high <= low || high > result.Length {
			return fail()
		}
		lows[i] = min(lows[i], low)
		result.Cuts[i].Depth = max(result.Cuts[i].Depth, high)
	}
	if len(index) != 2 || lows[0] != 0 || lows[1] != 0 {
		return fail()
	}
	sort.Slice(result.Cuts[:], func(i, j int) bool {
		a, b := result.Cuts[i], result.Cuts[j]
		if a.Center.X != b.Center.X {
			return a.Center.X < b.Center.X
		}
		if a.Center.Y != b.Center.Y {
			return a.Center.Y < b.Center.Y
		}
		return a.Radius < b.Radius
	})
	return result, nil
}
