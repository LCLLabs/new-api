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
import type { BillDetail, BillSearch } from '../types'

export const BILL_PAGE_SIZE = 20

export function billRange(search: BillSearch, now = new Date()) {
  const monthStart = new Date(
    now.getFullYear(),
    now.getMonth(),
    1,
    0,
    0,
    0,
    0
  )
  return {
    start: search.start ?? Math.floor(monthStart.getTime() / 1000),
    end: search.end ?? Math.floor(now.getTime() / 1000),
  }
}

export function billRequestParams(search: BillSearch, now = new Date()) {
  const range = billRange(search, now)
  return {
    start_timestamp: range.start,
    end_timestamp: range.end,
    ...(search.model ? { model_name: search.model } : {}),
    ...(search.token ? { token_name: search.token } : {}),
    ...(search.group ? { group: search.group } : {}),
  }
}

export function billPageCount(rowCount: number) {
  return Math.max(1, Math.ceil(rowCount / BILL_PAGE_SIZE))
}

export function billPageRows<T>(rows: T[], page: number | undefined) {
  const safePage = Math.min(
    Math.max(page ?? 1, 1),
    billPageCount(rows.length)
  )
  const start = (safePage - 1) * BILL_PAGE_SIZE
  return {
    page: safePage,
    rows: rows.slice(start, start + BILL_PAGE_SIZE),
  }
}

export function usageLogSearch(row: BillDetail, start: number, end: number) {
  return {
    model: row.adjustment ? undefined : row.model_name || undefined,
    token: row.adjustment ? undefined : row.token_name || undefined,
    group: row.adjustment ? undefined : row.group || undefined,
    startTime: start * 1000,
    endTime: end * 1000,
    type: row.adjustment ? (['6'] as const) : (['2', '6'] as const),
    page: 1,
  }
}
