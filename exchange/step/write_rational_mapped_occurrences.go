package step

import (
	"bytes"
	"io"
	"math/big"
)

func mappedTerminatingDecimal(q *big.Rat) (string, error) {
	if q == nil || q.Num().BitLen() > 4096 || q.Denom().BitLen() > 4096 {
		return "", mappedPlanarFailure()
	}
	d := new(big.Int).Set(q.Denom())
	counts := [2]int{}
	for j, p := range []int64{2, 5} {
		for {
			quo, rem := new(big.Int), new(big.Int)
			quo.QuoRem(d, big.NewInt(p), rem)
			if rem.Sign() != 0 {
				break
			}
			d = quo
			counts[j]++
			if counts[j] > 128 {
				return "", mappedPlanarFailure()
			}
		}
	}
	if d.Cmp(big.NewInt(1)) != 0 {
		return "", mappedPlanarFailure()
	}
	s := q.FloatString(max(counts[0], counts[1]))
	if len(s) > 128 {
		return "", mappedPlanarFailure()
	}
	return s, nil
}

// WriteRationalMappedPlanarPlacements authors all occurrence poses over a retained
// validated mapped source. Placements must identify each original item exactly
// once. Local geometry, names, units and membership are preserved; no lossy
// rational-to-binary conversion is used. Output is staged and fully revalidated.
func WriteRationalMappedPlanarPlacements(w io.Writer, source *File, placements []RationalMappedPlacement, l Limits) error {
	if w == nil || source == nil || len(placements) > 16 {
		return mappedPlanarFailure()
	}
	raw, e := mappedPlanarSourceBytes(source)
	if e != nil {
		return e
	}
	a, e := ReadRationalMappedPlanarMaterials(bytes.NewReader(raw), l)
	if e != nil {
		return e
	}
	if len(placements) != len(a.Occurrences) {
		return mappedPlanarFailure()
	}
	f := a.Source
	known := map[int64]bool{}
	for _, u := range a.Occurrences {
		known[u.ItemID] = true
	}
	var next, rootID int64
	for _, id := range f.Order {
		if id > next {
			next = id
		}
		en := f.Entities[id]
		if len(en.Records) == 1 && en.Records[0].Type == "SHAPE_REPRESENTATION" {
			rootID = id
		}
	}
	if next > 1<<60 {
		return mappedPlanarFailure()
	}
	emit := func(kind string, args ...Value) int64 {
		next++
		f.Order = append(f.Order, next)
		f.Entities[next] = Entity{ID: next, Records: []Record{{Type: kind, Args: args}}}
		return next
	}
	empty := Value{Kind: String}
	refv := func(id int64) Value { return Value{Kind: Reference, Ref: id} }
	coord := func(kind string, v [3]*big.Rat) (int64, error) {
		if kind == "DIRECTION" {
			den := big.NewInt(1)
			for _, q := range v {
				g := new(big.Int).GCD(nil, nil, den, q.Denom())
				den.Mul(new(big.Int).Quo(den, g), q.Denom())
			}
			for i, q := range v {
				v[i] = new(big.Rat).Mul(q, new(big.Rat).SetInt(den))
			}
		}
		vals := []Value{}
		for _, q := range v {
			s, e := mappedTerminatingDecimal(q)
			if e != nil {
				return 0, e
			}
			vals = append(vals, Value{Kind: Number, Text: s})
		}
		return emit(kind, empty, Value{Kind: Aggregate, Items: vals}), nil
	}
	for _, p := range placements {
		if !known[p.ItemID] {
			return mappedPlanarFailure()
		}
		delete(known, p.ItemID)
		b, e := mappedRationalPose(p)
		if e != nil {
			return e
		}
		o, e := coord("CARTESIAN_POINT", b.Origin)
		if e != nil {
			return e
		}
		x, e := coord("DIRECTION", b.X)
		if e != nil {
			return e
		}
		y, e := coord("DIRECTION", b.Y)
		if e != nil {
			return e
		}
		z, e := coord("DIRECTION", b.Z)
		if e != nil {
			return e
		}
		target := emit("CARTESIAN_TRANSFORMATION_OPERATOR_3D", empty, empty, empty, refv(x), refv(y), refv(o), Value{Kind: Number, Text: "1"}, refv(z))
		en := f.Entities[p.ItemID]
		en.Records[0].Args[2] = refv(target)
		f.Entities[p.ItemID] = en
	}
	root := f.Entities[rootID].Records[0]
	items, e := listRefs(root.Args[1], 1)
	if e != nil {
		return e
	}
	lim, e := l.defaults()
	if e != nil {
		return e
	}
	f, e = mappedPlanarPartition(f, rootID, items, lim)
	if e != nil {
		return e
	}
	raw, e = mappedPlanarSourceBytes(f)
	if e != nil {
		return e
	}
	if _, e = ReadRationalMappedPlanarMaterials(bytes.NewReader(raw), l); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		return io.ErrShortWrite
	}
	return e
}

// WriteRationalMappedPlanarSource preserves a validated original rational graph.
func WriteRationalMappedPlanarSource(w io.Writer, source *File, l Limits) error {
	if w == nil || source == nil {
		return mappedPlanarFailure()
	}
	raw, e := mappedPlanarSourceBytes(source)
	if e != nil {
		return e
	}
	if _, e = ReadRationalMappedPlanarMaterials(bytes.NewReader(raw), l); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		return io.ErrShortWrite
	}
	return e
}
