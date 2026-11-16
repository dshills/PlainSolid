package iges

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"plainsolid/geom"
)

// CorrectedModelIGES emits a strictly reopenable model and maps every
// emitted directory ID to its original source ID. The corrected type 128
// net occupies the type 194 support's slot and incorporates its original
// location, axis and phase. Those three separate directory records are
// removed. All other records retain their relative order. Original
// one-based edge and vertex list indices still identify the same geometry.
func (m RepairedConicalRimFilletBoreIGES) CorrectedModelIGES(ctx context.Context) ([]byte, map[int]int, error) {
	if ctx == nil || len(m.corrected.entities) == 0 {
		return nil, nil, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	raw, toOriginal, err := writeConicalRimFilletBoreCarrier(m.corrected)
	if err != nil {
		return nil, nil, err
	}
	reopened, err := ReadConicalRimFilletBore(ctx, bytes.NewReader(raw), Limits{}, geom.SurfacePatchOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("iges: verify corrected conical carrier: %w", err)
	}
	if err := verifyConicalCarrierOwnership(m.material, reopened, toOriginal); err != nil {
		return nil, nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	return raw, toOriginal, nil
}

func verifyConicalCarrierOwnership(original, reopened ConicalRimFilletBoreIGES, toOriginal map[int]int) error {
	for _, pair := range [][2]int{
		{original.SourceGroupID, reopened.SourceGroupID}, {original.SourceSolidID, reopened.SourceSolidID},
		{original.SourceShellID, reopened.SourceShellID}, {original.SourceEdgeListID, reopened.SourceEdgeListID},
		{original.SourceVertexListID, reopened.SourceVertexListID},
	} {
		if pair[0] != toOriginal[pair[1]] {
			return geom.ErrInvalid
		}
	}
	sameOwners := func(prev, next []int) bool {
		if len(prev) != len(next) {
			return false
		}
		for i, id := range next {
			if prev[i] != toOriginal[id] {
				return false
			}
		}
		return true
	}
	if original.LengthUnit != reopened.LengthUnit ||
		!sameOwners(original.NativeFaceSourceIDs, reopened.NativeFaceSourceIDs) ||
		!sameOwners(original.NativeCurveSourceIDs, reopened.NativeCurveSourceIDs) ||
		!slices.Equal(original.NativeEdgeSourceIndices, reopened.NativeEdgeSourceIndices) ||
		!slices.Equal(original.NativeVertexSourceIndices, reopened.NativeVertexSourceIndices) {
		return geom.ErrInvalid
	}
	a, err := igesPrismBytes(original.Solid)
	if err != nil {
		return err
	}
	b, err := igesPrismBytes(reopened.Solid)
	if err != nil {
		return err
	}
	if !bytes.Equal(a, b) {
		return geom.ErrInvalid
	}
	return nil
}

func writeConicalRimFilletBoreCarrier(doc topologySource) ([]byte, map[int]int, error) {
	var file bytes.Buffer
	var global strings.Builder
	var startLines, globalLines int
	var directory []string
	scan := bufio.NewScanner(bytes.NewReader(doc.bytes))
	for scan.Scan() {
		line := scan.Text()
		if len(line) != 80 {
			return nil, nil, geom.ErrInvalid
		}
		switch line[72] {
		case 'S':
			startLines++
			file.WriteString(line + "\n")
		case 'G':
			globalLines++
			global.WriteString(line[:72])
			file.WriteString(line + "\n")
		case 'D':
			directory = append(directory, line)
		}
	}
	if err := scan.Err(); err != nil {
		return nil, nil, err
	}
	globalBudget := 4096
	_, sep, end, err := globalParameters(global.String(), &globalBudget)
	if err != nil {
		return nil, nil, err
	}
	toOriginal, toCorrected := make(map[int]int, len(doc.entities)), make(map[int]int, len(doc.entities))
	for i, rec := range doc.entities {
		id := 2*i + 1
		toOriginal[id], toCorrected[rec.id] = rec.id, id
	}
	var params []record
	for _, rec := range doc.entities {
		if rec.id < 1 || rec.id >= len(directory) {
			return nil, nil, geom.ErrInvalid
		}
		values, err := remapConicalCarrierPointers(rec, toCorrected)
		if err != nil {
			return nil, nil, err
		}
		lines, err := conicalCarrierParameterLines(values, sep, end)
		if err != nil {
			return nil, nil, err
		}
		transform := 0
		if rec.transform != 0 {
			transform = toCorrected[rec.transform]
			if transform == 0 {
				return nil, nil, geom.ErrInvalid
			}
		}
		id := toCorrected[rec.id]
		first, second := []byte(directory[rec.id-1]), []byte(directory[rec.id])
		setConicalCarrierDirectoryField(first, 0, rec.kind)
		setConicalCarrierDirectoryField(first, 1, len(params)+1)
		setConicalCarrierDirectoryField(first, 6, transform)
		setConicalCarrierDirectoryField(second, 0, rec.kind)
		setConicalCarrierDirectoryField(second, 3, len(lines))
		if rec.kind == 128 {
			setConicalCarrierDirectoryField(second, 4, 0)
		}
		fmt.Fprintf(&file, "%sD%07d\n%sD%07d\n", first[:72], id, second[:72], id+1)
		for _, line := range lines {
			params = append(params, record{data: line, owner: id})
		}
	}
	for i, line := range params {
		fmt.Fprintf(&file, "%-64s%8dP%07d\n", line.data, line.owner, i+1)
	}
	term := fmt.Sprintf("S%7dG%7dD%7dP%7d", startLines, globalLines, 2*len(doc.entities), len(params))
	fmt.Fprintf(&file, "%-72sT%07d\n", term, 1)
	return file.Bytes(), toOriginal, nil
}

func setConicalCarrierDirectoryField(line []byte, field, value int) {
	copy(line[8*field:8*(field+1)], fmt.Sprintf("%8d", value))
}

func conicalCarrierParameterLines(values []parameter, sep, end byte) ([]string, error) {
	var lines []string
	var line strings.Builder
	for i, p := range values {
		// Keep a numeric lexeme within a physical parameter record, as
		// required by readers that treat boundary padding as whitespace.
		if p.stringValue || len(p.text) >= 64 {
			return nil, geom.ErrUnrepresentable
		}
		if line.Len()+len(p.text)+1 > 64 {
			lines = append(lines, line.String())
			line.Reset()
		}
		line.WriteString(p.text)
		if i+1 == len(values) {
			line.WriteByte(end)
		} else {
			line.WriteByte(sep)
		}
	}
	return append(lines, line.String()), nil
}

func remapConicalCarrierPointers(rec topologyRecord, ids map[int]int) ([]parameter, error) {
	var pointers []int
	switch rec.kind {
	case 100, 110, 116, 123, 124, 128, 502:
		return rec.values, nil
	case 186:
		pointers = append(pointers, 1)
	case 190:
		pointers = append(pointers, 1, 2, 3)
	case 192:
		pointers = append(pointers, 1, 2, 4)
	case 198:
		pointers = append(pointers, 1, 2, 5)
	case 402:
		pointers = append(pointers, 2)
	case 504:
		for i := 2; i < len(rec.values); i += 5 {
			pointers = append(pointers, i, i+1, i+3)
		}
	case 508:
		for i := 3; i < len(rec.values); i += 5 {
			pointers = append(pointers, i)
		}
	case 510:
		pointers = append(pointers, 1)
		for i := 4; i < len(rec.values); i++ {
			pointers = append(pointers, i)
		}
	case 514:
		for i := 2; i < len(rec.values); i += 2 {
			pointers = append(pointers, i)
		}
	default:
		return nil, geom.ErrInvalid
	}
	out := slices.Clone(rec.values)
	for _, i := range pointers {
		old, err := intValue(out[i])
		if err != nil || ids[old] == 0 {
			return nil, geom.ErrInvalid
		}
		out[i].text = strconv.Itoa(ids[old])
	}
	return out, nil
}
