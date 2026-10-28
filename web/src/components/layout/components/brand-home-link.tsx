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
import { Link } from '@tanstack/react-router'
import type { ComponentProps, ReactNode } from 'react'

import { useRootRedirectHref } from '@/hooks/use-root-redirect-href'

type BrandHomeLinkProps = {
  children: ReactNode
  className?: string
  'aria-label'?: string
  title?: string
  onClick?: ComponentProps<'a'>['onClick']
  /**
   * When the caller already chose a non-root destination (e.g. public header
   * `homeUrl`), keep that target and skip the root-redirect override.
   */
  to?: string
}

export function BrandHomeLink({
  children,
  className,
  to = '/',
  ...props
}: BrandHomeLinkProps) {
  const rootRedirectHref = useRootRedirectHref()
  const externalHref = to === '/' ? rootRedirectHref : null

  if (externalHref) {
    return (
      <a href={externalHref} className={className} {...props}>
        {children}
      </a>
    )
  }

  if (to.startsWith('http://') || to.startsWith('https://')) {
    return (
      <a href={to} className={className} {...props}>
        {children}
      </a>
    )
  }

  return (
    <Link
      to={to as ComponentProps<typeof Link>['to']}
      className={className}
      {...props}
    >
      {children}
    </Link>
  )
}
