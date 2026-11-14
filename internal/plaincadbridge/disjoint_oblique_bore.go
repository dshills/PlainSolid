package plaincadbridge

import (
	"math"
	"sort"

	"plainsolid/brep"
	"plainsolid/brep/trimmed"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

type disjointObliqueGeometry struct {
	removed, areaDelta float64
	openings           [6]float64
}

func newDisjointObliqueBore(base brep.Solid, tools []*cylinderBody) (*boreBody, bool, error) {
	if len(tools) < 2 || len(tools) > 3 || len(base.Faces()) != 6 {
		return nil, false, nil
	}
	oblique := 0
	for _, tool := range tools {
		if !cardinalFrame(tool.frame) {
			oblique++
		}
	}
	if oblique == 0 {
		return nil, false, nil
	}
	world, err := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if err != nil {
		return nil, true, err
	}
	box, ok := orthogonalPrism(base, world)
	if !ok || !rectangularFaces(base, world, box.Min, box.Max) {
		return nil, false, nil
	}
	cylinders := append([]*cylinderBody(nil), tools...)
	// Sort by the physical axis station on the opening plane after qualification.
	type site struct {
		tool   *cylinderBody
		cut    trimmed.CylinderCut
		length float64
		areas  [2]float64
	}
	sites := make([]site, len(cylinders))
	measure := disjointObliqueGeometry{}
	for i, tool := range cylinders {
		p := site{tool: tool}
		if cardinalFrame(tool.frame) {
			c, lo, hi, err := boreSpan(base, tool, true)
			if err != nil {
				return nil, true, err
			}
			p.cut, p.length = c, hi-lo
			disk := math.Pi * tool.radius * tool.radius
			p.areas = [2]float64{disk, disk}
		} else {
			single, handled, err := newObliqueThroughBore(base, tool)
			if err != nil {
				return nil, true, err
			}
			if !handled {
				return nil, false, nil
			}
			p.cut, p.length, p.areas = single.cuts[0], single.spans[0], single.oblique.openingAreas
		}
		f, err := base.FacePlane(p.cut.Entry)
		if err != nil {
			return nil, true, err
		}
		axis := p.cut.Direction
		t := -f.SignedDistance(p.cut.AxisPoint) / f.Normal().Dot(axis)
		p.cut.AxisPoint = p.cut.AxisPoint.Add(axis.Scale(t))
		// Anchor the represented point to its support plane without
		// changing either transverse coordinate.
		anchor := coordinates(p.cut.AxisPoint)
		origin := coordinates(f.Origin())
		for k, n := range coordinates(f.Normal()) {
			if math.Abs(n) == 1 {
				anchor[k] = origin[k]
			}
		}
		p.cut.AxisPoint = anchor.vec()
		sites[i] = p
	}
	sort.Slice(sites, func(i, j int) bool {
		a, b := sites[i].cut, sites[j].cut
		if a.Entry != b.Entry {
			return a.Entry < b.Entry
		}
		if a.AxisPoint.X != b.AxisPoint.X {
			return a.AxisPoint.X < b.AxisPoint.X
		}
		if a.AxisPoint.Y != b.AxisPoint.Y {
			return a.AxisPoint.Y < b.AxisPoint.Y
		}
		if a.AxisPoint.Z != b.AxisPoint.Z {
			return a.AxisPoint.Z < b.AxisPoint.Z
		}
		return a.Radius < b.Radius
	})
	cuts := make([]trimmed.CylinderCut, len(sites))
	spans := make([]float64, len(sites))
	for i, p := range sites {
		cuts[i], spans[i], cylinders[i] = p.cut, p.length, p.tool
		disk := math.Pi * p.cut.Radius * p.cut.Radius
		measure.removed += disk * p.length
		measure.areaDelta += 2*math.Pi*p.cut.Radius*p.length - p.areas[0] - p.areas[1]
		measure.openings[p.cut.Entry] += p.areas[0]
		measure.openings[p.cut.Exit] += p.areas[1]
	}
	s, _, err := trimmed.BoreCylinders(base, cuts, 1e-9)
	if err != nil {
		return nil, true, err
	}
	return &boreBody{base: base, solid: s, cylinders: cylinders, cuts: cuts, spans: spans, blind: make([]bool, len(sites)), disjointOblique: &measure}, true, nil
}

func overlapDisjointObliqueBore(a stored, tool *cylinderBody) (float64, bool, error) {
	b := a.bore
	if b.obliqueCrossing != nil || b.crossed != nil || b.union != nil || b.layered != nil ||
		b.disjointOblique == nil && b.oblique == nil && cardinalFrame(tool.frame) {
		return 0, false, nil
	}
	for i, old := range b.cylinders {
		if !b.blind[i] && old.radius >= tool.radius &&
			(sameCardinalBoreLine(old, tool) || old.frame.Origin() == tool.frame.Origin() &&
				(old.frame.ZAxis() == tool.frame.ZAxis() || old.frame.ZAxis() == tool.frame.ZAxis().Scale(-1))) {
			return 0, true, nil
		}
	}
	tools := append(append([]*cylinderBody(nil), b.cylinders...), tool)
	body, handled, err := newDisjointObliqueBore(b.base, tools)
	if !handled || err != nil {
		return 0, handled, err
	}
	volume, _, err := body.measures()
	if err != nil {
		return 0, true, err
	}
	delta := a.result.GeometryAssertions.Volume - volume
	tolerance := math.Max(1e-9, math.Abs(a.result.GeometryAssertions.Volume)*1e-10)
	if !finite(delta) || delta < -tolerance || delta > min(a.result.GeometryAssertions.Volume, math.Pi*tool.radius*tool.radius*tool.height)+tolerance {
		return 0, true, geom.ErrUnrepresentable
	}
	return math.Max(0, delta), true, nil
}

func disjointObliqueSTEPRecipe(b *boreBody) (step.DisjointObliqueBoreModel, error) {
	m := step.DisjointObliqueBoreModel{Name: "PlainSolid", LengthUnit: .001}
	var err error
	m.Stock, err = b.base.Bounds()
	if err != nil {
		return m, err
	}
	box, err := brep.Box(m.Stock.Min, m.Stock.Max)
	if err != nil {
		return m, err
	}
	for _, cut := range b.cuts {
		for _, id := range []*int{&cut.Entry, &cut.Exit} {
			from, err := b.base.FacePlane(*id)
			if err != nil {
				return m, err
			}
			*id = -1
			for i := range box.Faces() {
				to, err := box.FacePlane(i)
				if err == nil && to.Normal() == from.Normal() && to.SignedDistance(from.Origin()) == 0 {
					*id = i
					break
				}
			}
			if *id < 0 {
				return m, geom.ErrUnrepresentable
			}
		}
		m.Cuts = append(m.Cuts, cut)
	}
	return m, nil
}

func disjointObliqueFromSTEP(m step.DisjointObliqueBoreModel) (*boreBody, error) {
	scale := m.LengthUnit / .001
	if !finite(scale) || scale <= 0 {
		return nil, geom.ErrInvalid
	}
	base, err := brep.Box(m.Stock.Min.Scale(scale), m.Stock.Max.Scale(scale))
	if err != nil {
		return nil, err
	}
	var tools []*cylinderBody
	for _, cut := range m.Cuts {
		first, err := base.FacePlane(cut.Entry)
		if err != nil {
			return nil, err
		}
		last, err := base.FacePlane(cut.Exit)
		if err != nil {
			return nil, err
		}
		axis, err := cut.Direction.Unit()
		if err != nil {
			return nil, err
		}
		start := cut.AxisPoint.Scale(scale)
		denom := min(math.Abs(first.Normal().Dot(axis)), math.Abs(last.Normal().Dot(axis)))
		if denom == 0 {
			return nil, geom.ErrDomain
		}
		padding := 2 * cut.Radius * scale / denom
		if cardinalFrameFromAxis(axis) {
			padding = 0
		}
		length := -last.SignedDistance(start) / last.Normal().Dot(axis)
		frame, err := geom.FrameFromNormal(start.Sub(axis.Scale(padding)), cut.Direction)
		if err != nil {
			return nil, err
		}
		tool, err := newCylinder(frame, cut.Radius*scale, length+2*padding)
		if err != nil {
			return nil, err
		}
		tools = append(tools, tool)
	}
	b, handled, err := newDisjointObliqueBore(base, tools)
	if err != nil {
		return nil, err
	}
	if !handled {
		return nil, geom.ErrUnrepresentable
	}
	return b, nil
}

func cardinalFrameFromAxis(axis geom.Vec3) bool {
	return math.Abs(axis.X)+math.Abs(axis.Y)+math.Abs(axis.Z) == 1 &&
		(math.Abs(axis.X) == 1 || math.Abs(axis.Y) == 1 || math.Abs(axis.Z) == 1)
}
