package plaincadbridge

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"strings"

	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

const maxSTEPBytes = 1 << 20 // Also bounded by the bridge request envelope.
func stepLimits() step.Limits {
	return step.Limits{MaxBytes: maxSTEPBytes, MaxEntities: 50000, MaxValues: 200000, MaxVertices: 20000, MaxFaces: 10000, MaxEdgeUses: 50000, MaxSolids: 32}
}
func (b *Bridge) importSTEP(source string) (Shape, error) {
	if len(source) == 0 || len(source) > maxSTEPBytes {
		return Shape{}, &failureError{"LIMIT", "STEP source must contain between 1 byte and 1 MiB"}
	}
	r := func() *strings.Reader { return strings.NewReader(source) }
	if m, e := step.Read(r(), stepLimits()); e == nil {
		if len(m.Bodies) != 1 {
			return Shape{}, &failureError{"UNSUPPORTED", "STEP import requires exactly one solid"}
		}
		s := m.Bodies[0].Solid
		scale := m.LengthUnit / .001
		if scale != 1 {
			s, e = s.Transformed(geom.Affine{{scale, 0, 0, 0}, {0, scale, 0, 0}, {0, 0, scale, 0}})
			if e != nil {
				return Shape{}, e
			}
		}
		return b.put(s)
	}
	// Canonical and independently authored full-circle cap graphs use distinct
	// strict readers. Both prove the complete graph before producing a recipe.
	readers := []func() (step.CircularHoleExtrusionModel, error){
		func() (step.CircularHoleExtrusionModel, error) {
			return step.ReadCircularHoleExtrusions(r(), stepLimits())
		},
		func() (step.CircularHoleExtrusionModel, error) {
			return step.ReadAnalyticCircularThroughExtrusions(r(), stepLimits())
		},
		func() (step.CircularHoleExtrusionModel, error) {
			return step.ReadAnalyticCircularPocketExtrusions(r(), stepLimits())
		},
	}
	for _, read := range readers {
		m, e := read()
		if e != nil {
			continue
		}
		if len(m.Bodies) != 1 {
			return Shape{}, &failureError{"UNSUPPORTED", "STEP import requires exactly one solid"}
		}
		p := m.Bodies[0]
		if m.LengthUnit == .001 {
			return b.putSTEPCircularRecipe(p)
		}
		s, e := trimmed.ExtrudePolygonWithHoles(context.Background(), p.Frame, p.Outer, p.Holes, p.Blind, p.Height)
		if e != nil {
			return Shape{}, e
		}
		return b.putSTEPAnalytic(s, m.LengthUnit)
	}
	if m, e := step.ReadCapsuleExtrusions(r(), stepLimits()); e == nil && len(m.Bodies) == 1 {
		p := m.Bodies[0]
		var s trimmed.Solid
		if p.InnerRadius == 0 {
			s, e = trimmed.ExtrudeCapsule(context.Background(), p.Frame, p.Left, p.Right, p.Radius, p.Height)
		} else {
			s, e = trimmed.ExtrudeCapsuleRing(context.Background(), p.Frame, p.Left, p.Right, p.Radius, p.InnerRadius, p.Height)
		}
		if e != nil {
			return Shape{}, e
		}
		return b.putSTEPAnalytic(s, m.LengthUnit)
	}

	m, e := step.ReadTrimmedBodies(r(), stepLimits())
	if e == nil && len(m.Bodies) == 1 {
		return b.putSTEPAnalytic(m.Bodies[0].Solid, m.LengthUnit)
	}
	return Shape{}, &failureError{"UNSUPPORTED", "STEP source is not a supported complete planar or analytic solid"}
}
func (b *Bridge) putSTEPAnalytic(s trimmed.Solid, unit float64) (Shape, error) {
	scale := unit / .001
	if !finite(scale) || scale <= 0 {
		return Shape{}, errors.New("invalid STEP length unit")
	}
	if scale != 1 {
		var e error
		s, e = s.Transformed(geom.Affine{{scale, 0, 0, 0}, {0, scale, 0, 0}, {0, 0, scale, 0}})
		if e != nil {
			return Shape{}, e
		}
	}
	m, e := s.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-10})
	if e != nil {
		return Shape{}, e
	}
	return b.putAnalytic(s, m.Volume, m.Area)
}

