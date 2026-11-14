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

// CylinderTorusCavitySTEPOccurrence preserves the exact local native cavity and
// a proper signed-coordinate pose. Translation and transformed coordinates must
// have bounded finite decimal expansions; no rounded world native BRep is made.
type CylinderTorusCavitySTEPOccurrence struct {
	original ImportedCylinderTorusCavity
	pose     rigidplacement.RationalRigid
	data     []byte
}

func (s CylinderTorusCavitySTEPOccurrence) Original() ImportedCylinderTorusCavity { return s.original }
func (s CylinderTorusCavitySTEPOccurrence) Pose() ([3][4]*big.Rat, error)         { return s.pose.Matrix() }
func (s CylinderTorusCavitySTEPOccurrence) WriteSTEP(w io.Writer) error {
	if w == nil || len(s.data) == 0 {
		return geom.ErrInvalid
	}
	n, e := w.Write(s.data)
	if e == nil && n != len(s.data) {
		return io.ErrShortWrite
	}
	return e
}

// PlaceSTEP transforms at most512 entities with4096-bit rational operations.
func (s ImportedCylinderTorusCavity) PlaceSTEP(ctx context.Context, p rigidplacement.RationalRigid) (CylinderTorusCavitySTEPOccurrence, error) {
	var zero CylinderTorusCavitySTEPOccurrence
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
		for j := 0; j < 3; j++ {
			if row[j].Sign() != 0 && row[j].Cmp(big.NewRat(1, 1)) != 0 && row[j].Cmp(big.NewRat(-1, 1)) != 0 {
				return zero, geom.ErrDomain
			}
		}
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
	return CylinderTorusCavitySTEPOccurrence{s, p, world.Bytes()}, nil
}
