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
	"plainsolid/csg/fillet"
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
	if m, e := step.ReadBoxPocketExtrusions(r(), stepLimits()); e == nil && len(m.Bodies) == 1 {
		p := m.Bodies[0]
		if m.LengthUnit == .001 {
			return b.putPocketStock(pocketStock{frame: p.Frame, size: p.Size, pockets: p.Pockets, bores: p.Bores})
		}
		solid, err := trimmed.ExtrudeBoxWithPocketsAndBores(context.Background(), p.Frame, p.Size, p.Pockets, p.Bores)
		if err != nil {
			return Shape{}, err
		}
		return b.putSTEPAnalytic(solid, m.LengthUnit)
	}
	diskReaders := []func() (step.DiskHoleExtrusionModel, error){
		func() (step.DiskHoleExtrusionModel, error) {
			return step.ReadDiskHoleExtrusions(r(), stepLimits())
		},
		func() (step.DiskHoleExtrusionModel, error) {
			return step.ReadAnalyticDiskHoleExtrusions(r(), stepLimits())
		},
	}
	for _, read := range diskReaders {
		if m, e := read(); e == nil && len(m.Bodies) == 1 {
			p := m.Bodies[0]
			s, e := trimmed.ExtrudeDiskWithHoles(context.Background(), p.Frame, p.Outer, p.Holes, p.Blind, p.Height)
			if e != nil {
				return Shape{}, e
			}
			if m.LengthUnit != .001 {
				return b.putSTEPAnalytic(s, m.LengthUnit)
			}
			properties, err := s.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-10})
			if err != nil {
				return Shape{}, err
			}
			return b.putAnalyticBody(&analyticBody{solid: s, volume: properties.Volume, area: properties.Area, stepDisk: &p})
		}
	}
	if m, e := step.ReadLayeredDiskBores(r(), stepLimits()); e == nil {
		p := m.Bodies[0]
		s, h, err := trimmed.BoreDiskDepthUnion(context.Background(), p.Frame, p.Outer, p.Holes, p.Blind, p.Height)
		if err != nil {
			return Shape{}, err
		}
		if m.LengthUnit != .001 {
			return b.putSTEPAnalytic(s, m.LengthUnit)
		}
		return b.putAnalyticBody(layeredCircularBody(s, p, h))
	}
	if m, e := step.ReadSideOpeningCuts(r(), stepLimits()); e == nil {
		body, err := sideOpeningCutsBody(m.Body)
		if err != nil {
			return Shape{}, err
		}
		if m.LengthUnit != .001 {
			return b.putSTEPAnalytic(body.solid, m.LengthUnit)
		}
		return b.putAnalyticBody(body)
	}
	if m, e := step.ReadDisjointBores(r(), stepLimits()); e == nil {
		body, err := disjointBoresFromSTEP(m.Body, m.LengthUnit)
		if err != nil {
			return Shape{}, err
		}
		return b.putBore(body)
	}
	if m, e := step.ReadCrossedBore(r(), stepLimits()); e == nil {
		p := m.Body
		if p.BlindDepth > 0 {
			body, err := blindCrossedFromSTEP(p, m.LengthUnit)
			if err != nil {
				return Shape{}, err
			}
			return b.putBore(body)
		}
		body, err := crossedBoreFromSTEP(crossedBoreGeometry{frame: p.Frame, dimensions: p.Dimensions, crossing: p.Crossing, radii: [2]float64{p.Radius, p.Radius}}, m.LengthUnit)
		if err != nil {
			return Shape{}, err
		}
		return b.putBore(body)
	}
	if m, e := step.ReadObliqueBore(r(), stepLimits()); e == nil {
		body, err := obliqueBoreFromSTEP(m.Body, m.LengthUnit)
		if err != nil {
			return Shape{}, err
		}
		return b.putBore(body)
	}
	if m, e := step.ReadUnequalBore(r(), stepLimits()); e == nil {
		p := m.Body
		body, err := crossedBoreFromSTEP(crossedBoreGeometry{frame: p.Frame, dimensions: p.Dimensions, crossing: p.Crossing, radii: [2]float64{p.XRadius, p.ZRadius}, offset: p.Offset}, m.LengthUnit)
		if err != nil {
			return Shape{}, err
		}
		return b.putBore(body)
	}
	if m, e := step.ReadSideNotch(r(), stepLimits()); e == nil {
		p := m.Body
		unit := m.LengthUnit
		if m.LengthUnit != .001 {
			scaled, err := scaleSTEPNotch(p, m.LengthUnit/.001)
			if err == nil {
				p, unit = scaled, .001
			}
		}
		s, err := p.Solid(context.Background())
		if err != nil && m.LengthUnit != .001 && unit == .001 {
			p, unit = m.Body, m.LengthUnit
			s, err = p.Solid(context.Background())
		}
		if err != nil {
			return Shape{}, err
		}
		if unit != .001 {
			return b.putSTEPAnalytic(s, unit)
		}
		properties, err := s.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-10})
		if err != nil {
			return Shape{}, err
		}
		return b.putAnalyticBody(&analyticBody{solid: s, sideNotch: &p, volume: properties.Volume, area: properties.Area})
	}
	roundedReaders := []func() (step.RoundedRectangleExtrusionModel, error){
		func() (step.RoundedRectangleExtrusionModel, error) {
			return step.ReadRoundedRectangleExtrusions(r(), stepLimits())
		},
		func() (step.RoundedRectangleExtrusionModel, error) {
			return step.ReadAnalyticRoundedPocketExtrusions(r(), stepLimits())
		},
	}
	for _, read := range roundedReaders {
		if m, e := read(); e == nil && len(m.Bodies) == 1 {
			p := m.Bodies[0]
			s, e := trimmed.ExtrudeRoundedRectangle(context.Background(), p.Frame, p.Profile, p.Holes, p.Blind, p.Height)
			if e != nil {
				return Shape{}, e
			}
			if m.LengthUnit != .001 {
				return b.putSTEPAnalytic(s, m.LengthUnit)
			}
			properties, err := s.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-10})
			if err != nil {
				return Shape{}, err
			}
			return b.putAnalyticBody(&analyticBody{solid: s, volume: properties.Volume, area: properties.Area, stepRounded: &p})
		}
	}
	if m, e := step.ReadLongEdgeChains(r(), stepLimits()); e == nil && len(m.Bodies) == 1 {
		p := m.Bodies[0]
		return b.putLongEdgeSTEP(p.LongEdgeTreatmentBody, &p.Bore, &p.Pocket, &p.Boss, m.LengthUnit)
	}
	if m, e := step.ReadLongEdgeTreatments(r(), stepLimits()); e == nil && len(m.Bodies) == 1 {
		return b.putLongEdgeSTEP(m.Bodies[0], nil, nil, nil, m.LengthUnit)
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

	if m, err := step.ReadSplitCylinderModel(r(), stepLimits()); err == nil {
		return b.putSTEPCylinder(m.Frame, m.Radius, m.Height, m.LengthUnit)
	}
	if m, err := step.ReadSplitCylinderProduct(r(), stepLimits()); err == nil {
		p := m.Model
		return b.putSTEPCylinder(p.Frame, p.Radius, p.Height, p.LengthUnit)
	}
	if m, err := step.ReadQuarterCylinderModel(r(), stepLimits()); err == nil {
		return b.putSTEPCylinder(m.Frame, m.Radius, m.Height, m.LengthUnit)
	}
	if m, err := step.ReadQuarterCylinderProduct(r(), stepLimits()); err == nil {
		p := m.Model
		return b.putSTEPCylinder(p.Frame, p.Radius, p.Height, p.LengthUnit)
	}
	m, e := step.ReadTrimmedBodies(r(), stepLimits())
	if e == nil && len(m.Bodies) == 1 {
		if p, ok := m.Bodies[0].CompleteCylinder(); ok {
			return b.putSTEPCylinder(p.Frame, p.Radius, p.Height, m.LengthUnit)
		}
		return b.putSTEPAnalytic(m.Bodies[0].Solid, m.LengthUnit)
	}
	return Shape{}, &failureError{"UNSUPPORTED", "STEP source is not a supported complete planar or analytic solid"}
}

