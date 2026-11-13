package document

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

type lensCurveRecipe struct {
	Degree         int
	Poles          []geom.Vec3
	Knots, Weights []float64
}
type refinedLensRecipe struct {
	Geometry, Provenance string
	Curves               [][]lensCurveRecipe
}

// ReadRefinedSplineLensSTEP preserves the independent source curve bases as a
// lossless bounded native recipe alongside the canonical exact material asset.
func ReadRefinedSplineLensSTEP(r io.Reader, l step.Limits, o step.CurvedOvalOptions, historyLimit int) (*Document, error) {
	m, e := step.ReadRefinedSplineLensBody(r, l, o)
	if e != nil {
		return nil, e
	}
	return fromVerifiedRefinedLens(m, historyLimit)
}
func fromVerifiedRefinedLens(m step.RefinedSplineLens, historyLimit int) (*Document, error) {
	d, e := fromVerifiedPartitionedLens(m.PartitionedSplineLens, historyLimit)
	if e != nil {
		return nil, e
	}
	s, _ := d.Snapshot()
	label := s.Labels["step:body-000001"]
	digest, e := lensGeometryDigest(m.Body.Solid)
	if e != nil {
		return nil, e
	}
	recipe := refinedLensRecipe{Geometry: digest, Provenance: label.Attributes["step.lens.partitioned.binding"].Text}
	for _, cs := range m.Curves {
		row := make([]lensCurveRecipe, 3)
		for i, c := range cs {
			if c == nil {
				return nil, geom.ErrInvalid
			}
			row[i] = lensCurveRecipe{c.Degree(), c.Poles(), c.Knots(), c.Weights()}
		}
		recipe.Curves = append(recipe.Curves, row)
	}
	raw, e := json.Marshal(recipe)
	if e != nil {
		return nil, e
	}
	if len(raw) > 65536 {
		return nil, geom.ErrConvergence
	}
	label.Attributes["step.lens.curve.recipe"] = Attribute{Kind: "string", Text: string(raw)}
	label.Attributes["step.lens.curve.binding"] = Attribute{Kind: "string", Text: lensProvenanceDigest(raw)}
	s.Labels["step:body-000001"] = label
	return New(s, historyLimit)
}

// FromRefinedSplineLensSTEP stages and proves the complete supplied source recipe.
func FromRefinedSplineLensSTEP(m step.RefinedSplineLens, historyLimit int) (*Document, error) {
	var b bytes.Buffer
	if e := step.WriteRefinedSplineLensBody(&b, m, step.CurvedOvalOptions{}); e != nil {
		return nil, e
	}
	return ReadRefinedSplineLensSTEP(&b, step.Limits{}, step.CurvedOvalOptions{}, historyLimit)
}

// WriteRefinedSplineLensSTEP rejects stale native/source geometry and preserves
// model and both owner knot bases. It exports one identity-placed occurrence.
func (s State) WriteRefinedSplineLensSTEP(w io.Writer) error {
	if w == nil {
		return geom.ErrInvalid
	}
	var canonical bytes.Buffer
	if e := s.writeSplineLensSTEP(&canonical, true, true); e != nil {
		return e
	}
	m, e := step.ReadPartitionedSplineLensBody(&canonical, step.Limits{}, step.CurvedOvalOptions{})
	if e != nil {
		return e
	}
	var label Label
	for asset := range s.Trimmed {
		label = s.Labels["step:"+asset]
	}
	raw, ok := label.Attributes["step.lens.curve.recipe"]
	binding, bok := label.Attributes["step.lens.curve.binding"]
	if !ok || !bok || raw.Kind != "string" || binding.Kind != "string" || len(raw.Text) > 65536 || binding.Text != lensProvenanceDigest([]byte(raw.Text)) {
		return fmt.Errorf("document: stale lens curve recipe")
	}
	var recipe refinedLensRecipe
	dec := json.NewDecoder(bytes.NewBufferString(raw.Text))
	dec.DisallowUnknownFields()
	if e = dec.Decode(&recipe); e != nil {
		return e
	}
	if e = dec.Decode(new(any)); e != io.EOF {
		return geom.ErrInvalid
	}
	digest, e := lensGeometryDigest(m.Body.Solid)
	if e != nil {
		return e
	}
	if recipe.Geometry != digest || recipe.Provenance != label.Attributes["step.lens.partitioned.binding"].Text || len(recipe.Curves) != len(m.Pieces) {
		return fmt.Errorf("document: stale curve geometry or ownership")
	}
	refined := step.RefinedSplineLens{PartitionedSplineLens: m}
	for _, row := range recipe.Curves {
		if len(row) != 3 {
			return geom.ErrInvalid
		}
		var cs [3]*geom.BSpline
		for i, c := range row {
			if c.Degree < 1 || c.Degree > 16 || len(c.Poles) < 2 || len(c.Poles) > 64 || len(c.Weights) != len(c.Poles) || len(c.Knots) != len(c.Poles)+c.Degree+1 {
				return geom.ErrInvalid
			}
			maximum := 0.
			for _, v := range c.Weights {
				maximum = max(maximum, v)
			}
			if maximum != 1 {
				return geom.ErrUnrepresentable
			}
			cs[i], e = geom.NewBSpline(c.Degree, c.Poles, c.Knots, c.Weights)
			if e != nil {
				return e
			}
		}
		refined.Curves = append(refined.Curves, cs)
	}
	return step.WriteRefinedSplineLensBody(w, refined, step.CurvedOvalOptions{})
}
