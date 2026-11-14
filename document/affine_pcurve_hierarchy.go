package document

import (
	"bytes"
	"context"
	"io"

	"plainsolid/exchange/step"
)

// ReadAffinePCurveRationalHierarchyDocumentSTEP preserves the original STEP
// with verified affine pcurves and the explicit neutral-child frame convention.
func ReadAffinePCurveRationalHierarchyDocumentSTEP(ctx context.Context, r io.Reader, o STEPAssemblyOptions, tolerance float64) (*RationalSphericalHierarchyDocument, error) {
	m, err := step.ReadAffinePCurveRationalHierarchy(ctx, r, o.Geometry, tolerance)
	if err != nil {
		return nil, err
	}
	return materializeAffinePCurveHierarchyDocument(ctx, m, o)
}

// ReadOCCTAffinePCurveRationalHierarchyDocumentSTEP uses the distinct original
// OCCT relationship convention for an authenticated affine-pcurve hierarchy.
func ReadOCCTAffinePCurveRationalHierarchyDocumentSTEP(ctx context.Context, r io.Reader, o STEPAssemblyOptions, tolerance float64) (*RationalSphericalHierarchyDocument, error) {
	m, err := step.ReadOCCTAffinePCurveRationalHierarchy(ctx, r, o.Geometry, tolerance)
	if err != nil {
		return nil, err
	}
	return materializeAffinePCurveHierarchyDocument(ctx, m, o)
}

// ReadAffinePCurveRationalHierarchyDocumentNative re-proves the original STEP
// and the exact affine evidence stored in its distinct native envelope.
func ReadAffinePCurveRationalHierarchyDocumentNative(ctx context.Context, r io.Reader, limits step.IndependentMaterialJSONLimits, o STEPAssemblyOptions) (*RationalSphericalHierarchyDocument, error) {
	m, err := step.ReadAffinePCurveHierarchyJSON(ctx, r, limits)
	if err != nil {
		return nil, err
	}
	return materializeAffinePCurveHierarchyDocument(ctx, m, o)
}

func materializeAffinePCurveHierarchyDocument(ctx context.Context, m step.AffinePCurveHierarchy, o STEPAssemblyOptions) (*RationalSphericalHierarchyDocument, error) {
	proof, err := m.ProofHierarchy(ctx)
	if err != nil {
		return nil, err
	}
	uses, err := rationalHierarchyDocumentUses(ctx, proof, o)
	if err != nil {
		return nil, err
	}
	var native bytes.Buffer
	if err := step.WriteAffinePCurveHierarchyJSON(ctx, &native, m); err != nil {
		return nil, err
	}
	return &RationalSphericalHierarchyDocument{
		source: []byte(m.OriginalSTEP()), native: native.Bytes(), model: proof, uses: uses,
		vendor: m.UsesOCCTFrameConvention(), affine: &m,
	}, nil
}

// AffinePCurves returns independent copies of any original affine-pcurve proofs.
func (d *RationalSphericalHierarchyDocument) AffinePCurves() []step.AffinePCurveEvidence {
	if d == nil || d.affine == nil {
		return nil
	}
	return d.affine.AffinePCurves()
}
