package document

import (
	"bytes"
	"context"
	"io"
	"math/big"
	"strconv"
	"strings"

	"plainsolid/brep"
	"plainsolid/brep/trimmed"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

// RationalSphericalHierarchyDocument retains a verified source hierarchy and
// exact occurrence poses. A snapshot is immutable and keeps geometry in local
// charts. Placement edits return a new document with independently proved poses.
type RationalSphericalHierarchyDocument struct {
	source []byte
	native []byte
	model  step.RationalSphericalHierarchy
	uses   []RationalHierarchyDocumentOccurrence
	vendor bool
}

// RationalHierarchyDocumentOccurrence names one original use and material
// asset. World holds model-coordinate rationals, including the placement of
// the product's chart origin. Path contains original NAUO entity IDs.
type RationalHierarchyDocumentOccurrence struct {
	DefinitionID, SourceProductID, SourceBodyID int64
	Key, Family                                 string
	Path, SourceFaceIDs                         []int64
	World                                       [3][4]*big.Rat
}

// RationalHierarchyDocumentAsset returns immutable local geometry. Exactly one
// of Planar or Trimmed is set. NativeFaceSources maps native face indices to
// original STEP faces and is copied on each access.
type RationalHierarchyDocumentAsset struct {
	DefinitionID, SourceProductID, SourceBodyID int64
	Family                                      string
	NativeFaceSources                           []int64
	Planar                                      *brep.Solid
	Trimmed                                     *trimmed.Solid
}

// ReadRationalSphericalHierarchyDocumentSTEP validates and imports the exact
// neutral-child STEP convention into a read-only source document.
func ReadRationalSphericalHierarchyDocumentSTEP(ctx context.Context, r io.Reader, o STEPAssemblyOptions, tolerance float64) (*RationalSphericalHierarchyDocument, error) {
	m, err := step.ReadRationalSphericalHierarchy(ctx, r, o.Geometry, tolerance)
	if err != nil {
		return nil, err
	}
	return materializeRationalSphericalHierarchyDocument(ctx, m, o, false)
}

// ReadOCCTRationalSphericalHierarchyDocumentSTEP uses the distinct exact OCCT
// nonneutral frame convention. No convention is inferred on failed import.
func ReadOCCTRationalSphericalHierarchyDocumentSTEP(ctx context.Context, r io.Reader, o STEPAssemblyOptions, tolerance float64) (*RationalSphericalHierarchyDocument, error) {
	m, err := step.ReadOCCTRationalSphericalHierarchy(ctx, r, o.Geometry, tolerance)
	if err != nil {
		return nil, err
	}
	return materializeRationalSphericalHierarchyDocument(ctx, step.RationalSphericalHierarchy(m), o, true)
}

// ReadRationalSphericalHierarchyDocumentNative restores only the verified
// neutral-child native envelope. Its STEP source and exact frames are re-proved.
func ReadRationalSphericalHierarchyDocumentNative(ctx context.Context, r io.Reader, limits step.IndependentMaterialJSONLimits, o STEPAssemblyOptions) (*RationalSphericalHierarchyDocument, error) {
	m, err := step.ReadRationalSphericalHierarchyJSON(ctx, r, limits)
	if err != nil {
		return nil, err
	}
	return materializeRationalSphericalHierarchyDocument(ctx, m, o, false)
}

// ReadOCCTRationalSphericalHierarchyDocumentNative restores only the distinct
// native envelope carrying the OCCT composition convention.
func ReadOCCTRationalSphericalHierarchyDocumentNative(ctx context.Context, r io.Reader, limits step.IndependentMaterialJSONLimits, o STEPAssemblyOptions) (*RationalSphericalHierarchyDocument, error) {
	m, err := step.ReadOCCTRationalSphericalHierarchyJSON(ctx, r, limits)
	if err != nil {
		return nil, err
	}
	return materializeRationalSphericalHierarchyDocument(ctx, step.RationalSphericalHierarchy(m), o, true)
}

func materializeRationalSphericalHierarchyDocument(ctx context.Context, m step.RationalSphericalHierarchy, o STEPAssemblyOptions, vendor bool) (*RationalSphericalHierarchyDocument, error) {
	if o.MaxNodes == 0 {
		o.MaxNodes = 10000
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 64
	}
	if o.MaxNodes < 1 || o.MaxDepth < 1 || o.MaxDepth > 256 {
		return nil, geom.ErrInvalid
	}
	if len(m.Roots) != 1 {
		return nil, geom.ErrUnrepresentable
	}
	root := rationalHierarchyDocumentKey(m.Roots[0], nil)
	prefixes := map[string]bool{root: true}
	var uses []RationalHierarchyDocumentOccurrence
	for _, u := range m.Instances {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if len(u.Path)+1 > o.MaxDepth {
			return nil, geom.ErrConvergence
		}
		for i := range u.Path {
			prefixes[rationalHierarchyDocumentKey(m.Roots[0], u.Path[:i+1])] = true
			if len(prefixes) > o.MaxNodes {
				return nil, geom.ErrConvergence
			}
		}
		body, ok := m.Products[u.ProductID]
		if !ok {
			return nil, geom.ErrUnrepresentable
		}
		faces, err := step.SphericalHierarchyNativeFaceSources(body)
		if err != nil {
			return nil, err
		}
		uses = append(uses, RationalHierarchyDocumentOccurrence{
			DefinitionID: u.ProductID, SourceProductID: body.SourceProductID, SourceBodyID: body.SourceBodyID,
			Key: rationalHierarchyDocumentKey(m.Roots[0], u.Path), Family: rationalHierarchyDocumentFamily(body),
			Path: append([]int64(nil), u.Path...), SourceFaceIDs: faces, World: u.Matrix(),
		})
	}
	var native, source bytes.Buffer
	var err error
	if vendor {
		err = step.WriteOCCTRationalSphericalHierarchyJSON(ctx, &native, step.OCCTRationalSphericalHierarchy(m))
	} else {
		err = step.WriteRationalSphericalHierarchyJSON(ctx, &native, m)
	}
	if err != nil {
		return nil, err
	}
	tol := m.Tolerance()
	if vendor {
		err = step.WriteOCCTRationalSphericalHierarchySource(ctx, &source, m.Source, o.Geometry, tol)
	} else {
		err = step.WriteRationalSphericalHierarchySource(ctx, &source, m.Source, o.Geometry, tol)
	}
	if err != nil {
		return nil, err
	}
	return &RationalSphericalHierarchyDocument{source: source.Bytes(), native: native.Bytes(), model: m, uses: uses, vendor: vendor}, nil
}

func rationalHierarchyDocumentKey(root int64, path []int64) string {
	var b strings.Builder
	b.WriteString("step-product-")
	b.WriteString(strconv.FormatInt(root, 10))
	for _, id := range path {
		b.WriteString("/occ-")
		b.WriteString(strconv.FormatInt(id, 10))
	}
	return b.String()
}

func rationalHierarchyDocumentFamily(b step.IndependentExactMaterial) string {
	switch {
	case b.Planar != nil:
		return "planar"
	case b.Torus != nil:
		return "torus"
	case b.Cylinder != nil:
		return "cylinder"
	case b.Sphere != nil:
		return "sphere"
	}
	return ""
}

// LengthUnit returns metres per original model coordinate unit.
func (d *RationalSphericalHierarchyDocument) LengthUnit() float64 {
	if d == nil {
		return 0
	}
	return d.model.LengthUnit
}

// Occurrences returns independent copies in proved occurrence path order.
func (d *RationalSphericalHierarchyDocument) Occurrences() []RationalHierarchyDocumentOccurrence {
	if d == nil {
		return nil
	}
	out := append([]RationalHierarchyDocumentOccurrence(nil), d.uses...)
	for i := range out {
		v := &out[i]
		v.Path = append([]int64(nil), v.Path...)
		v.SourceFaceIDs = append([]int64(nil), v.SourceFaceIDs...)
		for j := range v.World {
			for k, r := range v.World[j] {
				v.World[j][k] = new(big.Rat).Set(r)
			}
		}
	}
	return out
}

// Asset returns the proved local geometry shared by occurrences of a definition.
func (d *RationalSphericalHierarchyDocument) Asset(id int64) (RationalHierarchyDocumentAsset, bool) {
	if d == nil {
		return RationalHierarchyDocumentAsset{}, false
	}
	b, ok := d.model.Products[id]
	if !ok {
		return RationalHierarchyDocumentAsset{}, false
	}
	faces, err := step.SphericalHierarchyNativeFaceSources(b)
	if err != nil {
		return RationalHierarchyDocumentAsset{}, false
	}
	out := RationalHierarchyDocumentAsset{
		DefinitionID: id, SourceProductID: b.SourceProductID, SourceBodyID: b.SourceBodyID,
		Family: rationalHierarchyDocumentFamily(b), NativeFaceSources: faces,
	}
	if b.Planar != nil {
		solid := b.Planar.Solid
		out.Planar = &solid
	} else {
		solid := b.Solid
		out.Trimmed = &solid
	}
	return out, true
}

// WriteSTEP writes the verified complete original graph without pose baking.
func (d *RationalSphericalHierarchyDocument) WriteSTEP(w io.Writer) error {
	if d == nil {
		return geom.ErrInvalid
	}
	return writeRationalHierarchyDocumentBytes(w, d.source)
}

// Write writes the verified native envelope containing the original source and
// the exact protected local material and placement claims.
func (d *RationalSphericalHierarchyDocument) Write(w io.Writer) error {
	if d == nil {
		return geom.ErrInvalid
	}
	return writeRationalHierarchyDocumentBytes(w, d.native)
}

func writeRationalHierarchyDocumentBytes(w io.Writer, data []byte) error {
	if w == nil || len(data) == 0 {
		return geom.ErrInvalid
	}
	n, err := w.Write(data)
	if err == nil && n != len(data) {
		return io.ErrShortWrite
	}
	return err
}

// Bounds encloses the actual geometry in the original model-coordinate units.
func (d *RationalSphericalHierarchyDocument) Bounds(ctx context.Context, o step.RationalHierarchyBoundsOptions) (step.RationalHierarchyBounds, error) {
	if d == nil {
		return step.RationalHierarchyBounds{}, geom.ErrInvalid
	}
	if d.vendor {
		return step.BoundOCCTRationalSphericalHierarchy(ctx, step.OCCTRationalSphericalHierarchy(d.model), o)
	}
	return step.BoundRationalSphericalHierarchy(ctx, d.model, o)
}

// Locate classifies each supported material occurrence independently, including
// authenticated reentrant planar boundaries.
func (d *RationalSphericalHierarchyDocument) Locate(ctx context.Context, point [3]*big.Rat, o step.RationalHierarchyPointOptions) (step.RationalHierarchyPointLocations, error) {
	if d == nil {
		return step.RationalHierarchyPointLocations{}, geom.ErrInvalid
	}
	if d.vendor {
		return step.LocateGeneralOCCTRationalHierarchyPoint(ctx, step.OCCTRationalSphericalHierarchy(d.model), point, o)
	}
	return step.LocateGeneralRationalHierarchyPoint(ctx, d.model, point, o)
}

// ClipLine retains original line parameters and every exact occurrence owner.
func (d *RationalSphericalHierarchyDocument) ClipLine(ctx context.Context, origin, direction [3]*big.Rat, lower, upper *big.Rat, o step.RationalHierarchyLineOptions) (step.RationalHierarchyLine, error) {
	if d == nil {
		return step.RationalHierarchyLine{}, geom.ErrInvalid
	}
	if d.vendor {
		return step.ClipOCCTRationalHierarchyAnalyticLine(ctx, step.OCCTRationalSphericalHierarchy(d.model), origin, direction, lower, upper, o)
	}
	return step.ClipRationalHierarchyAnalyticLine(ctx, d.model, origin, direction, lower, upper, o)
}

// ClipGeneralLine adds transverse clipping of authenticated nonconvex planar
// occurrences. A planar occurrence is unsupported when the line lies in one
// of its supporting planes; use ClipLine for coplanar convex occurrences.
func (d *RationalSphericalHierarchyDocument) ClipGeneralLine(ctx context.Context, origin, direction [3]*big.Rat, lower, upper *big.Rat, o step.RationalHierarchyLineOptions) (step.RationalHierarchyLine, error) {
	if d == nil {
		return step.RationalHierarchyLine{}, geom.ErrInvalid
	}
	if d.vendor {
		return step.ClipGeneralOCCTRationalHierarchyLine(ctx, step.OCCTRationalSphericalHierarchy(d.model), origin, direction, lower, upper, o)
	}
	return step.ClipGeneralRationalHierarchyLine(ctx, d.model, origin, direction, lower, upper, o)
}

// Mass returns exact symbolic unit-density moments summed across occurrences.
func (d *RationalSphericalHierarchyDocument) Mass(ctx context.Context, o step.RationalHierarchyMassOptions) (step.RationalHierarchyMass, error) {
	if d == nil {
		return step.RationalHierarchyMass{}, geom.ErrInvalid
	}
	if d.vendor {
		return step.MeasureOCCTRationalSphericalHierarchy(ctx, step.OCCTRationalSphericalHierarchy(d.model), o)
	}
	return step.MeasureRationalSphericalHierarchy(ctx, d.model, o)
}

// Relationships returns the proved source uses, including their original frame
// IDs and immutable exact local matrices. One relationship may expand to more
// than one material path.
func (d *RationalSphericalHierarchyDocument) Relationships() []step.ExactNAUOOccurrence {
	if d == nil {
		return nil
	}
	return append([]step.ExactNAUOOccurrence(nil), d.model.Relationships...)
}

// EditRelationships returns a new source-bound document after applying exact
// local edits. All paths using a selected relationship adopt the same placement.
// The original document is unchanged if validation or publication fails.
func (d *RationalSphericalHierarchyDocument) EditRelationships(ctx context.Context, edits []step.RationalSphericalHierarchyEdit, o STEPAssemblyOptions) (*RationalSphericalHierarchyDocument, error) {
	if d == nil {
		return nil, geom.ErrInvalid
	}
	var output bytes.Buffer
	if d.vendor {
		if err := step.WriteOCCTRationalSphericalHierarchyEdits(ctx, &output, d.model.Source, edits, o.Geometry, d.model.Tolerance()); err != nil {
			return nil, err
		}
		return ReadOCCTRationalSphericalHierarchyDocumentSTEP(ctx, &output, o, d.model.Tolerance())
	}
	if err := step.WriteRationalSphericalHierarchyEdits(ctx, &output, d.model.Source, edits, o.Geometry, d.model.Tolerance()); err != nil {
		return nil, err
	}
	return ReadRationalSphericalHierarchyDocumentSTEP(ctx, &output, o, d.model.Tolerance())
}
