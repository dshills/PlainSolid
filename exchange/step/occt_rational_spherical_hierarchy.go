package step

import (
	"context"
	"io"
)

const occtRationalSphericalHierarchyFormat = "plainsolid-step-occt-rational-spherical-hierarchy"

// OCCTRationalSphericalHierarchy explicitly interprets original relationships
// using pinned OCCT inverse(from)*to, with exact rational proper frames. Local
// native geometry remains separate from immutable rational occurrence matrices.
type OCCTRationalSphericalHierarchy RationalSphericalHierarchy

// ReadOCCTRationalSphericalHierarchy admits nonneutral rational child frames in
// common metre/mm units. Both source frames, their relative pose and every root
// prefix must pass the finite-decimal/256-bit gate. No float pose is synthesized.
func ReadOCCTRationalSphericalHierarchy(ctx context.Context, r io.Reader, l Limits, tol float64) (OCCTRationalSphericalHierarchy, error) {
	m, e := readRationalSphericalHierarchy(ctx, r, l, tol, true)
	return OCCTRationalSphericalHierarchy(m), e
}

// WriteOCCTRationalSphericalHierarchySource re-proves and emits original records
// and relationship/frame IDs under this explicit consumer convention.
func WriteOCCTRationalSphericalHierarchySource(ctx context.Context, w io.Writer, source *File, l Limits, tol float64) error {
	return writeRationalSphericalHierarchySource(ctx, w, source, l, tol, true)
}

// WriteOCCTRationalSphericalHierarchyJSON uses a distinct version-one tag; all
// source, local native and exact rational occurrence claims are re-proved.
func WriteOCCTRationalSphericalHierarchyJSON(ctx context.Context, w io.Writer, m OCCTRationalSphericalHierarchy) error {
	return writeRationalSphericalHierarchyJSON(ctx, w, RationalSphericalHierarchy(m), true)
}

// ReadOCCTRationalSphericalHierarchyJSON does not accept the neutral-child tag
// or infer transfer convention from an unsupported source.
func ReadOCCTRationalSphericalHierarchyJSON(ctx context.Context, r io.Reader, l IndependentMaterialJSONLimits) (OCCTRationalSphericalHierarchy, error) {
	m, e := readRationalSphericalHierarchyJSON(ctx, r, l, true)
	return OCCTRationalSphericalHierarchy(m), e
}
