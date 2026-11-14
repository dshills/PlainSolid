package step

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"math/big"
	rigidplacement "plainsolid/brep/placement"
	"plainsolid/geom"
	"strconv"
	"strings"
)

// TorusCylinderSTEPOccurrence retains the original native body and exact proper
// world query. Its STEP representation maps analytic placements/model curves;
// it does not claim a rounded transformed native BRep. Pose coordinates must
// have finite exact decimal expansions, and all transformed values are bounded.
type TorusCylinderSTEPOccurrence struct {
	original ImportedTorusCylinder
	query    rigidplacement.TorusCylinderOccurrence
	data     []byte
}

func (s TorusCylinderSTEPOccurrence) Original() ImportedTorusCylinder               { return s.original }
func (s TorusCylinderSTEPOccurrence) Query() rigidplacement.TorusCylinderOccurrence { return s.query }
func (s TorusCylinderSTEPOccurrence) Pose() ([3][4]*big.Rat, error)                 { return s.query.Pose() }
func (s TorusCylinderSTEPOccurrence) ExportProof() (TorusCylinderExportProof, error) {
	if len(s.data) == 0 {
		return TorusCylinderExportProof{}, geom.ErrInvalid
	}
	return CertifyTorusCylinderExport(s.original.source, s.original.tolerance)
}
func (s TorusCylinderSTEPOccurrence) WriteSTEP(w io.Writer) error {
	if w == nil || len(s.data) == 0 {
		return geom.ErrInvalid
	}
	n, e := w.Write(s.data)
	if e != nil {
		return e
	}
	if n != len(s.data) {
		return io.ErrShortWrite
	}
	return nil
}

// NewTorusCylinderSTEPOccurrence revalidates source native topology before
// rigidplacement. Common/Cut omit unobservable caps in the original paired result;
// retained cap provenance follows ReadTorusCylinderSet. No payload is exported.
// Preparation runs at most four bounded native factory reproofs. Call PlaceSTEP
// on an already prepared imported result to reuse its native validation.
func NewTorusCylinderSTEPOccurrence(ctx context.Context, b TorusCylinderBody, p rigidplacement.RationalRigid, unit, tolerance float64) (TorusCylinderSTEPOccurrence, error) {
	var zero TorusCylinderSTEPOccurrence
	if ctx == nil {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	var raw bytes.Buffer
	if e := WriteTorusCylinderSets(&raw, TorusCylinderModel{[]TorusCylinderBody{b}, unit, tolerance}); e != nil {
		return zero, e
	}
	source, e := ReadTorusCylinderSet(&raw, Limits{}, tolerance)
	if e != nil {
		return zero, e
	}
	return source.PlaceSTEP(ctx, p)
}

// PlaceSTEP preserves proper-rigid orientation, all original native owner IDs
// and source t. Because the pose is emitted exactly, the canonical certified
// model-space rim error is unchanged. Nonterminating decimals (e.g.1/7), values
// beyond the bounded STEP scalar range and improper transforms fail atomically.
// There are at most512 entities and each exact transform is limited to4096 bits.
func (s ImportedTorusCylinder) PlaceSTEP(ctx context.Context, p rigidplacement.RationalRigid) (TorusCylinderSTEPOccurrence, error) {
	var zero TorusCylinderSTEPOccurrence
	if ctx == nil || s.radius == nil {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	query, e := rigidplacement.PlaceTorusCylinder(ctx, s.query, p)
	if e != nil {
		return zero, e
	}
	matrix, e := query.Pose()
	if e != nil {
		return zero, e
	}
	for _, row := range matrix {
		for _, q := range row {
			if _, e = torusCylinderDecimal(q); e != nil {
				return zero, e
			}
		}
	}
	var raw bytes.Buffer
	if e = s.WriteSTEP(&raw); e != nil {
		return zero, e
	}
	f, e := Parse(bytes.NewReader(raw.Bytes()), Limits{MaxEntities: 512, MaxValues: 16384, MaxBytes: 2 << 20})
	if e != nil {
		return zero, e
	}
	for _, id := range f.Order {
		if e = ctx.Err(); e != nil {
			return zero, e
		}
		ent := f.Entities[id]
		if len(ent.Records) != 1 {
			continue
		}
		rec := &ent.Records[0]
		if rec.Type != "CARTESIAN_POINT" && rec.Type != "DIRECTION" {
			continue
		}
		if len(rec.Args) != 2 || rec.Args[1].Kind != Aggregate {
			return zero, geom.ErrDomain
		}
		if len(rec.Args[1].Items) == 2 {
			continue
		}
		if len(rec.Args[1].Items) != 3 {
			return zero, geom.ErrDomain
		}
		var point [3]*big.Rat
		for j := range point {
			point[j], e = torusCylinderImportNumber(rec.Args[1].Items[j])
			if e != nil {
				return zero, e
			}
		}
		var world [3]*big.Rat
		if rec.Type == "DIRECTION" {
			world, e = p.TransformVector(ctx, point, false, 4096)
		} else {
			world, e = p.TransformPoint(ctx, point, false, 4096)
		}
		if e != nil {
			return zero, e
		}
		for j, q := range world {
			txt, e := torusCylinderDecimal(q)
			if e != nil {
				return zero, e
			}
			rec.Args[1].Items[j] = Value{Kind: Number, Text: txt}
		}
		f.Entities[id] = ent
	}
	ids := map[int64]int64{}
	for _, id := range f.Order {
		ids[id] = id
	}
	var world bytes.Buffer
	world.Write(raw.Bytes()[:bytes.Index(raw.Bytes(), []byte("DATA;"))+5])
	world.WriteByte('\n')
	for _, id := range f.Order {
		fmt.Fprintf(&world, "#%d=%s;\n", id, assemblyEntity(f.Entities[id], ids))
	}
	world.WriteString("ENDSEC;\nEND-ISO-10303-21;\n")
	if world.Len() > 2<<20 {
		return zero, geom.ErrConvergence
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	return TorusCylinderSTEPOccurrence{s, query, world.Bytes()}, nil
}

// A rational has a finite decimal iff its reduced denominator has only2/5
// factors. The loop and emitted token are explicitly bounded; no rounding.
func torusCylinderDecimal(q *big.Rat) (string, error) {
	if q == nil || q.Num().BitLen() > 4096 || q.Denom().BitLen() > 4096 {
		return "", geom.ErrConvergence
	}
	denominator := new(big.Int).Set(q.Denom())
	counts := [2]int{}
	for i, p := range []int64{2, 5} {
		factor := big.NewInt(p)
		for denominator.Cmp(big.NewInt(1)) > 0 {
			quotient, remainder := new(big.Int), new(big.Int)
			quotient.QuoRem(denominator, factor, remainder)
			if remainder.Sign() != 0 {
				break
			}
			counts[i]++
			if counts[i] > 1074 {
				return "", geom.ErrConvergence
			}
			denominator = quotient
		}
	}
	if denominator.Cmp(big.NewInt(1)) != 0 {
		return "", geom.ErrDomain
	}
	digits := max(counts[0], counts[1])
	txt := q.FloatString(digits)
	if digits == 0 {
		txt += "."
	} else {
		txt = strings.TrimRight(txt, "0")
	}
	if len(txt) > 1100 {
		return "", geom.ErrConvergence
	}
	value, e := strconv.ParseFloat(txt, 64)
	if e != nil || math.IsInf(value, 0) || (value == 0 && q.Sign() != 0) {
		return "", geom.ErrConvergence
	}
	return txt, nil
}
