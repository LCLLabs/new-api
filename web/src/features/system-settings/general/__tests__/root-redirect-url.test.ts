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
import { describe, expect, it } from 'vitest'

import {
  isValidRootRedirectURL,
  resolveRootRedirectHref,
} from '../root-redirect-url'

describe('resolveRootRedirectHref', () => {
  it('returns the absolute URL when redirect is enabled', () => {
    expect(
      resolveRootRedirectHref(true, 'https://basamp.com/')
    ).toBe('https://basamp.com/')
    expect(
      resolveRootRedirectHref('true', 'https://basamp.com')
    ).toBe('https://basamp.com')
  })

  it('returns null when disabled, empty, or invalid', () => {
    expect(resolveRootRedirectHref(false, 'https://basamp.com/')).toBeNull()
    expect(resolveRootRedirectHref(true, '')).toBeNull()
    expect(resolveRootRedirectHref(true, '/relative')).toBeNull()
    expect(resolveRootRedirectHref(true, 'javascript:alert(1)')).toBeNull()
    expect(isValidRootRedirectURL('')).toBe(true)
  })
})
