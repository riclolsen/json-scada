import { createI18n } from 'vue-i18n'
import { getUserSetting } from './lib/viewerHelpers'

const loadLocaleMessages = async () => {
  const locales = import.meta.glob('./locales/*.json')
  const messages = {}
  for (const path in locales) {
    const matched = path.match(/([A-Za-z0-9-_]+)\./i)
    if (matched && matched.length > 1) {
      const locale = matched[1]
      messages[locale] = await locales[path]()
    }
  }
  return messages
}

export const STORAGE_KEY = 'user-locale'

// per user (the signed-in user's own choice, else this browser's last one)
export const getStoredLocale = () => getUserSetting(STORAGE_KEY) || 'en'

export const setupI18n = async () => {
  const messages = await loadLocaleMessages()

  return createI18n({
    legacy: false,
    locale: getStoredLocale(),
    fallbackLocale: 'en',
    messages,
  })
}
