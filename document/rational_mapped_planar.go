package document

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"strconv"

	"plainsolid/brep"
	"plainsolid/exchange/step"
)

// RationalMappedPlanarDocument preserves shared local material and exact rational
// poses outside ordinary State.Nodes. No rounded or identity stand-in transforms
// are installed in an ordinary document. This is an immutable versioned envelope.
type RationalMappedPlanarDocument struct {
	model  step.RationalMappedPlanarMaterials
	native []byte
	source []byte
	paths  [][2]step.MappedSourceStep
}

// RationalWorldBounds contains independently owned exact extrema in model units.
type RationalWorldBounds struct{ Min, Max [3]*big.Rat }
type rationalMappedEnvelope struct {
	Format  string          `json:"format"`
	Version int             `json:"version"`
	Source  string          `json:"source"`
	Local   json.RawMessage `json:"local"`
}

func rationalMappedDocumentError() error {
	return fmt.Errorf("document: unsupported or stale rational mapped planar envelope")
}

// ReadRationalMappedPlanarSTEP imports validated rational mapped material and
// creates shared native local assets, preserving names, units and source identity.
func ReadRationalMappedPlanarSTEP(r io.Reader, l step.Limits) (*RationalMappedPlanarDocument, error) {
	if r == nil {
		return nil, rationalMappedDocumentError()
	}
	a, e := step.ReadRationalMappedPlanarMaterials(r, l)
	if e != nil {
		return nil, e
	}
	var source bytes.Buffer
	if e = step.WriteRationalMappedPlanarSource(&source, a.Source, l); e != nil {
		return nil, e
	}
	return newRationalMappedPlanarDocument(a, source.Bytes())
}
func newRationalMappedPlanarDocument(a step.RationalMappedPlanarMaterials, source []byte) (*RationalMappedPlanarDocument, error) {
	s := State{LengthUnit: a.LengthUnit, Planar: map[string]brep.Solid{}, Labels: map[string]Label{}}
	for _, mid := range a.MapOrder {
		body := a.Bodies[mid]
		key := "map-" + strconv.FormatInt(mid, 10)
		s.Planar[key] = body.Solid
		s.Labels["source:"+key] = Label{Target: AssetLabel, TargetID: key, Name: body.Name, Attributes: map[string]Attribute{"step.source.map": {Kind: "integer", Integer: mid}}}
	}
	d, e := New(s, 0)
	if e != nil {
		return nil, e
	}
	var native bytes.Buffer
	if e = d.Write(&native); e != nil {
		return nil, e
	}
	return &RationalMappedPlanarDocument{model: a, native: native.Bytes(), source: append([]byte(nil), source...)}, nil
}
func (d *RationalMappedPlanarDocument) Name() string {
	if d == nil {
		return ""
	}
	return d.model.Name
}
func (d *RationalMappedPlanarDocument) LengthUnit() float64 {
	if d == nil {
		return 0
	}
	return d.model.LengthUnit
}
func (d *RationalMappedPlanarDocument) MapIDs() []int64 {
	if d == nil {
		return nil
	}
	return append([]int64(nil), d.model.MapOrder...)
}
func (d *RationalMappedPlanarDocument) LocalBody(mapID int64) (step.Body, bool) {
	if d == nil {
		return step.Body{}, false
	}
	b, ok := d.model.Bodies[mapID]
	return b, ok
}
func (d *RationalMappedPlanarDocument) Occurrences() []step.ExactMappedOccurrence {
	if d == nil {
		return nil
	}
	return append([]step.ExactMappedOccurrence(nil), d.model.Occurrences...)
}

