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
export function isValidRootRedirectURL(value: string): boolean {
  if (value === '') return true
  if (value !== value.trim() || !/^https?:\/\//i.test(value)) {
    return false
  }

  try {
    const url = new URL(value)
    return (
      (url.protocol === 'https:' || url.protocol === 'http:') &&
      url.hostname.length > 0 &&
      url.username === '' &&
      url.password === ''
    )
  } catch {
    return false
  }
}

/** Absolute destination when root redirect is enabled; otherwise null. */
export function resolveRootRedirectHref(
  enabled: unknown,
  url: unknown
): string | null {
  const isEnabled = enabled === true || enabled === 'true'
  if (!isEnabled || typeof url !== 'string' || url === '') {
    return null
  }
  if (!isValidRootRedirectURL(url)) {
    return null
  }
  return url
}
