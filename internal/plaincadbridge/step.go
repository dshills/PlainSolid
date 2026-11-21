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

type stepFileUnit string

const (
	stepMillimetre stepFileUnit = "mm"
	stepMetre      stepFileUnit = "m"
	stepInch       stepFileUnit = "in"
)

func stepLimits() step.Limits {
	return step.Limits{MaxBytes: maxSTEPBytes, MaxEntities: 50000, MaxValues: 200000, MaxVertices: 20000, MaxFaces: 10000, MaxEdgeUses: 50000, MaxSolids: 32}
}
func (b *Bridge) importSTEP(source string) (Shape, error) {
	return b.importSTEPInUnits(source, "")
}

func (b *Bridge) importSTEPInUnits(source string, units stepFileUnit) (Shape, error) {
	if len(source) == 0 || len(source) > maxSTEPBytes {
		return Shape{}, &failureError{"LIMIT", "STEP source must contain between 1 byte and 1 MiB"}
	}
	r := func() *strings.Reader { return strings.NewReader(source) }
	// The product identifiers are dispatch hints. The readers authenticate
	// the complete rooted graphs and their original geometry.
	if strings.Contains(source, "'rational-posed-exact-circular-loop-through-hole-extrusion'") {
		ctx := context.Background()
		if m, e := step.ReadRationalPosedExactCircularLoopThroughHoleExtrusion(ctx, r(), stepLimits()); e == nil {
			if e = checkRationalSTEPUnit(m.Exterior.LengthUnit, units); e != nil {
				return Shape{}, e
			}
			return b.putRationalSTEPArcLoopWithHole(ctx, m.Native, m.Exterior, m.Pose, &m.Hole)
		}
	}
	if strings.Contains(source, "'rational-posed-exact-circular-loop-extrusion'") {
		ctx := context.Background()
		if m, e := step.ReadRationalPosedExactCircularLoopExtrusion(ctx, r(), stepLimits()); e == nil {
			if e = checkRationalSTEPUnit(m.Descriptor.LengthUnit, units); e != nil {
				return Shape{}, e
			}
			return b.putRationalSTEPArcLoop(ctx, m.Native, m.Descriptor, m.Pose)
		}
	}
	if units != "" {
		return Shape{}, unsupportedEdge("explicit source-unit import requires a qualified rational-world STEP solid")
	}
	if strings.Contains(source, "'exact-circular-loop-extrusion'") {
		ctx := context.Background()
		if m, e := step.ReadExactCircularLoopExtrusion(ctx, r(), stepLimits()); e == nil {
			return b.putExactSTEPArcLoop(ctx, m.Native, m.Descriptor)
		}
	}
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
	// The complete periodic tube dialect also matches the independent
	// disk-hole reader. Retain its full native recipe before that fallback.
	supports := strings.ToUpper(source)
	if strings.Contains(supports, "SEAM_CURVE") && strings.Contains(supports, "CYLINDRICAL_SURFACE") {
		if m, e := step.ReadAnalyticBodies(r(), stepLimits()); e == nil {
			if recipe, ok := tubeFromSTEP(m); ok {
				return b.putRadialSector(recipe)
			}
		}
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
	if m, e := step.ReadDisjointBores(r(), stepLimits()); e == nil {
		base, tools, err := disjointBoresFromSTEP(m.Body, m.LengthUnit)
		if err != nil {
			return Shape{}, err
		}
		return b.putBoredStock(base, tools)
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
	if m, e := step.ReadSteppedShaft(r(), stepLimits()); e == nil {
		return b.putSteppedShaftSTEP(m)
	}
	if m, e := step.ReadQuarterAnnularExtrusion(r(), stepLimits()); e == nil {
		return b.putQuarterAnnularSTEP(m)
	}
	if m, e := step.ReadQuarterSectorExtrusion(r(), stepLimits()); e == nil {
		return b.putQuarterSectorSTEP(m)
	}
	if m, e := step.ReadSemicircleExtrusion(r(), stepLimits()); e == nil {
		return b.putSemicircleSTEP(m)
	}
	if m, e := step.ReadCircularSegmentExtrusion(r(), stepLimits()); e == nil {
		return b.putCircularSegmentSTEP(m)
	}
	if m, e := step.ReadCircularLensExtrusion(r(), stepLimits()); e == nil {
		return b.putCircularLensSTEP(m)
	}
	if strings.Contains(supports, "CYLINDRICAL_SURFACE") && strings.Contains(supports, "PCURVE") {
		if m, e := step.ReadConvexArcExtrusion(r(), stepLimits()); e == nil {
			return b.putConvexArcExtrusionSTEP(m)
		}
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
	if strings.Contains(supports, "TOROIDAL_SURFACE") || strings.Contains(supports, "CYLINDRICAL_SURFACE") {
		if m, e := step.ReadAnalyticBodies(r(), stepLimits()); e == nil {
			if recipe, ok := torusFromSTEP(m); ok {
				body, err := recipe.body()
				if err != nil {
					return Shape{}, err
				}
				return b.putAnalyticBody(body)
			}
			if recipe, ok := cylinderSectorFromSTEP(m); ok {
				return b.putRadialSector(recipe)
			}
		}
	}
	if strings.Contains(supports, "B_SPLINE_SURFACE_WITH_KNOTS") && strings.Contains(supports, "SEAM_CURVE") {
		if p, axis, e := step.ReadPlacedWrappedRimFillet(r(), stepLimits(), step.WrappedSurfaceOptions{}); e == nil {
			return b.putRimFilletSTEP(p, axis)
		}
		if p, e := step.ReadPlacedWrappedRimChamfer(r(), stepLimits(), step.WrappedSurfaceOptions{}); e == nil {
			return b.putRimChamferSTEP(p)
		}
	}
	if strings.Contains(supports, "SURFACE_OF_REVOLUTION") {
		if shape, recognized, err := b.importProvedSTEPRevolution(context.Background(), source); recognized {
			return shape, err
		}
	}
	m, e := step.ReadTrimmedBodies(r(), stepLimits())
	if e == nil && len(m.Bodies) == 1 {
		if p, ok := m.Bodies[0].CompleteCylinder(); ok {
			return b.putSTEPCylinder(p.Frame, p.Radius, p.Height, m.LengthUnit)
		}
		return b.putSTEPAnalytic(m.Bodies[0].Solid, m.LengthUnit)
	}
	// The last reader takes a boundary of planes and cylinders from its
	// entities and ignores p-curves. A general bridge tries it for every
	// source. The default bridge tries it for a source that states no
	// p-curve, as a file that exportStep wrote for a result of the general
	// Boolean does, and for a source this kernel wrote, whose p-curves are
	// its own: the files of bores of unequal radius or with skew axes
	// from before the general Boolean cut them have no other reader
	// (specs/general-boolean-quartic, section 11). Any other source with
	// p-curves is left to the readers and the repairs that check them.
	// Where the reader does not take the source, the default bridge
	// refuses as it did before the reader existed.
	if b.general || !strings.Contains(source, "PCURVE") || ownSTEP(source) {
		if s, err := b.importGeneralSTEP(source); err == nil || b.general {
			return s, err
		}
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
	case s.rationalStepArc != nil:
		p := s.rationalStepArc
		if p.hole != nil {
			e = step.WriteRationalPosedExactCircularLoopThroughHoleExtrusion(context.Background(), &out, p.descriptor, *p.hole, p.pose)
		} else {
			e = step.WriteRationalPosedExactCircularLoopExtrusion(context.Background(), &out, p.descriptor, p.pose)
		}
	case s.tangent != nil:
		return nil, unsupportedEdge("STEP export requires a manifold boundary. The side-tangent blind bore has a non-manifold contact edge.")
	case s.cylinder != nil:
		c := s.cylinder
		e = step.WriteCylinders(&out, step.CylinderModel{LengthUnit: .001, Bodies: []step.CylinderBody{{Name: "PlainSolid", Frame: c.frame, Radius: c.radius, Height: c.height}}})
	case s.treated != nil:
		e = writeLongEdgeSTEP(&out, s.treated)
	case s.analytic != nil && s.analytic.layeredDisk && s.analytic.stepDisk != nil:
		e = step.WriteLayeredDiskBores(&out, step.DiskHoleExtrusionModel{LengthUnit: .001, Bodies: []step.DiskHoleExtrusionBody{*s.analytic.stepDisk}})
	case s.analytic != nil && s.analytic.stepDisk != nil:
		e = step.WriteDiskHoleExtrusions(&out, step.DiskHoleExtrusionModel{LengthUnit: .001, Bodies: []step.DiskHoleExtrusionBody{*s.analytic.stepDisk}})
	case s.analytic != nil && s.analytic.stepRounded != nil:
		e = step.WriteRoundedRectangleExtrusions(&out, step.RoundedRectangleExtrusionModel{LengthUnit: .001, Bodies: []step.RoundedRectangleExtrusionBody{*s.analytic.stepRounded}})
	case s.analytic != nil && s.analytic.rounded != nil && s.analytic.rounded.isPlainExtrusion():
		p := s.analytic.rounded
		e = step.WriteRoundedRectangleExtrusions(&out, step.RoundedRectangleExtrusionModel{LengthUnit: .001, Bodies: []step.RoundedRectangleExtrusionBody{{Name: "PlainSolid", Frame: p.frame, Profile: p.profile, Height: p.height}}})
	case s.analytic != nil && s.analytic.pockets != nil:
		p := s.analytic.pockets
		e = step.WriteBoxPocketExtrusions(&out, step.BoxPocketModel{LengthUnit: .001, Bodies: []step.BoxPocketBody{{Name: "PlainSolid", Frame: p.frame, Size: p.size, Pockets: p.pockets, Bores: p.bores}}})
	case s.analytic != nil && s.analytic.stepArcLoop != nil:
		d := s.analytic.stepArcLoop.descriptor
		if !s.analytic.stepArcLoop.exactPlacement {
			return nil, unsupportedEdge("exact circular-loop STEP export requires millimetres and a signed-coordinate frame")
		}
		e = step.WriteExactCircularLoopExtrusion(context.Background(), &out, d)
	case s.analytic != nil && s.analytic.torus != nil:
		e = s.analytic.torus.writeSTEP(&out)
	case s.analytic != nil:
		if p := s.analytic.shaft; p != nil {
			e = step.WriteSteppedShaft(&out, step.SteppedShaftModel{Name: "PlainSolid", Body: *p, LengthUnit: .001})
			break
		}
		if p := s.analytic.rimTreatment; p != nil {
			e = p.writeSTEP(&out)
			break
		}
		if sector := s.analytic.sector; sector != nil {
			if sector.inner == 0 {
				e = sector.writeSectorSTEP(&out)
				break
			}
			if sector.angle == 2*math.Pi {
				e = sector.writeTubeSTEP(&out)
				break
			}
			if p, ok := sector.quarterAnnularSTEP(); ok {
				e = step.WriteQuarterAnnularExtrusion(&out, step.QuarterAnnularExtrusionModel{Body: p, LengthUnit: .001})
				break
			}
		}
		if s.analytic.segment != nil {
			if p, ok := s.analytic.segment.semicircleSTEP(); ok {
				e = step.WriteSemicircleExtrusion(&out, step.SemicircleExtrusionModel{Body: p, LengthUnit: .001})
				break
			}
			if p, ok := s.analytic.segment.circularSegmentSTEP(); ok {
				e = step.WriteCircularSegmentExtrusion(&out, step.CircularSegmentExtrusionModel{Body: p, LengthUnit: .001})
				break
			}
		}
		if s.analytic.arcLoop != nil {
			if p, ok := s.analytic.arcLoop.quarterSectorSTEP(); ok {
				e = step.WriteQuarterSectorExtrusion(&out, step.QuarterSectorExtrusionModel{Body: p, LengthUnit: .001})
				break
			}
			if p, ok := s.analytic.arcLoop.circularLensSTEP(); ok {
				e = step.WriteCircularLensExtrusion(&out, step.CircularLensExtrusionModel{Body: p, LengthUnit: .001})
				break
			}
			if p, ok := s.analytic.arcLoop.convexArcSTEP(); ok {
				if e = step.WriteConvexArcExtrusion(&out, step.ConvexArcExtrusionModel{Body: p, LengthUnit: .001}); e == nil {
					break
				}
			}
		}
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

type stepProof struct {
	Assertions
	Bounds Bounds `json:"bounds"`
}
type stepExport struct {
	Base64     string       `json:"base64"`
	Before     []stepProof  `json:"before"`
	After      []stepProof  `json:"after"`
	Units      stepFileUnit `json:"units"`
	ProofUnits stepFileUnit `json:"proofUnits,omitempty"`
}

func (b *Bridge) exportSTEP(handles []string) (stepExport, error) {
	return b.exportSTEPInUnits(handles, "")
}

func (b *Bridge) exportSTEPInUnits(handles []string, units stepFileUnit) (stepExport, error) {
	fail := func(e error) (stepExport, error) { return stepExport{}, e }
	if len(handles) == 0 || len(handles) > 32 {
		return fail(&failureError{"LIMIT", "STEP export requires 1 to 32 bodies"})
	}
	out := stepExport{Units: stepMillimetre}
	if units != "" {
		out.Units, out.ProofUnits = units, stepMillimetre
	}
	sources := make([][]byte, len(handles))
	written := make([]bool, len(handles)) // by step.WriteTrimmedBoundary
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
		if s.result.StepUnits != units {
			return fail(&failureError{"INVALID", "STEP export unit must match each retained source unit"})
		}
		if s.mappedChainSTEP != nil {
			if len(handles) != 1 {
				return fail(unsupportedEdge("ordered mapped STEP export requires one original source body"))
			}
			return s.exportOrderedMappedSTEP(context.Background())
		}
		if s.analytic != nil && s.analytic.stepRevolution != nil {
			if len(handles) != 1 {
				return fail(unsupportedEdge("source-preserving revolution STEP export requires one original source body"))
			}
			return s.exportProvedSTEPRevolution(context.Background())
		}
		if s.analytic != nil && b.generalResult(s.analytic) {
			sources[i], written[i], e = writeGeneralSTEPBody(s)
		} else {
			sources[i], e = writeSTEPBody(s)
		}
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
	verify.general = b.general
	for i, p := range parts {
		s, e := verify.importSTEPInUnits(string(p), units)
		if e != nil && written[i] {
			return fail(generalSTEPReadback(e))
		}
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
	return b.putBoredStock(base, tools)
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

// ownSTEP reports a source whose header names this kernel as the system
// that wrote it, as every writer of exchange/step states.
func ownSTEP(source string) bool {
	header, _, _ := strings.Cut(source, "ENDSEC;")
	return strings.Contains(header, "'PlainSolid','PlainSolid'")
}
