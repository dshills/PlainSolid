package step

import (
	"bytes"
	"context"
	"io"
	"slices"

	"plainsolid/geom"
)

// MixedConicalAP242SurfaceColorEdit repaints an existing, source-owned
// product, face or contextual fill using its original styled item ID.
type MixedConicalAP242SurfaceColorEdit struct {
	StyledItemID int64
	Color        RGB
}

// WriteMixedConicalAP242DAGAppearanceEdits updates occurrence names and
// translations and paints existing style owners. The original SHUO continues
// to select the same full path, including when its terminal use moves. The
// writer re-proves the edited graph and appearance before writing any bytes.
func WriteMixedConicalAP242DAGAppearanceEdits(ctx context.Context, w io.Writer, original io.Reader, occurrences []MixedConicalAP242OccurrenceEdit, colors []MixedConicalAP242SurfaceColorEdit, l MixedConicalAP242DAGLimits, options geom.SurfacePatchOptions) error {
	if ctx == nil || w == nil || original == nil || len(occurrences) > 64 || len(colors) > 64 {
		return geom.ErrInvalid
	}
	model, err := ReadMixedConicalAP242DAGAppearance(ctx, original, l, options)
	if err != nil {
		return err
	}
	raw := []byte(model.OriginalSTEP())
	if len(occurrences)+len(colors) != 0 {
		raw, err = mixedConicalDAGAppearanceEdits(ctx, model, occurrences, colors, l, options)
		if err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	n, err := w.Write(raw)
	if err == nil && n != len(raw) {
		return io.ErrShortWrite
	}
	return err
}

func mixedConicalDAGAppearanceEdits(ctx context.Context, model MixedConicalAP242DAGAppearance, occurrences []MixedConicalAP242OccurrenceEdit, colors []MixedConicalAP242SurfaceColorEdit, l MixedConicalAP242DAGLimits, options geom.SurfacePatchOptions) ([]byte, error) {
	base := mixedConicalEditInput{model.DAG.Assembly, model.DAG.Occurrences, len(model.DAG.Instances)}
	f := base.a.Source
	override := &model.PathOverride.Context
	contextRep := f.Entities[override.ContextRepresentationID].Records[0]
	// The authenticated style context follows its terminal use as a unit.
	// Detach this one owner during the frame editor's sharing check to retain
	// the original placement ID whenever those two owners can move together.
	contextPlacements := contextRep.Args[1]
	contextRep.Args[1] = Value{Kind: Aggregate}
	known, err := editMixedConicalAP242Occurrences(ctx, base, occurrences)
	if err != nil {
		return nil, err
	}
	override.PlacementID = known[override.OccurrenceID].ToPlacementID
	contextPlacements.Items[0].Ref = override.PlacementID
	contextRep.Args[1] = contextPlacements

	styles := make(map[int64]*RGB, len(model.ProductStyles)+len(model.FaceStyles)+1)
	for i := range model.ProductStyles {
		style := &model.ProductStyles[i]
		styles[style.StyledItemID] = &style.Color
	}
	for i := range model.FaceStyles {
		style := &model.FaceStyles[i]
		styles[style.StyledItemID] = &style.Color
	}
	styles[override.StyledItemID] = &override.Color
	for _, e := range colors {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		c, ok := styles[e.StyledItemID]
		if !ok || !validRGB(e.Color) {
			return nil, geom.ErrInvalid
		}
		delete(styles, e.StyledItemID)
		if *c != e.Color {
			if err := paintMixedConicalDAGStyle(f, e); err != nil {
				return nil, err
			}
			*c = e.Color
		}
	}
	raw, err := mappedPlanarSourceBytes(f)
	if err != nil {
		return nil, err
	}
	fresh, err := ReadMixedConicalAP242DAGAppearance(ctx, bytes.NewReader(raw), l, options)
	if err != nil {
		return nil, err
	}
	got := mixedConicalEditInput{fresh.DAG.Assembly, fresh.DAG.Occurrences, len(fresh.DAG.Instances)}
	if err := checkMixedConicalAP242Edits(base, got, known); err != nil {
		return nil, err
	}
	if !slices.Equal(model.ProductStyles, fresh.ProductStyles) || !slices.Equal(model.FaceStyles, fresh.FaceStyles) ||
		model.PathOverride != fresh.PathOverride {
		return nil, geom.ErrUnrepresentable
	}
	return raw, nil
}

func paintMixedConicalDAGStyle(f *File, e MixedConicalAP242SurfaceColorEdit) error {
	// The source proof requires a singleton assignment and a private fill
	// chain. Preserve its entity IDs while replacing the final colour.
	item := f.Entities[e.StyledItemID].Records[0]
	assignment := f.Entities[item.Args[1].Items[0].Ref].Records[0]
	_, chain, err := occurrenceSurfaceFill(f, assignment.Args[0].Items[0].Ref)
	if err != nil {
		return err
	}
	id := chain[len(chain)-1]
	en := f.Entities[id]
	name := Value{Kind: String}
	if en.Records[0].Type == "COLOUR_RGB" {
		name = en.Records[0].Args[0]
	}
	en.Records[0] = Record{Type: "COLOUR_RGB", Args: []Value{
		name, {Kind: Number, Text: real(e.Color.R)}, {Kind: Number, Text: real(e.Color.G)}, {Kind: Number, Text: real(e.Color.B)},
	}}
	f.Entities[id] = en
	return nil
}
