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

import { getLobeIcon } from '@/lib/lobe-icon'

import {
  billLocale,
  formatBillQuota,
  formatBillTokens,
} from '../lib/format'
import type { BillMetrics, BillVendorTotal } from '../types'

function vendorLabel(name: string, unassigned: string) {
  return name || unassigned
}

function MetricLine(props: {
  label: string
  metrics: BillMetrics
  locale: Intl.LocalesArgument
}) {
  const { t } = useTranslation()
  const items = [
    [t('List price'), formatBillQuota(props.metrics.list_quota)],
    [t('Actual charge'), formatBillQuota(props.metrics.actual_quota)],
    [t('Requests'), formatBillTokens(props.metrics.request_count, props.locale)],
    [
      t('Uncached input'),
      formatBillTokens(props.metrics.uncached_input, props.locale),
    ],
    [t('Cache read'), formatBillTokens(props.metrics.cache_read, props.locale)],
    [
      t('Cache write 5 minutes'),
      formatBillTokens(props.metrics.cache_write_5m, props.locale),
    ],
    [
      t('Cache write 1 hour'),
      formatBillTokens(props.metrics.cache_write_1h, props.locale),
    ],
    [
      t('Other cache write'),
      formatBillTokens(props.metrics.cache_write_other, props.locale),
    ],
    [
      t('Output tokens'),
      formatBillTokens(props.metrics.output_tokens, props.locale),
    ],
  ] as const

  return (
    <div className='flex flex-wrap items-baseline gap-x-3 gap-y-1 text-sm'>
      <span className='text-foreground min-w-24 font-medium'>{props.label}</span>
      {items.map(([label, value]) => (
        <span key={label} className='inline-flex items-baseline gap-1'>
          <span className='text-muted-foreground'>{label}</span>
          <span className='text-foreground font-medium tabular-nums'>
            {value}
          </span>
        </span>
      ))}
    </div>
  )
}

export function BillSummary(props: {
  total: BillMetrics
  vendors: BillVendorTotal[]
}) {
  const { t, i18n } = useTranslation()
  const locale = billLocale(i18n.resolvedLanguage || i18n.language)
  const unassigned = t('Unassigned')

  return (
    <section className='bg-card rounded-lg border px-3 py-3'>
      <MetricLine label={t('Total')} metrics={props.total} locale={locale} />
      {props.vendors.map((vendor) => (
        <div
          key={`${vendor.vendor_name}:${vendor.vendor_icon ?? ''}`}
          className='border-border/70 mt-2 border-t pt-2'
        >
          <div className='flex items-start gap-2'>
            <span className='mt-0.5 shrink-0' aria-hidden='true'>
              {vendor.vendor_icon
                ? getLobeIcon(vendor.vendor_icon, 16)
                : null}
            </span>
            <MetricLine
              label={vendorLabel(vendor.vendor_name, unassigned)}
              metrics={vendor}
              locale={locale}
            />
          </div>
        </div>
      ))}
    </section>
  )
}
