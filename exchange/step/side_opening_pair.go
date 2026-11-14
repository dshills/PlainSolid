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

// SideOpeningCutsBody retains bounded finite cylindrical side openings on
// rectangular stock. Each disk crosses exactly one side, which may differ
// between cuts, and each cut enters its selected end cap;
// Depth=Length opens both caps. Dimensions and centers are in Frame coordinates.
// Buried wall endpoints use adjacent floor orientation to recover a canonical
// interval; complete graph regeneration verifies hidden extensions. Completely
// occluded cutters have no recoverable support and are not retained as history.
type SideOpeningCutsBody struct {
	Name                string
	Frame               geom.Frame
	Width, Back, Length float64
	Cuts                []trimmed.SideOpeningCut
}

type SideOpeningCutsModel struct {
	Body       SideOpeningCutsBody
	LengthUnit float64
}

func (p SideOpeningCutsBody) Solid(ctx context.Context) (trimmed.Solid, error) {
	result, err := trimmed.BoxSideOpeningCuts(ctx, p.Frame, p.Width, p.Back, p.Length, p.Cuts, 1e-9)
	return result.Solid, err
}

// WriteSideOpeningCuts publishes only a graph accepted by the canonical reader.
func WriteSideOpeningCuts(w io.Writer, m SideOpeningCutsModel) error {
	if w == nil {
		return geom.ErrInvalid
	}
	var out bytes.Buffer
	if err := writeSideOpeningCutsBody(&out, m); err != nil {
		return err
	}
	if _, err := ReadSideOpeningCuts(bytes.NewReader(out.Bytes()), Limits{}); err != nil {
		return err
	}
	_, err := io.Copy(w, &out)
	return err
}
func writeSideOpeningCutsBody(w io.Writer, m SideOpeningCutsModel) error {
	s, err := m.Body.Solid(context.Background())
	if err != nil {
		return err
	}
	return writeNativeLongEdgeTopologyBounded(w, m.LengthUnit, m.Body.Name, s, sideOpeningTopologyLimits())
}