// writeSTEPBody emits the actual supported boundary, never a tessellated
// replacement. Additional recipe writers can extend this bounded dispatch.
func writeSTEPBody(s stored) ([]byte, error) {
	var out bytes.Buffer
	var e error
	planar, isPlanar, err := s.planarCutBoundary()
	if err != nil {
		return nil, err
	}
	switch {
	case isPlanar:
		e = step.Write(&out, step.Model{LengthUnit: .001, Bodies: []step.Body{{Name: "PlainSolid", Solid: planar}}})
	case s.cylinder != nil:
		c := s.cylinder
		e = step.WriteCylinders(&out, step.CylinderModel{LengthUnit: .001, Bodies: []step.CylinderBody{{Name: "PlainSolid", Frame: c.frame, Radius: c.radius, Height: c.height}}})
	case s.bore != nil:
		var p step.CircularHoleExtrusionBody
		p, e = boreSTEPRecipe(s.bore)
		if e == nil {
			e = step.WriteCircularHoleExtrusions(&out, step.CircularHoleExtrusionModel{LengthUnit: .001, Bodies: []step.CircularHoleExtrusionBody{p}})
		}
	case s.analytic != nil:
		e = step.WriteCapsuleBoundary(&out, s.analytic.solid, .001, "PlainSolid")
	default:
		return nil, &failureError{"UNSUPPORTED", "STEP export has no qualified writer for this retained analytic boundary"}
	}
	if e != nil {
		return nil, fmt.Errorf("STEP export: %w", e)
	}
	if out.Len() > maxSTEPBytes {
		return nil, &failureError{"LIMIT", "STEP export exceeds 1 MiB"}
	}
	return out.Bytes(), nil
}

func boreSTEPRecipe(a *boreBody) (step.CircularHoleExtrusionBody, error) {
	if a.union != nil || a.layered != nil || len(a.cylinders) == 0 {
		return step.CircularHoleExtrusionBody{}, &failureError{"UNSUPPORTED", "STEP overlapping bore export is not qualified"}
	}
	c := a.cylinders[0]
	cut, start, end, e := boreSpan(a.base, c, false)
	if e != nil {
		return step.CircularHoleExtrusionBody{}, e
	}
	frame, e := geom.NewFrame(c.frame.Origin().Add(c.frame.ZAxis().Scale(start)), c.frame.XAxis(), c.frame.ZAxis())
	if e != nil {
		return step.CircularHoleExtrusionBody{}, e
	}
	p := step.CircularHoleExtrusionBody{Name: "PlainSolid", Frame: frame, Height: end - start}
	edges, vertices := a.base.Edges(), a.base.Vertices()
	for _, u := range a.base.Faces()[cut.Entry].Boundary {
		edge := edges[u.Edge]
		id := edge.Start
		if u.Reversed {
			id = edge.End
		}
		v := frame.ToLocal(vertices[id])
		if v.Z != 0 || frame.ToWorld(v) != vertices[id] {
			return p, geom.ErrUnrepresentable
		}
		p.Outer = append(p.Outer, geom.Vec2{X: v.X, Y: v.Y})
	}
	for i, tool := range a.cylinders {
		if tool.frame.ZAxis() != frame.ZAxis() || a.cuts[i].Entry != cut.Entry || a.cuts[i].Exit != cut.Exit {
			return p, geom.ErrDomain
		}
		v := frame.ToLocal(tool.frame.Origin())
		hole := trimmed.ExtrusionCircle{Center: geom.Vec2{X: v.X, Y: v.Y}, Radius: tool.radius}
		if a.blind[i] {
			p.Blind = append(p.Blind, trimmed.ExtrusionBlindHole{ExtrusionCircle: hole, Depth: a.spans[i]})
		} else {
			p.Holes = append(p.Holes, hole)
		}
	}
	return p, nil
}

