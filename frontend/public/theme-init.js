// Runs synchronously before paint, so the correct theme is applied with
// zero flash. Default is dark — unlike 1st-Pouf's own suggested script,
// this deliberately ignores prefers-color-scheme: dark is the default
// regardless of the visitor's OS setting, per this project's design
// choice (see design-system spec, "Theming").
//
// Lives as a same-origin file rather than inline in index.html so it's
// covered by the CSP's default-src 'self' without needing 'unsafe-inline'
// or a script hash — see PLANNING.md's Security TODO / the theme-reset
// bug this fixed (inline scripts were silently blocked by CSP in
// production, so the stored preference never applied on reload).
;(function () {
  try {
    var stored = localStorage.getItem('theme')
    document.documentElement.classList.toggle('dark', stored !== 'light')
  } catch {
    // localStorage can throw in private/restricted contexts — fall back
    // to the default (dark) rather than losing the page.
    document.documentElement.classList.add('dark')
  }
})()
