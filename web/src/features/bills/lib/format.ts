/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { toIntlLocale } from '@/i18n/languages'
import { formatCompactNumber, formatLogQuota } from '@/lib/format'

export function billLocale(language: string | undefined) {
  return toIntlLocale(language)
}

export function formatBillTokens(
  value: number,
  locale: Intl.LocalesArgument
): string {
  return formatCompactNumber(value, locale)
}

export function formatBillQuota(value: number | undefined): string {
  if (value == null) return '—'
  return formatLogQuota(value)
}

export function formatBillDiscount(
  value: number | undefined,
  locale: Intl.LocalesArgument,
  dynamic: boolean | undefined,
  t: (key: string, options?: Record<string, unknown>) => string
): string {
  if (value == null) return '—'
  const ratio = new Intl.NumberFormat(locale, {
    maximumFractionDigits: 6,
  }).format(value)
  if (dynamic) return t('Dynamic billing × {{ratio}}', { ratio })
  return ratio
}
