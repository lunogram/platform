---
version: alpha
name: Lunogram Console
description: >-
  The design system for the Lunogram operator console — a calm, light-first
  developer-infrastructure dashboard for an automated messaging and customer
  engagement platform.
colors:
  # Primary ("Ink") — the single near-black that carries all primary type and chrome.
  primary: "#151C2D"
  ink-soft: "#6B707B"
  # Surfaces
  surface: "#FFFFFF"
  surface-soft: "#F9FAFB"
  surface-muted: "#F5F5F7"
  surface-editor: "#0D121E"
  # Hairlines
  border: "#E8E8EA"
  border-strong: "#CECFD2"
  # Status — red / blue / amber / green / purple, each with a soft tint and a hard shade.
  red: "#D92D20"
  red-soft: "#FECDCA"
  red-hard: "#B42419"
  blue: "#2970FF"
  blue-soft: "#D1E0FF"
  blue-hard: "#004EBB"
  amber: "#FEC84B"
  amber-soft: "#FEF0C7"
  amber-hard: "#F79009"
  green: "#32D583"
  green-soft: "#D1FADF"
  green-hard: "#039855"
  purple: "#8729C1"
  purple-soft: "#F7D8FF"
  purple-hard: "#4E0C77"
# Dark theme. Lists only the tokens that differ from the light palette; everything
# else (status base hues, rounded, typography, spacing) carries over unchanged.
# Defined ahead of a shipping theme toggle so new components are built theme-aware.
colors-dark:
  primary: "#FFFFFF"
  ink-soft: "#919496"
  surface: "#121721"
  surface-soft: "#1A1F2B"
  surface-muted: "#252B3A"
  surface-editor: "#0D121E"
  border: "#2B3245"
  border-strong: "#3A4358"
  # Status soft/hard roles invert on dark: the soft tint deepens (badge fill) and
  # the hard shade lightens (badge text). Purple is the example already in the CSS.
  purple-soft: "#4E0C77"
  purple-hard: "#D1A7EA"
typography:
  display:
    fontFamily: Inter
    fontSize: 30px
    fontWeight: 600
    lineHeight: 1.15
    letterSpacing: -0.02em
  title:
    fontFamily: Inter
    fontSize: 24px
    fontWeight: 600
    lineHeight: 1.2
    letterSpacing: -0.02em
  heading:
    fontFamily: Inter
    fontSize: 18px
    fontWeight: 600
    lineHeight: 1.3
    letterSpacing: -0.01em
  body:
    fontFamily: Inter
    fontSize: 15px
    fontWeight: 400
    lineHeight: 1.5
  body-sm:
    fontFamily: Inter
    fontSize: 14px
    fontWeight: 400
    lineHeight: 1.45
  label:
    fontFamily: Inter
    fontSize: 14px
    fontWeight: 500
    lineHeight: 1.4
  caption:
    fontFamily: Inter
    fontSize: 12px
    fontWeight: 400
    lineHeight: 1.35
  mono:
    fontFamily: "ui-monospace, SFMono-Regular, Menlo, Monaco, monospace"
    fontSize: 13px
    fontWeight: 400
    lineHeight: 1.4
rounded:
  sm: 6px
  md: 8px
  lg: 10px
  xl: 14px
  full: 9999px
spacing:
  base: 8px
  xs: 4px
  sm: 8px
  gap: 10px
  md: 16px
  card: 24px
  page-x: 40px
  page-x-mobile: 20px
  sidebar: 256px
elevation:
  hairline: "0 0 0 1px {colors.border}"
  sm: "0 1px 2px 0 rgba(0,0,0,0.05)"
  card: "0 1px 3px 0 rgba(0,0,0,0.1)"
motion:
  feedback: 150ms
  content: 200ms
  easing: "cubic-bezier(0.4, 0, 0.2, 1)"
components:
  button-primary:
    backgroundColor: "{colors.primary}"
    textColor: "{colors.surface}"
    rounded: "{rounded.md}"
    height: 36px
    padding: 16px
  button-primary-hover:
    backgroundColor: "color-mix(in srgb, {colors.primary} 90%, transparent)"
  button-outline:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.primary}"
    rounded: "{rounded.md}"
  button-ghost:
    textColor: "{colors.primary}"
  button-ghost-hover:
    backgroundColor: "{colors.surface-muted}"
  button-destructive:
    backgroundColor: "{colors.red}"
    textColor: "{colors.surface}"
    rounded: "{rounded.md}"
  card:
    backgroundColor: "{colors.surface}"
    rounded: "{rounded.xl}"
    padding: "{spacing.card}"
  input:
    backgroundColor: transparent
    rounded: "{rounded.md}"
    height: 36px
    padding: 12px
  badge:
    rounded: "{rounded.md}"
    typography: "{typography.caption}"
  badge-source:
    backgroundColor: "{colors.surface-muted}"
    textColor: "{colors.ink-soft}"
    typography: "{typography.mono}"
    rounded: "{rounded.md}"
  table-header:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.ink-soft}"
    typography: "{typography.label}"
  avatar-entity:
    rounded: "{rounded.xl}"
    textColor: "{colors.surface}"
