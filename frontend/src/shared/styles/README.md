# CSS Rules

- `tokens.css` owns raw colors and semantic design tokens.
- `base.css` owns reset, document background, and element defaults.
- `primitives.css` owns reusable app primitives such as page widths, headings, controls, actions, and soft rows.
- Vue scoped styles should only describe component-specific layout or states that cannot be expressed by primitives.
- New colors should become tokens before they appear in component CSS.
