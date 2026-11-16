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
	"plainsolid/mesh"
)

type provedSTEPRevolution struct {
	source  string
	conical *step.ConicalRimFilletBoreSTEP
}

func (p provedSTEPRevolution) read(ctx context.Context) (trimmed.Solid, float64, error) {
	r := strings.NewReader(p.source)
	if p.conical != nil {
		s, err := step.ReadConicalRimFilletBore(ctx, r, stepLimits(), geom.SurfacePatchOptions{})
		return s.Solid, s.LengthUnit, err
	}
	s, err := step.ReadEccentricAnnularChamfer(ctx, r, stepLimits(), geom.SurfacePatchOptions{})
	return s.Solid, s.LengthUnit, err
}

func (b *Bridge) importProvedSTEPRevolution(ctx context.Context, source string) (Shape, bool, error) {
	p := provedSTEPRevolution{source: source}
	s, unit, err := p.read(ctx)
	if err != nil {
		conical, err := step.ReadConicalRimFilletBore(ctx, strings.NewReader(source), stepLimits(), geom.SurfacePatchOptions{})
		if err != nil {
			return Shape{}, false, nil
		}
		s, unit, p.conical = conical.Solid, conical.LengthUnit, &conical
	}
	if unit != .001 {
		shape, err := b.putSTEPAnalytic(s, unit)
		return shape, true, err
	}
	shape, err := b.putProvedSTEPRevolution(ctx, s, p)
	return shape, true, err
}

func (b *Bridge) putProvedSTEPRevolution(ctx context.Context, s trimmed.Solid, p provedSTEPRevolution) (Shape, error) {
	m, err := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-10})
	if err != nil {
		return Shape{}, err
	}
	return b.putAnalyticBody(&analyticBody{solid: s, volume: m.Volume, area: m.Area, stepRevolution: &p})
}

func (p provedSTEPRevolution) tessellateConical(ctx context.Context, o parametric.TessellationOptions) (mesh.Mesh, error) {
	// For this route the half-angle bounds the entire source triangle,
	// not just sample normals. It therefore proves the requested turn.
	m, err := p.conical.TessellateNormalsCertified(ctx, min(o.Angle, 1), trimmed.CertifiedWrappedFrustumMeshOptions{
		Patch:        trimmed.CertifiedPatchMeshOptions{Deflection: o.Deflection, MaxSubdivisions: min(o.MaxSubdivisions, 128)},
		MaxTriangles: min(o.MaxTriangles, 100_000),
	})
	return m.Mesh, err
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