---

# Lunogram Console

## Overview

The Lunogram Console is the operator surface of an automated messaging and
customer-engagement platform — the place where a growth engineer composes a
campaign, inspects a user's event stream, wires up a journey, or rotates an API
key. The right reference is **the dashboard of a modern developer-infrastructure
product — Stripe Dashboard, Vercel, Linear, Clerk.** It is a working console,
not a marketing site. Its job is to make dense operational data legible at a
glance and to get out of the way.

The personality is **calm, precise, and quietly technical.** It is light-first,
near-monochrome, and structured. Hierarchy comes from typographic weight,
generous whitespace, and hairline borders — not from color or shadow. Color is
rationed: the interface is built almost entirely from ink-on-white neutrals, and
a saturated hue appears only when it carries meaning (a status, a destructive
action, a brand color of an integrated provider).

The product talks to developers, so it never hides the machine. Identifiers,
event payloads, attribute keys, and locale codes are shown verbatim in
monospace. The console respects that its user knows what a UUID is and wants to
copy it, not have it summarized away.

## Colors

A near-monochrome system: one ink, a short ladder of neutral surfaces, and a
five-hue status palette held in reserve.

- **Ink** {colors.primary} is an ink-navy black, never pure `#000`. It carries all
  primary type, all icons, the primary button fill, and the active sidebar
  avatar. Its desaturated navy cast is what keeps the interface feeling
  considered rather than stark.
- **Ink Soft** {colors.ink-soft} is a warm slate for secondary text: metadata,
  descriptions, table-header labels, placeholder text, and muted timestamps.
- **Surface** {colors.surface} is pure white — the canvas for cards, tables,
  inputs, and the content column.
- **Surface Soft** {colors.surface-soft} and **Surface Muted**
  {colors.surface-muted} are the two off-whites used for page background wash,
  hover states, secondary buttons, and "filled" zones like the inline-edit
  hover and the promo cards. Depth is built by stepping between these tones, not
  by stacking shadows.
- **Border** {colors.border} is the default hairline; **Border Strong**
  {colors.border-strong} is reserved for inputs and emphasized dividers.
- **Status hues** — **Red** {colors.red} (errors, destructive, opt-outs),
  **Blue** {colors.blue} (informational, progress, links in context),
  **Amber** {colors.amber-hard} (warnings, pending), **Green** {colors.green-hard}
  (success, delivered, active), and **Purple** {colors.purple} (the Lunogram
  brand accent, used sparingly for emphasis and the product mark). Each hue has
  a **soft** tint for backgrounds/badges and a **hard** shade for text/icons on
  light fills.
- **Surface Editor** {colors.surface-editor} is the one deliberately dark zone:
  the code/template editor, which is always dark regardless of theme so that
  syntax highlighting reads correctly.

Generated entity avatars (users, lists, providers) derive a **deterministic**
color from the entity's name — hashed to a hue, then desaturated 30% and
darkened 10%. The desaturation is intentional: it keeps a wall of avatars from
turning into a candy box and holds them inside the muted, professional register.

