# Room/tag editing decisions — 2026-09-30

- Use one shared editor on inventory rows and device details.
- Room: search, select, create, or clear. Tags: removable chips, suggestions, inline creation, and comma/newline paste; preserve multiword names.
- Save includes unfinished tag text. Cancel/Escape discards drafts; failures retain them.
- Suggest values from the entire inventory, including filtered-out devices.
- Trim names, remove empty/duplicate tags, and reuse existing spelling case-insensitively. Keep one room and multiple tags in the existing fields.
- Room names allow 100 characters; tags allow 500 total. Reject invalid updates atomically.
- Save from cached data, preserving notes, readings, baselines, filters, expansion, focus, and scroll. Pause polling while editing.
- Keep login/CSRF protection. Comboboxes support arrows, Enter, Escape, and `aria-activedescendant`; support touch.
- Keep desktop rows on one line; rooms are blue and tags violet.

Implemented and verified; see [status](../IMPLEMENTATION.md).

References: [Carbon combo boxes](https://carbondesignsystem.com/components/dropdown/usage/), [tags](https://carbondesignsystem.com/components/tag/usage/), [WAI-ARIA combobox](https://www.w3.org/WAI/ARIA/apg/patterns/combobox/).
