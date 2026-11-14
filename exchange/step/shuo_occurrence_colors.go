package step

import (
	"bytes"
	"context"
	"io"
	"plainsolid/geom"
)

// SHUOSurfaceColor records the two original usage conditions and the contextual
// chain. Context.ProductShapeID is the original SHUO PDS; its CDSR/transform
// identifies the independently authenticated next-use placement.
type SHUOSurfaceColor struct {
	SHUOID                                            int64
	Path                                              [2]int64
	RootDefinitionID, GroupDefinitionID, DefinitionID int64
	Context                                           OccurrenceSurfaceColor
}

// SHUOOccurrenceColoredAssembly is the selected two-level path-specific style
// subset. It is distinct from an override on a shared internal NAUO.
type SHUOOccurrenceColoredAssembly struct {
	model  NestedOccurrenceColoredAssembly
	claims []SHUOSurfaceColor
}

func (m SHUOOccurrenceColoredAssembly) Paths() []NestedColoredPath { return m.model.Paths() }
func (m SHUOOccurrenceColoredAssembly) Overrides() []SHUOSurfaceColor {
	return append([]SHUOSurfaceColor(nil), m.claims...)
}

// ReadSHUOOccurrenceColoredAssembly authenticates ordinary two-use SHUO paths
// on one repeated group, with exactly one SHUO override. Mixed direct/shared
// overrides, additional or recursive SHUOs, group
// colors and deeper graphs are unsupported. No source geometry is repaired.
func ReadSHUOOccurrenceColoredAssembly(ctx context.Context, r io.Reader, l Limits) (SHUOOccurrenceColoredAssembly, error) {
	var zero SHUOOccurrenceColoredAssembly
	if ctx == nil || r == nil {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	l, e := occurrenceColorSourceLimits(l)
	if e != nil {
		return zero, e
	}
	f, e := Parse(r, l)
	if e != nil {
		return zero, e
	}
	raw, e := mappedPlanarSourceBytes(f)
	if e != nil {
		return zero, e
	}
	view, e := Parse(bytes.NewReader(raw), l)
	if e != nil {
		return zero, e
	}
	incoming := map[int64][]int64{}
	var walk func(int64, Value)
	walk = func(id int64, v Value) {
		if v.Kind == Reference {
			incoming[v.Ref] = append(incoming[v.Ref], id)
		}
		for _, q := range v.Items {
			walk(id, q)
		}
	}
	for id, en := range f.Entities {
		for _, rec := range en.Records {
			for _, v := range rec.Args {
				walk(id, v)
			}
		}
	}
	type route struct {
		claim                      SHUOSurfaceColor
		sdr, originalPDS, innerPDS int64
	}
	var routes []route
	remove := map[int64]bool{}
	for _, id := range f.Order {
		en := f.Entities[id]
		has := false
		for _, r := range en.Records {
			if r.Type == "SPECIFIED_HIGHER_USAGE_OCCURRENCE" {
				has = true
			}
		}
		if !has {
			continue
		}
		s, e := simple(f, id, "SPECIFIED_HIGHER_USAGE_OCCURRENCE", 8)
		if e != nil {
			return zero, e
		}
		for _, j := range []int{0, 1} {
			if _, e = textValue(s.Args[j]); e != nil {
				return zero, e
			}
		}
		for _, j := range []int{2, 5} {
			if s.Args[j].Kind != Omitted {
				if _, e = textValue(s.Args[j]); e != nil {
					return zero, e
				}
			}
		}
		refs, e := namedReferences(s, 3, 4, 6, 7)
		if e != nil {
			return zero, e
		}
		root, leaf, upper, next := refs[0], refs[1], refs[2], refs[3]
		u, e := simple(f, upper, "NEXT_ASSEMBLY_USAGE_OCCURRENCE", 6)
		if e != nil {
			return zero, e
		}
		n, e := simple(f, next, "NEXT_ASSEMBLY_USAGE_OCCURRENCE", 6)
		if e != nil {
			return zero, e
		}
		ur, e := namedReferences(u, 3, 4)
		if e != nil {
			return zero, e
		}
		nr, e := namedReferences(n, 3, 4)
		if e != nil {
			return zero, e
		}
		if upper == next || ur[0] != root || ur[1] != nr[0] || nr[1] != leaf || root == leaf || root == ur[1] || leaf == ur[1] {
			return zero, geom.ErrInvalid
		}
		if len(incoming[id]) != 1 {
			return zero, geom.ErrInvalid
		}
		pdsID := incoming[id][0]
		pds, e := simple(f, pdsID, "PRODUCT_DEFINITION_SHAPE", 3)
		if e != nil || pds.Args[2].Ref != id || pds.Args[2].Kind != Reference {
			return zero, geom.ErrInvalid
		}
		if _, e = textValue(pds.Args[0]); e != nil {
			return zero, e
		}
		if pds.Args[1].Kind != Omitted {
			if _, e = textValue(pds.Args[1]); e != nil {
				return zero, e
			}
		}
		if len(incoming[pdsID]) != 1 {
			return zero, geom.ErrInvalid
		}
		sdrID := incoming[pdsID][0]
		sdr, e := simple(f, sdrID, "SHAPE_DEFINITION_REPRESENTATION", 2)
		if e != nil || sdr.Args[0].Kind != Reference || sdr.Args[0].Ref != pdsID {
			return zero, geom.ErrInvalid
		}
		var innerPDS int64
		for _, owner := range incoming[next] {
			q, e := simple(f, owner, "PRODUCT_DEFINITION_SHAPE", 3)
			if e != nil {
				continue
			}
			if q.Args[2].Kind == Reference && q.Args[2].Ref == next {
				if innerPDS != 0 {
					return zero, geom.ErrInvalid
				}
				innerPDS = owner
			}
		}
		if innerPDS == 0 {
			return zero, geom.ErrInvalid
		}
		// Only this private proof view substitutes the already proved next-use PDS.
		// The original SHUO and both path conditions remain in the returned evidence.
		sdr.Args = append([]Value(nil), sdr.Args...)
		sdr.Args[0] = Value{Kind: Reference, Ref: innerPDS}
		view.Entities[sdrID] = Entity{ID: sdrID, Records: []Record{sdr}}
		remove[id] = true
		remove[pdsID] = true
		routes = append(routes, route{SHUOSurfaceColor{SHUOID: id, Path: [2]int64{upper, next}, RootDefinitionID: root, GroupDefinitionID: ur[1], DefinitionID: leaf}, sdrID, pdsID, innerPDS})
	}
	if len(routes) != 1 {
		return zero, geom.ErrInvalid
	}
	var order []int64
	for _, id := range view.Order {
		if remove[id] {
			delete(view.Entities, id)
		} else {
			order = append(order, id)
		}
	}
	view.Order = order
	proofRaw, e := mappedPlanarSourceBytes(view)
	if e != nil {
		return zero, e
	}
	proof, e := ReadNestedOccurrenceColoredAssembly(ctx, bytes.NewReader(proofRaw), l)
	if e != nil {
		return zero, e
	}
	// The selected internal use can differ between the two outer paths.
	// The reused chain proof temporarily interprets it as a shared override; here
	// all defaults are restored before applying the original two-condition claim.
	for i := range proof.paths {
		p := &proof.paths[i]
		p.Color = *proof.model.Assembly.ProductBodies[p.DefinitionID][0].SurfaceColor
		p.Overridden = false
	}
	out := SHUOOccurrenceColoredAssembly{model: proof}
	used := map[int64]bool{}
	pathColors := map[[2]int64]RGB{}
	for _, c := range proof.model.Overrides {
		var found *route
		for i := range routes {
			q := &routes[i]
			if c.DefinitionRepresentationID == q.sdr {
				if found != nil {
					return zero, geom.ErrInvalid
				}
				found = q
			}
		}
		if found == nil || c.ProductShapeID != found.innerPDS || c.OccurrenceID != found.claim.Path[1] {
			return zero, geom.ErrInvalid
		}
		q := found.claim
		q.Context = c
		q.Context.ProductShapeID = found.originalPDS
		used[found.sdr] = true
		if old, ok := pathColors[q.Path]; ok && old != c.Color {
			return zero, geom.ErrInvalid
		}
		pathColors[q.Path] = c.Color
		matches := 0
		for i := range out.model.paths {
			p := &out.model.paths[i]
			if p.Path == q.Path {
				if p.RootDefinitionID != q.RootDefinitionID || p.GroupDefinitionID != q.GroupDefinitionID || p.DefinitionID != q.DefinitionID || p.BodyID != c.BodyID {
					return zero, geom.ErrInvalid
				}
				p.Color = c.Color
				p.Overridden = true
				matches++
			}
		}
		if matches != 1 {
			return zero, geom.ErrInvalid
		}
		out.claims = append(out.claims, q)
	}
	if len(used) != len(routes) {
		return zero, geom.ErrInvalid
	}
	out.model.model.Assembly.Assembly.Source = f
	// Keep source context ancestry, not the private proof-view PDS substitution.
	for i := range out.model.model.Overrides {
		out.model.model.Overrides[i] = out.claims[i].Context
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	return out, nil
}

func (m SHUOOccurrenceColoredAssembly) NativeAssembly(ctx context.Context) (ColoredAssembly, error) {
	raw, e := m.source()
	if e != nil {
		return ColoredAssembly{}, e
	}
	q, e := ReadSHUOOccurrenceColoredAssembly(ctx, bytes.NewReader(raw), Limits{})
	if e != nil {
		return ColoredAssembly{}, e
	}
	return q.model.model.Assembly, nil
}
func (m SHUOOccurrenceColoredAssembly) source() ([]byte, error) {
	f := m.model.model.Assembly.Assembly.Source
	if !independentNativeSourceRecordBound(f, 4096, 8) {
		return nil, geom.ErrInvalid
	}
	return mappedPlanarSourceBytes(f)
}
func WriteSHUOOccurrenceColoredAssemblySource(ctx context.Context, w io.Writer, m SHUOOccurrenceColoredAssembly) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	raw, e := m.source()
	if e != nil {
		return e
	}
	if _, e = ReadSHUOOccurrenceColoredAssembly(ctx, bytes.NewReader(raw), Limits{}); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		return io.ErrShortWrite
	}
	return e
}