A **dark theme** is fully defined (see [Dark Mode](#dark-mode)) but not yet
shipped in the product; it inverts ink and surface while keeping the same status
hues.

## Dark Mode

A dark theme is **defined but not yet shipped** — there is no theme toggle in the
product today. It is specified now so new components are built theme-aware from
the start (referencing semantic tokens, never literal `#fff`/`#000`), making the
eventual switch a token swap rather than a rewrite. The dark values live under
`colors-dark` and list only what changes; status base hues, radius, type, and
spacing carry over from the light palette.

Dark mode is an **inversion, not a second design.** The same calm,
near-monochrome, border-led character holds — only the polarity flips.

- **Ink and surface swap.** White (`#FFFFFF`) becomes the type and icon color; a
  dark ink-navy (`#121721`) becomes the canvas. The "never pure black" rule still
  holds — the dark canvas is navy-tinged, not `#000`.
- **The surface ladder inverts.** In light mode raised surfaces get *whiter*; in
  dark mode they get *lighter than the canvas* — canvas `#121721`, then
  `#1A1F2B`, then `#252B3A`. A card lifts by becoming a step lighter, not by
  casting a shadow.
- **Borders soften** to a low-contrast `#2B3245`, with `#3A4358` for the
  emphasized input border.
- **Status hues keep their base values** ({colors.red}, {colors.blue}, and the
  rest read fine on dark) but their **soft/hard roles invert**: the soft tint
  deepens into the badge *background* and the hard shade lightens into the badge
  *text*. Purple is the worked example already in the CSS — soft `#4E0C77` (deep)
  as the fill, hard `#D1A7EA` (light) as the text.
- **The editor is unchanged** (`#0D121E`); already dark in both themes, in dark
  mode it nearly merges with the canvas, set apart only by a hairline.
- **Ambient map and icon mosaics** stay just as low-contrast — faint light marks
  against the dark canvas instead of faint dark ones.

## Typography

One typeface does almost everything: **Inter**, at a 15px base. Inter is the
quiet, neutral grotesque of the developer-tools world — it carries the
"infrastructure dashboard" reference without any styling. Technical strings are
the only exception and are always set in a **monospace** system stack.

- **Display** and **Title** are Inter Semi-Bold with tight tracking
  (`-0.02em`), used for page heroes ("Users", "Campaigns") and the name of the
  entity on a detail page. Size differences are modest — a page title is roughly
  2× body, never a billboard.
- **Heading** {typography.heading} marks section headers inside a page
  ("Identifiers", "Project Details", "Custom attributes"), Semi-Bold with a
  faint negative tracking.
- **Body** {typography.body} is Inter Regular at 15px — the reading size for
  most content. **Body Small** drops to 14px for table cells and dense regions.
- **Label** {typography.label} is Inter Medium at 14px for form field labels and
  table-header text.
- **Caption** {typography.caption} is 12px, almost always in Ink Soft, for
  metadata, helper text under fields, and badge text.
- **Mono** {typography.mono} is the monospace voice for everything the machine
  owns: user IDs, external identifiers, attribute keys, source tags, locale
  codes, and code/template content. If a value is something a developer would
  paste into an API call, it is monospace.

Headings lean on weight and tracking for hierarchy, not size jumps. Two weights
on screen at once is the norm — Regular for prose, Semi-Bold/Medium for
structure.

## Layout

A classic **fixed-sidebar console**. A persistent left navigation rail
(~{spacing.sidebar}) holds the project switcher at top, a grouped navigation
list in the middle, and the API-docs card plus the account avatar pinned to the
bottom. The main content column scrolls independently with **40px** of
horizontal page padding (20px on mobile).

Spacing follows an **8px rhythm** with a recurring **10px** gap (the dominant
flex gap between controls) and **24px** of internal padding inside cards. Pages
are built as a vertical stack: a page header, then sections separated by
whitespace and the occasional hairline rule — not boxed-in panels everywhere.

The signature page header is an **icon-in-a-rounded-square** (a muted
{rounded.xl} tile holding a Ink-Soft glyph) set beside a Display/Title heading
and a one-line Ink-Soft description, with primary actions right-aligned on the
same baseline. Sub-navigation within a section uses an underline tab row
directly beneath the header.

Content stays comfortably measured; tables and forms breathe rather than packing
edge to edge.

## Elevation & Depth

Depth is **near-flat and border-led.** The default way one surface separates
from another is a single hairline {colors.border} and, at most, a whisper of
shadow ({elevation.sm}). Cards add a slightly stronger but still subtle drop
({elevation.card}); modals and popovers lift a touch more. There are no heavy
shadows, no glows, no glassmorphism.

When more separation is needed, the system reaches for a **tonal step** —
placing content on Surface Muted instead of Surface — before it reaches for a
shadow. The promo/"automate via API" cards are the clearest example: a muted
fill with a hairline border, no elevation at all. This tonal-first approach is
what lets the system carry over to dark mode, where shadows all but vanish and
separation comes entirely from stepping the surface lighter plus the hairline.

## Shapes

Soft, consistent rounding with a clear ladder. Buttons, inputs, badges, and
small controls use {rounded.md}; cards and the entity-avatar / page-header tiles
step up to {rounded.xl}; the very small radius {rounded.sm} is for nested or
compact controls. Account and navigation avatars are {rounded.full} circles;
**entity avatars** (users, lists, providers) are deliberately
{rounded.xl} **squares**, which is how you tell a "thing in the system" apart
from a "person using the system" at a glance.

Corners are never sharp. The radius is generous enough to read as modern and
friendly, restrained enough to stay professional.

## Iconography & Imagery

- **Icons** are line icons (Lucide), drawn at `1.5px`-weight equivalent, sized
  16px inline and 28px inside header tiles, tinted Ink or Ink-Soft. FontAwesome
  solid glyphs appear in a few legacy spots; new work uses Lucide.
- **Ambient world map.** Detail headers (e.g. a user page) carry a faint,
  desaturated world-map watermark bleeding out of the top-right. It nods to the
  global, geographic nature of a messaging platform without ever competing with
  content. It is decoration, not data — always low-contrast and behind the type.
- **Decorative icon mosaics.** "Automate via API" promo cards render a staggered
  grid of low-opacity rounded tiles that fades out under a radial mask and
  bleeds off the card's right edge, with the relevant brand/provider icon
  centered and softly glowing. Used only on educational/CTA cards, never on
  operational surfaces.

## Motion

Motion is functional and brief — feedback, not choreography.

- Interactive feedback (hover, press, toggle): {motion.feedback},
  {motion.easing}. Buttons and rows transition color, not size.
- Content transitions (popover, dialog, tab change): {motion.content}, same
  curve.
- Nothing bounces, overshoots, or lingers past ~250ms. Loading uses a quiet
  spinner or skeleton blocks, never a flashy animation.

## Components

- **Buttons.** Primary is a solid **Ink** fill with white text, {rounded.md},
  36px tall, with a leading `+`/action icon and a soft shadow; hover darkens the
  fill ~10%. **Outline** and **Secondary** sit on white/muted with a hairline.
  **Ghost** is transparent until hovered (muted fill). **Destructive** uses
  Red. **Link** is underline-on-hover. Disabled drops to 50% opacity.
- **Inputs.** {rounded.md}, 36px tall, transparent fill with a Border-Strong
  hairline, Ink-Soft placeholder, and a `1px` focus ring. Search inputs carry a
  leading magnifier icon.
- **Cards.** {rounded.xl}, white fill, hairline border, faint shadow, 24px
  padding. Used for grouped data and tables; promo cards drop the shadow and use
  a muted fill.
- **Badges / tags.** {rounded.md}, 12px text. **Source/identifier tags** are a
  distinctive pattern: monospace text on a muted fill (e.g. the `admin` source
  pill). Status badges use the soft-tint background with the hard-shade text of
  their hue.
- **Tables.** Hairline-separated rows, an Ink-Soft Label header row on white, a
  generous row height, and a footer line summarizing count ("1 user"). Leading
  cells often pair a square entity avatar with the primary identifier.
- **Tabs.** A horizontal row with an icon + label per tab and an underline under
  the active item; inactive tabs are Ink-Soft.
- **Inline edit.** Editable metadata (email, phone, locale on a detail page) is
  shown as plain text that reveals a {rounded.md} muted hover background and an
  edit affordance — editing happens in place, not in a separate form.
- **Empty states.** Centered within the would-be table: a muted glyph, a short
  Ink-Soft line ("No campaigns yet"), nothing more.

## Do's and Don'ts

- **Do** treat color as information. A saturated hue should mean something — a
  status, a destructive action, a provider's brand. Default to neutrals.
- **Don't** introduce a new accent color for decoration. The palette is ink,
  neutrals, and five reserved status hues.
- **Do** set every machine-owned string — IDs, keys, codes, payloads — in
  monospace, and make it copyable.
- **Don't** summarize or truncate identifiers away. Developers want the real
  value.
- **Do** build hierarchy from weight, whitespace, and hairlines.
- **Don't** reach for heavy shadows, glows, gradients, or glass. Step a tone
  (Surface → Surface Muted) before you reach for elevation.
- **Do** keep the entity-avatar convention: **square** {rounded.xl} tiles for
  things in the system, **circular** avatars for people.
- **Don't** let the ambient map or icon mosaics rise in contrast. They live
  behind content and must never compete with it.
- **Do** keep size jumps modest — a page title is ~2× body, not 5×.
- **Don't** use more than two type weights in a single view.
- **Do** keep the code/template editor dark in every theme; it is the one
  intentional dark surface.
- **Don't** use pure black {colors.primary} as `#000000` or pure-white borders.
  Ink is navy-tinged; borders are soft grey.
- **Do** build new components against semantic tokens (`surface`, `primary`,
  `border`, `ink-soft`) even though dark mode isn't shipped — a hardcoded
  `#fff`/`#000` is a future dark-mode bug.
- **Don't** carry light-mode drop shadows into dark or darken raised surfaces in
  dark; in dark the surface ladder runs the other way (raised = lighter) and
  separation is tonal plus hairline.
