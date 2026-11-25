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
export type BillMetrics = {
  request_count: number
  uncached_input: number
  cache_read: number
  cache_write_5m: number
  cache_write_1h: number
  cache_write_other: number
  output_tokens: number
  actual_quota: number
  list_quota?: number
  discount?: number
}

export type BillVendorTotal = BillMetrics & {
  vendor_id: number
  vendor_name: string
  vendor_icon?: string
}

export type BillDetail = BillVendorTotal & {
  adjustment: boolean
  model_name: string
  token_name: string
  token_id: number
  group: string
}

export type BillFacetVendor = {
  id: number
  name: string
  icon?: string
}

export type UserBill = {
  total: BillMetrics
  vendors: BillVendorTotal[]
  rows: BillDetail[]
  facets: {
    vendors: BillFacetVendor[]
    models: string[]
    tokens: string[]
    groups: string[]
  }
}

export type BillSearch = {
  start?: number
  end?: number
  vendorId?: string
  model?: string
  token?: string
  group?: string
  page?: number
}
