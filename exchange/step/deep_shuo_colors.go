package step

import (
	"bytes"
	"context"
	"io"
	"plainsolid/geom"
)

// DeepSHUOColoredPath retains three original NAUOs and the four product
// definitions along one expanded occurrence. Geometry remains local/shared.
type DeepSHUOColoredPath struct {
	Path        [3]int64
	Definitions [4]int64
	BodyID      int64
	Transform   geom.Affine
	Color       RGB
	Overridden  bool
}

// DeepSHUOColor binds OCCT's prefix-anchored contextual style to its unique
// terminal continuation and all three original usage conditions.
type DeepSHUOColor struct {
	PrefixSHUOID, TerminalSHUOID int64
	Path                         [3]int64
	Context                      OccurrenceSurfaceColor
}

// OCCTDeepSHUOAssembly is the bounded original OCCT depth-three SHUO dialect.
// It does not generalize arbitrary recursive SHUO or style inheritance.
type OCCTDeepSHUOAssembly struct {
	model OccurrenceColoredAssembly
	paths []DeepSHUOColoredPath
	claim DeepSHUOColor
}

func (m OCCTDeepSHUOAssembly) Paths() []DeepSHUOColoredPath {
	return append([]DeepSHUOColoredPath(nil), m.paths...)
}
func (m OCCTDeepSHUOAssembly) Override() DeepSHUOColor { return m.claim }

// ReadOCCTDeepSHUOAssembly requires one uniquely continued two-record SHUO
// chain and one style anchored on its prefix, as emitted by the pinned writer.
// All three usage endpoints and the terminal body's contextual chain are proved.
func ReadOCCTDeepSHUOAssembly(ctx context.Context, r io.Reader, l Limits) (OCCTDeepSHUOAssembly, error) {
	var zero OCCTDeepSHUOAssembly
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
		for _, r := range en.Records {
			for _, v := range r.Args {
				walk(id, v)
			}
		}
	}
	source, e := proveDeepSHUOSource(f, incoming)
	if e != nil {
		return zero, e
	}
	prefix, terminal, pdsID, sdrID := source.prefix, source.terminal, source.pdsID, source.sdrID
	path, defs, sdr := source.path, source.definitions, source.sdr
	var leafPDS int64
	for _, owner := range incoming[path[2]] {
		q, e := simple(f, owner, "PRODUCT_DEFINITION_SHAPE", 3)
		if e == nil && q.Args[2].Kind == Reference && q.Args[2].Ref == path[2] {
			if leafPDS != 0 {
				return zero, geom.ErrInvalid
			}
			leafPDS = owner
		}
	}
	if leafPDS == 0 {
		return zero, geom.ErrInvalid
	}
	sdr.Args = append([]Value(nil), sdr.Args...)
	sdr.Args[0] = Value{Kind: Reference, Ref: leafPDS}
	view.Entities[sdrID] = Entity{ID: sdrID, Records: []Record{sdr}}
	remove := map[int64]bool{prefix: true, terminal: true, pdsID: true}
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
	base, e := readOccurrenceColoredAssembly(bytes.NewReader(proofRaw), l, false)
	if e != nil {
		return zero, e
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	if len(base.Overrides) != 1 {
		return zero, geom.ErrInvalid
	}
	claim := base.Overrides[0]
	if claim.OccurrenceID != path[2] || claim.ProductShapeID != leafPDS || claim.DefinitionRepresentationID != sdrID {
		return zero, geom.ErrInvalid
	}
	paths, e := deepColorPaths(ctx, base)
	if e != nil {
		return zero, e
	}
	matches := 0
	for i := range paths {
		q := &paths[i]
		if q.Path == path {
			if q.Definitions != defs || q.BodyID != claim.BodyID {
				return zero, geom.ErrInvalid
			}
			q.Color = claim.Color
			q.Overridden = true
			matches++
		}
	}
	if matches != 1 {
		return zero, geom.ErrInvalid
	}
	claim.ProductShapeID = pdsID
	base.Overrides[0] = claim
	base.Assembly.Assembly.Source = f
	return OCCTDeepSHUOAssembly{base, paths, DeepSHUOColor{prefix, terminal, path, claim}}, nil
}

