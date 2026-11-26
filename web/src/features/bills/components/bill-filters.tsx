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
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import type { UserBill } from '../types'

const ALL = 'all'

function FilterSelect(props: {
  label: string
  value: string
  allLabel: string
  options: { value: string; label: string }[]
  onChange: (value: string) => void
}) {
  const items = [{ value: ALL, label: props.allLabel }, ...props.options]
  const selected = items.find((item) => item.value === (props.value || ALL))
  return (
    <label className='flex min-w-0 flex-col gap-1 text-xs'>
      <span className='text-muted-foreground'>{props.label}</span>
      <Select
        items={items}
        value={props.value || ALL}
        onValueChange={(value) => {
          // Opening another select can report a null value. That is not a choice.
          if (value == null) return
          props.onChange(value === ALL ? '' : String(value))
        }}
      >
        <SelectTrigger className='w-full'>
          <SelectValue>{selected?.label}</SelectValue>
        </SelectTrigger>
        <SelectContent>
          <SelectGroup>
            {items.map((item) => (
              <SelectItem key={item.value} value={item.value}>
                {item.label}
              </SelectItem>
            ))}
          </SelectGroup>
        </SelectContent>
      </Select>
    </label>
  )
}

export function BillFilters(props: {
  vendorId: string
  model: string
  token: string
  group: string
  vendorOptions: { value: string; label: string }[]
  bill?: UserBill
  onVendorChange: (value: string) => void
  onModelChange: (value: string) => void
  onTokenChange: (value: string) => void
  onGroupChange: (value: string) => void
  onClear: () => void
}) {
  const { t } = useTranslation()
  const unassigned = t('Unassigned')
  const hasFilter = Boolean(
    props.vendorId || props.model || props.token || props.group
  )

  return (
    <div className='flex flex-col gap-3'>
      <div className='grid gap-3 sm:grid-cols-2 xl:grid-cols-4'>
        <FilterSelect
          label={t('Model vendor')}
          value={props.vendorId}
          allLabel={t('All model vendors')}
          options={props.vendorOptions.map((vendor) => ({
            value: vendor.value,
            label: vendor.label || unassigned,
          }))}
          onChange={props.onVendorChange}
        />
        <FilterSelect
          label={t('Model')}
          value={props.model}
          allLabel={t('All models')}
          options={(props.bill?.facets.models ?? []).map((model) => ({
            value: model,
            label: model,
          }))}
          onChange={props.onModelChange}
        />
        <FilterSelect
          label={t('Token')}
          value={props.token}
          allLabel={t('All tokens')}
          options={(props.bill?.facets.tokens ?? []).map((token) => ({
            value: token,
            label: token,
          }))}
          onChange={props.onTokenChange}
        />
        <FilterSelect
          label={t('Group')}
          value={props.group}
          allLabel={t('All groups')}
          options={(props.bill?.facets.groups ?? []).map((group) => ({
            value: group,
            label: group,
          }))}
          onChange={props.onGroupChange}
        />
      </div>
      {hasFilter ? (
        <div>
          <Button variant='ghost' size='sm' onClick={props.onClear}>
            {t('Clear filters')}
          </Button>
        </div>
      ) : null}
      <p className='text-muted-foreground text-xs'>
        {t('Filters apply to the summary, the table, and both exports.')}{' '}
        {t('This bill is calculated from usage logs that are still kept.')}
      </p>
    </div>
  )
}
