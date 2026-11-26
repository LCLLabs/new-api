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

import type { BillDetail } from '../types'
import {
  billPageRows,
  billRange,
  billRequestParams,
  usageLogSearch,
} from '../lib/query'
import {
  billVendorIdentity,
  billVendorTotals,
  filterBillRows,
} from '../lib/vendor'

const now = new Date(2026, 10, 24, 15, 30, 0)

describe('bill request parameters', () => {
  it('uses the current month when the search has no range', () => {
    const range = billRange({}, now)
    expect(range.start).toBe(Math.floor(new Date(2026, 10, 1).getTime() / 1000))
    expect(range.end).toBe(Math.floor(now.getTime() / 1000))
    expect(billRequestParams({}, now)).toEqual({
      start_timestamp: range.start,
      end_timestamp: range.end,
    })
  })

  it('leaves vendor filtering to the page so model names can be grouped after the query', () => {
    expect(
      billRequestParams({
        start: 10,
        end: 20,
        vendorId: 'provider:OpenAI',
        model: 'claude',
        token: 'trial',
        group: 'default',
      })
    ).toEqual({
      start_timestamp: 10,
      end_timestamp: 20,
      model_name: 'claude',
      token_name: 'trial',
      group: 'default',
    })
  })
})

describe('bill pages and usage-log links', () => {
  it('returns the requested page and clamps a page past the end', () => {
    const rows = Array.from({ length: 21 }, (_, index) => index)
    expect(billPageRows(rows, 2).rows).toEqual([20])
    expect(billPageRows(rows, 9).page).toBe(2)
  })

  it('opens consume and refund logs for a model row', () => {
    const row: BillDetail = {
      adjustment: false,
      vendor_id: 1,
      vendor_name: 'Anthropic',
      model_name: 'claude',
      token_name: 'trial',
      token_id: 3,
      group: 'default',
      request_count: 1,
      uncached_input: 0,
      cache_read: 0,
      cache_write_5m: 0,
      cache_write_1h: 0,
      cache_write_other: 0,
      output_tokens: 0,
      actual_quota: 1,
    }
    expect(billVendorIdentity(row).name).toBe('Claude')
    expect(usageLogSearch(row, 10, 20)).toMatchObject({
      model: 'claude',
      token: 'trial',
      group: 'default',
      startTime: 10000,
      endTime: 20000,
      type: ['2', '6'],
    })
  })
})

function billRow(model: string, quota: number): BillDetail {
  return {
    adjustment: false,
    vendor_id: 0,
    vendor_name: '',
    model_name: model,
    token_name: 'trial',
    token_id: 1,
    group: 'default',
    request_count: 1,
    uncached_input: 0,
    cache_read: 0,
    cache_write_5m: 0,
    cache_write_1h: 0,
    cache_write_other: 0,
    output_tokens: 0,
    actual_quota: quota,
  }
}

describe('bill vendor names', () => {
  it('uses the same model-name match as the usage logs', () => {
    expect(billVendorIdentity(billRow('gpt-4o', 1)).name).toBe('OpenAI')
    expect(billVendorIdentity(billRow('claude-sonnet-4-5', 1)).name).toBe(
      'Claude'
    )
    expect(billVendorIdentity(billRow('gemini-2.5-flash', 1)).name).toBe(
      'Gemini'
    )
    expect(billVendorIdentity(billRow('dall-e-3', 1)).name).toBe('OpenAI')
    expect(billVendorIdentity(billRow('custom-local', 1)).name).toBe('')
  })

  it('groups recognized models and can filter one vendor', () => {
    const rows = [
      billRow('gpt-4o', 3),
      billRow('gpt-4o-mini', 2),
      billRow('deepseek-v3', 4),
    ]
    const totals = billVendorTotals(rows)
    expect(totals.map((vendor) => vendor.vendor_name)).toEqual([
      'OpenAI',
      'DeepSeek',
    ])
    expect(totals[0]?.actual_quota).toBe(5)
    expect(
      filterBillRows(rows, 'provider:OpenAI').map((row) => row.model_name)
    ).toEqual(['gpt-4o', 'gpt-4o-mini'])
  })

  it('keeps one dynamic rate and still sums list prices when rates differ', () => {
    const same = billVendorTotals([
      { ...billRow('gpt-4o', 40), list_quota: 50, discount: 0.8, dynamic: true },
      {
        ...billRow('gpt-4o-mini', 40),
        list_quota: 50,
        discount: 0.8,
        dynamic: true,
      },
    ])
    expect(same[0]?.list_quota).toBe(100)
    expect(same[0]?.discount).toBe(0.8)
    expect(same[0]?.dynamic).toBe(true)

    const mixed = billVendorTotals([
      { ...billRow('gpt-4o', 40), list_quota: 50, discount: 0.8, dynamic: true },
      { ...billRow('gpt-4o-mini', 80), list_quota: 80, discount: 1 },
    ])
    expect(mixed[0]?.list_quota).toBe(130)
    expect(mixed[0]?.discount).toBeUndefined()
    expect(mixed[0]?.dynamic).toBeUndefined()
  })
})
