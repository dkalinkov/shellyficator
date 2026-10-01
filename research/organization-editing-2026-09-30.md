# Room and tag editing — 2026-09-30

## Current friction

Rooms and tags are plain text fields on the device detail page. Assigning an existing room requires remembering its spelling. Tags require manually maintaining a comma-separated string. Neither can be changed from the dashboard.

## Research

- [Carbon: dropdowns and combo boxes](https://carbondesignsystem.com/components/dropdown/usage/) distinguishes a single selection from multiple selections and recommends a combo box when users can select an existing value or enter their own.
- [Carbon: tags](https://carbondesignsystem.com/components/tag/usage/) describes dismissible tags as removable labels applied to items. Visible individual chips make the current selection easy to inspect and remove.
- [WAI-ARIA: combobox pattern](https://www.w3.org/WAI/ARIA/apg/patterns/combobox/) specifies editable inputs, filtered suggestions, arrow-key navigation, Enter to accept an option, Escape to dismiss suggestions, and focus remaining in the input using `aria-activedescendant`.

## Chosen flow

1. Open a compact **Room & tags** editor from a room button or tag icon in the dashboard row. Keep the table in place.
2. Room: choose a searchable existing room, enter a new name, or clear the assignment.
3. Tags: show selected tags as removable chips; search existing tags or add a new one with Enter or a visible Add option. Allow multiple selections without closing the picker. Saving also includes any unfinished tag text.
4. Save explicitly, with Cancel/Escape discarding the draft. Keep errors beside the editor and retain the draft on failure.
5. Reuse these controls on the device detail page. Suggest values from the whole inventory, including devices hidden by a dashboard filter.

This is a design choice for the small homelab inventory: reuse and creation belong in the assignment flow. A separate taxonomy administration screen is unnecessary for this slice.

## Data and interaction constraints

- One room and multiple tags per device. Keep the existing database fields; parse tags at the boundary and preserve tag names containing spaces.
- Trim names, remove empty/duplicate tags, and reuse existing spelling without regard to case. Retain existing notes and configuration when updating only room/tags.
- Keep the desktop rows on one line. Use blue for rooms and violet for tags, matching the existing table palette.
- Pause automatic table requests while the editor is open. Save from cached application data without reading or changing a Shelly device.
- Maintain CSRF/login protection and keyboard/touch operation. Verify saving, clearing, cancellation, failures, filters, refresh stability, and narrow screens against a temporary database.
