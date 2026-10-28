import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { SystemInfoSection } from '../system-info-section'

vi.mock('../../components/form-navigation-guard', () => ({
  FormNavigationGuard: () => null,
}))
vi.mock('../../components/settings-page-context', async (importOriginal) => {
  const actual =
    await importOriginal<
      typeof import('../../components/settings-page-context')
    >()
  return {
    ...actual,
    SettingsPageFormActions: () => null,
    useSuppressSettingsSectionHeader: () => false,
  }
})

let client: QueryClient
beforeEach(() => {
  client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
})
afterEach(() => {
  cleanup()
  client.clear()
})

const defaults = {
  SystemName: 'New API',
  ServerAddress: '',
  TaskPublicAddress: '',
  Logo: '',
  Footer: '',
  About: '',
  HomePageContent: '',
  RootRedirectEnabled: false,
  RootRedirectURL: '',
  general_setting: { docs_link: '' },
  legal: { user_agreement: '', privacy_policy: '' },
}

describe('root redirect controls', () => {
  it('renders switch and url input', () => {
    render(
      <QueryClientProvider client={client}>
        <SystemInfoSection defaultValues={defaults as never} />
      </QueryClientProvider>
    )
    const label = screen.queryByText('Redirect root path')
    const urlLabel = screen.queryByText('Root redirect URL')
    const sw = screen.getByRole('switch', { name: /Redirect root path/i })
    const input = screen.getByPlaceholderText('https://example.com/')
    expect(label).toBeTruthy()
    expect(urlLabel).toBeTruthy()
    expect(sw).toBeVisible()
    expect(input).toBeVisible()
  })
})
