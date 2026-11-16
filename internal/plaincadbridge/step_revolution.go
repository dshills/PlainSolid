package plaincadbridge

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

type provedSTEPRevolution struct {
	source string
}

func (p provedSTEPRevolution) read(ctx context.Context) (trimmed.Solid, float64, error) {
	r := strings.NewReader(p.source)
	s, err := step.ReadEccentricAnnularChamfer(ctx, r, stepLimits(), geom.SurfacePatchOptions{})
	return s.Solid, s.LengthUnit, err
}

func (b *Bridge) importProvedSTEPRevolution(ctx context.Context, source string) (Shape, bool, error) {
	p := provedSTEPRevolution{source: source}
	s, unit, err := p.read(ctx)
	if err != nil {
		return Shape{}, false, nil
	}
	if unit != .001 {
		shape, err := b.putSTEPAnalytic(s, unit)
		return shape, true, err
	}
	m, err := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-10})
	if err != nil {
		return Shape{}, true, err
	}
	shape, err := b.putAnalyticBody(&analyticBody{solid: s, volume: m.Volume, area: m.Area, stepRevolution: &p})
	return shape, true, err
}

func (s stored) exportProvedSTEPRevolution(ctx context.Context) (stepExport, error) {
	p := s.analytic.stepRevolution
	native, unit, err := p.read(ctx)
	if err != nil {
		return stepExport{}, err
	}
	if unit != .001 {
		return stepExport{}, geom.ErrInvalid
	}
	shape, err := New().putSTEPAnalytic(native, unit)
	if err != nil {
		return stepExport{}, err
	}
	before := stepProof{s.result.GeometryAssertions, s.result.Bounds}
	after := stepProof{shape.GeometryAssertions, shape.Bounds}
	if before != after {
		return stepExport{}, errors.New("STEP source replay changed material or world bounds")
	}
	return stepExport{Units: stepMillimetre, Base64: base64.StdEncoding.EncodeToString([]byte(p.source)),
		Before: []stepProof{before}, After: []stepProof{after}}, nil
}
