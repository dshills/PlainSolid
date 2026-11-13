package exchange

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math"
	"unicode/utf8"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

type cylinderContactRecipe struct {
	Center, Vector [3]string
	Radicand       string
	RootSign       int
}
type cylinderSetEnvelope struct {
	Version                                        int
	Kind                                           string
	Recipe                                         CylinderPairCommonRecipe
	Boundaries                                     []json.RawMessage
	WallFaces, LowerCapFaces, UpperCapFaces        [2][]int
	StockWallFaces, CutterWallFaces, StockCapFaces [2][]int
	CommonVolumeSquared, MinimumGapSquared         string
	ContactNodes                                   *[2]cylinderContactRecipe
	CommonVolumePiCoefficient                      string `json:",omitempty"`
}

func cylinderSetPayload(ctx context.Context, r CylinderPairCommonRecipe, difference bool) (env cylinderSetEnvelope, u parametric.FiniteCylinderPairUnionResult, d parametric.FiniteCylinderPairDifferenceResult, err error) {
	if ctx == nil || !utf8.ValidString(r.Name) || len(r.Name) > 4096 || r.LengthUnit <= 0 || math.IsNaN(r.LengthUnit) || math.IsInf(r.LengthUnit, 0) {
		err = geom.ErrDomain
		return
	}
	if err = ctx.Err(); err != nil {
		return
	}
	env.Version = 1
	env.Recipe = r
	env.Kind = "analytic-finite-cylinder-pair-union"
	var solids []parametric.Solid
	if difference {
		env.Kind = "analytic-finite-cylinder-pair-difference"
		d, err = parametric.FiniteCylinderPairDifference(ctx, r.A, r.B, r.Tolerance)
		if err != nil {
			return
		}
		solids = d.Solids[:]
		env.StockWallFaces, env.CutterWallFaces, env.StockCapFaces = d.StockWallFaces, d.CutterWallFaces, d.StockCapFaces
		if d.MinimumGapSquared != nil {
			env.MinimumGapSquared = d.MinimumGapSquared.RatString()
		}
		if d.ContactNodes != nil {
			env.ContactNodes = new([2]cylinderContactRecipe)
			for i, n := range d.ContactNodes {
				env.ContactNodes[i].Radicand = n.Radicand.RatString()
				env.ContactNodes[i].RootSign = n.RootSign
				for j := 0; j < 3; j++ {
					env.ContactNodes[i].Center[j] = n.Center[j].RatString()
					env.ContactNodes[i].Vector[j] = n.Vector[j].RatString()
				}
			}
		}
	} else {
		u, err = parametric.FiniteCylinderPairUnion(ctx, r.A, r.B, r.Tolerance)
		if err != nil {
			return
		}
		solids = []parametric.Solid{u.Solid}
		env.WallFaces, env.LowerCapFaces, env.UpperCapFaces = u.WallFaces, u.LowerCapFaces, u.UpperCapFaces
		if u.CommonVolumeSquared != nil {
			env.CommonVolumeSquared = u.CommonVolumeSquared.RatString()
		}
		if u.CommonVolumePiCoefficient != nil {
			env.CommonVolumePiCoefficient = u.CommonVolumePiCoefficient.RatString()
		}
	}
	for _, s := range solids {
		var b bytes.Buffer
		if err = parametric.WriteJSON(&b, s); err != nil {
			return
		}
		env.Boundaries = append(env.Boundaries, json.RawMessage(bytes.TrimSpace(b.Bytes())))
	}
	return
}
func writeCylinderSet(ctx context.Context, w io.Writer, r CylinderPairCommonRecipe, difference bool) error {
	if w == nil {
		return geom.ErrDomain
	}
	env, _, _, e := cylinderSetPayload(ctx, r, difference)
	if e != nil {
		return e
	}
	b, e := json.Marshal(env)
	if e != nil {
		return e
	}
	if e = ctx.Err(); e != nil {
		return e
	}
	b = append(b, '\n')
	n, e := w.Write(b)
	if e == nil && n != len(b) {
		e = io.ErrShortWrite
	}
	return e
}
func readCylinderSet(ctx context.Context, r io.Reader, maxBytes int64, difference bool) (recipe CylinderPairCommonRecipe, u parametric.FiniteCylinderPairUnionResult, d parametric.FiniteCylinderPairDifferenceResult, err error) {
	fail := func(e error) (CylinderPairCommonRecipe, parametric.FiniteCylinderPairUnionResult, parametric.FiniteCylinderPairDifferenceResult, error) {
		if e == nil {
			e = geom.ErrDomain
		}
		return CylinderPairCommonRecipe{}, parametric.FiniteCylinderPairUnionResult{}, parametric.FiniteCylinderPairDifferenceResult{}, e
	}
	if ctx == nil || r == nil || maxBytes < 0 || maxBytes > 16<<20 {
		return fail(nil)
	}
	if e := ctx.Err(); e != nil {
		return fail(e)
	}
	if maxBytes == 0 {
		maxBytes = 1 << 20
	}
	b, e := io.ReadAll(io.LimitReader(r, maxBytes+1))
	if e != nil {
		return fail(e)
	}
	if int64(len(b)) > maxBytes {
		return fail(nil)
	}
	var env cylinderSetEnvelope
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if e = dec.Decode(&env); e != nil {
		return fail(e)
	}
	kind := "analytic-finite-cylinder-pair-union"
	if difference {
		kind = "analytic-finite-cylinder-pair-difference"
	}
	if env.Version != 1 || env.Kind != kind {
		return fail(nil)
	}
	expected, u, d, e := cylinderSetPayload(ctx, env.Recipe, difference)
	if e != nil {
		return fail(e)
	}
	canonical, e := json.Marshal(expected)
	if e != nil {
		return fail(e)
	}
	var compact bytes.Buffer
	if e = json.Compact(&compact, b); e != nil || !bytes.Equal(compact.Bytes(), canonical) {
		return fail(nil)
	}
	if e = ctx.Err(); e != nil {
		return fail(e)
	}
	return env.Recipe, u, d, nil
}

