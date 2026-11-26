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
import { getRouteApi, useNavigate } from '@tanstack/react-router'
import { Eye, EyeOff } from 'lucide-react'
import { useState, useCallback, useMemo, lazy, Suspense } from 'react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { FadeIn } from '@/components/page-transition'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { getLobeIcon } from '@/lib/lobe-icon'
import { resolveModelProvider } from '@/lib/model-provider'
import { ROLE } from '@/lib/roles'
import { cn } from '@/lib/utils'
import { useAuthStore } from '@/stores/auth-store'

import { ModelsChartPreferences } from './components/models/models-chart-preferences'
import { ModelsFilter } from './components/models/models-filter-dialog'
import { SummaryCards } from './components/overview/summary-cards'
import { DEFAULT_TIME_GRANULARITY } from './constants'
import {
  buildDefaultDashboardFilters,
  getDefaultDays,
  getSavedChartPreferences,
  getSavedGranularity,
  saveChartPreferences,
} from './lib'
import {
  type DashboardSectionId,
  DASHBOARD_DEFAULT_SECTION,
  DASHBOARD_SECTION_IDS,
} from './section-registry'
import type {
  DashboardChartPreferences,
  DashboardFilters,
  QuotaDataItem,
  UserChartsFilters,
} from './types'

const route = getRouteApi('/_authenticated/dashboard/$section')

const LOG_STAT_CARD_FALLBACK_KEYS = [
  'count',
  'quota',
  'tokens',
  'cache-read',
  'cache-creation',
  'average-rpm',
  'average-tpm',
] as const
const PERFORMANCE_METRIC_FALLBACK_KEYS = [
  'success-rate',
  'average-latency',
  'throughput',
] as const
const PERFORMANCE_MODEL_FALLBACK_KEYS = [
  'primary-model',
  'secondary-model',
] as const

const LazyLogStatCards = lazy(() =>
  import('./components/models/log-stat-cards').then((m) => ({
    default: m.LogStatCards,
  }))
)

const LazyModelCharts = lazy(() =>
  import('./components/models/model-charts').then((m) => ({
    default: m.ModelCharts,
  }))
)

const LazyConsumptionDistributionChart = lazy(() =>
  import('./components/models/consumption-distribution-chart').then((m) => ({
    default: m.ConsumptionDistributionChart,
  }))
)

const LazyPerformanceOverview = lazy(() =>
  import('./components/models/performance-overview').then((m) => ({
    default: m.PerformanceOverview,
  }))
)

const LazyUserCharts = lazy(() =>
  import('./components/users/user-charts').then((m) => ({
    default: m.UserCharts,
  }))
)

const LazyFlowCharts = lazy(() =>
  import('./components/flow/flow-charts').then((m) => ({
    default: m.FlowCharts,
  }))
)

function LogStatCardsFallback() {
  return (
    <div className='overflow-hidden rounded-lg border'>
      <div className='divide-border/60 grid min-w-0 grid-cols-2 divide-x sm:grid-cols-3 lg:grid-cols-7'>
        {LOG_STAT_CARD_FALLBACK_KEYS.map((key, index) => (
          <div
            key={key}
            className={cn(
              'min-w-0 px-2.5 py-1.5 sm:px-3 sm:py-4 xl:px-4',
              index === LOG_STAT_CARD_FALLBACK_KEYS.length - 1 &&
                'col-span-2 sm:col-span-1'
            )}
          >
            <div className='flex items-center gap-1.5 sm:gap-2'>
              <Skeleton className='size-4 rounded-sm sm:size-7 sm:rounded-md' />
              <Skeleton className='h-4 w-16' />
            </div>
            <Skeleton className='mt-1 h-5 w-16 sm:mt-2 sm:h-7 sm:w-20' />
            <Skeleton className='mt-1 hidden h-3.5 w-28 md:block' />
          </div>
        ))}
      </div>
      <div className='border-t px-3 py-2'>
        <Skeleton className='h-4 w-full max-w-md' />
      </div>
    </div>
  )
}