// ExactWorldBounds bounds the complete finite planar material by its transformed
// vertices using rational comparisons. It never converts bounds to binary64.
func (d *RationalMappedPlanarDocument) ExactWorldBounds() (b RationalWorldBounds, e error) {
	if d == nil || len(d.model.Occurrences) == 0 {
		return b, rationalMappedDocumentError()
	}
	first := true
	for _, u := range d.model.Occurrences {
		body, ok := d.model.Bodies[u.MapID]
		if !ok {
			return RationalWorldBounds{}, rationalMappedDocumentError()
		}
		for _, p := range body.Solid.Vertices() {
			q, e := u.ApplyExact(p)
			if e != nil {
				return RationalWorldBounds{}, e
			}
			for i, x := range q {
				if first || x.Cmp(b.Min[i]) < 0 {
					b.Min[i] = new(big.Rat).Set(x)
				}
				if first || x.Cmp(b.Max[i]) > 0 {
					b.Max[i] = new(big.Rat).Set(x)
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

// BakeOccurrence constructs one native placed body only when every exact world
// vertex is represented without rounding. It preserves original edge/face order.
// Failure returns no partial material. Shared original local assets are unchanged.
func (d *RationalMappedPlanarDocument) BakeOccurrence(itemID int64) (step.Body, error) {
	if d == nil || len(d.paths) > 0 {
		return step.Body{}, rationalMappedDocumentError()
	}
	for _, u := range d.model.Occurrences {
		if u.ItemID != itemID {
			continue
		}
		b, ok := d.model.Bodies[u.MapID]
		if !ok {
			return step.Body{}, rationalMappedDocumentError()
		}
		vs := b.Solid.Vertices()
		for i, p := range vs {
			var e error
			vs[i], e = u.ApplyRepresented(p)
			if e != nil {
				return step.Body{}, e
			}
		}
		solid, e := brep.NewSolid(vs, b.Solid.Edges(), b.Solid.Faces())
		if e != nil {
			return step.Body{}, e
		}
		return step.Body{Name: u.Name, Solid: solid}, nil
	}
	return step.Body{}, rationalMappedDocumentError()
}
func (d *RationalMappedPlanarDocument) WriteSTEP(w io.Writer, l step.Limits) error {
	if d == nil || w == nil {
		return rationalMappedDocumentError()
	}
	if len(d.paths) > 0 {
		a, e := step.ReadNestedRationalMappedPlanarMaterials(bytes.NewReader(d.source), l)
		if e != nil {
			return e
		}
		return step.WriteNestedRationalMappedPlanarSource(w, a.Source, l)
	}
	a, e := step.ReadRationalMappedPlanarMaterials(bytes.NewReader(d.source), l)
	if e != nil {
		return e
	}
	return step.WriteRationalMappedPlanarSource(w, a.Source, l)
}

// Write persists version1 with the exact source pose graph and bound native local
// assets. Unlike generic Document, this envelope does not have floating nodes.
func (d *RationalMappedPlanarDocument) Write(w io.Writer) error {
	if d == nil || w == nil || len(d.source) == 0 || len(d.native) == 0 {
		return rationalMappedDocumentError()
	}
	format := "plainsolid-rational-mapped-planar"
	if len(d.paths) > 0 {
		format = "plainsolid-nested-rational-mapped-planar"
	}
	raw, e := json.Marshal(rationalMappedEnvelope{Format: format, Version: 1, Source: string(d.source), Local: d.native})
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

// ReadRationalMappedPlanarDocument bounds the outer native envelope, validates
// both native assets and exact STEP evidence, and rejects stale geometry/metadata.
// maxBytes must be positive; STEP limits independently bound source reconstruction.
func ReadRationalMappedPlanarDocument(r io.Reader, maxBytes int64, l step.Limits) (*RationalMappedPlanarDocument, error) {
	return readRationalMappedPlanarDocument(r, maxBytes, l, false)
}
func readRationalMappedPlanarDocument(r io.Reader, maxBytes int64, l step.Limits, nested bool) (*RationalMappedPlanarDocument, error) {
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
	if e = dec.Decode(&extra); e != io.EOF {
		return nil, rationalMappedDocumentError()
	}
	format := "plainsolid-rational-mapped-planar"
	if nested {
		format = "plainsolid-nested-rational-mapped-planar"
	}
	if env.Format != format || env.Version != 1 || len(env.Local) == 0 {
		return nil, rationalMappedDocumentError()
	}
	local, e := Read(bytes.NewReader(env.Local), maxBytes, 0)
	if e != nil {
		return nil, e
	}
	read := ReadRationalMappedPlanarSTEP
	if nested {
		read = ReadNestedRationalMappedPlanarSTEP
	}
	fresh, e := read(bytes.NewReader([]byte(env.Source)), l)
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
