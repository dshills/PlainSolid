package document

import (
	"bytes"
	"encoding/json"
	"io"
	"math/big"
	"plainsolid/brep"
	"plainsolid/exchange/step"
	"strconv"
)

// ExactDAGPlanarNAUODocument keeps independent exact local products and original
// relationship occurrences. It does not recast NAUO links as representation maps.
type ExactDAGPlanarNAUODocument struct {
	model          step.ExactDAGPlanarNAUO
	source, native []byte
}

func ReadExactDAGPlanarNAUOSTEP(r io.Reader, l step.Limits) (*ExactDAGPlanarNAUODocument, error) {
	if r == nil {
		return nil, rationalMappedDocumentError()
	}
	a, e := step.ReadExactDAGPlanarNAUO(r, l)
	if e != nil {
		return nil, e
	}
	var source bytes.Buffer
	if e = step.WriteExactDAGPlanarNAUOSource(&source, a.Source, l); e != nil {
		return nil, e
	}
	s := State{LengthUnit: a.LengthUnit, Planar: map[string]brep.Solid{}, Labels: map[string]Label{}}
	for _, part := range a.Parts {
		key := "product-" + strconv.FormatInt(part.Product.DefinitionID, 10)
		s.Planar[key] = part.Body.Solid
		s.Labels["source:"+key] = Label{Target: AssetLabel, TargetID: key, Name: part.Product.Name, Attributes: map[string]Attribute{"step.source.product_definition": {Kind: "integer", Integer: part.Product.DefinitionID}, "step.source.body": {Kind: "integer", Integer: part.SourceBodyID}}}
	}
	d, e := New(s, 0)
	if e != nil {
		return nil, e
	}
	var native bytes.Buffer
	if e = d.Write(&native); e != nil {
		return nil, e
	}
	return &ExactDAGPlanarNAUODocument{model: a, source: source.Bytes(), native: native.Bytes()}, nil
}
func (d *ExactDAGPlanarNAUODocument) Name() string {
	if d == nil {
		return ""
	}
	return d.model.Root.Name
}
func (d *ExactDAGPlanarNAUODocument) LengthUnit() float64 {
	if d == nil {
		return 0
	}
	return d.model.LengthUnit
}

// Groups returns copied original intermediate product definitions.
func (d *ExactDAGPlanarNAUODocument) Groups() []step.AssemblyProduct {
	if d == nil {
		return nil
	}
	groups := append([]step.AssemblyProduct(nil), d.model.Groups...)
	for i := range groups {
		groups[i].Representations = append([]int64(nil), groups[i].Representations...)
	}
	return groups
}

// Relationships returns original direct uses without expanding shared group uses.
func (d *ExactDAGPlanarNAUODocument) Relationships() []step.ExactNAUOOccurrence {
	if d == nil {
		return nil
	}
	return append([]step.ExactNAUOOccurrence(nil), d.model.Relationships...)
}

// Parts returns copied definitions with immutable native bodies.
func (d *ExactDAGPlanarNAUODocument) Parts() []step.ExactNAUOPlanarProduct {
	if d == nil {
		return nil
	}
	p := append([]step.ExactNAUOPlanarProduct(nil), d.model.Parts...)
	for i := range p {
		p[i].Product.Representations = append([]int64(nil), p[i].Product.Representations...)
	}
	return p
}
func (d *ExactDAGPlanarNAUODocument) Occurrences() []step.ExactDAGNAUOOccurrence {
	if d == nil {
		return nil
	}
	uses := append([]step.ExactDAGNAUOOccurrence(nil), d.model.Occurrences...)
	for i := range uses {
		uses[i].Path = append([]step.ExactNAUOOccurrence(nil), uses[i].Path...)
	}
	return uses
}
func (d *ExactDAGPlanarNAUODocument) ExactWorldBounds() (b RationalWorldBounds, e error) {
	if d == nil {
		return b, rationalMappedDocumentError()
	}
	first := true
	for _, u := range d.model.Occurrences {
		var body brep.Solid
		for _, part := range d.model.Parts {
			if part.Product.DefinitionID == u.Path[len(u.Path)-1].ChildID {
				body = part.Body.Solid
			}
		}
		for _, p := range body.Vertices() {
			q, e := u.ApplyExact(p)
			if e != nil {
				return RationalWorldBounds{}, e
			}
			for i, v := range q {
				if first || v.Cmp(b.Min[i]) < 0 {
					b.Min[i] = new(big.Rat).Set(v)
				}
				if first || v.Cmp(b.Max[i]) > 0 {
					b.Max[i] = new(big.Rat).Set(v)
				}
			}
			first = false
		}
	}
	if first {
		return RationalWorldBounds{}, rationalMappedDocumentError()
	}
	return b, nil
}
func (d *ExactDAGPlanarNAUODocument) Write(w io.Writer) error {
	if d == nil || w == nil || len(d.source) == 0 {
		return rationalMappedDocumentError()
	}
	raw, e := json.Marshal(rationalMappedEnvelope{Format: "plainsolid-exact-dag-planar-nauo", Version: 1, Source: string(d.source), Local: d.native})
	if e != nil {
		return e
	}
	raw = append(raw, '\n')
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		return io.ErrShortWrite
	}
	return e
}
func (d *ExactDAGPlanarNAUODocument) WriteSTEP(w io.Writer, l step.Limits) error {
	if d == nil || w == nil {
		return rationalMappedDocumentError()
	}
	a, e := step.ReadExactDAGPlanarNAUO(bytes.NewReader(d.source), l)
	if e != nil {
		return e
	}
	return step.WriteExactDAGPlanarNAUOSource(w, a.Source, l)
}
func ReadExactDAGPlanarNAUODocument(r io.Reader, maxBytes int64, l step.Limits) (*ExactDAGPlanarNAUODocument, error) {
	if r == nil || maxBytes <= 0 || maxBytes > 1<<30 {
		return nil, rationalMappedDocumentError()
	}
	raw, e := io.ReadAll(io.LimitReader(r, maxBytes+1))
	if e != nil {
		return nil, e
	}
	if int64(len(raw)) > maxBytes {
		return nil, rationalMappedDocumentError()
	}
	var env rationalMappedEnvelope
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if e = dec.Decode(&env); e != nil {
		return nil, e
	}
	var extra any
	if e = dec.Decode(&extra); e != io.EOF || env.Format != "plainsolid-exact-dag-planar-nauo" || env.Version != 1 {
		return nil, rationalMappedDocumentError()
	}
	local, e := Read(bytes.NewReader(env.Local), maxBytes, 0)
	if e != nil {
		return nil, e
	}
	fresh, e := ReadExactDAGPlanarNAUOSTEP(bytes.NewReader([]byte(env.Source)), l)
	if e != nil {
		return nil, e
	}
	var canonical bytes.Buffer
	if e = local.Write(&canonical); e != nil {
		return nil, e
	}
	if !bytes.Equal(canonical.Bytes(), fresh.native) {
		return nil, rationalMappedDocumentError()
	}
	return fresh, nil
}