// ReadSideOpeningCuts recovers dimensions and cutter spans from the actual
// model supports, then verifies every entity and edge use by regeneration.
func ReadSideOpeningCuts(r io.Reader, limits Limits) (SideOpeningCutsModel, error) {
	f, rep, unit, ids, err := parseCanonicalCircularExtrusions(r, limits)
	if err != nil || len(ids) != 1 {
		return SideOpeningCutsModel{}, circularSourceFailure()
	}
	p, err := recoverSideOpeningCuts(f, ids[0])
	if err != nil {
		return SideOpeningCutsModel{}, err
	}
	result := SideOpeningCutsModel{Body: p, LengthUnit: unit}
	var out bytes.Buffer
	if err := writeSideOpeningCutsBody(&out, result); err != nil {
		return SideOpeningCutsModel{}, err
	}
	if err := verifyCanonicalCircularExtrusionGraph(f, rep, &out); err != nil {
		return SideOpeningCutsModel{}, err
	}
	return result, nil
}
func recoverSideOpeningCuts(f *File, id int64) (SideOpeningCutsBody, error) {
	fail := func() (SideOpeningCutsBody, error) { return SideOpeningCutsBody{}, circularSourceFailure() }
	limit := sideOpeningTopologyLimits()
	g, err := readCircularGraphBounded(f, id, true, limit.faces, limit.loops, limit.uses)
	if err != nil || len(g.faces) < 8 || len(g.faces) > limit.faces || g.faces[0].kind != "PLANE" {
		return fail()
	}
	frame := g.faces[0].frame
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
	if err != nil || bounds.Min != (geom.Vec3{}) || bounds.Max.X <= 0 || bounds.Max.Y <= 0 || bounds.Max.Z <= 0 {
		return fail()
	}
	// One cap may contain multiple disconnected planar patches even when
	// material reconnects behind a blind floor. Recover axial extent from all
	// model vertices, and require the canonical upper support before complete
	// graph regeneration verifies cap patch ownership and ordering.
	upper := false
	for _, face := range g.faces {
		if face.kind == "PLANE" && face.frame.XAxis() == frame.XAxis() && face.frame.ZAxis() == frame.ZAxis() && frame.ToLocal(face.frame.Origin()) == (geom.Vec3{Z: bounds.Max.Z}) {
			upper = true
		}
	}
	if !upper {
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
	result := SideOpeningCutsBody{Name: name, Frame: frame, Width: bounds.Max.X, Back: bounds.Max.Y, Length: bounds.Max.Z}
	index := map[trimmed.ExtrusionCircle]int{}
	var lows, highs []float64
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
			if len(index) == trimmed.MaxSideOpeningCuts {
				return fail()
			}
			i = len(index)
			index[circle] = i
			result.Cuts = append(result.Cuts, trimmed.SideOpeningCut{ExtrusionCircle: circle})
			lows = append(lows, math.Inf(1))
			highs = append(highs, math.Inf(-1))
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
		highs[i] = max(highs[i], high)
	}
	if len(index) < 2 {
		return fail()
	}
	for i := range result.Cuts {
		switch {
		case lows[i] == 0:
			result.Cuts[i].Depth = highs[i]
		case highs[i] == result.Length:
			result.Cuts[i].Depth = result.Length - lows[i]
			result.Cuts[i].OppositeCap = true
			if result.Length-result.Cuts[i].Depth != lows[i] {
				return fail()
			}
		default:
			cut, ok := recoverBuriedSideOpening(g, frame, result.Cuts[i].ExtrusionCircle, lows[i], highs[i], result.Length)
			if !ok {
				return fail()
			}
			result.Cuts[i] = cut
		}
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

// A lower-entry cutter ends against a -Z floor; an upper-entry cutter
// ends against a +Z floor. Oppositely oriented adjacent floors hide an endpoint
// inside material already removed by another cutter. No orientation combinations
// are searched: regeneration must prove the one canonical interval inferred here.
func recoverBuriedSideOpening(g circularSourceGraph, frame geom.Frame, circle trimmed.ExtrusionCircle, low, high, length float64) (trimmed.SideOpeningCut, bool) {
	lowerStop, upperStop := false, false
	for id, edge := range g.edges {
		if edge.kind != "CIRCLE" || edge.radius != circle.Radius {
			continue
		}
		p := frame.ToLocal(edge.frame.Origin())
		if p.X != circle.Center.X || p.Y != circle.Center.Y || (p.Z != low && p.Z != high) {
			continue
		}
		for _, owner := range g.owners[id] {
			face := g.faces[owner]
			if face.kind != "PLANE" {
				continue
			}
			normal := face.frame.ZAxis()
			if !face.sense {
				normal = normal.Scale(-1)
			}
			lowerStop = lowerStop || (p.Z == high && normal == frame.ZAxis().Scale(-1))
			upperStop = upperStop || (p.Z == low && normal == frame.ZAxis())
		}
	}
	cut := trimmed.SideOpeningCut{ExtrusionCircle: circle}
	switch {
	case lowerStop && upperStop:
		return cut, false
	case lowerStop:
		cut.Depth = high
	case upperStop:
		cut.Depth = length - low
		cut.OppositeCap = true
		if length-cut.Depth != low {
			return cut, false
		}
	default:
		cut.Depth = length
	}
	return cut, true
}

// Circle/rectangle arrangements contain O(n²) arcs and regions and at most
// n+1 axial slabs. These conservative bounds are local to this constructor;
// ordinary STEP parser limits and all legacy codec budgets remain unchanged.
func sideOpeningTopologyLimits() nativeLongEdgeLimits {
	n := trimmed.MaxSideOpeningCuts
	arcs := 2*n*n + 8*n + 8
	faces := (n + 2) * (arcs + n*n + 2)
	return nativeLongEdgeLimits{faces: faces, edges: 4 * faces, vertices: 4 * faces, loops: n*n + 2, uses: arcs}
}

// SideOpeningPairBody is the two-cutter compatibility form of SideOpeningCutsBody.
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

func (p SideOpeningPairBody) cutsBody() SideOpeningCutsBody {
	return SideOpeningCutsBody{Name: p.Name, Frame: p.Frame, Width: p.Width, Back: p.Back, Length: p.Length, Cuts: append([]trimmed.SideOpeningCut(nil), p.Cuts[:]...)}
}
func (p SideOpeningPairBody) Solid(ctx context.Context) (trimmed.Solid, error) {
	return p.cutsBody().Solid(ctx)
}
func WriteSideOpeningPair(w io.Writer, m SideOpeningPairModel) error {
	return WriteSideOpeningCuts(w, SideOpeningCutsModel{Body: m.Body.cutsBody(), LengthUnit: m.LengthUnit})
}
func ReadSideOpeningPair(r io.Reader, limits Limits) (SideOpeningPairModel, error) {
	m, e := ReadSideOpeningCuts(r, limits)
	if e != nil {
		return SideOpeningPairModel{}, e
	}
	if len(m.Body.Cuts) != 2 {
		return SideOpeningPairModel{}, circularSourceFailure()
	}
	p := m.Body
	return SideOpeningPairModel{Body: SideOpeningPairBody{Name: p.Name, Frame: p.Frame, Width: p.Width, Back: p.Back, Length: p.Length, Cuts: [2]trimmed.SideOpeningCut{p.Cuts[0], p.Cuts[1]}}, LengthUnit: m.LengthUnit}, nil
}