type stepProof struct {
	Assertions
	Bounds Bounds `json:"bounds"`
}
type stepExport struct {
	Base64 string      `json:"base64"`
	Before []stepProof `json:"before"`
	After  []stepProof `json:"after"`
	Units  string      `json:"units"`
}

func (b *Bridge) exportSTEP(handles []string) (stepExport, error) {
	fail := func(e error) (stepExport, error) { return stepExport{}, e }
	if len(handles) == 0 || len(handles) > 32 {
		return fail(&failureError{"LIMIT", "STEP export requires 1 to 32 bodies"})
	}
	out := stepExport{Units: "mm"}
	sources := make([][]byte, len(handles))
	seen := map[string]bool{}
	for i, h := range handles {
		if seen[h] {
			return fail(errors.New("duplicate STEP body handle"))
		}
		seen[h] = true
		s, e := b.get(h)
		if e != nil {
			return fail(e)
		}
		sources[i], e = writeSTEPBody(s)
		if e != nil {
			return fail(e)
		}
		out.Before = append(out.Before, stepProof{s.result.GeometryAssertions, s.result.Bounds})
	}
	var data bytes.Buffer
	if e := step.JoinIndependentModels(&data, sources, stepLimits()); e != nil {
		return fail(e)
	}
	parts, e := step.SplitIndependentModels(bytes.NewReader(data.Bytes()), stepLimits())
	if e != nil {
		return fail(e)
	}
	if len(parts) != len(handles) {
		return fail(errors.New("STEP round trip changed body count"))
	}
	// Readback uses a private registry so no caller handles are published or lost.
	verify := New()
	for i, p := range parts {
		s, e := verify.importSTEP(string(p))
		if e != nil {
			return fail(e)
		}
		proof := stepProof{s.GeometryAssertions, s.Bounds}
		before := out.Before[i]
		close := func(a, b float64) bool {
			return math.Abs(a-b) <= math.Max(1e-7, math.Max(math.Abs(a), math.Abs(b))*1e-8)
		}
		if !close(proof.Volume, before.Volume) || !close(proof.SurfaceArea, before.SurfaceArea) {
			return fail(errors.New("STEP readback changed analytic measures"))
		}
		for j := 0; j < 3; j++ {
			if !close(proof.Bounds.Min[j], before.Bounds.Min[j]) || !close(proof.Bounds.Max[j], before.Bounds.Max[j]) {
				return fail(errors.New("STEP readback changed world bounds"))
			}
		}
		out.After = append(out.After, proof)
	}
	out.Base64 = base64.StdEncoding.EncodeToString(data.Bytes())
	return out, nil
}

func (b *Bridge) putSTEPCircularRecipe(p step.CircularHoleExtrusionBody) (Shape, error) {
	outer := make([]geom.Vec3, len(p.Outer))
	for i, v := range p.Outer {
		outer[i] = p.Frame.ToWorld(geom.Vec3{X: v.X, Y: v.Y})
	}
	base, e := brep.Prism(outer, p.Frame.ZAxis().Scale(p.Height))
	if e != nil {
		return Shape{}, e
	}
	var tools []*cylinderBody
	add := func(c trimmed.ExtrusionCircle, depth float64) error {
		axis := p.Frame.ZAxis()
		if p.Height < 0 {
			axis = axis.Scale(-1)
		}
		frame, e := geom.NewFrame(p.Frame.ToWorld(geom.Vec3{X: c.Center.X, Y: c.Center.Y}), p.Frame.XAxis(), axis)
		if e != nil {
			return e
		}
		tool, e := newCylinder(frame, c.Radius, depth)
		if e != nil {
			return e
		}
		tools = append(tools, tool)
		return nil
	}
	for _, h := range p.Holes {
		if e = add(h, math.Abs(p.Height)); e != nil {
			return Shape{}, e
		}
	}
	for _, h := range p.Blind {
		if e = add(h.ExtrusionCircle, h.Depth); e != nil {
			return Shape{}, e
		}
	}
	body, e := newBore(base, tools)
	if e != nil {
		return Shape{}, e
	}
	return b.putBore(body)
}
