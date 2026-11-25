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

import { StaticDataTable } from '@/components/data-table'
import { Button } from '@/components/ui/button'
import { TableCell, TableRow } from '@/components/ui/table'
import { getLobeIcon } from '@/lib/lobe-icon'
import { cn } from '@/lib/utils'

import { billVendorIdentity } from '../lib/vendor'

import { BILL_PAGE_SIZE, billPageCount } from '../lib/query'
import {
  billLocale,
  formatBillDiscount,
  formatBillQuota,
  formatBillTokens,
} from '../lib/format'
import type { BillDetail } from '../types'

export function BillTable(props: {
  rows: BillDetail[]
  page: number
  total: number
  onPageChange: (page: number) => void
  onOpen: (row: BillDetail) => void
}) {
  const { t, i18n } = useTranslation()
  const locale = billLocale(i18n.resolvedLanguage || i18n.language)
  const unassigned = t('Unassigned')
  const pages = billPageCount(props.total)
  const from = props.total === 0 ? 0 : (props.page - 1) * BILL_PAGE_SIZE + 1
  const to = Math.min(props.page * BILL_PAGE_SIZE, props.total)

  const columns = [
    {
      id: 'vendor',
      header: t('Model vendor'),
      cell: (row: BillDetail) => {
        if (row.adjustment) return '—'
        const vendor = billVendorIdentity(row)
        return (
          <span className='inline-flex items-center gap-1.5'>
            {vendor.icon ? getLobeIcon(vendor.icon, 14) : null}
            {vendor.name || unassigned}
          </span>
        )
      },
    },
    {
      id: 'model',
      header: t('Model'),
      cell: (row: BillDetail) =>
        row.adjustment ? t('Adjustment') : row.model_name || '—',
    },
    {
      id: 'token',
      header: t('Token'),
      cell: (row: BillDetail) => row.token_name || '—',
    },
    {
      id: 'group',
      header: t('Group'),
      cell: (row: BillDetail) => row.group || '—',
    },
    {
      id: 'requests',
      header: t('Requests'),
      className: 'text-right',
      cell: (row: BillDetail) =>
        formatBillTokens(row.request_count, locale),
    },
    {
      id: 'uncached',
      header: t('Uncached input'),
      className: 'text-right',
      cell: (row: BillDetail) => formatBillTokens(row.uncached_input, locale),
    },
    {
      id: 'cache-read',
      header: t('Cache read'),
      className: 'text-right',
      cell: (row: BillDetail) => formatBillTokens(row.cache_read, locale),
    },
    {
      id: 'cache-5m',
      header: t('Cache write 5 minutes'),
      className: 'text-right',
      cell: (row: BillDetail) => formatBillTokens(row.cache_write_5m, locale),
    },
    {
      id: 'cache-1h',
      header: t('Cache write 1 hour'),
      className: 'text-right',
      cell: (row: BillDetail) => formatBillTokens(row.cache_write_1h, locale),
    },
    {
      id: 'cache-other',
      header: t('Other cache write'),
      className: 'text-right',
      cell: (row: BillDetail) =>
        formatBillTokens(row.cache_write_other, locale),
    },
    {
      id: 'output',
      header: t('Output tokens'),
      className: 'text-right',
      cell: (row: BillDetail) => formatBillTokens(row.output_tokens, locale),
    },
    {
      id: 'list',
      header: t('List price'),
      className: 'text-right',
      cell: (row: BillDetail) => formatBillQuota(row.list_quota),
    },
    {
      id: 'discount',
      header: t('Discount coefficient'),
      className: 'text-right',
      cell: (row: BillDetail) => formatBillDiscount(row.discount, locale),
    },
    {
      id: 'actual',
      header: t('Actual charge'),
      className: 'text-right',
      cell: (row: BillDetail) => formatBillQuota(row.actual_quota),
    },
  ]

  return (
    <div className='flex flex-col gap-3'>
      <StaticDataTable
        className='overflow-x-auto'
        tableClassName='min-w-[1100px]'
        columns={columns}
        data={props.rows}
        getRowKey={(row) =>
          [
            row.adjustment ? 'adjustment' : row.vendor_id,
            row.model_name,
            row.token_id,
            row.token_name,
            row.group,
          ].join(':')
        }
        empty={props.rows.length === 0}
        emptyContent={t('No usage in this time range')}
        renderRow={(row) => (
          <TableRow
            className='cursor-pointer'
            onClick={() => props.onOpen(row)}
          >
            {columns.map((column) => (
              <TableCell
                key={column.id}
                className={cn('whitespace-nowrap', column.className)}
              >
                {column.cell(row)}
              </TableCell>
            ))}
          </TableRow>
        )}
      />
      {props.total > BILL_PAGE_SIZE ? (
        <div className='flex items-center justify-between gap-3'>
          <p className='text-muted-foreground text-sm'>
            {t('Showing {{from}}–{{to}} of {{total}}', { from, to, total: props.total })}
          </p>
          <div className='flex gap-2'>
            <Button
              variant='outline'
              size='sm'
              disabled={props.page <= 1}
              onClick={() => props.onPageChange(props.page - 1)}
            >
              {t('Previous page')}
            </Button>
            <Button
              variant='outline'
              size='sm'
              disabled={props.page >= pages}
              onClick={() => props.onPageChange(props.page + 1)}
            >
              {t('Next page')}
            </Button>
          </div>
        </div>
      ) : null}
    </div>
  )
}