function ModelChartsFallback() {
  return (
    <div className='overflow-hidden rounded-lg border'>
      <div className='flex items-center justify-between border-b px-4 py-3 sm:px-5'>
        <Skeleton className='h-5 w-32' />
        <Skeleton className='h-8 w-72' />
      </div>
      <div className='h-96 p-2'>
        <Skeleton className='h-full w-full' />
      </div>
    </div>
  )
}

function PerformanceOverviewFallback() {
  return (
    <div className='overflow-hidden rounded-lg border'>
      <div className='flex flex-wrap items-center gap-x-6 gap-y-2 px-4 py-3 sm:px-5'>
        <div className='flex items-center gap-2'>
          <Skeleton className='h-4 w-24' />
        </div>
        {PERFORMANCE_METRIC_FALLBACK_KEYS.map((key) => (
          <div key={key} className='flex items-center gap-1.5'>
            <Skeleton className='h-3 w-14' />
            <Skeleton className='h-4 w-16' />
          </div>
        ))}
        <div className='ml-auto flex items-center gap-2'>
          {PERFORMANCE_MODEL_FALLBACK_KEYS.map((key) => (
            <Skeleton key={key} className='h-5 w-28 rounded-full' />
          ))}
        </div>
      </div>
    </div>
  )
}

const SECTION_META: Record<DashboardSectionId, { titleKey: string }> = {
  overview: {
    titleKey: 'Overview',
  },
  models: {
    titleKey: 'Model Call Analytics',
  },
  flow: {
    titleKey: 'Flow',
  },
  users: {
    titleKey: 'User Analytics',
  },
}

