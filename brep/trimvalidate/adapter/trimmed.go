// Package adapter applies trimvalidate's UV topology proof to trimmed faces.
package adapter

import (
	"context"

	"plainsolid/brep/trimmed"
	"plainsolid/brep/trimvalidate"
)

// Validate checks UV topology independently of model-space agreement.
// Reported edge indices refer to original uses, including for multispan trims.
func Validate(ctx context.Context, f trimmed.Face, o trimvalidate.Options) (trimvalidate.Report, error) {
	return trimmed.ValidateFaceTrimLoops(ctx, f, o)
}
