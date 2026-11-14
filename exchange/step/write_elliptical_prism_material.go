package step

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/big"
	"plainsolid/geom"
	"unicode/utf8"
)

// EllipticalPrismSTEPOptions authors a new product, optionally translating its
// native geometry exactly and changing its name. Empty Name retains the source
// product name. Units remain the qualified millimeter context. These edits do
// not mutate the supplied material or its retained original source.
type EllipticalPrismSTEPOptions struct {
	Name        string
	Translation geom.Vec3
}

// WriteEllipticalPrismMaterialSTEP writes a new rooted model-only analytic
// B-rep of the re-proved native solid. Full natural ellipse rims, their shared
// seam LINE, extrusion support and cap PLANE records carry the geometry; no
// approximate decimal pcurve period is emitted. This exporter is distinct from
// original source re-export and does not broaden any strict source reader.
// All required translated source points AND native quarter control points
// must be exactly representable before output begins.
func WriteEllipticalPrismMaterialSTEP(ctx context.Context, w io.Writer, m EllipticalPrismMaterial, o EllipticalPrismSTEPOptions) error {
	if ctx == nil || w == nil || !o.Translation.IsFinite() || len(o.Name) > 1024 || !utf8.ValidString(o.Name) {
		return geom.ErrInvalid
	}
	for _, c := range o.Name {
		if c < 32 || c == 127 {
			return geom.ErrInvalid
		}
	}
	// Existing strict recipe validation binds every native surface, curve,
	// repaired source and ancestry field. It also handles cancellation.
	var checked bytes.Buffer
	if e := WriteEllipticalPrismMaterialJSON(ctx, &checked, m); e != nil {
		return e
	}
	translate := func(v geom.Vec3) (geom.Vec3, error) {
		a, b := [3]float64{v.X, v.Y, v.Z}, [3]float64{o.Translation.X, o.Translation.Y, o.Translation.Z}
		for i := range a {
			q := new(big.Rat).Add(new(big.Rat).SetFloat64(a[i]), new(big.Rat).SetFloat64(b[i]))
			var ok bool
			a[i], ok = q.Float64()
			if !ok || !finiteNumber(a[i]) {
				return geom.Vec3{}, geom.ErrUnrepresentable
			}
		}
		return geom.Vec3{X: a[0], Y: a[1], Z: a[2]}, nil
	}
	for _, v := range m.Solid.Vertices() {
		if _, e := translate(v); e != nil {
			return e
		}
	}
	for _, f := range m.Solid.Faces() {
		switch s := f.Surface.(type) {
		case *geom.BSplineSurface:
			for _, v := range s.Data().Poles {
				if _, e := translate(v); e != nil {
					return e
				}
			}
		case geom.PlaneSurface:
			if _, e := translate(s.Frame().Origin()); e != nil {
				return e
			}
		default:
			return fmt.Errorf("step: unexpected ellipse material support")
		}
	}
	raw, e := mappedPlanarSourceBytes(m.Shell.Source)
	if e != nil {
		return e
	}
	f, e := Parse(bytes.NewReader(raw), Limits{})
	if e != nil {
		return e
	}
	var template bytes.Buffer
	analyticSupportHeader(&template)
	template.WriteString(ellipticalPrismTemplate)
	template.WriteString("ENDSEC;END-ISO-10303-21;")
	want, e := Parse(&template, Limits{})
	if e != nil {
		return e
	}
	roots := [][2]int64{{1, 0}, {3, 0}, {121, 0}}
	for id, en := range f.Entities {
		if len(en.Records) != 1 {
			continue
		}
		for i, k := range []string{"APPLICATION_PROTOCOL_DEFINITION", "SHAPE_DEFINITION_REPRESENTATION", "PRODUCT_RELATED_PRODUCT_CATEGORY"} {
			if en.Records[0].Type == k {
				roots[i][1] = id
			}
		}
	}
	roles, ok := ellipsePrismDiscovery(want, f, roots)
	if !ok {
		return fmt.Errorf("step: invalid bound prism topology")
	}
	for id, en := range f.Entities {
		if id == roles[12] || len(en.Records) != 1 || en.Records[0].Type != "CARTESIAN_POINT" || len(en.Records[0].Args[1].Items) != 3 {
			continue
		}
		v, e := mappedExactCoordinate(f, id, "CARTESIAN_POINT", 3)
		if e != nil {
			return e
		}
		v, e = translate(v)
		if e != nil {
			return e
		}
		for j, c := range []float64{v.X, v.Y, v.Z} {
			s, e := mappedTerminatingDecimal(new(big.Rat).SetFloat64(c))
			if e != nil {
				return e
			}
			en.Records[0].Args[1].Items[j] = Value{Kind: Number, Text: s}
		}
		f.Entities[id] = en
	}
	if o.Name != "" {
		en := f.Entities[roles[7]]
		en.Records[0].Args[0] = Value{Kind: String, Text: o.Name}
		en.Records[0].Args[1] = Value{Kind: String, Text: o.Name}
		f.Entities[roles[7]] = en
	}
	for _, pair := range [][2]int64{{21, 27}, {54, 56}, {82, 84}} {
		en := f.Entities[roles[pair[0]]]
		en.Records[0].Args[3] = Value{Kind: Reference, Ref: roles[pair[1]]}
		f.Entities[roles[pair[0]]] = en
	}
	// Prune only dependencies made unreachable by eliminating pcurve wrappers.
	// Original geometry was fully owned and authenticated before this rewrite.
	keep := map[int64]int64{}
	var visit func(int64) error
	var value func(Value) error
	value = func(v Value) error {
		if v.Kind == Reference {
			return visit(v.Ref)
		}
		for _, x := range v.Items {
			if e := value(x); e != nil {
				return e
			}
		}
		return nil
	}
	visit = func(id int64) error {
		if _, ok := keep[id]; ok {
			return nil
		}
		en, ok := f.Entities[id]
		if !ok {
			return fmt.Errorf("step: missing native prism dependency")
		}
		keep[id] = id
		for _, r := range en.Records {
			for _, v := range r.Args {
				if e := value(v); e != nil {
					return e
				}
			}
		}
		return nil
	}
	for _, root := range roots {
		if e := visit(root[1]); e != nil {
			return e
		}
	}
	if len(keep) != 76 {
		return fmt.Errorf("step: unexpected native prism graph")
	}
	var out bytes.Buffer
	fmt.Fprintln(&out, "ISO-10303-21;\nHEADER;\nFILE_DESCRIPTION(('Native analytic elliptical prism'),'2;1');\nFILE_NAME('elliptical-prism.step','1970-01-01T00:00:00',(''),(''),'PlainSolid','PlainSolid','');\nFILE_SCHEMA(('AUTOMOTIVE_DESIGN'));\nENDSEC;\nDATA;")
	for _, id := range f.Order {
		if _, ok := keep[id]; ok {
			fmt.Fprintf(&out, "#%d=%s;\n", id, assemblyEntity(f.Entities[id], keep))
		}
	}
	fmt.Fprintln(&out, "ENDSEC;\nEND-ISO-10303-21;")
	if e := ctx.Err(); e != nil {
		return e
	}
	n, e := w.Write(out.Bytes())
	if e == nil && n != out.Len() {
		e = io.ErrShortWrite
	}
	return e
}
