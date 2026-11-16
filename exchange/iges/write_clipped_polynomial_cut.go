package iges

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/big"
	"reflect"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// ClippedPolynomialCutIGESOptions controls exact native reproof and decimal
// serialization of derived curve poles as for the single stepped Fuse.
type ClippedPolynomialCutIGESOptions = ClippedPolynomialFuseIGESOptions

// WriteClippedPolynomialCut transfers the exact source-backed components in
// chamber order as separate type 186 solids in a type 402 form 1 group. Each
// body has its own type 502 vertex and type 504 edge list. The source contact
// points occur at identical coordinates in separate lists.
func WriteClippedPolynomialCut(ctx context.Context, w io.Writer, cut *trimmed.ClippedPolynomialCut, unit float64, l Limits, o ClippedPolynomialCutIGESOptions) error {
	if cut == nil {
		return geom.ErrInvalid
	}
	contacts := cut.Contacts()
	return writeClippedCutIGESBodies(ctx, w, cut.Components(), contacts[:], unit, l, o)
}

// WriteClippedPolynomialThreeTransverseCut transfers the positive bodies of
// a source-proved three-crossing Cut. All three original point contacts
// retain their separately indexed vertex occurrences and exact coordinates.
func WriteClippedPolynomialThreeTransverseCut(ctx context.Context, w io.Writer, cut *trimmed.ClippedPolynomialThreeTransverseCut, unit float64, l Limits, o ClippedPolynomialCutIGESOptions) error {
	if cut == nil {
		return geom.ErrInvalid
	}
	return writeClippedCutIGESBodies(ctx, w, cut.Components(), cut.Contacts(), unit, l, o)
}

func writeClippedCutIGESBodies(ctx context.Context, w io.Writer, components []trimmed.Solid, contacts []trimmed.PolynomialMixedCutContact, unit float64, l Limits, o ClippedPolynomialCutIGESOptions) error {
	bound := new(big.Rat).SetFloat64(o.PoleTolerance)
	if ctx == nil || w == nil || !finite(unit) || unit <= 0 || bound == nil || bound.Sign() < 0 {
		return geom.ErrInvalid
	}
	l, err := l.defaults()
	if err != nil {
		return err
	}
	if len(components) < 2 || len(components) > l.MaxEntities {
		return geom.ErrConvergence
	}
	boundaries := make([]trimmed.ExactBezierBoundary, len(components))
	for i, body := range components {
		if err := ctx.Err(); err != nil {
			return err
		}
		if len(body.Topology().Shells) != 1 {
			return geom.ErrConvergence
		}
		boundaries[i], err = body.ExtractExactBezierBoundary(ctx, o.Proof)
		if err != nil {
			return fmt.Errorf("iges: clipped Cut component %d: %w", i, err)
		}
	}
	if err := auditClippedCutIGESContacts(ctx, contacts, boundaries); err != nil {
		return err
	}

	a := topologyAuthor{groupedSolids: true}
	remaining := l.MaxControlPoints
	consume := func(n int) error {
		if n > remaining {
			return geom.ErrConvergence
		}
		remaining -= n
		return ctx.Err()
	}
	group := []int{402, len(boundaries)}
	for i, b := range boundaries {
		root, err := emitExactBezierIGESBody(&a, b, bound, consume)
		if err != nil {
			return fmt.Errorf("iges: clipped Cut component %d: %w", i, err)
		}
		if len(a.records) >= l.MaxEntities {
			return geom.ErrConvergence
		}
		group = append(group, root)
	}
	a.add(402, 1, topologyNumbers(group...))
	raw, err := a.file(unit)
	if err != nil {
		return err
	}
	if _, err := readClippedPolynomialCutIGES(bytes.NewReader(raw), l); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	n, err := w.Write(raw)
	if err == nil && n != len(raw) {
		err = io.ErrShortWrite
	}
	return err
}

// VerifyClippedPolynomialCutIGES authenticates the original source-backed
// group, local vertex occurrences, coedge laws and physical units. Start
// section annotations may differ from the canonical transfer.
func VerifyClippedPolynomialCutIGES(ctx context.Context, input io.Reader, cut *trimmed.ClippedPolynomialCut, unit float64, l Limits, o ClippedPolynomialCutIGESOptions) error {
	if cut == nil {
		return geom.ErrInvalid
	}
	return verifyClippedCutIGESBodies(ctx, input, l, func(w io.Writer) error {
		return WriteClippedPolynomialCut(ctx, w, cut, unit, l, o)
	})
}

