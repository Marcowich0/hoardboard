// Sets the theme before the first paint (see src/theme.ts): dark unless this
// browser chose light.
;(function () {
  var theme = 'dark'
  try {
    if (localStorage.getItem('hoardboard.theme') === 'light') theme = 'light'
  } catch {
    // Storage blocked: use the default dark theme.
  }
  document.documentElement.dataset.theme = theme
})()
