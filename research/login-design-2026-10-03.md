# Login page

Research and implementation, 3 October 2026.

## References

- [Mailchimp](https://login.mailchimp.com/): a recognizable mascot mark, a
  bounded form, persistent labels, and one obvious primary action. Its current
  page was inspected in a browser; Shellyficator uses the same focused hierarchy
  with its own larger character artwork.
- [TunnelBear](https://www.tunnelbear.com/account/login/): direct Email and
  Password fields, a prominent colored action, and playful copy throughout the
  brand. Its current browser-rendered login was inspected. Use the personality
  without adding marketing navigation to a local device-management app.
- [Tailscale](https://login.tailscale.com/start): recognizable identity,
  restrained typography, generous space, and a focused authentication column.
  Its current identity-provider page was inspected in a browser. Borrow the
  clear hierarchy; Shellyficator continues to use its existing local account.
- [Discord product illustrations](https://www.kevinvqdam.com/discordproduct):
  the product illustrator documents using Wumpus across product surfaces to
  enrich the experience. This supports a character-led entrance; it is not a
  claim about Discord's current login layout.

Linear, Notion, and Discord's live login routes were also attempted, but their
application content did not finish rendering in the research browser. Their
unobserved layouts were excluded from the design findings.

## Direction

Keep the approved relay robot, Archivo 900 wordmark, upright 9000 numerals,
Plex body type, dark teal, lime, warm cream, and the full tagline. A split panel
gives the complete robot room to stand beneath “Keeping your Shellys in line.
Allegedly.” A quiet circuit backdrop and “Probably in charge.” caption carry
the playful identity. The sign-in panel has a clear heading, labeled fields,
an optional password visibility control, and a prominent lime action.

Below 700px, the hero becomes a compact strip with the complete robot beside
the tagline, followed by the form. The narrowest layout keeps 16px inputs,
a 44px password control, and a 52px submit button. At the user's request,
the page header and footer, both eyebrow captions, the form's lock ornament,
and “Your devices. Your rules.” are omitted. The mascot and form panel center
in the viewport with breathing room around them. No perpetual or introductory
motion is needed.

The login template remains a native POST with its existing CSRF token and
password-manager autocomplete. Password visibility is progressive enhancement:
the control is hidden until JavaScript initializes it, and ordinary sign-in
works with JavaScript disabled. Errors retain their alert semantics. All
runtime fonts and artwork are served locally; the full mascot is an unchanged
copy of the transparent brand master in the embedded static directory.

The setup page keeps its existing layout. Authentication and account behavior
are unchanged.

## Verification

- Browser layouts checked at 320, 360, 390, 640, 699, 700, 768, 900, 1024, 1280,
  1440, and 1920px, including short viewports. No horizontal overflow; the full
  mascot loads and form controls stay inside the viewport.
- Native required-field validation, keyboard tab order and focus rings,
  mouse/keyboard/touch password visibility, failed credentials, successful
  sign-in and sign-out, and sign-in without JavaScript passed. Interaction checks
  used an isolated database, with no changes to live accounts or inventory.
- Reduced-motion mode has no mascot animation or transition. No browser errors.
- `make test` (Go tests and vet), `make build`, template formatting, and
  `git diff --check` passed.

The canvas contains the original layout and the implemented redesign as
revertible versions. Research screenshots and browser evidence are under ignored
`test-results/login-design/`; design context is under ignored `.superdesign/`.
