//go:build research

package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math/big"
	"plainsolid/geom"
)

type nativeAlgebraicGraphOccurrence struct {
	Version          int             `json:"version"`
	Kind             string          `json:"kind"`
	Pose             [3][4]string    `json:"pose"`
	SourceProjection [2]int          `json:"sourceProjection"`
	WorldProjection  [2]int          `json:"worldProjection"`
	Source           json.RawMessage `json:"source"`
}

// WriteAlgebraicGraphOccurrenceJSON stores canonical exact rationals and the
// original Boolean recipe only. It never exports rounded transformed supports
// or treats serialized classification/ancestry as proof.
func WriteAlgebraicGraphOccurrenceJSON(w io.Writer, o *AlgebraicGraphOccurrence) error {
	if w == nil || o == nil || o.query == nil {
		return geom.ErrInvalid
	}
	var source bytes.Buffer
	if e := WriteAlgebraicShellBooleanJSON(&source, o.source); e != nil {
		return e
	}
	envelope := nativeAlgebraicGraphOccurrence{Version: 1, Kind: "trimmed-algebraic-graph-occurrence", SourceProjection: o.query.axes, WorldProjection: o.axes, Source: source.Bytes()}
	for i := 0; i < 3; i++ {
		for j := 0; j < 4; j++ {
			envelope.Pose[i][j] = o.pose[i][j].RatString()
		}
	}
	raw, e := json.Marshal(envelope)
	if e != nil {
		return e
	}
	raw = append(raw, '\n')
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		e = io.ErrShortWrite
	}
	return e
}

// ReadAlgebraicGraphOccurrenceJSON verifies the bounded exact pose, replays the
// original native Boolean section/imprint/material proof, and prepares its exact
// original query anew. Existing JSON and section/Boolean budgets remain in force;
// pose preflight and occurrence preparation each use query.Exact independently.
// A supplied query.Projection must agree with the stored source projection.
// The result is an occurrence, not a newly sewn world-space Solid.
func ReadAlgebraicGraphOccurrenceJSON(ctx context.Context, r io.Reader, l JSONLimits, section AlgebraicShellSectionOptions, boolean AlgebraicShellBooleanOptions, query AlgebraicGraphMaterialOptions) (*AlgebraicGraphOccurrence, error) {
	if ctx == nil || r == nil || l.MaxBytes < 0 || l.MaxBytes > 1<<30 || l.MaxEntities < 0 || l.MaxControlPoints < 0 {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if l.MaxBytes == 0 {
		l.MaxBytes = 32 << 20
	}
	raw, e := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if e != nil {
		return nil, e
	}
	if int64(len(raw)) > l.MaxBytes {
		return nil, geom.ErrConvergence
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var f nativeAlgebraicGraphOccurrence
	if e = decoder.Decode(&f); e != nil {
		return nil, e
	}
	if e = decoder.Decode(new(any)); e != io.EOF || f.Version != 1 || f.Kind != "trimmed-algebraic-graph-occurrence" || len(f.Source) == 0 {
		return nil, geom.ErrInvalid
	}
	if query.Projection != nil && *query.Projection != f.SourceProjection {
		return nil, geom.ErrInvalid
	}
	exact, e := graphMaterialLimits(query.Exact)
	if e != nil {
		return nil, e
	}
	var pose [3][4]*big.Rat
	for i := 0; i < 3; i++ {
		for j := 0; j < 4; j++ {
			value := f.Pose[i][j]
			if len(value) > 2*exact.MaxCoefficientBits+32 {
				return nil, geom.ErrConvergence
			}
			x, ok := new(big.Rat).SetString(value)
			if !ok || x.RatString() != value {
				return nil, geom.ErrInvalid
			}
			pose[i][j] = x
		}
	}
	arithmetic := graphOccurrenceArithmetic{ctx: ctx, options: exact}
	if _, _, e = prepareGraphOccurrencePose(&arithmetic, pose, f.SourceProjection, f.WorldProjection); e != nil {
		return nil, e
	}
	source, e := ReadAlgebraicShellBooleanJSON(ctx, bytes.NewReader(f.Source), l, section, boolean)
	if e != nil {
		return nil, e
	}
	query.Projection = &f.SourceProjection
	return NewAlgebraicGraphOccurrence(ctx, source, pose, f.WorldProjection, query)
}
