package plaincadbridge

import (
	"context"
	"errors"

	"plainsolid/brep"
	"plainsolid/brep/boolean"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
)

// This file is the adapter of the general Boolean trial
// (specs/general-boolean). It is reached only on a Bridge whose general
// field is set, which only a test constructor does; the default bridge is
// unchanged.

// generalOptions are the tolerances the bridge passes to the general
// Boolean.
var generalOptions = boolean.Options{
	Linear:      1e-7,
	Angular:     1e-9,
	Volume:      1e-7,
	Integration: parametric.IntegrationOptions{RelativeTolerance: 1e-9},
}

var generalOperations = map[string]boolean.Operation{"fuse": boolean.Union, "common": boolean.Common, "cut": boolean.Cut}

// generalBoolean serves cut, fuse and common with boolean.Combine. There
// is no fall back to a legacy path.
func (b *Bridge) generalBoolean(ctx context.Context, a, c stored, op string) (Shape, error) {
	x, err := generalOperand(a)
	if err != nil {
		return Shape{}, err
	}
	y, err := generalOperand(c)
	if err != nil {
		return Shape{}, err
	}
	r, err := boolean.Combine(ctx, x, y, generalOperations[op], generalOptions)
	if err != nil {
		return Shape{}, generalFailure(err)
	}
	return b.putGeneral(ctx, r)
}

// generalOperand reads a stored shape only to obtain its trimmed solid.
func generalOperand(s stored) (trimmed.Solid, error) {
	if !s.planar() {
		return trimmed.Solid{}, &failureError{"UNSUPPORTED", "the general Boolean does not take this stored shape yet"}
	}
	t, err := trimmed.FromPlanar(s.solid, generalOptions.Linear)
	if err != nil {
		return trimmed.Solid{}, &failureError{"UNSUPPORTED", "the stored shape has no trimmed solid: " + err.Error()}
	}
	return t, nil
}

// putGeneral publishes a result of the general Boolean: the empty solid
// as the legacy paths publish it, and a solid of one shell as an analytic
// body with its integrated volume and area.
func (b *Bridge) putGeneral(ctx context.Context, r boolean.Result) (Shape, error) {
	if r.Solids == 0 {
		return b.put(brep.Solid{})
	}
	if len(r.Shells) != 1 {
		return Shape{}, &failureError{"UNSUPPORTED", "a general Boolean result with a void or several solids is not published yet"}
	}
	p, err := r.Solid.MassProperties(ctx, generalOptions.Integration)
	if err != nil {
		return Shape{}, err
	}
	return b.putAnalytic(r.Solid, p.Volume, p.Area)
}

// generalFailure maps the errors of the general Boolean to bridge
// failures.
func generalFailure(err error) error {
	code := "INVALID"
	switch {
	case errors.Is(err, boolean.ErrInvalid):
	case errors.Is(err, boolean.ErrUnsupported), errors.Is(err, boolean.ErrNonManifold), errors.Is(err, boolean.ErrTolerance):
		code = "UNSUPPORTED"
	case errors.Is(err, boolean.ErrBudget):
		code = "LIMIT"
	case errors.Is(err, boolean.ErrInconsistent), errors.Is(err, boolean.ErrValidation):
		code = "INTERNAL"
	default:
		return err
	}
	return &failureError{code, err.Error()}
}
