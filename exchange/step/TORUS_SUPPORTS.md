# Exact standalone ring-torus supports

`ReadTorusSupports` / `WriteTorusSupports` exchange complete `TOROIDAL_SURFACE`
supports through a generic `REPRESENTATION` and `GEOMETRIC_SET`. Each has an
explicit uniquely owned placement, center, axis and reference. `Geometry`
returns `geom.TorusSurface`: U follows the major circle from the reference ray,
V follows the tube, both periodic in radians. No seam topology, finite trim,
material orientation or solid closure is inferred.

The strict domain is major radius > minor radius > 0 with perpendicular signed
coordinate unit directions and exactly represented source geometry scalars.
Horn/spindle cases, implicit directions, normalization, transformed/trimmed
shapes, product/assembly graphs, shared dependencies and unowned entities fail.
Units and all surface/placement/datum plus representation/set names survive
without rescaling. Existing exact scalar/frame and unit checks are reused.
Limits include 64 supports and 512 entities plus parser byte/value/depth limits.
Writer output is staged and strictly re-read before caller I/O.

Independent OCCT fixtures and `PLAINSOLID_OCCT_TORUS_SUPPORT` qualify analytic
entity transfer in both directions and axis signs by position and both partials.
These checks do not claim bounded-face or solid transfer. Preflight hints are
structural routing only. The matching IGES198 subset has the same natural
parameter conventions, but format-specific metadata requires explicit gating.
