package icon

import (
	"context"
	"fmt"
	"io"

	"github.com/a-h/templ"
)

// compassSVG is the branded Compass.AI logo — an elegant compass rose with a
// diamond needle, delicate cardinal ticks, and a jewel-bearing center.
const compassSVG = `<svg class="%s" viewBox="0 0 32 32" fill="none" xmlns="http://www.w3.org/2000/svg">` +
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
		_, err := w.Write([]byte(fmt.Sprintf(compassSVG, class)))
		return err
	})
}

// CompassLogoAnimated renders the compass logo with a realistic needle-seek
// animation. The needle oscillates and settles toward north like a physical
// compass. Use wrapperClass to control the outer container and svgClass for
// the SVG sizing.
func CompassLogoAnimated(svgClass, wrapperClass string) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		html := fmt.Sprintf(
			`<div class="%s compass-logo-animated">`+compassSVG+`</div>`,
			wrapperClass, svgClass,
		)
		_, err := w.Write([]byte(html))
		return err
	})
}

// CompassLogoCSS returns the <style> block for the compass needle-seek
// animation. Include this once per page that uses CompassLogoAnimated.
func CompassLogoCSS() templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		css := `<style>
/* ── Compass.AI needle-seek animation ── */
@keyframes needleSeek {
  0%   { transform: rotate(0deg); }
  6%   { transform: rotate(22deg); }
  14%  { transform: rotate(-14deg); }
  22%  { transform: rotate(9deg); }
  32%  { transform: rotate(-5deg); }
  44%  { transform: rotate(3deg); }
  58%  { transform: rotate(-1.5deg); }
  72%  { transform: rotate(0.5deg); }
  100% { transform: rotate(0deg); }
}
@keyframes needleHover {
  0%   { transform: rotate(0deg); }
  25%  { transform: rotate(1.5deg); }
  50%  { transform: rotate(-1deg); }
  75%  { transform: rotate(0.5deg); }
  100% { transform: rotate(0deg); }
}
/* Initial settle — plays once */
.compass-logo-animated .compass-needle {
  animation: needleSeek 2.4s cubic-bezier(0.22, 1, 0.36, 1) forwards,
             needleHover 4s ease-in-out 2.4s infinite;
}
/* On parent hover — needle disturbed, re-settles */
.compass-logo-animated:hover .compass-needle {
  animation: needleSeek 1.8s cubic-bezier(0.22, 1, 0.36, 1) forwards;
}
/* Subtle ring glow on hover */
.compass-logo-animated {
  transition: filter 0.3s ease;
}
.compass-logo-animated:hover {
  filter: drop-shadow(0 0 6px currentColor);
}
</style>`
		_, err := w.Write([]byte(css))
		return err
	})
}
