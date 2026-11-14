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

// ExactPlanarNAUODocument keeps one exact local product and its two original
// relationship occurrences. It does not recast NAUO links as representation maps.
type ExactPlanarNAUODocument struct {
	model          step.ExactPlanarNAUO
	source, native []byte
}

func ReadExactPlanarNAUOSTEP(r io.Reader, l step.Limits) (*ExactPlanarNAUODocument, error) {
	if r == nil {
		return nil, rationalMappedDocumentError()
	}
	a, e := step.ReadExactPlanarNAUO(r, l)
	if e != nil {
		return nil, e
	}
	var source bytes.Buffer
	if e = step.WriteExactPlanarNAUOSource(&source, a.Source, l); e != nil {
		return nil, e
	}
	key := "product-" + strconv.FormatInt(a.Part.DefinitionID, 10)
	s := State{LengthUnit: a.LengthUnit, Planar: map[string]brep.Solid{key: a.Body.Solid}, Labels: map[string]Label{"source:" + key: {Target: AssetLabel, TargetID: key, Name: a.Part.Name, Attributes: map[string]Attribute{"step.source.product_definition": {Kind: "integer", Integer: a.Part.DefinitionID}, "step.source.body": {Kind: "integer", Integer: a.SourceBodyID}}}}}
	d, e := New(s, 0)
	if e != nil {
		return nil, e
	}
	var native bytes.Buffer
	if e = d.Write(&native); e != nil {
		return nil, e
	}
	return &ExactPlanarNAUODocument{model: a, source: source.Bytes(), native: native.Bytes()}, nil
}
func (d *ExactPlanarNAUODocument) Name() string {
	if d == nil {
		return ""
	}
	return d.model.Root.Name
}
func (d *ExactPlanarNAUODocument) LengthUnit() float64 {
	if d == nil {
		return 0
	}
	return d.model.LengthUnit
}
func (d *ExactPlanarNAUODocument) PartDefinition() step.AssemblyProduct {
	if d == nil {
		return step.AssemblyProduct{}
	}
	p := d.model.Part
	p.Representations = append([]int64(nil), p.Representations...)
	return p
}
func (d *ExactPlanarNAUODocument) LocalBody() step.Body {
	if d == nil {
		return step.Body{}
	}
	return d.model.Body
}
func (d *ExactPlanarNAUODocument) Occurrences() []step.ExactNAUOOccurrence {
	if d == nil {
		return nil
	}
	return append([]step.ExactNAUOOccurrence(nil), d.model.Occurrences...)
}
func (d *ExactPlanarNAUODocument) ExactWorldBounds() (b RationalWorldBounds, e error) {
	if d == nil {
		return b, rationalMappedDocumentError()
	}
	first := true
	for _, u := range d.model.Occurrences {
		for _, p := range d.model.Body.Solid.Vertices() {
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
func (d *ExactPlanarNAUODocument) Write(w io.Writer) error {
	if d == nil || w == nil || len(d.source) == 0 {
		return rationalMappedDocumentError()
	}
	raw, e := json.Marshal(rationalMappedEnvelope{Format: "plainsolid-exact-planar-nauo", Version: 1, Source: string(d.source), Local: d.native})
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
func (d *ExactPlanarNAUODocument) WriteSTEP(w io.Writer, l step.Limits) error {
	if d == nil || w == nil {
		return rationalMappedDocumentError()
	}
	a, e := step.ReadExactPlanarNAUO(bytes.NewReader(d.source), l)
	if e != nil {
		return e
	}
	return step.WriteExactPlanarNAUOSource(w, a.Source, l)
}
func ReadExactPlanarNAUODocument(r io.Reader, maxBytes int64, l step.Limits) (*ExactPlanarNAUODocument, error) {
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
	if e = dec.Decode(&extra); e != io.EOF || env.Format != "plainsolid-exact-planar-nauo" || env.Version != 1 {
		return nil, rationalMappedDocumentError()
	}
	local, e := Read(bytes.NewReader(env.Local), maxBytes, 0)
	if e != nil {
		return nil, e
	}
	fresh, e := ReadExactPlanarNAUOSTEP(bytes.NewReader([]byte(env.Source)), l)
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
