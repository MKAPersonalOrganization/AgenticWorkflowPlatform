package icon

import (
	"context"
	"fmt"
	"io"

	"github.com/a-h/templ"
)

// compassBase is the branded Compass.AI logo — an elegant compass rose with a
// diamond needle, delicate cardinal ticks, and a jewel-bearing center.
// %s is the class list; %s is an optional extra class injected after sizing.
const compassBase = `<svg class="%s" viewBox="0 0 32 32" fill="none" xmlns="http://www.w3.org/2000/svg">` +
	// — Outer bezel ring (double stroke for depth) —
	`<circle cx="16" cy="16" r="14.5" stroke="currentColor" stroke-width="0.6" opacity="0.25"/>` +
	`<circle cx="16" cy="16" r="13" stroke="currentColor" stroke-width="1.2" opacity="0.7"/>` +
	// — Cardinal tick marks (N is bolder) —
	`<line x1="16" y1="3.5" x2="16" y2="5.8" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/>` +
	`<line x1="28.5" y1="16" x2="26.2" y2="16" stroke="currentColor" stroke-width="0.8" stroke-linecap="round" opacity="0.5"/>` +
	`<line x1="16" y1="28.5" x2="16" y2="26.2" stroke="currentColor" stroke-width="0.8" stroke-linecap="round" opacity="0.5"/>` +
	`<line x1="3.5" y1="16" x2="5.8" y2="16" stroke="currentColor" stroke-width="0.8" stroke-linecap="round" opacity="0.5"/>` +
	// — Intercardinal tick marks (tiny, elegant) —
	`<line x1="24.1" y1="7.9" x2="22.8" y2="9.2" stroke="currentColor" stroke-width="0.5" stroke-linecap="round" opacity="0.25"/>` +
	`<line x1="24.1" y1="24.1" x2="22.8" y2="22.8" stroke="currentColor" stroke-width="0.5" stroke-linecap="round" opacity="0.25"/>` +
	`<line x1="7.9" y1="24.1" x2="9.2" y2="22.8" stroke="currentColor" stroke-width="0.5" stroke-linecap="round" opacity="0.25"/>` +
	`<line x1="7.9" y1="7.9" x2="9.2" y2="9.2" stroke="currentColor" stroke-width="0.5" stroke-linecap="round" opacity="0.25"/>` +
	// — Needle group (class allows CSS animation on needle only) —
	`<g class="compass-needle" style="transform-origin:16px 16px">` +
	// North needle — bold, sharp diamond
	`<path d="M16 5.5 L17.8 14 L16 13 L14.2 14 Z" fill="currentColor"/>` +
	// South needle — lighter, ghosted
	`<path d="M16 26.5 L14.2 18 L16 19 L17.8 18 Z" fill="currentColor" opacity="0.2"/>` +
	`</g>` +
	// — Center jewel bearing (layered circles) —
	`<circle cx="16" cy="16" r="2.2" fill="currentColor" opacity="0.15"/>` +
	`<circle cx="16" cy="16" r="1.3" fill="currentColor"/>` +
	`</svg>`

// CompassLogo renders the static branded Compass.AI logo.
// class controls sizing (e.g. "size-4", "size-5", "size-12").
func CompassLogo(class string) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		_, err := fmt.Fprintf(w, compassBase, class)
		return err
	})
}

// CompassLogoAnimated renders the compass logo with the needle-seek animation
// baked directly into the SVG element — no wrapper div. The SVG flows inline
// like any other icon. The needle responds to ancestor :hover (a, button, etc.)
// so the animation feels native to the UI, not layered on top.
func CompassLogoAnimated(class string) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		_, err := fmt.Fprintf(w, compassBase, class+" compass-live")
		return err
	})
}

// CompassLogoCSS returns the <style> block for the compass needle animation.
// Include once per page that uses CompassLogoAnimated.
func CompassLogoCSS() templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		css := `<style>
/* ── Compass.AI — embedded needle animation ──
   No wrapper div. The .compass-live class sits directly on the <svg>.
   Needle reacts to ancestor hover so it feels native, not overlaid. */

@keyframes compassSettle {
  0%   { transform: rotate(0deg); }
  8%   { transform: rotate(15deg); }
  20%  { transform: rotate(-9deg); }
  34%  { transform: rotate(5deg); }
  50%  { transform: rotate(-2deg); }
  68%  { transform: rotate(0.8deg); }
  100% { transform: rotate(0deg); }
}
@keyframes compassIdle {
  0%, 100% { transform: rotate(0deg); }
  40%      { transform: rotate(0.6deg); }
  70%      { transform: rotate(-0.4deg); }
}
@keyframes compassDisturb {
  0%   { transform: rotate(0deg); }
  12%  { transform: rotate(8deg); }
  30%  { transform: rotate(-5deg); }
  50%  { transform: rotate(2.5deg); }
  72%  { transform: rotate(-0.8deg); }
  100% { transform: rotate(0deg); }
}

/* On load: settle once, then idle drift (barely visible, like a real compass) */
.compass-live .compass-needle {
  animation: compassSettle 2s cubic-bezier(0.22, 1, 0.36, 1) forwards,
             compassIdle 6s ease-in-out 2s infinite;
}

/* Ancestor hover → needle gets disturbed & re-settles (feels native) */
a:hover > * .compass-live .compass-needle,
a:hover > .compass-live .compass-needle,
button:hover > * .compass-live .compass-needle,
button:hover > .compass-live .compass-needle,
.group:hover .compass-live .compass-needle,
div:hover > .compass-live .compass-needle {
  animation: compassDisturb 1.4s cubic-bezier(0.22, 1, 0.36, 1) forwards;
}
</style>`
		_, err := w.Write([]byte(css))
		return err
	})
}
