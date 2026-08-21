<script>
  import { get } from 'svelte/store'
  import { settings } from '../stores/app.js'
  import { t } from '../locales/index.js'

  export let chrome = false

  $: isDark = ($settings.theme || 'dark') !== 'light'

  async function applyTheme(next) {
    settings.update((s) => ({ ...s, theme: next }))
    try {
      const cur = get(settings)
      await window.go.main.App.SaveSettings({ ...cur, theme: next })
    } catch (_) {}
  }

  function toggle() {
    applyTheme(isDark ? 'light' : 'dark')
  }
</script>

<button
  type="button"
  class="theme-switch"
  class:is-dark={isDark}
  class:is-chrome={chrome}
  on:click={toggle}
  aria-label={isDark ? $t.theme_light : $t.theme_dark}
  title={$t.theme}
>
  <span class="theme-switch-track"></span>
  <span class="theme-switch-icon theme-switch-sun" aria-hidden="true">
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
      <circle cx="12" cy="12" r="4" />
      <path d="M12 3v1.5M12 19.5V21M4.93 4.93l1.06 1.06M18.01 18.01l1.06 1.06M3 12h1.5M19.5 12H21M4.93 19.07l1.06-1.06M18.01 5.99l1.06-1.06" />
    </svg>
  </span>
  <span class="theme-switch-icon theme-switch-moon" aria-hidden="true">
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
      <path d="M16.5 13.5A7 7 0 0 1 10.5 5a7 7 0 1 0 6 8.5Z" />
    </svg>
  </span>
  <span class="theme-switch-thumb"></span>
</button>