func (b *Bridge) putSTEPCylinder(frame geom.Frame, radius, height, unit float64) (Shape, error) {
	scale := unit / .001
	if !finite(scale) || scale <= 0 {
		return Shape{}, geom.ErrInvalid
	}
	f, err := geom.NewFrame(frame.Origin().Scale(scale), frame.XAxis(), frame.ZAxis())
	if err != nil {
		return Shape{}, err
	}
	cylinder, err := newCylinder(f, radius*scale, height*scale)
	if err != nil {
		return Shape{}, err
	}
	return b.putCylinder(cylinder)
}

func scaleSTEPNotch(p step.SideNotchBody, scale float64) (step.SideNotchBody, error) {
	if !finite(scale) || scale <= 0 {
		return step.SideNotchBody{}, geom.ErrInvalid
	}
	frame, err := geom.NewFrame(p.Frame.Origin().Scale(scale), p.Frame.XAxis(), p.Frame.ZAxis())
	if err != nil {
		return step.SideNotchBody{}, err
	}
	p.Frame = frame
	p.Left, p.Right, p.Back, p.Inset = p.Left*scale, p.Right*scale, p.Back*scale, p.Inset*scale
	p.Length, p.Radius, p.Depth = p.Length*scale, p.Radius*scale, p.Depth*scale
	p.Through = append([]trimmed.ExtrusionCircle(nil), p.Through...)
	p.Blind = append([]trimmed.ExtrusionBlindHole(nil), p.Blind...)
	for i := range p.Through {
		h := &p.Through[i]
		h.Center.X, h.Center.Y, h.Radius = h.Center.X*scale, h.Center.Y*scale, h.Radius*scale
	}
	for i := range p.Blind {
		h := &p.Blind[i]
		h.Center.X, h.Center.Y, h.Radius = h.Center.X*scale, h.Center.Y*scale, h.Radius*scale
		h.Depth *= scale
	}
	return p, nil
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
	case s.tangent != nil:
		return nil, unsupportedEdge("STEP export requires a manifold boundary. The side-tangent blind bore has a non-manifold contact edge.")
	case s.cylinder != nil:
		c := s.cylinder
		e = step.WriteCylinders(&out, step.CylinderModel{LengthUnit: .001, Bodies: []step.CylinderBody{{Name: "PlainSolid", Frame: c.frame, Radius: c.radius, Height: c.height}}})
	case s.bore != nil && s.bore.blindCrossed != nil:
		recipe := s.bore.blindCrossed
		p := step.CrossedBoreBody{Name: "PlainSolid", Frame: recipe.frame, Dimensions: recipe.dimensions, Crossing: recipe.crossing, Radius: recipe.radii[0], BlindDepth: recipe.depth[1], OppositeCap: recipe.oppositeCap[1], XBlindDepth: recipe.depth[0], XOppositeCap: recipe.oppositeCap[0]}
		e = step.WriteCrossedBore(&out, step.CrossedBoreModel{Body: p, LengthUnit: .001})
	case s.bore != nil && s.bore.crossed != nil:
		recipe, ok := crossedBoreRecipe(s.bore.base, s.bore.cylinders, s.bore.blind)
		if !ok {
			return nil, geom.ErrUnrepresentable
		}
		if recipe.radii[0] == recipe.radii[1] && !recipe.partialSkew() {
			p := step.CrossedBoreBody{Name: "PlainSolid", Frame: recipe.frame, Dimensions: recipe.dimensions, Crossing: recipe.crossing, Radius: recipe.radii[0]}
			e = step.WriteCrossedBore(&out, step.CrossedBoreModel{Body: p, LengthUnit: .001})
		} else {
			p := step.UnequalBoreBody{Name: "PlainSolid", Frame: recipe.frame, Dimensions: recipe.dimensions, Crossing: recipe.crossing, XRadius: recipe.radii[0], ZRadius: recipe.radii[1], Offset: recipe.offset}
			e = step.WriteUnequalBore(&out, step.UnequalBoreModel{Body: p, LengthUnit: .001})
		}
	case s.bore != nil:
		if s.bore.oblique != nil {
			p, err := obliqueBoreSTEPRecipe(s.bore)
			if err != nil {
				return nil, err
			}
			e = step.WriteObliqueBore(&out, step.ObliqueBoreModel{Body: p, LengthUnit: .001})
			break
		}
		var p step.CircularHoleExtrusionBody
		p, e = boreSTEPRecipe(s.bore)
		if e == nil {
			e = step.WriteCircularHoleExtrusions(&out, step.CircularHoleExtrusionModel{LengthUnit: .001, Bodies: []step.CircularHoleExtrusionBody{p}})
		} else if mixed, ok := disjointBoreSTEPRecipe(s.bore); ok {
			e = step.WriteDisjointBores(&out, step.DisjointBoresModel{Body: mixed, LengthUnit: .001})
		}
	case s.treated != nil:
		e = writeLongEdgeSTEP(&out, s.treated)
	case s.analytic != nil && s.analytic.layeredDisk && s.analytic.stepDisk != nil:
		e = step.WriteLayeredDiskBores(&out, step.DiskHoleExtrusionModel{LengthUnit: .001, Bodies: []step.DiskHoleExtrusionBody{*s.analytic.stepDisk}})
	case s.analytic != nil && s.analytic.stepDisk != nil:
		e = step.WriteDiskHoleExtrusions(&out, step.DiskHoleExtrusionModel{LengthUnit: .001, Bodies: []step.DiskHoleExtrusionBody{*s.analytic.stepDisk}})
	case s.analytic != nil && s.analytic.sideNotch != nil:
		e = step.WriteSideNotch(&out, step.SideNotchModel{Body: *s.analytic.sideNotch, LengthUnit: .001})
	case s.analytic != nil && s.analytic.sideOpeningCuts != nil:
		e = step.WriteSideOpeningCuts(&out, step.SideOpeningCutsModel{Body: *s.analytic.sideOpeningCuts, LengthUnit: .001})
	case s.analytic != nil && s.analytic.stepRounded != nil:
		e = step.WriteRoundedRectangleExtrusions(&out, step.RoundedRectangleExtrusionModel{LengthUnit: .001, Bodies: []step.RoundedRectangleExtrusionBody{*s.analytic.stepRounded}})
	case s.analytic != nil && s.analytic.rounded != nil && s.analytic.rounded.isPlainExtrusion():
		p := s.analytic.rounded
		e = step.WriteRoundedRectangleExtrusions(&out, step.RoundedRectangleExtrusionModel{LengthUnit: .001, Bodies: []step.RoundedRectangleExtrusionBody{{Name: "PlainSolid", Frame: p.frame, Profile: p.profile, Height: p.height}}})
	case s.analytic != nil && s.analytic.pockets != nil:
		p := s.analytic.pockets
		e = step.WriteBoxPocketExtrusions(&out, step.BoxPocketModel{LengthUnit: .001, Bodies: []step.BoxPocketBody{{Name: "PlainSolid", Frame: p.frame, Size: p.size, Pockets: p.pockets, Bores: p.bores}}})
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

// Only the origin-anchored two-corner recipes verified by the native reader
// are admitted to this writer. Grouped treatments can have other topology.
func writeLongEdgeSTEP(out *bytes.Buffer, body *edgeBody) error {
	c := body.corners
	if body.frame.Origin() != (geom.Vec3{}) || body.frame.XAxis() != (geom.Vec3{X: 1}) || body.frame.ZAxis() != (geom.Vec3{Z: 1}) ||
		c[0] != (fillet.LongEdgeCorner{}) || c[1] != (fillet.LongEdgeCorner{}) || c[2].Radius != 0 ||
		c[2].YSetback <= 0 || c[2].ZSetback <= 0 || c[3].Radius <= 0 || c[3].YSetback != 0 || c[3].ZSetback != 0 {
		return unsupportedEdge("STEP long-edge export requires the canonical opposite-edge treatment and placement")
	}
	p := step.LongEdgeTreatmentBody{Name: "PlainSolid", Length: body.length, Width: body.width, Height: body.height,
		Radius: c[3].Radius, TopSetback: c[2].YSetback, SideSetback: c[2].ZSetback}
	if body.bore == nil && body.pocket == nil && body.boss == nil {
		return step.WriteLongEdgeTreatments(out, step.LongEdgeTreatmentModel{LengthUnit: .001, Bodies: []step.LongEdgeTreatmentBody{p}})
	}
	if body.bore == nil || body.pocket == nil || body.boss == nil {
		return unsupportedEdge("STEP long-edge export requires the complete bore, through pocket and boss chain")
	}
	chain := step.LongEdgeChainBody{LongEdgeTreatmentBody: p, Bore: *body.bore, Pocket: *body.pocket, Boss: *body.boss}
	return step.WriteLongEdgeChains(out, step.LongEdgeChainModel{LengthUnit: .001, Bodies: []step.LongEdgeChainBody{chain}})
}

func (b *Bridge) putLongEdgeSTEP(p step.LongEdgeTreatmentBody, bore *trimmed.ExtrusionCircle, pocket *trimmed.ExtrusionRectangle, boss *trimmed.ExtrusionBoss, unit float64) (Shape, error) {
	frame, err := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if err != nil {
		return Shape{}, err
	}
	body, err := rebuildEdge(edgeBody{frame: frame, length: p.Length, width: p.Width, height: p.Height, bore: bore, pocket: pocket, boss: boss,
		corners: [4]fillet.LongEdgeCorner{2: {YSetback: p.TopSetback, ZSetback: p.SideSetback}, 3: {Radius: p.Radius}}})
	if err != nil {
		return Shape{}, err
	}
	if unit != .001 {
		return b.putSTEPAnalytic(body.chain.Solid, unit)
	}
	return b.putEdge(body)
}
