# ADR-005: Dynamic column widths

**Status:** Accepted
**Date:** 2026-04-29
**Applies to:** `internal/ui/dashboard.go`, `internal/app/view.go`

## Context

Grove's tab renderers originally used hardcoded column widths with a 160-character content cap. This caused two problems: on wide terminals, content was unnecessarily narrow with wasted space; on narrower terminals (or after adding the 6th Issues tab column to the dashboard), rows could overflow the terminal width and wrap.

Canopy (`~/src/apps/canopy`) solved this with a constraint-based layout: fixed columns (status icons, numbers) keep their widths, while text columns (repo names, titles, authors) share the remaining terminal width proportionally.

## Decision

Replace hardcoded column widths with a `flexCols` helper that distributes available width across flexible columns using weighted proportional allocation.

### Layout model

Each tab defines:
- **Fixed columns**: widths that never change (indent, status icons, PR numbers, review/checks indicators)
- **Flex columns**: text content that benefits from more space, each with a minimum width and a growth weight

The helper computes: `available = terminal_width - sum(fixed)`, then distributes available space across flex columns proportionally to their weights, never going below each column's minimum.

### Every column is budgeted

All columns — including trailing timestamps ("Last Commit", "When", "Updated") — are wrapped in `cell()` with a computed width and included in the flex budget. This guarantees that `sum(all column widths) == terminal_width`, preventing overflow.

### No content width cap

The previous `cw = min(width, 160)` cap is removed. Content fills the full terminal width.

## Alternatives Considered

**Keep the 160-char cap and just fix overflow** — Would still waste space on wide terminals. The cap was a workaround for not having proper column distribution.

**Per-column max caps** — Adding a `Max` field to prevent columns from growing too wide on ultra-wide terminals. Deferred as unnecessary — padding on wide terminals is acceptable, and adding caps increases complexity.

**Separator characters between columns** — Canopy uses explicit `" "` separators. Grove relies on the 2-char padding built into each `cell()` call (truncate to `width-2`, pad to `width`), which achieves the same visual separation without extra bookkeeping.

## Consequences

- Rows always fit within the terminal width regardless of terminal size.
- On narrow terminals (< sum of minimums + fixed), columns use their minimums and may overflow — same as before, but now only at truly narrow widths.
- Adding or removing a column requires updating the fixed width constant and the `colSpec` slice in the corresponding `Render*` function.
- The `flexCols` helper is reused across all 6 tabs, keeping the layout logic consistent and DRY.
