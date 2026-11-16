package step

import (
	"bytes"
	"context"
	"errors"
	"io"
	"slices"

	"plainsolid/geom"
)

// MixedConicalAP242SurfaceStyle retains a product or face fill and its
// original body, styled item, presentation and assignment IDs. FaceID is
// zero for a product fill. NativeFaceIndex is zero-based for a face fill.
type MixedConicalAP242SurfaceStyle struct {
	DefinitionID, BodyID, FaceID               int64
	StyledItemID, PresentationID, AssignmentID int64
	NativeFaceIndex                            int
	Color                                      RGB
}

// MixedConicalAP242DAGAppearance holds exact native local parts and the
// original XCAF appearance owners, including one depth-three SHUO.
type MixedConicalAP242DAGAppearance struct {
	DAG                       MixedConicalAP242DAG
	ProductStyles, FaceStyles []MixedConicalAP242SurfaceStyle
	PathOverride              DeepSHUOColor
	original                  string
}

// OriginalSTEP returns the complete, unmodified styled source.
func (m MixedConicalAP242DAGAppearance) OriginalSTEP() string { return m.original }

var errMixedConicalAP242DAGAppearance = errors.New("step: unsupported or inconsistent mixed conical AP242 DAG appearance")

// ReadMixedConicalAP242DAGAppearance proves two original product fills,
// original face fills and one OCCT depth-three contextual whole-body fill.
// It reconstructs the same strict curved DAG from a private style-free view.
func ReadMixedConicalAP242DAGAppearance(ctx context.Context, r io.Reader, l MixedConicalAP242DAGLimits, o geom.SurfacePatchOptions) (MixedConicalAP242DAGAppearance, error) {
	var zero MixedConicalAP242DAGAppearance
	if ctx == nil || r == nil {
		return zero, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	l, err := mixedConicalAP242DAGLimits(l)
	if err != nil {
		return zero, err
	}
	raw, err := io.ReadAll(io.LimitReader(r, l.Source.MaxBytes+1))
	if err != nil {
		return zero, err
	}
	if int64(len(raw)) > l.Source.MaxBytes {
		return zero, geom.ErrConvergence
	}
	style, err := readMixedConicalDAGStyleSource(raw, l.Source)
	if err != nil {
		return zero, err
	}
	model, err := ReadMixedConicalAP242DAG(ctx, bytes.NewReader(style.unstyled), l, o)
	if err != nil {
		return zero, err
	}
	return style.bind(model, string(raw))
}

type mixedConicalDAGStyleSource struct {
	file     *File
	unstyled []byte
	shuo     deepSHUOSource
	styles   []MixedConicalAP242SurfaceStyle
	override OccurrenceSurfaceColor
}

func readMixedConicalDAGStyleSource(raw []byte, l Limits) (mixedConicalDAGStyleSource, error) {
	var zero mixedConicalDAGStyleSource
	f, err := Parse(bytes.NewReader(raw), l)
	if err != nil {
		return zero, err
	}
	incoming := convexArcBoreIncoming(f)
	shuo, err := proveDeepSHUOSource(f, incoming)
	if err != nil {
		return zero, err
	}
	styles, override, stripped, err := proveMixedConicalDAGStyles(f, incoming, shuo)
	if err != nil {
		return zero, err
	}
	return mixedConicalDAGStyleSource{f, stripped, shuo, styles, override}, nil
}

func (s mixedConicalDAGStyleSource) bind(model MixedConicalAP242DAG, original string) (MixedConicalAP242DAGAppearance, error) {
	var zero MixedConicalAP242DAGAppearance
	out := MixedConicalAP242DAGAppearance{DAG: model, original: original}
	partIDs := map[int64]int64{}
	faceIDs := map[int64][]int64{}
	for id, part := range model.Parts {
		if part.Rounded != nil {
			p := part.Rounded
			partIDs[p.SourceBodyID], faceIDs[p.SourceBodyID] = id, p.NativeFaceSourceIDs
		} else {
			p := part.Chamfered
			partIDs[p.SourceBodyID], faceIDs[p.SourceBodyID] = id, p.NativeFaceSourceIDs
		}
	}
	for _, style := range s.styles {
		style.DefinitionID = partIDs[style.BodyID]
		if style.DefinitionID == 0 {
			return zero, errMixedConicalAP242DAGAppearance
		}
		if style.FaceID == 0 {
			out.ProductStyles = append(out.ProductStyles, style)
		} else {
			style.NativeFaceIndex = slices.Index(faceIDs[style.BodyID], style.FaceID)
			if style.NativeFaceIndex < 0 {
				return zero, errMixedConicalAP242DAGAppearance
			}
			out.FaceStyles = append(out.FaceStyles, style)
		}
	}
	path, err := bindMixedConicalDAGSHUO(model, partIDs[s.override.BodyID], s.shuo, s.override)
	if err != nil {
		return zero, err
	}
	out.PathOverride = path
	out.DAG.Assembly.Source = s.file
	return out, nil
}

func bindMixedConicalDAGSHUO(a MixedConicalAP242DAG, productID int64, shuo deepSHUOSource, c OccurrenceSurfaceColor) (DeepSHUOColor, error) {
	var zero DeepSHUOColor
	if productID != shuo.definitions[3] || a.Assembly.Roots[0] != shuo.definitions[0] {
		return zero, errMixedConicalAP242DAGAppearance
	}
	matches := 0
	for _, instance := range a.Instances {
		if instance.ProductID == productID && slices.Equal(instance.Path, shuo.path[:]) {
			matches++
		}
	}
	if matches != 1 {
		return zero, errMixedConicalAP242DAGAppearance
	}
	for _, use := range a.Occurrences {
		if use.NAUOID == shuo.path[2] && use.ChildID == productID && use.ToPlacementID == c.PlacementID {
			c.OccurrenceID, c.CDSRID, c.TransformationID = use.NAUOID, use.CDSRID, use.TransformationID
			return DeepSHUOColor{shuo.prefix, shuo.terminal, shuo.path, c}, nil
		}
	}
	return zero, errMixedConicalAP242DAGAppearance
}