export function Dashboard() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const params = route.useParams()
  const userRole = useAuthStore((state) => state.auth.user?.role)
  const activeSection = (params.section ??
    DASHBOARD_DEFAULT_SECTION) as DashboardSectionId

  const [modelData, setModelData] = useState<QuotaDataItem[]>([])
  const [dataLoading, setDataLoading] = useState(false)
  const [chartPreferences, setChartPreferences] =
    useState<DashboardChartPreferences>(() => getSavedChartPreferences())
  const [modelFilters, setModelFilters] = useState<DashboardFilters>(() =>
    buildDefaultDashboardFilters(getSavedChartPreferences())
  )
  const [userChartsFilters, setUserChartsFilters] = useState<UserChartsFilters>(
    () => {
      const granularity = getSavedGranularity()
      return {
        timeGranularity: granularity,
        selectedRange: getDefaultDays(granularity),
        topUserLimit: 10,
      }
    }
  )
  const [flowSensitiveVisible, setFlowSensitiveVisible] = useState(true)
  const [vendorLayout, setVendorLayout] = useState<'split' | 'combined'>(
    'combined'
  )

  const handleFilterChange = useCallback((filters: DashboardFilters) => {
    setModelFilters(filters)
  }, [])

  const handleResetFilters = useCallback(() => {
    setModelFilters(buildDefaultDashboardFilters(chartPreferences))
  }, [chartPreferences])

  const handleDataUpdate = useCallback(
    (data: QuotaDataItem[], loading: boolean) => {
      setModelData(data)
      setDataLoading(loading)
    },
    []
  )
  const vendorSections = useMemo(() => {
    const groups = new Map<
      string,
      {
        id: string
        label: string
        icon?: string
        rows: QuotaDataItem[]
        quota: number
      }
    >()
    for (const item of modelData) {
      const provider = resolveModelProvider(item.model_name ?? '')
      const id = provider?.name ?? 'unassigned'
      const section = groups.get(id) ?? {
        id,
        label: provider ? (provider.label ?? provider.name) : t('Unassigned'),
        icon: provider?.icon,
        rows: [],
        quota: 0,
      }
      section.rows.push(item)
      section.quota += Number(item.quota) || 0
      groups.set(id, section)
    }
    return [...groups.values()].sort((left, right) => {
      if (left.id === 'unassigned') return 1
      if (right.id === 'unassigned') return -1
      return right.quota - left.quota || left.label.localeCompare(right.label)
    })
  }, [modelData, t])

  const handleChartPreferencesChange = useCallback(
    (preferences: DashboardChartPreferences) => {
      setChartPreferences(preferences)
      setModelFilters(buildDefaultDashboardFilters(preferences))
      saveChartPreferences(preferences)
    },
    []
  )

  const meta = SECTION_META[activeSection] ?? SECTION_META.overview
  const isAdmin = Boolean(userRole && userRole >= ROLE.ADMIN)
  const visibleSections = useMemo(
    () =>
      DASHBOARD_SECTION_IDS.filter((section) => {
        if (section === 'models') return false
        if (section === 'flow' || section === 'users') return isAdmin
        return true
      }),
    [isAdmin]
  )
  const handleSectionChange = useCallback(
    (section: string) => {
      void navigate({
        to: '/dashboard/$section',
        params: { section: section as DashboardSectionId },
      })
    },
    [navigate]
  )
  const showSectionTabs = visibleSections.length > 1
  const showModelAnalytics = activeSection === 'overview'
  const splitByVendor = showModelAnalytics && vendorLayout === 'split'
  const modelActions = showModelAnalytics ? (
    <>
      <Tabs
        value={vendorLayout}
        onValueChange={(value) =>
          setVendorLayout(value === 'split' ? 'split' : 'combined')
        }
      >
        <TabsList aria-label={t('View by vendor')}>
          <TabsTrigger value='split'>{t('View separately')}</TabsTrigger>
          <TabsTrigger value='combined'>{t('Combined')}</TabsTrigger>
        </TabsList>
      </Tabs>
      <ModelsChartPreferences
        preferences={chartPreferences}
        onPreferencesChange={handleChartPreferencesChange}
      />
      <ModelsFilter
        preferences={chartPreferences}
        currentFilters={modelFilters}
        onFilterChange={handleFilterChange}
        onReset={handleResetFilters}
      />
    </>
  ) : null
  const flowActions =
    activeSection === 'flow' ? (
      <>
        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                variant='ghost'
                size='icon'
                onClick={() => setFlowSensitiveVisible((prev) => !prev)}
                aria-label={
                  flowSensitiveVisible
                    ? t('Hide sensitive data')
                    : t('Show sensitive data')
                }
                className='text-muted-foreground hover:text-foreground size-8'
              />
            }
          >
            {flowSensitiveVisible ? <Eye /> : <EyeOff />}
          </TooltipTrigger>
          <TooltipContent>
            {flowSensitiveVisible
              ? t('Hide sensitive data')
              : t('Show sensitive data')}
          </TooltipContent>
        </Tooltip>
        <ModelsFilter
          preferences={chartPreferences}
          currentFilters={modelFilters}
          onFilterChange={handleFilterChange}
          onReset={handleResetFilters}
          titleKey='Flow Filters'
          descriptionKey='Filter the traffic flow view by time range and user.'
        />
      </>
    ) : null
  const sectionActions = modelActions ?? flowActions
  let sectionNav = <div />
  if (showModelAnalytics) {
    sectionNav = (
      <div className='flex min-w-0 flex-wrap items-center gap-1'>
        <h3 className='px-1 text-sm font-semibold'>
          {t('Model Call Analytics')}
        </h3>
        {visibleSections
          .filter((section) => section !== 'overview')
          .map((section) => (
            <Button
              key={section}
              variant='ghost'
              size='sm'
              onClick={() => handleSectionChange(section)}
            >
              {t(SECTION_META[section].titleKey)}
            </Button>
          ))}
      </div>
    )
  } else if (showSectionTabs) {
    sectionNav = (
      <Tabs value={activeSection} onValueChange={handleSectionChange}>
        <TabsList className='max-w-full flex-wrap justify-start group-data-horizontal/tabs:h-auto'>
          {visibleSections.map((section) => (
            <TabsTrigger key={section} value={section}>
              {section === 'overview'
                ? t('Model Call Analytics')
                : t(SECTION_META[section].titleKey)}
            </TabsTrigger>
          ))}
        </TabsList>
      </Tabs>
    )
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        {showModelAnalytics ? t('Overview') : t(meta.titleKey)}
      </SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='space-y-3 sm:space-y-4'>
          {showModelAnalytics ? <SummaryCards /> : null}
          <div className='flex flex-wrap items-center justify-between gap-1.5 sm:gap-2'>
            {sectionNav}
            {sectionActions != null && (
              <div className='flex shrink-0 flex-wrap items-center gap-1.5 sm:gap-2'>
                {sectionActions}
              </div>
            )}
          </div>
          {showModelAnalytics && (
            <>
              <FadeIn>
                <Suspense fallback={<LogStatCardsFallback />}>
                  <LazyLogStatCards
                    filters={modelFilters}
                    headless={splitByVendor}
                    onDataUpdate={handleDataUpdate}
                  />
                </Suspense>
              </FadeIn>
              {isAdmin && !splitByVendor && (
                <FadeIn delay={0.05}>
                  <Suspense fallback={<PerformanceOverviewFallback />}>
                    <LazyPerformanceOverview />
                  </Suspense>
                </FadeIn>
              )}
              {splitByVendor && dataLoading ? <LogStatCardsFallback /> : null}
              {(splitByVendor
                ? vendorSections
                : [
                    {
                      id: 'all',
                      label: '',
                      icon: undefined,
                      rows: modelData,
                      quota: 0,
                    },
                  ]
              ).map((section) => (
                <div key={section.id} className='space-y-3 sm:space-y-4'>
                  {splitByVendor ? (
                    <>
                      <div className='flex items-center gap-2 px-1'>
                        {section.icon ? (
                          <span className='flex size-5 shrink-0 items-center justify-center'>
                            {getLobeIcon(section.icon, 18)}
                          </span>
                        ) : null}
                        <h4 className='text-sm font-semibold'>
                          {section.label}
                        </h4>
                      </div>
                      <Suspense fallback={<LogStatCardsFallback />}>
                        <LazyLogStatCards
                          rows={section.rows}
                          loading={dataLoading}
                          filters={modelFilters}
                        />
                      </Suspense>
                    </>
                  ) : null}
                  <Suspense fallback={<ModelChartsFallback />}>
                    <LazyConsumptionDistributionChart
                      data={section.rows}
                      loading={dataLoading}
                      defaultChartType={
                        chartPreferences.consumptionDistributionChart
                      }
                      timeGranularity={
                        modelFilters.time_granularity ||
                        DEFAULT_TIME_GRANULARITY
                      }
                    />
                  </Suspense>
                  <Suspense fallback={<ModelChartsFallback />}>
                    <LazyModelCharts
                      data={section.rows}
                      loading={dataLoading}
                      defaultChartTab={chartPreferences.modelAnalyticsChart}
                      timeGranularity={
                        modelFilters.time_granularity ||
                        DEFAULT_TIME_GRANULARITY
                      }
                    />
                  </Suspense>
                </div>
              ))}
              {isAdmin && splitByVendor && (
                <FadeIn delay={0.05}>
                  <Suspense fallback={<PerformanceOverviewFallback />}>
                    <LazyPerformanceOverview />
                  </Suspense>
                </FadeIn>
              )}
            </>
          )}
          {activeSection === 'users' && (
            <FadeIn>
              <Suspense fallback={<ModelChartsFallback />}>
                <LazyUserCharts
                  filters={userChartsFilters}
                  onFiltersChange={setUserChartsFilters}
                />
              </Suspense>
            </FadeIn>
          )}
          {activeSection === 'flow' && (
            <FadeIn>
              <Suspense fallback={<ModelChartsFallback />}>
                <LazyFlowCharts
                  filters={modelFilters}
                  sensitiveVisible={flowSensitiveVisible}
                />
              </Suspense>
            </FadeIn>
          )}
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
