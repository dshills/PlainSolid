package document

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"plainsolid/exchange/iges"
	"plainsolid/geom"
)

type igesLensRecipe struct {
	Geometry, Provenance string
	Curves               [][]iges.RationalLensCurve
}

// ReadRationalSplineLensIGES retains original raw positive curve weights,
// independently refined owner/model bases and their source topology lineage.
func ReadRationalSplineLensIGES(r io.Reader, l iges.Limits, historyLimit int) (*Document, error) {
	m, e := iges.ReadRationalSplineLensSolid(r, l)
	if e != nil {
		return nil, e
	}
	d, e := fromVerifiedIGESLens(m.Body, historyLimit)
	if e != nil {
		return nil, e
	}
	s, _ := d.Snapshot()
	label := s.Labels["iges:body-000001"]
	digest, e := lensGeometryDigest(m.Body.Solid)
	if e != nil {
		return nil, e
	}
	recipe := igesLensRecipe{Geometry: digest, Provenance: lensProvenanceDigest([]byte(label.Attributes["iges.lens.provenance"].Text))}
	for _, row := range m.Curves {
		recipe.Curves = append(recipe.Curves, append([]iges.RationalLensCurve(nil), row[:]...))
	}
	raw, e := json.Marshal(recipe)
	if e != nil {
		return nil, e
	}
	if len(raw) > 65536 {
		return nil, geom.ErrInvalid
	}
	label.Attributes["iges.lens.rational.recipe"] = Attribute{Kind: "string", Text: string(raw)}
	label.Attributes["iges.lens.rational.binding"] = Attribute{Kind: "string", Text: lensProvenanceDigest(raw)}
	s.Labels["iges:body-000001"] = label
	return New(s, historyLimit)
}

// FromRationalSplineLensIGES proves a supplied recipe through a staged IGES
// graph. Source IDs refer to this stage, not caller-supplied cached IDs.
func FromRationalSplineLensIGES(m iges.RationalSplineLens, historyLimit int) (*Document, error) {
	var b bytes.Buffer
	if e := iges.WriteRationalSplineLensSolid(&b, m); e != nil {
		return nil, e
	}
	return ReadRationalSplineLensIGES(&b, iges.Limits{}, historyLimit)
}

// WriteRationalSplineLensIGES exports one identity-placed native asset with its
// exact raw source curves; both geometry and source recipe are re-certified.
func (s State) WriteRationalSplineLensIGES(w io.Writer) error { return s.writeSplineLensIGES(w, true) }
func readIGESLensRecipe(label Label, p igesLensProvenance, provenance string) (igesLensRecipe, error) {
	raw, binding := label.Attributes["iges.lens.rational.recipe"], label.Attributes["iges.lens.rational.binding"]
	if raw.Kind != "string" || binding.Kind != "string" || len(raw.Text) > 65536 || binding.Text != lensProvenanceDigest([]byte(raw.Text)) {
		return igesLensRecipe{}, fmt.Errorf("document: stale raw IGES recipe")
	}
	var recipe igesLensRecipe
	dec := json.NewDecoder(bytes.NewBufferString(raw.Text))
	dec.DisallowUnknownFields()
	if e := dec.Decode(&recipe); e != nil {
		return recipe, e
	}
	if dec.Decode(new(any)) != io.EOF || recipe.Geometry != p.Geometry || recipe.Provenance != lensProvenanceDigest([]byte(provenance)) || len(recipe.Curves) != 4 {
		return recipe, geom.ErrInvalid
	}
	for _, row := range recipe.Curves {
		if len(row) != 3 {
			return recipe, geom.ErrInvalid
		}
		for _, c := range row {
			if c.Degree < 1 || c.Degree > 16 || len(c.Poles) < 2 || len(c.Poles) > 64 || len(c.Weights) != len(c.Poles) || len(c.Knots) != len(c.Poles)+c.Degree+1 {
				return recipe, geom.ErrInvalid
			}
		}
	}
	return recipe, nil
}
