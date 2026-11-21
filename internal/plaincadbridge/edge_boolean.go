package plaincadbridge

import (
	"errors"
	"math"

	"plainsolid/brep"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func cardinalFrame(f geom.Frame) bool {
	for _, v := range []geom.Vec3{f.XAxis(), f.YAxis(), f.ZAxis()} {
		ones := 0
		for _, x := range []float64{v.X, v.Y, v.Z} {
			if x == 1 || x == -1 {
				ones++
			} else if x != 0 {
				return false
			}
		}
		if ones != 1 {
			return false
		}
	}
	return true
}
func (body *edgeBody) rectangleTool(s brep.Solid) (geom.Vec3, geom.Vec3, error) {
	fail := func() (geom.Vec3, geom.Vec3, error) {
		return geom.Vec3{}, geom.Vec3{}, unsupportedEdge("treated-stock rectangular tool must be an aligned orthogonal prism")
	}
	if len(s.Vertices()) != 8 || len(s.Edges()) != 12 || len(s.Faces()) != 6 {
		return fail()
	}
	for _, f := range s.Faces() {
		if len(f.Boundary) != 4 || len(f.Holes) != 0 {
			return fail()
		}
	}
	var lo, hi geom.Vec3
	for i, p := range s.Vertices() {
		q := body.frame.ToLocal(p)
		if i == 0 {
			lo, hi = q, q
		}
		lo.X = math.Min(lo.X, q.X)
		lo.Y = math.Min(lo.Y, q.Y)
		lo.Z = math.Min(lo.Z, q.Z)
		hi.X = math.Max(hi.X, q.X)
		hi.Y = math.Max(hi.Y, q.Y)
		hi.Z = math.Max(hi.Z, q.Z)
	}
	if hi.X <= lo.X || hi.Y <= lo.Y || hi.Z <= lo.Z {
		return fail()
	}
	for _, p := range s.Vertices() {
		q := body.frame.ToLocal(p)
		if (q.X != lo.X && q.X != hi.X) || (q.Y != lo.Y && q.Y != hi.Y) || (q.Z != lo.Z && q.Z != hi.Z) || body.frame.ToWorld(q) != p {
			return fail()
		}
	}
	if !rectangularFaces(s, body.frame, lo, hi) {
		return fail()
	}
	return lo, hi, nil
}

// treatedCut builds the candidate from analytic recipes; no handles are created.
// The returned axial interval is useful for a measure-only overlap query.
func (body *edgeBody) cutCandidate(tool stored, through bool) (*edgeBody, float64, float64, error) {
	next := *body
	if tool.cylinder != nil {
		c := tool.cylinder
		axis := body.frame.ZAxis()
		if c.frame.ZAxis() != axis && c.frame.ZAxis() != axis.Scale(-1) {
			return nil, 0, 0, unsupportedEdge("treated-stock bore requires the stock sweep axis")
		}
		if !cardinalFrame(c.frame) {
			return nil, 0, 0, unsupportedEdge("treated-stock cylinder requires a represented cardinal frame")
		}
		o := body.frame.ToLocal(c.frame.Origin())
		end := body.frame.ToLocal(c.frame.Origin().Add(c.frame.ZAxis().Scale(c.height)))
		if body.frame.ToWorld(o) != c.frame.Origin() || body.frame.ToWorld(end) != c.frame.Origin().Add(c.frame.ZAxis().Scale(c.height)) {
			return nil, 0, 0, unsupportedEdge("cylinder placement cannot preserve represented tool endpoints")
		}
		lo, hi := math.Min(o.Z, end.Z), math.Max(o.Z, end.Z)
		if through && (lo > 0 || hi < body.height) {
			return nil, 0, 0, unsupportedEdge("treated-stock bore requires a finite through tool")
		}
		feature := trimmed.ExtrusionCircle{Center: geom.Vec2{X: o.X, Y: o.Y}, Radius: c.radius}
		if next.bore != nil {
			if *next.bore == feature {
				return &next, lo, hi, nil
			}
			return nil, 0, 0, unsupportedEdge("treated stock currently supports one disjoint cylindrical bore")
		}
		next.bore = &feature
		result, err := rebuildEdge(next)
		return result, lo, hi, err
	}
	if tool.treated != nil {
		return nil, 0, 0, unsupportedEdge("treated-stock cut tool must be a cylinder or rectangular prism")
	}
	lo, hi, err := body.rectangleTool(tool.solid)
	if err != nil {
		return nil, 0, 0, err
	}
	if through && (lo.Z > 0 || hi.Z < body.height) {
		return nil, 0, 0, unsupportedEdge("treated-stock rectangular pocket requires a finite through tool")
	}
	feature := trimmed.ExtrusionRectangle{Min: geom.Vec2{X: lo.X, Y: lo.Y}, Max: geom.Vec2{X: hi.X, Y: hi.Y}}
	if next.pocket != nil {
		if *next.pocket == feature {
			return &next, lo.Z, hi.Z, nil
		}
		return nil, 0, 0, unsupportedEdge("treated stock currently supports one disjoint rectangular pocket")
	}
	next.pocket = &feature
	result, err := rebuildEdge(next)
	return result, lo.Z, hi.Z, err
}
func (b *Bridge) treatedBoolean(a, c stored, op string) (any, error) {
	if a.treated == nil {
		return nil, unsupportedEdge("treated-stock operation requires the retained stock as first operand")
	}
	body := a.treated
	if op == "cut" {
		next, _, _, err := body.cutCandidate(c, true)
		if err != nil {
			return nil, err
		}
		if !removesMaterial(body.chain.NominalVolume, next.chain.NominalVolume) {
			return nil, unsupportedEdge("treated-stock cut removed no material")
		}
		return b.putEdge(next)
	}
	if op != "fuse" || c.cylinder != nil || c.treated != nil {
		return nil, unsupportedEdge("treated-stock Boolean combination is outside the qualified domain")
	}
	if body.boss != nil {
		return nil, unsupportedEdge("treated stock currently supports one upper rectangular boss")
	}
	lo, hi, err := body.rectangleTool(c.solid)
	if err != nil {
		return nil, err
	}
	if lo.Z != body.height || hi.Z <= body.height {
		return nil, unsupportedEdge("boss must meet the retained upper cap exactly")
	}
	if body.height+(hi.Z-lo.Z) != hi.Z {
		return nil, unsupportedEdge("boss height cannot preserve represented endpoint")
	}
	next := *body
	next.boss = &trimmed.ExtrusionBoss{Footprint: trimmed.ExtrusionRectangle{Min: geom.Vec2{X: lo.X, Y: lo.Y}, Max: geom.Vec2{X: hi.X, Y: hi.Y}}, Height: hi.Z - lo.Z, Upper: true}
	result, err := rebuildEdge(next)
	if err != nil {
		return nil, err
	}
	return b.putEdge(result)
}
func (b *Bridge) cutManyTreated(a stored, tools []stored) (Shape, error) {
	body := a.treated
	for _, tool := range tools {
		next, _, _, err := body.cutCandidate(tool, true)
		if err != nil {
			return Shape{}, err
		}
		body = next
	}
	if !removesMaterial(a.result.GeometryAssertions.Volume, body.chain.NominalVolume) {
		return Shape{}, errors.New("cutMany must collectively remove material")
	}
	return b.putEdge(body)
}
func treatedOverlap(a, c stored) (float64, error) {
	if a.treated == nil {
		a, c = c, a
	}
	if a.treated == nil || c.treated != nil {
		return 0, unsupportedEdge("unsupported treated-stock overlap combination")
	}
	body := a.treated
	candidate, lo, hi, err := body.cutCandidate(c, false)
	if err != nil {
		return 0, err
	}
	// A qualified footprint is disjoint from the retained boss. The core
	// constructor re-proves separation, including when a new hole is queried.
	depth := math.Max(0, math.Min(body.height, hi)-math.Max(0, lo))
	removed := body.chain.NominalVolume - candidate.chain.NominalVolume
	if removed < 0 {
		return 0, errors.New("invalid treated-stock overlap volume")
	}
	return removed * depth / body.height, nil
}

// Certify six distinct cardinal support planes, each with four unique corners
// and axis-aligned boundary edges; corner membership alone is insufficient.
func rectangularFaces(s brep.Solid, f geom.Frame, lo, hi geom.Vec3) bool {
	vertices, edges := s.Vertices(), s.Edges()
	used := [6]bool{}
	mins, maxs := [3]float64{lo.X, lo.Y, lo.Z}, [3]float64{hi.X, hi.Y, hi.Z}
	for _, face := range s.Faces() {
		if len(face.Boundary) != 4 || len(face.Holes) != 0 {
			return false
		}
		points := make([]geom.Vec3, 4)
		unique := map[geom.Vec3]bool{}
		for i, u := range face.Boundary {
			id := edges[u.Edge].Start
			if u.Reversed {
				id = edges[u.Edge].End
			}
			points[i] = f.ToLocal(vertices[id])
			unique[points[i]] = true
		}
		if len(unique) != 4 {
			return false
		}
		plane := -1
		for axis := 0; axis < 3; axis++ {
			for side, value := range []float64{mins[axis], maxs[axis]} {
				same := true
				for _, p := range points {
					q := [3]float64{p.X, p.Y, p.Z}
					same = same && q[axis] == value
				}
				if same {
					if plane >= 0 {
						return false
					}
					plane = axis*2 + side
				}
			}
		}
		if plane < 0 || used[plane] {
			return false
		}
		used[plane] = true
		for i, a := range points {
			b := points[(i+1)%4]
			different := 0
			for _, d := range []float64{a.X - b.X, a.Y - b.Y, a.Z - b.Z} {
				if d != 0 {
					different++
				}
			}
			if different != 1 {
				return false
			}
		}
	}
	for _, yes := range used {
		if !yes {
			return false
		}
	}
	return true
}
