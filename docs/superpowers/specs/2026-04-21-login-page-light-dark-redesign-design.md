# Login Page Light/Dark Redesign Design

## Goal

Redesign the login page so it works as a cohesive, polished entry screen in both light and dark themes.
The new screen must replace the current split marketing-style layout with a single centered login window that follows `DESIGN.md` and reads like a desktop tool rather than a generic admin template.

## Non-Goals

- Changing login behavior, authentication API shape, or routing flow
- Redesigning unrelated screens
- Introducing new branding copy blocks or marketing content on the login screen
- Creating separate light and dark layouts

## Requirements

### Functional Requirements

1. The login page must use the same structure in both light and dark themes.
2. The layout must remove the current left-side brand panel and use a single centered card/window.
3. Language switching and theme switching must remain available on the login page.
4. The login form must continue to use the existing validation and submit behavior.
5. The page must remain usable on desktop and mobile without layout jumps between themes.

### Visual Requirements

1. The page must feel minimal first, with strong hierarchy and no large decorative brand area.
2. The centered container must read as a compact desktop utility window.
3. Light theme must feel intentionally designed, not like a dark-theme layout with colors inverted.
4. Dark theme must preserve the near-black, desktop-tool feel described in `DESIGN.md`.
5. Interactive emphasis must come primarily from the blue accent/focus treatment, while red remains punctuation or error-only.

## Current-State Findings

The current login page in `src/frontend/src/views/Login.vue` is optimized around a two-column composition:

- the left side carries a large badge, title, supporting copy, and tags
- the right side contains the login form inside a heavy inner card
- the overall panel relies on broad tinted overlays and glow treatment

This structure works only for the darker presentation. In light theme it produces the wrong visual hierarchy:

- the left-side panel competes with the form even though login is the primary task
- the tinted overlays and translucent dark surfaces become muddy in light mode
- controls for language and theme feel detached from the card structure because they live below the form rather than in a window header
- the page reads more like a hero banner plus form than a focused product login screen

## Design

### 1. Single-Window Layout

Replace the split panel with one centered login window.

The page structure becomes:

1. page background
2. centered window container
3. thin window header
4. title block
5. login form

This keeps the visual focus on signing in and removes the need for a separate branding column.

### 2. Window Header

The container should begin with a narrow header row that makes the card feel like a desktop utility window.

Header contents:

- left: product logo plus `B-UI`
- right: language selector and theme control

The header is structural, not promotional. It should feel like a toolbar rather than a navigation bar.

### 3. Title Block

Below the header, keep a compact title block:

- title: existing localized login title
- subtitle: one short functional sentence describing access to the control surface

Do not retain the current `Home · Settings` subtitle or any left-column marketing copy.

### 4. Form Composition

The body remains a short vertical flow:

- username input
- password input
- primary submit button

Spacing should be tight but breathable. The button is the only strong call to action.

Validation and submit behavior stay unchanged.

### 5. Theme Model

The light and dark themes must share the same layout and component structure. Theme switching should feel like the same window changing material, not a different page loading.

#### Light Theme

The light theme should use a cool, restrained palette:

- page background: very light cool gray, not pure white
- window surface: cleaner and slightly brighter than the page background
- text: deep blue-black for primary content, cool gray for secondary content
- borders: subtle cool gray lines
- focus state: blue ring/glow

The result should feel like a precise desktop app window on a bright work surface.

#### Dark Theme

The dark theme should follow `DESIGN.md` more directly:

- page background: near-black blue-tinted base
- window surface: one level above the page background
- text: near white and controlled mid-gray
- borders: low-opacity white plus inner containment
- focus state: blue ring/glow

Avoid the current broad muddy translucency. The screen should feel cleaner and more deliberate.

### 6. Surface And Elevation System

The window should use the `DESIGN.md` containment language:

- clear border/ring definition
- restrained outer shadow
- subtle inset highlight where appropriate
- controlled corner radius that reads like a compact window, not a floating marketing card

This is the main way the page keeps the desktop-tool character after removing the left brand column.

### 7. Control Styling

#### Inputs

Inputs should use the same shape and spacing in both themes.

- medium corner radius
- clear border and contained background
- muted icons
- strong blue focus treatment

The input design should support legibility first and decorative styling second.

#### Submit Button

The submit button is the only emphasized action.

- light theme: dark button on light surface
- dark theme: high-contrast light or bright button on dark surface
- interaction: mostly opacity, ring, and subtle elevation changes rather than noisy gradients

#### Language And Theme Controls

These controls should become part of the window header instead of a detached footer row.

They should be styled as compact utility controls that share the window's border, radius, and density language.

If the current theme switcher remains menu-based, the activator must still read as a deliberate header tool button.

### 8. Responsive Behavior

The centered window must scale cleanly without changing structure.

#### Desktop

- fixed readable max width
- generous outer whitespace
- clear floating-window presentation

#### Tablet

- reduced outer margins
- slightly tighter internal padding

#### Mobile

- near-full-width window
- slightly reduced radius and shadow strength
- same header, title, and form order

The implementation must not reintroduce split-column behavior at larger sizes.

## File Scope

Primary file expected to change:

- `src/frontend/src/views/Login.vue`

No logic changes are expected beyond what is needed to support the visual restructuring.

## Error Handling

1. Required-field validation must continue to work as it does now.
2. Loading state must not cause layout shift.
3. Any login failure messaging that already exists or is added later must fit naturally inside the new card structure.
4. Theme switching must not break contrast or control discoverability.

## Testing Strategy

### Manual Verification

1. Open the login page in light theme.
Expected result: single centered window, clear hierarchy, no leftover split layout.

2. Open the login page in dark theme.
Expected result: same structure as light theme with correct dark-material treatment.

3. Toggle between light, dark, and system theme from the login page.
Expected result: layout remains stable while materials and colors update correctly.

4. Change language from the login page.
Expected result: selector remains usable and localized labels update as before.

5. Test desktop and mobile widths.
Expected result: one-card layout remains intact and readable at all sizes.

### Build Verification

- run the frontend build to confirm the view still compiles cleanly after restructuring

## Implementation Notes

This redesign should prefer minimal code churn inside `Login.vue`:

- keep the existing login script behavior if possible
- replace the current split-panel template structure with the single-window structure
- rewrite the scoped styles around theme-aware surfaces, spacing, and header controls
- avoid introducing extra components unless the existing file becomes meaningfully harder to understand
