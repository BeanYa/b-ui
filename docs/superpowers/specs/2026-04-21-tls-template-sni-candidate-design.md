# TLS Template And SNI Candidate Design

## Goal

Correct the built-in TLS-related templates and the SNI domain candidate picker so they match the intended defaults and selection behavior.

The change must make template-generated TLS settings predictable, keep only the requested options enabled by default, and ensure domain candidate diagnostics are displayed as non-editable reference labels rather than becoming part of the selected domain value.

## Non-Goals

- Changing backend TLS probe logic or domain hint APIs
- Changing unrelated TLS fields such as certificate generation, ECH, uTLS, or ACME behavior beyond template defaults already requested here
- Removing the ability to manually type a custom domain into the SNI candidate input
- Redesigning the modal outside the specific candidate-label rendering needed for this fix

## Current-State Findings

The current implementation spreads this behavior across two frontend files:

- `src/frontend/src/plugins/tlsTemplates.ts` defines the built-in `standard`, `hysteria2`, and `reality` presets
- `src/frontend/src/layouts/modals/Tls.vue` renders the TLS editor, preset application behavior, advanced-option toggles, and domain candidate comboboxes

The current defaults do not match the requested behavior:

1. The `standard` TLS preset enables `server_name`, `alpn`, `min_version`, and `max_version` by default.
2. The `hysteria2` preset enables `server_name`, `alpn`, `min_version`, and `max_version` by default.
3. Neither `standard` nor `hysteria2` explicitly enables `client.insecure`.
4. The `reality` preset sets `server.reality.max_time_difference` to `1m`, which means the option is on by default.
5. The SNI domain candidate combobox maps each item to a single concatenated `title` string such as `example.com · Recommended · TLS 1.3 · H2 · 8ms`, so the display text and the saved value are too tightly coupled.

That last point causes the UX problem the user described: the domain diagnostics look like editable input content instead of fixed metadata attached to the selected domain.

## Requirements

### Functional Requirements

1. The standard TLS preset must default to only `SNI` and `ALPN` being enabled on the server side.
2. The standard TLS preset must not pre-enable `min_version` or `max_version`.
3. The hysteria2 preset must default to only `SNI` being enabled on the server side.
4. The hysteria2 preset must not pre-enable `ALPN`, `min_version`, or `max_version`.
5. Both the standard TLS preset and the hysteria2 preset must set `client.insecure` to enabled by default.
6. The reality preset must leave `max_time_difference` disabled by default.
7. Selecting an SNI candidate must save only the domain string.
8. The associated diagnostics must remain visible only as display labels.
9. Users must still be able to type a custom domain that is not in the candidate list.

### UI Requirements

1. Candidate items must show the domain as the primary value.
2. Candidate diagnostics must be rendered as non-editable labels or badges, for example `Recommended`, `TLS 1.3`, `H2`, `8ms`.
3. After selection, the field must display only the domain string.
4. The same rendering behavior must apply to both TLS `server_name` and Reality `handshake.server` candidate pickers.

## Design

### 1. Template Defaults

Update `src/frontend/src/plugins/tlsTemplates.ts` so each preset expresses only the requested enabled options.

#### Standard Preset

The `standard` preset should include:

- `server.enabled: true`
- `server.server_name: ''`
- `server.alpn: ['h2', 'http/1.1']`
- certificate path fields as they exist today
- `client.insecure: true`

The `standard` preset should not include:

- `server.min_version`
- `server.max_version`

This means the corresponding advanced-option toggles remain off until the user explicitly enables them.

#### Hysteria2 Preset

The `hysteria2` preset should include:

- `server.enabled: true`
- `server.server_name: ''`
- certificate path fields as they exist today
- `client.insecure: true`

The `hysteria2` preset should not include:

- `server.alpn`
- `server.min_version`
- `server.max_version`

This keeps the preset aligned with the user's rule that only SNI should be opened by default for hysteria2.

#### Reality Preset

The `reality` preset should continue enabling Reality-specific fields and uTLS as it does now, but it must no longer write `server.reality.max_time_difference` by default.

That means the advanced toggle for max time difference is off on first apply, and the input field remains hidden until the user enables it.

### 2. Option Toggle Behavior

No broad rewrite of the TLS option-toggle system is needed.

