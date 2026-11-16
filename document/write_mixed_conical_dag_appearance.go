package document

import (
	"context"
	"io"
	"strings"

	"plainsolid/exchange/step"
	"plainsolid/geom"
)

// WriteMixedConicalAP242DAGAppearanceSTEP transfers occurrence names,
// placements and repainted fills to the original styled STEP. Rotations
// require an explicit exact edit. The original
// source, full style owner paths, local geometry and units are re-proved.
// Unchanged documents write the original byte sequence.
func (s State) WriteMixedConicalAP242DAGAppearanceSTEP(ctx context.Context, w io.Writer, original io.Reader, l MixedConicalAP242DAGDocumentLimits, geometry geom.SurfacePatchOptions) error {
	if ctx == nil || w == nil || original == nil || len(s.Features) != 0 || l.MaxNodes < 0 || l.MaxNodes > mixedConicalDAGDocumentNodeCap {
		return geom.ErrInvalid
	}
	if err := s.Validate(); err != nil {
		return err
	}
	a, err := step.ReadMixedConicalAP242DAGAppearance(ctx, original, l.Assembly, geometry)
	if err != nil {
		return err
	}
	d, err := mixedConicalDAGAppearanceDocument(ctx, a, l.MaxNodes, 0)
	if err != nil {
		return err
	}
	base, _ := d.Snapshot()
	colors, err := mixedConicalDAGColorEdits(base, s)
	if err != nil {
		return err
	}
	occurrences, err := s.mixedConicalDocumentEdits(ctx, base, a.DAG.Occurrences)
	if err != nil {
		return err
	}
	return step.WriteMixedConicalAP242DAGAppearanceEdits(ctx, w, strings.NewReader(a.OriginalSTEP()), occurrences, colors, l.Assembly, geometry)
}
