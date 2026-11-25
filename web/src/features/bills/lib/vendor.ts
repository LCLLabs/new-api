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
import { resolveModelProvider } from '@/lib/model-provider'

import type { BillDetail, BillMetrics, BillVendorTotal, UserBill } from '../types'

const COUNT_FIELDS = [
  'request_count',
  'uncached_input',
  'cache_read',
  'cache_write_5m',
  'cache_write_1h',
  'cache_write_other',
  'output_tokens',
  'actual_quota',
] as const

export type BillVendorIdentity = {
  key: string
  name: string
  icon?: string
}

export function billVendorIdentity(row: {
  adjustment: boolean
  vendor_id: number
  vendor_name: string
  vendor_icon?: string
  model_name: string
}): BillVendorIdentity {
  if (row.adjustment) return { key: 'adjustment', name: '' }
  const provider = resolveModelProvider(row.model_name)
  if (provider) {
    return {
      key: `provider:${provider.name}`,
      name: provider.label ?? provider.name,
      icon: provider.icon,
    }
  }
  if (row.vendor_id !== 0 && row.vendor_name) {
    return {
      key: `id:${row.vendor_id}`,
      name: row.vendor_name,
      icon: row.vendor_icon,
    }
  }
  return { key: 'id:0', name: '' }
}

function vendorFilterKey(vendorId: string | undefined) {
  if (!vendorId) return undefined
  return vendorId === '0' ? 'id:0' : vendorId
}

export function filterBillRows(rows: BillDetail[], vendorId: string | undefined) {
  const key = vendorFilterKey(vendorId)
  if (!key) return rows
  return rows.filter((row) => billVendorIdentity(row).key === key)
}

export function sumBillMetrics(rows: BillDetail[]): BillMetrics {
  const total: BillMetrics = {
    request_count: 0,
    uncached_input: 0,
    cache_read: 0,
    cache_write_5m: 0,
    cache_write_1h: 0,
    cache_write_other: 0,
    output_tokens: 0,
    actual_quota: 0,
  }
  for (const row of rows) {
    for (const field of COUNT_FIELDS) {
      total[field] += row[field]
    }
  }
  if (rows.length > 0 && rows.every((row) => row.list_quota != null)) {
    total.list_quota = rows.reduce((sum, row) => sum + (row.list_quota ?? 0), 0)
  }
  const discount = rows[0]?.discount
  if (
    rows.length > 0 &&
    discount != null &&
    rows.every((row) => row.discount === discount)
  ) {
    total.discount = discount
  }
  return total
}

export function billVendorTotals(rows: BillDetail[]): BillVendorTotal[] {
  const groups = new Map<string, BillDetail[]>()
  const identities = new Map<string, BillVendorIdentity>()
  for (const row of rows) {
    const identity = billVendorIdentity(row)
    if (identity.key === 'adjustment') continue
    const bucket = groups.get(identity.key)
    if (bucket) bucket.push(row)
    else groups.set(identity.key, [row])
    identities.set(identity.key, identity)
  }
  return [...groups.entries()]
    .map(([key, bucket]) => ({
      key,
      metrics: sumBillMetrics(bucket),
      identity: identities.get(key),
    }))
    .sort((left, right) => {
      if (left.key === 'id:0') return 1
      if (right.key === 'id:0') return -1
      return right.metrics.actual_quota - left.metrics.actual_quota
    })
    .map((entry) => ({
      ...entry.metrics,
      vendor_id: 0,
      vendor_name: entry.identity?.name ?? '',
      vendor_icon: entry.identity?.icon,
    }))
}

export function billVendorChoices(bill: UserBill | undefined) {
  const choices = new Map<string, string>()
  const add = (identity: BillVendorIdentity) => {
    if (identity.key === 'adjustment' || choices.has(identity.key)) return
    choices.set(identity.key, identity.name)
  }
  for (const model of bill?.facets.models ?? []) {
    add(
      billVendorIdentity({
        adjustment: false,
        vendor_id: 0,
        vendor_name: '',
        model_name: model,
      })
    )
  }
  for (const row of bill?.rows ?? []) add(billVendorIdentity(row))
  for (const vendor of bill?.facets.vendors ?? []) {
    if (!vendor.id || !vendor.name) continue
    add({ key: `id:${vendor.id}`, name: vendor.name, icon: vendor.icon })
  }
  return [...choices.entries()]
    .map(([value, label]) => ({ value, label }))
    .sort((left, right) => {
      if (!left.label) return 1
      if (!right.label) return -1
      return left.label.localeCompare(right.label)
    })
}
