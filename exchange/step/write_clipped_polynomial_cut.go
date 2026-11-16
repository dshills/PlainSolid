package step

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"reflect"
	"unicode/utf8"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// ClippedPolynomialCutSTEPOptions has the same native proof and decimal
// serialization limits as the single stepped Fuse transfer.
type ClippedPolynomialCutSTEPOptions = ClippedPolynomialFuseSTEPOptions

// WriteClippedPolynomialCut transfers each positive native component as a
// separate MANIFOLD_SOLID_BREP in original chamber order. Shared exact
// points retain separate vertex IDs and identical coordinates. No weld,
// tessellation or sampled support reconstruction is used.
func WriteClippedPolynomialCut(ctx context.Context, w io.Writer, cut *trimmed.ClippedPolynomialCut, name string, lengthUnit float64, l Limits, o ClippedPolynomialCutSTEPOptions) error {
	if cut == nil {
		return geom.ErrInvalid
	}
	contacts := cut.Contacts()
	return writeClippedCutSTEPBodies(ctx, w, cut.Components(), contacts[:], name, lengthUnit, l, o)
}

// WriteClippedPolynomialThreeTransverseCut retains all three exact original
// contact pairs and transfers all four or five separate positive bodies.
func WriteClippedPolynomialThreeTransverseCut(ctx context.Context, w io.Writer, cut *trimmed.ClippedPolynomialThreeTransverseCut, name string, lengthUnit float64, l Limits, o ClippedPolynomialCutSTEPOptions) error {
	if cut == nil {
		return geom.ErrInvalid
	}
	return writeClippedCutSTEPBodies(ctx, w, cut.Components(), cut.Contacts(), name, lengthUnit, l, o)
}

func writeClippedCutSTEPBodies(ctx context.Context, w io.Writer, parts []trimmed.Solid, contacts []trimmed.PolynomialMixedCutContact,
	name string, lengthUnit float64, l Limits, o ClippedPolynomialCutSTEPOptions) error {
	if ctx == nil || w == nil || !utf8.ValidString(name) || len(name) > 1024 || !validLengthUnit(lengthUnit) ||
		!finiteNumber(o.PoleTolerance) || o.PoleTolerance < 0 {
		return geom.ErrInvalid
	}
	l, err := l.defaults()
	if err != nil {
		return err
	}
	if len(parts) == 0 || len(parts) > l.MaxSolids {
		return geom.ErrConvergence
	}
	boundaries := make([]trimmed.ExactBezierBoundary, len(parts))
	vertices, faces, uses := 0, 0, 0
	for i, s := range parts {
		if err := ctx.Err(); err != nil {
			return err
		}
		if len(s.Topology().Shells) != 1 {
			return fmt.Errorf("step: clipped Cut component %d: %w", i, geom.ErrConvergence)
		}
		v, f, e := len(s.Vertices()), len(s.Faces()), len(s.Edges())
		if v > l.MaxVertices-vertices || f > l.MaxFaces-faces || e > (l.MaxEdgeUses-uses)/2 {
			return geom.ErrConvergence
		}
		vertices, faces, uses = vertices+v, faces+f, uses+2*e
		boundaries[i], err = s.ExtractExactBezierBoundary(ctx, o.Proof)
		if err != nil {
			return fmt.Errorf("step: clipped Cut component %d: %w", i, err)
		}
	}
	if err := auditClippedCutSTEPContacts(ctx, contacts, boundaries); err != nil {
		return err
	}
	staged := clippedFuseSTEPStage{limit: l.MaxBytes}
	var geometryErr error
	tol := mappedChainStepRat(o.PoleTolerance)
	err = writeStandalone(&staged, lengthUnit, "", "ADVANCED_BREP_SHAPE_REPRESENTATION", func(emit func(string) int64) []int64 {
		ids := make([]int64, len(boundaries))
		for i, part := range boundaries {
			label := fmt.Sprintf("%s [%d]", name, i+1)
			ids[i], geometryErr = emitExactBezierSTEPBody(ctx, emit, part, label, tol)
			if geometryErr != nil {
				geometryErr = fmt.Errorf("step: clipped Cut component %d: %w", i, geometryErr)
				return nil
			}
		}
		return ids
	})
	if geometryErr != nil {
		return geometryErr
	}
	if err != nil {
		return err
	}
	if _, err := Parse(bytes.NewReader(staged.Bytes()), l); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	n, err := w.Write(staged.Bytes())
	if err == nil && n != staged.Len() {
		return io.ErrShortWrite
	}
	return err
}

