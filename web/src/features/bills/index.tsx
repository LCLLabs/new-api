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
import { useQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { Download } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { ErrorState } from '@/components/error-state'
import { SectionPageLayout } from '@/components/layout'
import { LoadingState } from '@/components/loading-state'
import { Button } from '@/components/ui/button'
import { CompactDateTimeRangePicker } from '@/features/usage-logs/components/compact-date-time-range-picker'
import { downloadCsv, toCsvLine } from '@/features/usage-logs/lib/export'
import { toIntlLocale } from '@/i18n/languages'
import { requireServerSuccess } from '@/lib/server-error-message'

import { getUserBill } from './api'
import { BillFilters } from './components/bill-filters'
import { BillSummary } from './components/bill-summary'
import { BillTable } from './components/bill-table'
import { formatBillDiscount, formatBillQuota } from './lib/format'
import {
  billPageRows,
  billRange,
  billRequestParams,
  usageLogSearch,
} from './lib/query'
import {
  billVendorChoices,
  billVendorIdentity,
  billVendorTotals,
  filterBillRows,
  sumBillMetrics,
} from './lib/vendor'
import type { BillDetail, BillMetrics, BillSearch } from './types'

function metricCells(
  metrics: BillMetrics,
  locale: Intl.LocalesArgument
): string[] {
  return [
    String(metrics.request_count),
    String(metrics.uncached_input),
    String(metrics.cache_read),
    String(metrics.cache_write_5m),
    String(metrics.cache_write_1h),
    String(metrics.cache_write_other),
    String(metrics.output_tokens),
    formatBillQuota(metrics.list_quota),
    formatBillDiscount(metrics.discount, locale),
    formatBillQuota(metrics.actual_quota),
  ]
}

export function BillsPage(props: { search: BillSearch }) {
  const { t, i18n } = useTranslation()
  const navigate = useNavigate()
  const range = billRange(props.search)
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const params = billRequestParams(props.search)

  const query = useQuery({
    queryKey: ['user-bill', params],
    queryFn: async () => {
      const response = requireServerSuccess(await getUserBill(params))
      return response.data
    },
  })

  const updateSearch = (patch: Partial<BillSearch>) => {
    navigate({
      to: '/bills',
      search: {
        start: range.start,
        end: range.end,
        vendorId: props.search.vendorId,
        model: props.search.model,
        token: props.search.token,
        group: props.search.group,
        page: 1,
        ...patch,
      },
    })
  }

  const rows = filterBillRows(query.data?.rows ?? [], props.search.vendorId)
  const vendors = billVendorTotals(rows)
  const total = props.search.vendorId
    ? sumBillMetrics(rows)
    : query.data?.total
  const page = billPageRows(rows, props.search.page)
  const timeZone = Intl.DateTimeFormat().resolvedOptions().timeZone
  const timeZoneOffset = new Intl.DateTimeFormat(locale, {
    timeZone,
    timeZoneName: 'shortOffset',
  })
    .formatToParts(new Date())
    .find((part) => part.type === 'timeZoneName')?.value
  const timeZoneLabel = timeZoneOffset
    ? `${timeZone} (${timeZoneOffset})`
    : timeZone

  const exportSummary = () => {
    if (!total) return
    const header = [
      t('Model vendor'),
      t('Requests'),
      t('Uncached input'),
      t('Cache read'),
      t('Cache write 5 minutes'),
      t('Cache write 1 hour'),
      t('Other cache write'),
      t('Output tokens'),
      t('List price'),
      t('Discount coefficient'),
      t('Actual charge'),
    ]
    const lines = [
      toCsvLine(header),
      toCsvLine([t('Total'), ...metricCells(total, locale)]),
      ...vendors.map((vendor) =>
        toCsvLine([
          vendor.vendor_name || t('Unassigned'),
          ...metricCells(vendor, locale),
        ])
      ),
    ]
    downloadCsv('bill-summary.csv', lines)
  }

  const exportDetails = () => {
    if (!query.data) return
    const header = [
      t('Model vendor'),
      t('Model'),
      t('Token'),
      t('Group'),
      t('Requests'),
      t('Uncached input'),
      t('Cache read'),
      t('Cache write 5 minutes'),
      t('Cache write 1 hour'),
      t('Other cache write'),
      t('Output tokens'),
      t('List price'),
      t('Discount coefficient'),
      t('Actual charge'),
    ]
    const lines = [
      toCsvLine(header),
      ...rows.map((row: BillDetail) => {
        const vendor = billVendorIdentity(row)
        return toCsvLine([
          row.adjustment ? '' : vendor.name || t('Unassigned'),
          row.adjustment ? t('Adjustment') : row.model_name,
          row.token_name,
          row.group,
          ...metricCells(row, locale),
        ])
      }),
    ]
    downloadCsv('bill-details.csv', lines)
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Bill')}</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='flex flex-col gap-4'>
          <div className='flex flex-wrap items-center gap-2'>
            <CompactDateTimeRangePicker
              className='w-auto'
              start={new Date(range.start * 1000)}
              end={new Date(range.end * 1000)}
              onChange={(next) =>
                updateSearch({
                  start: next.start
                    ? Math.floor(next.start.getTime() / 1000)
                    : undefined,
                  end: next.end
                    ? Math.floor(next.end.getTime() / 1000)
                    : undefined,
                })
              }
            />
            <span className='text-muted-foreground text-xs whitespace-nowrap'>
              {t('Timezone')}: {timeZoneLabel}
            </span>
            <div className='ml-auto flex flex-wrap items-center gap-2'>
              <Button
                variant='outline'
                size='sm'
                disabled={!query.data}
                onClick={exportSummary}
              >
                <Download />
                {t('Export summary')}
              </Button>
              <Button
                variant='outline'
                size='sm'
                disabled={!query.data}
                onClick={exportDetails}
              >
                <Download />
                {t('Export details')}
              </Button>
            </div>
          </div>
          <BillFilters
            vendorId={props.search.vendorId ?? ''}
            model={props.search.model ?? ''}
            token={props.search.token ?? ''}
            group={props.search.group ?? ''}
            vendorOptions={billVendorChoices(query.data)}
            bill={query.data}
            onVendorChange={(vendorId) =>
              updateSearch({ vendorId: vendorId || undefined })
            }
            onModelChange={(model) =>
              updateSearch({ model: model || undefined })
            }
            onTokenChange={(token) =>
              updateSearch({ token: token || undefined })
            }
            onGroupChange={(group) =>
              updateSearch({ group: group || undefined })
            }
            onClear={() =>
              updateSearch({
                vendorId: undefined,
                model: undefined,
                token: undefined,
                group: undefined,
              })
            }
          />
          {query.isLoading ? <LoadingState /> : null}
          {query.isError ? (
            <ErrorState
              title={t('Failed to load the bill')}
              onRetry={() => query.refetch()}
            />
          ) : null}
          {query.data && total ? (
            <>
              <BillSummary total={total} vendors={vendors} />
              <BillTable
                rows={page.rows}
                page={page.page}
                total={rows.length}
                onPageChange={(nextPage) => updateSearch({ page: nextPage })}
                onOpen={(row) =>
                  navigate({
                    to: '/usage-logs/$section',
                    params: { section: 'common' },
                    search: usageLogSearch(row, range.start, range.end),
                  })
                }
              />
            </>
          ) : null}
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