The current `optionSNI`, `optionALPN`, `optionMinV`, `optionMaxV`, and `optionTime` computed properties already derive their state from whether the underlying fields are present. The change should therefore be limited to preset payloads and not to the toggle model itself.

Expected results after applying presets:

1. Standard preset:
only `SNI` and `ALPN` are on, `min/max version` are off, `insecure` is on.

2. Hysteria2 preset:
only `SNI` is on, `ALPN` and `min/max version` are off, `insecure` is on.

3. Reality preset:
`Max Time Difference` is off by default.

### 3. Domain Candidate Data Model

Replace the current flattened `domainHintItems()` mapping with structured display data.

Each candidate item should expose:

- `value`: the domain string used for selection and saved configuration
- `title` or `domain`: the main domain text shown in the UI
- `metaLabels`: an ordered array of visual labels derived from probe results

Example shape:

```ts
{
  value: 'example.com',
  domain: 'example.com',
  metaLabels: ['Recommended', 'TLS 1.3', 'H2', '8ms'],
}
```

Rules for labels:

- include status text first
- then TLS version when present
- then ALPN when present, uppercased as the UI already does
- then redirect marker when present
- then latency when present

The important separation is that `value` remains the pure domain string while `metaLabels` are presentation-only.

### 4. Domain Candidate Rendering

Keep `v-combobox` rather than replacing it with `v-select`.

This preserves custom-domain entry while allowing custom rendering through slots.

The combobox should be updated so that:

1. The dropdown list item shows the domain as the main label.
2. The diagnostics render alongside it as compact visual labels, not as editable text.
3. The selected field display shows only the domain string.
4. The bound model value remains a plain `string`.

In practice, this means using structured items plus `item` and selection/display slots rather than building one combined title string.

### 5. Scope Of UI Change

Only the two candidate pickers in `src/frontend/src/layouts/modals/Tls.vue` should change:

- `inTls.server_name`
- `inTls.reality.handshake.server`

No change is needed to the domain hint refresh behavior or backend request format.

## Error Handling And Edge Cases

1. If a domain hint item has only a status and no other probe data, the UI should still render the domain and the single label.
2. If a domain hint item has no labels at all, the UI should render only the domain.
3. If the user types a custom domain not present in the list, the field must preserve that exact typed domain.
4. If the domain hint API returns an empty list, both candidate fields must continue working as free-form inputs.
5. If a candidate is selected and later the hint list refreshes, the existing selected domain should remain a plain string value.

## File Scope

Files expected to change:

- `src/frontend/src/plugins/tlsTemplates.ts`
- `src/frontend/src/layouts/modals/Tls.vue`

Files likely to be added:

- one frontend unit test file for TLS preset defaults
- one frontend unit test file for domain hint item mapping and label separation, unless both checks fit naturally into a single new test module

## Testing Strategy

### Automated Tests

1. Add a unit test for `createTlsPreset()` covering:
- standard preset keeps `server_name` and `alpn`
- standard preset omits `min_version` and `max_version`
- standard preset sets `client.insecure` to `true`
- hysteria2 preset keeps `server_name`
- hysteria2 preset omits `alpn`, `min_version`, and `max_version`
- hysteria2 preset sets `client.insecure` to `true`
- reality preset omits `server.reality.max_time_difference`

2. Add a small unit test around the domain-hint item transformation so that:
- item `value` is the domain only
- labels are built separately from the saved value
- label ordering remains stable

### Manual Verification

1. Open the TLS modal and apply the standard preset.
Expected result: `SNI`, `ALPN`, and `Allow insecure` are on; `Min version` and `Max version` are off.

2. Apply the hysteria2 preset.
Expected result: only `SNI` and `Allow insecure` are on; `ALPN`, `Min version`, and `Max version` are off.

3. Apply the reality preset.
Expected result: `Max Time Difference` is off by default and its input is hidden.

4. Open the SNI candidate dropdown for TLS or Reality.
Expected result: each option shows the domain plus non-editable diagnostic labels.

5. Select a candidate.
Expected result: the field shows only the domain after selection.

6. Save the modal data.
Expected result: the saved TLS payload contains only the selected domain string, not any appended labels.

## Open Questions

No open questions remain for this scope.