// VerifyClippedPolynomialCutSTEP authenticates the canonical standard DATA
// graph and its separate source components. The original native proof is
// required; this does not infer compound material from arbitrary STEP.
func VerifyClippedPolynomialCutSTEP(ctx context.Context, input io.Reader, cut *trimmed.ClippedPolynomialCut, name string, lengthUnit float64, l Limits, o ClippedPolynomialCutSTEPOptions) error {
	if cut == nil {
		return geom.ErrInvalid
	}
	return verifyClippedCutSTEPBodies(ctx, input, l, func(w io.Writer) error {
		return WriteClippedPolynomialCut(ctx, w, cut, name, lengthUnit, l, o)
	})
}

// VerifyClippedPolynomialThreeTransverseCutSTEP authenticates the canonical
// DATA graph and every component-local vertex occurrence against source.
func VerifyClippedPolynomialThreeTransverseCutSTEP(ctx context.Context, input io.Reader, cut *trimmed.ClippedPolynomialThreeTransverseCut, name string, lengthUnit float64, l Limits, o ClippedPolynomialCutSTEPOptions) error {
	if cut == nil {
		return geom.ErrInvalid
	}
	return verifyClippedCutSTEPBodies(ctx, input, l, func(w io.Writer) error {
		return WriteClippedPolynomialThreeTransverseCut(ctx, w, cut, name, lengthUnit, l, o)
	})
}

func verifyClippedCutSTEPBodies(ctx context.Context, input io.Reader, l Limits, write func(io.Writer) error) error {
	if ctx == nil || input == nil {
		return geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	actual, err := Parse(input, l)
	if err != nil {
		return err
	}
	var expected bytes.Buffer
	if err := write(&expected); err != nil {
		return err
	}
	want, err := Parse(&expected, l)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(actual.Entities, want.Entities) {
		return fmt.Errorf("step: clipped Cut DATA differs from the source component proofs: %w", geom.ErrConvergence)
	}
	return ctx.Err()
}

func auditClippedCutSTEPContacts(ctx context.Context, contacts []trimmed.PolynomialMixedCutContact, bodies []trimmed.ExactBezierBoundary) error {
	type occurrence struct{ solid, vertex int }
	declared := make(map[[2]occurrence]geom.Vec3, len(contacts))
	for _, contact := range contacts {
		pair := [2]occurrence{{contact.Components[0], contact.Vertices[0]}, {contact.Components[1], contact.Vertices[1]}}
		if pair[0].solid >= pair[1].solid {
			return geom.ErrConvergence
		}
		var exact [3]float64
		for i, p := range contact.Point {
			var err error
			exact[i], err = geom.RepresentedExactScalar(p)
			if err != nil {
				return err
			}
		}
		point := geom.Vec3{X: exact[0], Y: exact[1], Z: exact[2]}
		for _, o := range pair {
			if o.solid < 0 || o.solid >= len(bodies) || o.vertex < 0 || o.vertex >= len(bodies[o.solid].Vertices) || bodies[o.solid].Vertices[o.vertex] != point {
				return geom.ErrConvergence
			}
		}
		declared[pair] = point
	}
	if len(declared) != len(contacts) {
		return geom.ErrConvergence
	}
	occurrences := make(map[geom.Vec3][]occurrence)
	for i, b := range bodies {
		for j, point := range b.Vertices {
			if err := ctx.Err(); err != nil {
				return err
			}
			current := occurrence{i, j}
			for _, previous := range occurrences[point] {
				pair := [2]occurrence{previous, current}
				if p, ok := declared[pair]; !ok || p != point {
					return fmt.Errorf("step: undeclared coincident component vertices: %w", geom.ErrConvergence)
				}
				delete(declared, pair)
			}
			occurrences[point] = append(occurrences[point], current)
		}
	}
	if len(declared) != 0 {
		return geom.ErrConvergence
	}
	return nil
}
