package step

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/big"
	rigidplacement "plainsolid/brep/placement"
	"plainsolid/geom"
)

// ReadTranslatedCylinderTorusCavityOccurrence accepts the paired placed STEP dialect, recovering
// its exact proper rational pose from its explicit standard source-frame representation item. Direction
// ratios must be exact unit perpendicular vectors: no binary conversion
// or silent normalization authenticates pose. All 3D geometry is pulled back
// exactly and validated by the canonical importer, then the complete forward
// graph is rechecked. General OCCT-healed/rounded dialects remain unsupported.
// Parsing is capped at512 entities/16384 values/2MiB per bounded pass,
// transforms at4096 bits, and at most three fixed-budget native reproofs run.
func ReadTranslatedCylinderTorusCavityOccurrence(ctx context.Context, r io.Reader, l Limits) (TranslatedCylinderTorusCavitySTEPOccurrence, error) {
	var zero TranslatedCylinderTorusCavitySTEPOccurrence
	if ctx == nil || r == nil {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	l, e := l.defaults()
	if e != nil {
		return zero, e
	}
	l.MaxEntities = min(l.MaxEntities, 512)
	l.MaxValues = min(l.MaxValues, 16384)
	l.MaxBytes = min(l.MaxBytes, 2<<20)
	f, e := Parse(r, l)
	if e != nil {
		return zero, e
	}
	pid, e := cavitySourceFrame(f, false, false)
	if e != nil {
		return zero, e
	}
	frame, e := simple(f, pid, "AXIS2_PLACEMENT_3D", 4)
	if e != nil {
		return zero, e
	}
	var vectors [3][3]*big.Rat
	for k := 0; k < 3; k++ {
		id, e := ref(frame.Args[k+1])
		if e != nil {
			return zero, e
		}
		kind := "DIRECTION"
		if k == 0 {
			kind = "CARTESIAN_POINT"
		}
		rec, e := simple(f, id, kind, 2)
		if e != nil || rec.Args[1].Kind != Aggregate || len(rec.Args[1].Items) != 3 {
			return zero, geom.ErrDomain
		}
		for j := 0; j < 3; j++ {
			vectors[k][j], e = torusCylinderImportNumber(rec.Args[1].Items[j])
			if e != nil {
				return zero, e
			}
		}
	}
	// STEP's orthonormal placement convention Y=Z cross X is made explicit and
	// then independently checked by the exact proper rigid constructor.
	var m [3][4]*big.Rat
	for i := 0; i < 3; i++ {
		j, k := (i+1)%3, (i+2)%3
		m[i][0] = vectors[2][i]
		m[i][2] = vectors[1][i]
		m[i][3] = vectors[0][i]
		m[i][1] = new(big.Rat).Sub(new(big.Rat).Mul(vectors[1][j], vectors[2][k]), new(big.Rat).Mul(vectors[1][k], vectors[2][j]))
	}
	pose, e := rigidplacement.NewRationalRigid(m, 4096)
	if e != nil {
		return zero, e
	}
	// Keep the parsed original immutable for the final world correspondence check.
	local, e := Parse(bytes.NewReader(serializeWrappedProofFile(f)), l)
	if e != nil {
		return zero, e
	}
	for _, id := range local.Order {
		if e = ctx.Err(); e != nil {
			return zero, e
		}
		ent := local.Entities[id]
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
		var p [3]*big.Rat
		for j := range p {
			p[j], e = torusCylinderImportNumber(rec.Args[1].Items[j])
			if e != nil {
				return zero, e
			}
		}
		var q [3]*big.Rat
		if rec.Type == "DIRECTION" {
			q, e = pose.TransformVector(ctx, p, true, 4096)
		} else {
			q, e = pose.TransformPoint(ctx, p, true, 4096)
		}
		if e != nil {
			return zero, e
		}
		for j := range q {
			txt, e := torusCylinderDecimal(q[j])
			if e != nil {
				return zero, e
			}
			rec.Args[1].Items[j] = Value{Kind: Number, Text: txt}
		}
		local.Entities[id] = ent
	}
	if _, e = cavitySourceFrame(local, false, true); e != nil {
		return zero, e
	}
	source, e := ReadTranslatedCylinderTorusCavity(bytes.NewReader(serializeWrappedProofFile(local)), l)
	if e != nil {
		return zero, e
	}
	placed, e := source.PlaceSTEP(ctx, pose)
	if e != nil {
		return zero, e
	}
	expected, e := Parse(bytes.NewReader(placed.data), l)
	if e != nil {
		return zero, e
	}
	if !torusCylinderSameGraph(f, expected) {
		return zero, fmt.Errorf("step: placed toroidal cavity correspondence failed")
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	return placed, nil
}
