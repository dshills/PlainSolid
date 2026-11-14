package plaincadbridge

import (
	"context"
	"errors"
	"math"
	"math/big"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

type leadRequest struct {
	Major       number `json:"major"`
	Minor       number `json:"minor"`
	Bore        number `json:"bore"`
	NutOuter    number `json:"nutOuter"`
	Pitch       number `json:"pitch"`
	Lead        number `json:"lead"`
	ScrewLength number `json:"screwLength"`
	NutLength   number `json:"nutLength"`
	Clearance   number `json:"clearance"`
	Slope       number `json:"slope"`
	Starts      int    `json:"starts"`
	Hand        int    `json:"hand"`
}

type leadPart struct {
	spec leadRequest
	nut  bool
}

func (d leadRequest) validate(nut bool) error {
	pitch, lead, major, minor, bore, outer := float64(d.Pitch), float64(d.Lead), float64(d.Major), float64(d.Minor), float64(d.Bore), float64(d.NutOuter)
	clearance, slope := float64(d.Clearance), float64(d.Slope)
	screwLength, nutLength := float64(d.ScrewLength), float64(d.NutLength)
	if d.Hand != 1 && d.Hand != -1 || d.Starts < 1 || d.Starts > 4 ||
		pitch < .5 || pitch > 6 || lead != pitch*float64(d.Starts) ||
		major < 1.5*pitch-1e-9 || major > 25 || minor != major-pitch/2 || bore != minor+clearance || outer != major+pitch ||
		clearance < .025*pitch-1e-9 || clearance > .10*pitch+1e-9 || slope <= 0 || slope > .3 {
		return errors.New("invalid lead screw dimensions")
	}
	maxScrew, maxNut := 14., 8.
	if d.Starts >= 3 {
		maxScrew, maxNut = 10, 6
	}
	if screwLength < 5*pitch-1e-9 || screwLength > maxScrew*pitch+1e-9 ||
		nut && (nutLength < 4.75*pitch-1e-9 || nutLength > maxNut*pitch+1e-9 || screwLength < nutLength+pitch-1e-9) {
		return errors.New("invalid lead screw or nut axial length")
	}
	return nil
}

type leadGeometry struct {
	recipe                trimmed.HelicalGrooveRecipe
	phase, reliefRadius   float64
	entryDepth, exitDepth float64
}

func leadRecipe(d leadRequest, nut bool) (leadGeometry, error) {
	if err := d.validate(nut); err != nil {
		return leadGeometry{}, err
	}
	frame, err := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	if err != nil {
		return leadGeometry{}, err
	}
	pitch, major, minor, slope := float64(d.Pitch), float64(d.Major), float64(d.Minor), float64(d.Slope)
	bore, clearance := float64(d.Bore), float64(d.Clearance)
	center, endMargin := pitch*.65, pitch*.85
	height, outer, inner, boreRadius, phase := float64(d.ScrewLength), major, minor, 0., .5/float64(d.Starts)
	half := func(r float64) float64 { return pitch/4 + slope*(r-major) }
	cutterExtra := pitch * .05
	reliefRadius := minor - pitch*.025
	maxHalf := half(major + cutterExtra)
	if nut {
		height, outer, inner, boreRadius, phase = float64(d.NutLength), major+clearance, bore, bore, 0
		half = func(r float64) float64 { return pitch/4 + slope*(major-r) + clearance*(1+slope) }
		maxHalf = half(bore - cutterExtra)
		reliefRadius = outer + pitch*.025
	}
	entryDepth := center + maxHalf + pitch*.01
	exitDepth := 0.
	if nut {
		exitDepth = endMargin + maxHalf + pitch*.01
	}
	stock := outer
	if nut {
		stock = float64(d.NutOuter)
	}
	recipe := trimmed.HelicalGrooveRecipe{Frame: frame, OuterRadius: stock, BoreRadius: boreRadius, Height: height,
		Profile: threadProfile(inner, outer, center, half(inner), half(outer)), Lead: float64(d.Lead),
		End: (height - center - endMargin) / float64(d.Lead), Hand: geom.HelixHandedness(d.Hand), Starts: d.Starts}
	return leadGeometry{recipe, phase, reliefRadius, entryDepth, exitDepth}, nil
}

func (b *Bridge) createLeadPart(d leadRequest, nut bool) (Shape, error) {
	p, err := leadRecipe(d, nut)
	if err != nil {
		return Shape{}, err
	}
	ctx := context.Background()
	var result trimmed.HelicalOverlapRelief
	if nut {
		ends, err := trimmed.GrooveWithPhasedEndReliefs(ctx, p.recipe, p.phase, p.reliefRadius, p.entryDepth, p.exitDepth, threadChartBudget(p.recipe))
		if err != nil {
			return Shape{}, err
		}
		result = ends.HelicalOverlapRelief
	} else {
		result, err = trimmed.GrooveWithPhasedOverlappingEntryRelief(ctx, p.recipe, p.phase, p.reliefRadius, p.entryDepth, threadChartBudget(p.recipe))
		if err != nil {
			return Shape{}, err
		}
	}
	removed, _ := new(big.Rat).Add(result.GrooveVolumePiCoefficient, result.EntryVolumePiCoefficient).Float64()
	r, bore, height := p.recipe.OuterRadius, p.recipe.BoreRadius, p.recipe.Height
	volume := math.Pi * ((r*r-bore*bore)*height - removed)
	area, err := result.Solid.HelicalSurfaceArea(ctx)
	if err != nil {
		return Shape{}, err
	}
	return b.putAnalyticBody(&analyticBody{solid: result.Solid, volume: volume, area: area,
		helical: true, lead: &leadPart{spec: d, nut: nut}})
}

func (b *Bridge) checkLeadPair(screw, nut stored, d leadRequest) error {
	if screw.analytic == nil || nut.analytic == nil || screw.analytic.lead == nil || nut.analytic.lead == nil ||
		screw.analytic.lead.nut || !nut.analytic.lead.nut || nut.analytic.lead.spec != d {
		return errors.New("Matching nut requires the original native lead screw and nut")
	}
	a := screw.analytic.lead.spec
	a.NutLength, a.Clearance, a.Bore = d.NutLength, d.Clearance, d.Bore
	if a != d {
		return errors.New("Lead screw and nut do not share their original thread dimensions")
	}
	shaft, err := leadRecipe(d, false)
	if err != nil {
		return err
	}
	collar, err := leadRecipe(d, true)
	if err != nil {
		return err
	}
	// In the engagement interval both grooves span each complete phase.
	// Both flanks vary linearly across radius, so the endpoints suffice
	// for the clearance between the retained shaft ridge and nut opening.
	pitch, major, slope, clearance := float64(d.Pitch), float64(d.Major), float64(d.Slope), float64(d.Clearance)
	for _, radius := range []float64{float64(d.Bore), major} {
		ridge := pitch/4 + slope*(major-radius)
		opening := ridge + clearance*(1+slope)
		if opening-ridge <= 1e-9 || opening >= pitch/2 {
			return errors.New("Native lead screw and nut flanks do not match the clearance envelope")
		}
	}
	limit := min(shaft.recipe.Height-shaft.exitDepth, collar.recipe.Height-collar.exitDepth)
	if limit-max(shaft.entryDepth, collar.entryDepth) < pitch {
		return errors.New("Lead pair lacks a full pitch of common engagement")
	}
	return nil
}
