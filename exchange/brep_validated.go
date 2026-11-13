package exchange

import (
	"context"
	"io"
	"plainsolid/brep"
	"plainsolid/collision"
)

// ReadBRepJSONValidated combines the existing bounded native reader with exact
// shell embedding/nesting validation. It preserves input entity indices and
// returns caller-owned shell/face ancestry diagnostics (source solid index0).
// Parser limits and assembly geometry budgets apply independently. Parsing is
// byte-bounded but io.Reader reads are not interruptible by context; cancellation
// is checked before parsing, immediately afterwards and during validation.
// Any failure returns neither partial geometry nor partial diagnostics.
func ReadBRepJSONValidated(ctx context.Context, r io.Reader, l BRepLimits, o collision.AssemblyOptions) (brep.Solid, collision.AssemblyReport, error) {
	if ctx == nil {
		return brep.Solid{}, collision.AssemblyReport{}, collision.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return brep.Solid{}, collision.AssemblyReport{}, e
	}
	s, e := ReadBRepJSON(r, l)
	if e != nil {
		return brep.Solid{}, collision.AssemblyReport{}, e
	}
	if e = ctx.Err(); e != nil {
		return brep.Solid{}, collision.AssemblyReport{}, e
	}
	report, e := collision.ValidateAssembly(ctx, []brep.Solid{s}, o)
	if e != nil {
		return brep.Solid{}, collision.AssemblyReport{}, e
	}
	return s, report, nil
}