// WriteCylinderPairUnionRecipeJSON retains original raw operands and canonical
// native union geometry, ancestry and exact common-volume-squared proof field.
// Construction failures publish nothing; underlying writer failures may be partial.
func WriteCylinderPairUnionRecipeJSON(ctx context.Context, w io.Writer, r CylinderPairCommonRecipe) error {
	return writeCylinderSet(ctx, w, r, false)
}

// ReadCylinderPairUnionRecipeJSON rebuilds and compares the entire canonical
// envelope. Limits default to 1 MiB, maximum 16 MiB. No external authenticity or
// general Boolean/import capability is implied.
func ReadCylinderPairUnionRecipeJSON(ctx context.Context, r io.Reader, maxBytes int64) (CylinderPairCommonRecipe, parametric.FiniteCylinderPairUnionResult, error) {
	v, u, _, e := readCylinderSet(ctx, r, maxBytes, false)
	return v, u, e
}

// WriteCylinderPairDifferenceRecipeJSON retains two ordered native components,
// component-local ancestry and exact contact expressions or separation bound.
// Contact refinement Bounds are deliberately regenerated, not serialized.
func WriteCylinderPairDifferenceRecipeJSON(ctx context.Context, w io.Writer, r CylinderPairCommonRecipe) error {
	return writeCylinderSet(ctx, w, r, true)
}

// ReadCylinderPairDifferenceRecipeJSON requires canonical reconstruction of all
// components and stable contact/gap data. A is stock, B is cutter; raw signed
// axes and parameter limits are retained. Limits match the union reader.
func ReadCylinderPairDifferenceRecipeJSON(ctx context.Context, r io.Reader, maxBytes int64) (CylinderPairCommonRecipe, parametric.FiniteCylinderPairDifferenceResult, error) {
	v, _, d, e := readCylinderSet(ctx, r, maxBytes, true)
	return v, d, e
}
