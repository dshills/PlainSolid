package step

import (
	"bytes"
	"context"
	"math"
	"plainsolid/geom"
	"reflect"
)

// SetOccurrenceSurfaceColor creates or edits a whole-body contextual override
// for one original NAUO in a flat planar assembly. Creation requires an existing
// product-local whole-body style and no face styles on that child. No style or
// placement is inferred. An existing equal override is an exact source no-op.
func SetOccurrenceSurfaceColor(ctx context.Context, source *File, occurrenceID int64, color RGB, l Limits) (OccurrenceColoredAssembly, error) {
	var zero OccurrenceColoredAssembly
	if ctx == nil || !validRGB(color) || !independentNativeSourceRecordBound(source, 4096, 8) {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	raw, e := mappedPlanarSourceBytes(source)
	if e != nil {
		return zero, e
	}
	if len(raw) > 1<<20 {
		return zero, geom.ErrInvalid
	}
	l, e = occurrenceColorSourceLimits(l)
	if e != nil {
		return zero, e
	}
	contextual := false
	for _, en := range source.Entities {
		for _, r := range en.Records {
			if r.Type == "PRESENTATION_STYLE_BY_CONTEXT" {
				contextual = true
			}
		}
	}
	var before OccurrenceColoredAssembly
	if contextual {
		before, e = ReadOccurrenceColoredAssembly(bytes.NewReader(raw), l)
	} else {
		before.Assembly, e = ReadColoredAssembly(bytes.NewReader(raw), l)
	}
	if e != nil {
		return zero, e
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	for _, c := range before.Overrides {
		if c.OccurrenceID == occurrenceID {
			return editAuthenticatedOccurrenceSurfaceColor(ctx, source, raw, before, occurrenceID, color, l)
		}
	}
	a := before.Assembly.Assembly
	if len(a.Roots) != 1 {
		return zero, geom.ErrInvalid
	}
	var target AssemblyOccurrence
	found := false
	for _, o := range a.Occurrences {
		if o.Parent != a.Roots[0] || len(before.Assembly.ProductBodies[o.Child]) == 0 {
			return zero, geom.ErrInvalid
		}
		if o.ID == occurrenceID {
			target = o
			found = true
		}
	}
	if !found {
		return zero, geom.ErrInvalid
	}
	bodies := before.Assembly.ProductBodies[target.Child]
	if len(bodies) != 1 || bodies[0].SurfaceColor == nil || len(bodies[0].FaceColors) != 0 {
		return zero, geom.ErrInvalid
	}
	bodyIDs, _, e := a.productItems(target.Child)
	if e != nil || len(bodyIDs) != 1 {
		return zero, geom.ErrInvalid
	}
	body := bodyIDs[0]
	_, contexts, e := colorBodyIDs(source)
	if e != nil {
		return zero, e
	}
	bodyContext := contexts[body]
	if bodyContext == 0 {
		return zero, geom.ErrInvalid
	}
	var baseStyle, presentation int64
	for _, id := range source.Order {
		en := source.Entities[id]
		if len(en.Records) != 1 || en.Records[0].Type != "STYLED_ITEM" {
			continue
		}
		r := en.Records[0]
		if len(r.Args) != 3 {
			return zero, geom.ErrInvalid
		}
		if r.Args[2].Kind == Reference && r.Args[2].Ref == body {
			if baseStyle != 0 {
				return zero, geom.ErrInvalid
			}
			baseStyle = id
		}
	}
	if baseStyle == 0 {
		return zero, geom.ErrInvalid
	}
	for _, id := range source.Order {
		en := source.Entities[id]
		if len(en.Records) != 1 || en.Records[0].Type != "MECHANICAL_DESIGN_GEOMETRIC_PRESENTATION_REPRESENTATION" {
			continue
		}
		r := en.Records[0]
		if len(r.Args) != 3 {
			return zero, geom.ErrInvalid
		}
		items, e := listRefs(r.Args[1], 1)
		if e != nil {
			return zero, e
		}
		for _, item := range items {
			if item == baseStyle {
				if presentation != 0 || r.Args[2].Kind != Reference || r.Args[2].Ref != bodyContext {
					return zero, geom.ErrInvalid
				}
				presentation = id
			}
		}
	}
	if presentation == 0 {
		return zero, geom.ErrInvalid
	}
	var pdsID, cdsrID, tid, placementID int64
	for _, id := range source.Order {
		en := source.Entities[id]
		if len(en.Records) != 1 || en.Records[0].Type != "CONTEXT_DEPENDENT_SHAPE_REPRESENTATION" {
			continue
		}
		cd, e := simple(source, id, "CONTEXT_DEPENDENT_SHAPE_REPRESENTATION", 2)
		if e != nil {
			return zero, e
		}
		pds, e := ref(cd.Args[1])
		if e != nil {
			return zero, e
		}
		r, e := simple(source, pds, "PRODUCT_DEFINITION_SHAPE", 3)
		if e != nil {
			return zero, e
		}
		if r.Args[2].Kind != Reference || r.Args[2].Ref != occurrenceID {
			continue
		}
		if cdsrID != 0 {
			return zero, geom.ErrInvalid
		}
		pdsID = pds
		cdsrID = id
		rel, e := ref(cd.Args[0])
		if e != nil {
			return zero, e
		}
		for _, rr := range source.Entities[rel].Records {
			if rr.Type == "REPRESENTATION_RELATIONSHIP_WITH_TRANSFORMATION" {
				if len(rr.Args) != 1 || tid != 0 {
					return zero, geom.ErrInvalid
				}
				tid, e = ref(rr.Args[0])
				if e != nil {
					return zero, e
				}
			}
		}
		tr, e := simple(source, tid, "ITEM_DEFINED_TRANSFORMATION", 4)
		if e != nil {
			return zero, e
		}
		placementID, e = ref(tr.Args[3])
		if e != nil {
			return zero, e
		}
	}
	if cdsrID == 0 || placementID == 0 {
		return zero, geom.ErrInvalid
	}
	view, e := Parse(bytes.NewReader(raw), Limits{})
	if e != nil {
		return zero, e
	}
	maxID := int64(0)
	for id := range source.Entities {
		maxID = max(maxID, id)
	}
	if maxID > math.MaxInt64-10 {
		return zero, geom.ErrConvergence
	}
	contextID, sdrID, styleID, assignmentID := maxID+1, maxID+2, maxID+3, maxID+4
	usageID, sideID, fillID, areaID, fillColorID, colorID := maxID+5, maxID+6, maxID+7, maxID+8, maxID+9, maxID+10
	refv := func(id int64) Value { return Value{Kind: Reference, Ref: id} }
	list := func(id int64) Value { return Value{Kind: Aggregate, Items: []Value{refv(id)}} }
	str := Value{Kind: String}
	emit := func(id int64, kind string, args ...Value) {
		view.Entities[id] = Entity{ID: id, Records: []Record{{Type: kind, Args: args}}}
		view.Order = append(view.Order, id)
	}
	emit(contextID, "SHAPE_REPRESENTATION", str, list(placementID), refv(bodyContext))
	emit(sdrID, "SHAPE_DEFINITION_REPRESENTATION", refv(pdsID), refv(contextID))
	emit(styleID, "OVER_RIDING_STYLED_ITEM", Value{Kind: String, Text: "overriding color"}, list(assignmentID), refv(body), refv(baseStyle))
	emit(assignmentID, "PRESENTATION_STYLE_BY_CONTEXT", list(usageID), refv(contextID))
	emit(usageID, "SURFACE_STYLE_USAGE", Value{Kind: Enumeration, Text: "BOTH"}, refv(sideID))
	emit(sideID, "SURFACE_SIDE_STYLE", str, list(fillID))
	emit(fillID, "SURFACE_STYLE_FILL_AREA", refv(areaID))
	emit(areaID, "FILL_AREA_STYLE", str, list(fillColorID))
	emit(fillColorID, "FILL_AREA_STYLE_COLOUR", str, refv(colorID))
	emit(colorID, "COLOUR_RGB", str, Value{Kind: Number, Text: real(color.R)}, Value{Kind: Number, Text: real(color.G)}, Value{Kind: Number, Text: real(color.B)})
	en := view.Entities[presentation]
	r := en.Records[0]
	r.Args[1].Items = append(r.Args[1].Items, refv(styleID))
	view.Entities[presentation] = Entity{ID: presentation, Records: []Record{r}}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	edited, e := mappedPlanarSourceBytes(view)
	if e != nil {
		return zero, e
	}
	after, e := ReadOccurrenceColoredAssembly(bytes.NewReader(edited), l)
	if e != nil {
		return zero, e
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	// The expected claim is authored from authenticated original relationships;
	// only the complete post-construction reader is allowed to accept it.
	before.Overrides = append(append([]OccurrenceSurfaceColor(nil), before.Overrides...), OccurrenceSurfaceColor{OccurrenceID: occurrenceID, BodyID: body, StyledItemID: styleID, BaseStyledItemID: baseStyle, AssignmentID: assignmentID, ContextRepresentationID: contextID, DefinitionRepresentationID: sdrID, ProductShapeID: pdsID, CDSRID: cdsrID, TransformationID: tid, PlacementID: placementID, Color: color})
	expected, e := occurrenceColorsClaims(ctx, before)
	if e != nil {
		return zero, e
	}
	actual, e := occurrenceColorsClaims(ctx, after)
	if e != nil {
		return zero, e
	}
	expected.Source = ""
	actual.Source = ""
	if !reflect.DeepEqual(expected, actual) {
		return zero, geom.ErrInvalid
	}
	return after, nil
}