// VerifyClippedPolynomialThreeTransverseCutIGES authenticates the separate
// type 186 solids and every local occurrence of their three original contacts.
func VerifyClippedPolynomialThreeTransverseCutIGES(ctx context.Context, input io.Reader, cut *trimmed.ClippedPolynomialThreeTransverseCut, unit float64, l Limits, o ClippedPolynomialCutIGESOptions) error {
	if cut == nil {
		return geom.ErrInvalid
	}
	return verifyClippedCutIGESBodies(ctx, input, l, func(w io.Writer) error {
		return WriteClippedPolynomialThreeTransverseCut(ctx, w, cut, unit, l, o)
	})
}

func verifyClippedCutIGESBodies(ctx context.Context, input io.Reader, l Limits, write func(io.Writer) error) error {
	if ctx == nil || input == nil {
		return geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	actual, err := readClippedPolynomialCutIGES(input, l)
	if err != nil {
		return err
	}
	var expected bytes.Buffer
	if err := write(&expected); err != nil {
		return err
	}
	want, err := readClippedPolynomialCutIGES(&expected, l)
	if err != nil {
		return err
	}
	if actual.unit != want.unit || !reflect.DeepEqual(actual.entities, want.entities) ||
		!bytes.Equal(exactBezierIGESHeaderRecords(actual.bytes), exactBezierIGESHeaderRecords(want.bytes)) {
		return fmt.Errorf("iges: clipped Cut topology, global records or directory differ from source proof: %w", geom.ErrConvergence)
	}
	return ctx.Err()
}

func readClippedPolynomialCutIGES(input io.Reader, l Limits) (topologySource, error) {
	return readTopologyRecordsWithPlanarShells(input, l, true, false, false, false, false, false, false, true)
}

// Global precision and member visibility affect transfer. Authenticate the
// global and directory records, which the parsed entities omit.
func exactBezierIGESHeaderRecords(raw []byte) []byte {
	var header []byte
	for len(raw) > 0 {
		var line []byte
		line, raw, _ = bytes.Cut(raw, []byte{'\n'})
		if len(line) >= 80 && (line[72] == 'G' || line[72] == 'D') {
			header = append(header, line[:80]...)
		}
	}
	return header
}

func auditClippedCutIGESContacts(ctx context.Context, contacts []trimmed.PolynomialMixedCutContact, bodies []trimmed.ExactBezierBoundary) error {
	type occurrence struct{ body, vertex int }
	declared := make(map[[2]occurrence]bool, len(contacts))
	for _, c := range contacts {
		pair := [2]occurrence{{c.Components[0], c.Vertices[0]}, {c.Components[1], c.Vertices[1]}}
		if pair[0].body >= pair[1].body || declared[pair] {
			return geom.ErrConvergence
		}
		var coordinates [3]float64
		for i, p := range c.Point {
			v, err := geom.RepresentedExactScalar(p)
			if err != nil {
				return err
			}
			coordinates[i] = v
		}
		point := geom.Vec3{X: coordinates[0], Y: coordinates[1], Z: coordinates[2]}
		for _, o := range pair {
			if o.body < 0 || o.body >= len(bodies) || o.vertex < 0 || o.vertex >= len(bodies[o.body].Vertices) ||
				bodies[o.body].Vertices[o.vertex] != point {
				return geom.ErrConvergence
			}
		}
		declared[pair] = true
	}
	seen := make(map[geom.Vec3][]occurrence)
	for i, body := range bodies {
		for j, p := range body.Vertices {
			if err := ctx.Err(); err != nil {
				return err
			}
			current := occurrence{i, j}
			for _, previous := range seen[p] {
				pair := [2]occurrence{previous, current}
				if !declared[pair] {
					return fmt.Errorf("iges: undeclared coincident clipped Cut vertices: %w", geom.ErrConvergence)
				}
				delete(declared, pair)
			}
			seen[p] = append(seen[p], current)
		}
	}
	if len(declared) != 0 {
		return geom.ErrConvergence
	}
	return nil
}