type deepSHUOSource struct {
	prefix, terminal, pdsID, sdrID int64
	path                           [3]int64
	definitions                    [4]int64
	sdr                            Record
}

func proveDeepSHUOSource(f *File, incoming map[int64][]int64) (deepSHUOSource, error) {
	var zero deepSHUOSource
	shuos := map[int64]Record{}
	for _, id := range f.Order {
		for _, r := range f.Entities[id].Records {
			if r.Type != "SPECIFIED_HIGHER_USAGE_OCCURRENCE" {
				continue
			}
			q, e := simple(f, id, r.Type, 8)
			if e != nil {
				return zero, e
			}
			for _, j := range []int{0, 1} {
				if _, e = textValue(q.Args[j]); e != nil {
					return zero, e
				}
			}
			for _, j := range []int{2, 5} {
				if q.Args[j].Kind != Omitted {
					if _, e = textValue(q.Args[j]); e != nil {
						return zero, e
					}
				}
			}
			if _, e = namedReferences(q, 3, 4, 6, 7); e != nil {
				return zero, e
			}
			shuos[id] = q
		}
	}
	if len(shuos) != 2 {
		return zero, geom.ErrInvalid
	}
	var prefix, terminal int64
	for id, q := range shuos {
		if _, ok := shuos[q.Args[6].Ref]; ok {
			if terminal != 0 {
				return zero, geom.ErrInvalid
			}
			terminal = id
			prefix = q.Args[6].Ref
		}
	}
	if prefix == 0 || terminal == prefix || len(incoming[terminal]) != 0 {
		return zero, geom.ErrInvalid
	}
	p, t := shuos[prefix], shuos[terminal]
	path := [3]int64{p.Args[6].Ref, p.Args[7].Ref, t.Args[7].Ref}
	var defs [4]int64
	for i, id := range path {
		q, e := simple(f, id, "NEXT_ASSEMBLY_USAGE_OCCURRENCE", 6)
		if e != nil {
			return zero, e
		}
		ends, e := namedReferences(q, 3, 4)
		if e != nil {
			return zero, e
		}
		if i == 0 {
			defs[0] = ends[0]
		} else if defs[i] != ends[0] {
			return zero, geom.ErrInvalid
		}
		defs[i+1] = ends[1]
	}
	seen := map[int64]bool{}
	for _, id := range defs {
		if seen[id] {
			return zero, geom.ErrInvalid
		}
		seen[id] = true
	}
	if p.Args[3].Ref != defs[0] || p.Args[4].Ref != defs[2] || t.Args[3].Ref != defs[0] || t.Args[4].Ref != defs[3] {
		return zero, geom.ErrInvalid
	}
	if len(incoming[prefix]) != 2 {
		return zero, geom.ErrInvalid
	}
	var pdsID int64
	for _, id := range incoming[prefix] {
		if id != terminal {
			if pdsID != 0 {
				return zero, geom.ErrInvalid
			}
			pdsID = id
		}
	}
	pds, e := simple(f, pdsID, "PRODUCT_DEFINITION_SHAPE", 3)
	if e != nil || pds.Args[2].Kind != Reference || pds.Args[2].Ref != prefix {
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
	return deepSHUOSource{prefix, terminal, pdsID, sdrID, path, defs, sdr}, nil
}

func deepColorPaths(ctx context.Context, m OccurrenceColoredAssembly) ([]DeepSHUOColoredPath, error) {
	a := m.Assembly.Assembly
	if len(a.Roots) != 1 || len(a.Products) > 16 {
		return nil, geom.ErrInvalid
	}
	root := a.Roots[0]
	for id, en := range a.Source.Entities {
		if len(en.Records) == 1 && en.Records[0].Type == "ITEM_DEFINED_TRANSFORMATION" {
			tr, e := simple(a.Source, id, "ITEM_DEFINED_TRANSFORMATION", 4)
			if e != nil {
				return nil, e
			}
			from, e := mappedExactFrame(a.Source, tr.Args[2].Ref)
			if e != nil || frameTransform(from) != geom.Identity() {
				return nil, geom.ErrInvalid
			}
			to, e := mappedExactFrame(a.Source, tr.Args[3].Ref)
			if e != nil || !planarSplineRigid(frameTransform(to)) {
				return nil, geom.ErrInvalid
			}
		}
	}
	commonChildren := func(parent int64) ([]AssemblyOccurrence, int64, error) {
		var uses []AssemblyOccurrence
		var child int64
		for _, o := range a.Occurrences {
			if o.Parent == parent {
				if child == 0 {
					child = o.Child
				}
				if child != o.Child {
					return nil, 0, geom.ErrInvalid
				}
				uses = append(uses, o)
			}
		}
		if len(uses) < 2 {
			return nil, 0, geom.ErrInvalid
		}
		return uses, child, nil
	}
	outer, mid, e := commonChildren(root)
	if e != nil {
		return nil, e
	}
	middle, group, e := commonChildren(mid)
	if e != nil {
		return nil, e
	}
	if root == mid || root == group || mid == group {
		return nil, geom.ErrInvalid
	}
	seen := map[int64]bool{root: true, mid: true, group: true}
	for _, id := range []int64{root, mid, group} {
		if len(m.Assembly.ProductBodies[id]) != 0 {
			return nil, geom.ErrInvalid
		}
	}
	var leaf []AssemblyOccurrence
	for _, o := range a.Occurrences {
		if !planarSplineRigid(o.Transform) {
			return nil, geom.ErrInvalid
		}
		if o.Parent == root || o.Parent == mid {
			continue
		}
		if o.Parent != group || o.Child == root || o.Child == mid || o.Child == group {
			return nil, geom.ErrInvalid
		}
		bs := m.Assembly.ProductBodies[o.Child]
		if len(bs) != 1 || bs[0].SurfaceColor == nil || len(bs[0].FaceColors) != 0 {
			return nil, geom.ErrInvalid
		}
		leaf = append(leaf, o)
		seen[o.Child] = true
	}
	if len(leaf) == 0 || len(outer) > 64 || len(middle) > 64/len(outer) || len(leaf) > 64/(len(outer)*len(middle)) || len(seen) != len(a.Products) {
		return nil, geom.ErrInvalid
	}
	var paths []DeepSHUOColoredPath
	for _, o := range outer {
		for _, u := range middle {
			pose, e := nestedColorExactCompose(o.Transform, u.Transform)
			if e != nil {
				return nil, e
			}
			for _, v := range leaf {
				if e = ctx.Err(); e != nil {
					return nil, e
				}
				world, e := nestedColorExactCompose(pose, v.Transform)
				if e != nil {
					return nil, e
				}
				ids, _, e := a.productItems(v.Child)
				if e != nil || len(ids) != 1 {
					return nil, geom.ErrInvalid
				}
				paths = append(paths, DeepSHUOColoredPath{[3]int64{o.ID, u.ID, v.ID}, [4]int64{root, mid, group, v.Child}, ids[0], world, *m.Assembly.ProductBodies[v.Child][0].SurfaceColor, false})
			}
		}
	}
	return paths, nil
}
func (m OCCTDeepSHUOAssembly) source() ([]byte, error) {
	f := m.model.Assembly.Assembly.Source
	if !independentNativeSourceRecordBound(f, 4096, 8) {
		return nil, geom.ErrInvalid
	}
	return mappedPlanarSourceBytes(f)
}
func (m OCCTDeepSHUOAssembly) NativeAssembly(ctx context.Context) (ColoredAssembly, error) {
	raw, e := m.source()
	if e != nil {
		return ColoredAssembly{}, e
	}
	q, e := ReadOCCTDeepSHUOAssembly(ctx, bytes.NewReader(raw), Limits{})
	if e != nil {
		return ColoredAssembly{}, e
	}
	return q.model.Assembly, nil
}
func WriteOCCTDeepSHUOAssemblySource(ctx context.Context, w io.Writer, m OCCTDeepSHUOAssembly) error {
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
	if _, e = ReadOCCTDeepSHUOAssembly(ctx, bytes.NewReader(raw), Limits{}); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		return io.ErrShortWrite
	}
	return e
}
