import { writable, derived } from 'svelte/store'
import en from './en.js'
import ru from './ru.js'
import zh from './zh.js'

const locales = { en, ru, zh }

function detectLocale() {
  const saved = localStorage.getItem('entangled_locale')
  if (saved && locales[saved]) return saved
  const lang = navigator.language?.slice(0, 2)
  if (lang === 'ru') return 'ru'
  if (lang === 'zh') return 'zh'
  return 'en'
}

const initialLang = detectLocale()
export const currentLang = writable(initialLang)
if (typeof document !== 'undefined') {
  document.documentElement.lang = initialLang
}

export const t = derived(currentLang, $lang => {
  const loc = locales[$lang] || en
  return new Proxy(loc, {
    get(target, prop) {
      if (typeof prop !== 'string') return undefined
      return target[prop] ?? en[prop] ?? prop
    },
  })
})

export function setLang(lang) {
  if (!locales[lang]) return
  localStorage.setItem('entangled_locale', lang)
  currentLang.set(lang)
  if (typeof document !== 'undefined') {
    document.documentElement.lang = lang
  }
}

export function fmt(str, vars) {
  if (!str) return ''
  return String(str).replace(/\{(\w+)\}/g, (_, k) => (vars && vars[k] != null ? vars[k] : `{${k}}`))
}
