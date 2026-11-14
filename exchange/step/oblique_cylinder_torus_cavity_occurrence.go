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

// ObliqueCylinderTorusCavitySTEPOccurrence preserves the exact local native cavity and
// a proper rational pose. Translation and transformed coordinates must
// have bounded finite decimal expansions; no rounded world native BRep is made.
type ObliqueCylinderTorusCavitySTEPOccurrence struct {
	original ImportedObliqueCylinderTorusCavity
	pose     rigidplacement.RationalRigid
	data     []byte
}

func (s ObliqueCylinderTorusCavitySTEPOccurrence) Original() ImportedObliqueCylinderTorusCavity {
	return s.original
}
func (s ObliqueCylinderTorusCavitySTEPOccurrence) Pose() ([3][4]*big.Rat, error) {
	return s.pose.Matrix()
}
func (s ObliqueCylinderTorusCavitySTEPOccurrence) WriteSTEP(w io.Writer) error {
	if w == nil || len(s.data) == 0 {
		return geom.ErrInvalid
	}
	n, e := w.Write(s.data)
	if e == nil && n != len(s.data) {
		return io.ErrShortWrite
	}
	return e
}

// The source coordinate frame is an explicit separately related standard STEP
// representation, necessary to disambiguate source centerZ and world pose.
// PlaceSTEP transforms at most512 entities with4096-bit rational operations.
func (s ImportedObliqueCylinderTorusCavity) PlaceSTEP(ctx context.Context, p rigidplacement.RationalRigid) (ObliqueCylinderTorusCavitySTEPOccurrence, error) {
	var zero ObliqueCylinderTorusCavitySTEPOccurrence
	if ctx == nil || !s.valid {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	matrix, e := p.Matrix()
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
	if _, e = cavitySourceFrame(f, true, false); e != nil {
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
	return ObliqueCylinderTorusCavitySTEPOccurrence{s, p, world.Bytes()}, nil
}
