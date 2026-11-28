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
import { Save } from 'lucide-react'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'

type PolicyPartProps = {
  title: string
  description: string
  note?: string
  saveLabel: string
  isSaving?: boolean
  onSave: () => void
  children: ReactNode
}

export function PolicyPart(props: PolicyPartProps) {
  const { t } = useTranslation()
  return (
    <section className='bg-card flex flex-col gap-4 rounded-lg border p-4 sm:p-5'>
      <div className='flex flex-wrap items-start justify-between gap-3 border-b pb-4'>
        <div className='min-w-0 flex-1 space-y-1'>
          <h3 className='text-base font-semibold tracking-tight'>
            {props.title}
          </h3>
          <p className='text-muted-foreground text-sm'>{props.description}</p>
          {props.note && (
            <p className='text-muted-foreground text-sm'>{props.note}</p>
          )}
        </div>
        <Button
          type='button'
          size='sm'
          onClick={props.onSave}
          disabled={props.isSaving}
        >
          <Save data-icon='inline-start' />
          <span>{t(props.isSaving ? 'Saving...' : props.saveLabel)}</span>
        </Button>
      </div>
      {props.children}
    </section>
  )
}
