import type { TFunction } from "i18next"
import countryJson from "flag-icons/country.json"
import { getCountryName } from "@/lib/countryNames"

interface FlagCountry {
  code: string
  name: string
  iso: boolean
}

const COUNTRIES: FlagCountry[] = (countryJson as FlagCountry[]).filter(
  c => c.iso && c.code && /^[a-z]{2}$/.test(c.code)
)

/**
 * Returns the full list of ISO countries as options for a select/combobox.
 * `code` is the lowercase ISO 3166-1 alpha-2 used by flag-icons.
 */
export function getCountryOptions(t?: TFunction): { code: string; label: string }[] {
  return COUNTRIES.map(c => ({
    code: c.code,
    label: getCountryName(c.code.toUpperCase(), c.name, t),
  })).sort((a, b) => a.label.localeCompare(b.label))
}

/** Renders a flag for a country code (lowercase ISO alpha-2). Returns null when empty. */
export function flagClass(countryCode?: string): string | null {
  if (!countryCode || !/^[a-z]{2}$/.test(countryCode)) return null
  return `fi fi-${countryCode}`
}
