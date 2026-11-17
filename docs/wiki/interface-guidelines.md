# Interface guidelines

These rules preserve useful interaction decisions from the original design work.
For application behavior, use [usage and monitoring](usage.md); for logos, fonts,
artwork, and animation assets, use the [brand guide](../brand/README.md).

## Inventory and device details

- Keep desktop rows compact and use stacked entries on smaller screens. Separate
  IP and room information; group status icons without reserving empty slots.
- Preserve filters, room grouping, expanded inputs, keyboard focus, and scroll
  through refreshes and edits. Keep device-name links usable with native browser
  actions, even when the row itself is clickable.
- Show readings once in the section that owns them. Keep device identity,
  channels/physical inputs, connection facts, configuration comparison/history,
  firmware, and notes easy to find. Use native anchors for section and note links.
- Reuse input-to-relay interpretations across inventory and details. An explicit
  mapping matters; an unknown mapping must not silently become relay 0. Detached
  mode is valid and is not itself a fault.
- Keep power, voltage, current, and energy out of the current UI. Put temperature,
  uptime, firmware, MAC, and connection details in device details or tooltips.
  Format uptime as a duration and preserve valid zero readings.
- Make unavailable readings, stale values, and settings-read times explicit.
  Do not present a cached connection as live. Hide healthy restart flags; show a
  restart warning only for a current reported requirement. Use text or accessible
  labels alongside color, including on firmware and connectivity icons.

## Editing rooms, tags, and notes

Use the same room/tag editor in inventory and details. Suggest values from the
whole inventory, including devices hidden by filters. Rooms can be searched,
selected, created, or cleared. Tags support removable chips, suggestions, inline
creation, and comma/newline paste while preserving multiword names.

Save includes unfinished tag text; Cancel or Escape discards it. Failed saves
retain drafts. Pause polling during editing and preserve device/channel-note
drafts through status refreshes, captures, and baseline approval.

Normalize whitespace, discard empty/duplicate tags, and reuse existing spelling
case-insensitively. Room names allow 100 characters; selected tags allow 500 total,
including their comma-space separators. Validate updates atomically. Keep
name-based room/tag colors consistent between labels and suggestions.

Tag filters apply immediately and match any selected tag. Keep their summary
compact rather than expanding the toolbar as more tags are selected.

## Accessibility and responsive behavior

Maintain visible keyboard focus, skip navigation, explicit form labels, CSRF
protection, and native form submission where supported. Comboboxes need arrow-key,
Enter, and Escape behavior plus an announced active option. Tooltips must work
with keyboard focus and touch as well as hover, and close with Escape.

Let long names, identities, notes, and values wrap. Give comparison tables their
own horizontal scroll container rather than overflowing the whole page. Check
narrow phone layouts, keyboard navigation, and touch targets with representative
empty, stale, failed, and populated states.

Keep login focused on labeled fields and the primary action. Preserve password
manager autocomplete and native sign-in when JavaScript is unavailable; password
visibility is progressive enhancement. Respect reduced motion and retain static
artwork when animation cannot run. Keep runtime fonts and artwork local.
