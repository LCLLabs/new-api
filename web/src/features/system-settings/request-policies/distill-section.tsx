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
import { zodResolver } from '@hookform/resolvers/zod'
import { useQuery } from '@tanstack/react-query'
import { useEffect, useMemo } from 'react'
import { useForm, useFormContext } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import * as z from 'zod'

import { Checkbox } from '@/components/ui/checkbox'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { getGroups } from '@/features/users/api'
import { handleServerError } from '@/lib/handle-server-error'

import {
  SettingsForm,
  SettingsControlGroup,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { PolicyPart } from './policy-part'
import {
  factoryDistillRules,
  type DistillRules,
} from './distill-defaults'
import { useSavePolicy } from './use-save-policy'

const distillSchema = z.object({
  groups: z.array(z.string()),
  extraGroups: z.string(),
  skipOpenAI: z.boolean(),
  hardNeedles: z.string(),
  patterns: z.string(),
  fingerprints: z.string(),
  needles: z.string(),
  cotPatterns: z.string(),
  minMaxTokens: z.number().int().min(0),
  requireNoTools: z.boolean(),
  requireSingleTurn: z.boolean(),
  contestPattern: z.string(),
})

type DistillFormValues = z.infer<typeof distillSchema>

export type DistillSectionValues = {
  DistillCheckGroups: string
  DistillRules: string
  DistillSkipOpenAI: boolean
}

type DistillSectionProps = {
  defaultValues: DistillSectionValues
}

function lines(items: string[] | undefined) {
  return (items ?? []).join('\n')
}

function parseLines(value: string) {
  return value
    .split('\n')
    .map((item) => item.trim())
    .filter(Boolean)
}

function parseGroups(value: string) {
  try {
    const parsed = JSON.parse(value)
    if (!Array.isArray(parsed)) return []
    return parsed
      .filter((item): item is string => typeof item === 'string')
      .map((item) => item.trim())
      .filter(Boolean)
  } catch {
    return []
  }
}

function compareGroupNames(a: string, b: string) {
  return a.localeCompare(b, 'zh-CN', { numeric: true, sensitivity: 'base' })
}

function groupsJSON(groups: string[]) {
  return JSON.stringify([...new Set(groups)].sort(compareGroupNames))
}

function parseRules(value: string): DistillRules {
  try {
    const parsed = JSON.parse(value) as Partial<DistillRules>
    return {
      hard_needles: parsed.hard_needles ?? [],
      patterns: parsed.patterns ?? [],
      fingerprints: parsed.fingerprints ?? [],
      needles: parsed.needles ?? [],
      cot_patterns: parsed.cot_patterns ?? [],
      structure: {
        min_max_tokens:
          parsed.structure?.min_max_tokens ??
          factoryDistillRules.structure.min_max_tokens,
        require_no_tools:
          parsed.structure?.require_no_tools ??
          factoryDistillRules.structure.require_no_tools,
        require_single_turn:
          parsed.structure?.require_single_turn ??
          factoryDistillRules.structure.require_single_turn,
        contest_pattern:
          parsed.structure?.contest_pattern ??
          factoryDistillRules.structure.contest_pattern,
      },
    }
  } catch {
    return factoryDistillRules
  }
}

function formValues(defaults: DistillSectionValues): DistillFormValues {
  const rules = parseRules(defaults.DistillRules)
  return {
    groups: parseGroups(defaults.DistillCheckGroups),
    extraGroups: '',
    skipOpenAI: defaults.DistillSkipOpenAI,
    hardNeedles: lines(rules.hard_needles),
    patterns: lines(rules.patterns),
    fingerprints: lines(rules.fingerprints),
    needles: lines(rules.needles),
    cotPatterns: lines(rules.cot_patterns),
    minMaxTokens: rules.structure.min_max_tokens,
    requireNoTools: rules.structure.require_no_tools,
    requireSingleTurn: rules.structure.require_single_turn,
    contestPattern: rules.structure.contest_pattern,
  }
}

function rulesFromForm(values: DistillFormValues): DistillRules {
  return {
    hard_needles: parseLines(values.hardNeedles),
    patterns: parseLines(values.patterns),
    fingerprints: parseLines(values.fingerprints),
    needles: parseLines(values.needles),
    cot_patterns: parseLines(values.cotPatterns),
    structure: {
      min_max_tokens: values.minMaxTokens,
      require_no_tools: values.requireNoTools,
      require_single_turn: values.requireSingleTurn,
      contest_pattern: values.contestPattern.trim(),
    },
  }
}

function selectedGroups(values: DistillFormValues) {
  return [...values.groups, ...parseLines(values.extraGroups)]
}

export function DistillSection({ defaultValues }: DistillSectionProps) {
  const { t } = useTranslation()
  const updateOption = useSavePolicy()
  const groupsQuery = useQuery({
    queryKey: ['distill-groups'],
    queryFn: async () => {
      try {
        const response = await getGroups()
        return Array.isArray(response.data) ? response.data : []
      } catch {
        return []
      }
    },
  })
  const form = useForm<DistillFormValues>({
    resolver: zodResolver(distillSchema),
    defaultValues: formValues(defaultValues),
  })

  const defaultsKey = JSON.stringify(defaultValues)
  useEffect(() => {
    form.reset(formValues(JSON.parse(defaultsKey) as DistillSectionValues))
  }, [defaultsKey, form])

  const knownGroups = groupsQuery.data ?? []
  const selected = form.watch('groups')
  const groupOptions = useMemo(() => {
    const extras = selected.filter((group) => !knownGroups.includes(group))
    return [...knownGroups, ...extras].sort(compareGroupNames)
  }, [knownGroups, selected])

  const onSubmit = async (values: DistillFormValues) => {
    const updates: Record<string, string> = {}
    const nextGroups = groupsJSON(selectedGroups(values))
    if (nextGroups !== groupsJSON(parseGroups(defaultValues.DistillCheckGroups))) {
      updates.DistillCheckGroups = nextGroups
    }
    if (values.skipOpenAI !== defaultValues.DistillSkipOpenAI) {
      updates.DistillSkipOpenAI = String(values.skipOpenAI)
    }
    const nextRules = JSON.stringify(rulesFromForm(values))
    const currentRules = JSON.stringify(rulesFromForm(formValues(defaultValues)))
    if (nextRules !== currentRules) {
      updates.DistillRules = nextRules
    }

    try {
      if (Object.keys(updates).length > 0) {
        await updateOption.mutateAsync(updates)
      }
    } catch (error) {
      handleServerError(error)
    }
  }

  return (
    <PolicyPart
      title={t('Distillation checks')}
      description={t(
        'Distillation checks run before billing. Only selected user groups are scanned. An empty list checks nobody.'
      )}
      saveLabel='Save distillation rules'
      isSaving={form.formState.isSubmitting}
      onSave={form.handleSubmit(onSubmit)}
    >
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <FormField
            control={form.control}
            name='groups'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Checked user groups')}</FormLabel>
                <div className='grid gap-2 sm:grid-cols-2'>
                  {groupOptions.map((group) => {
                    const checked = field.value.includes(group)
                    return (
                      <label
                        key={group}
                        className='flex items-center gap-2 text-sm'
                      >
                        <Checkbox
                          checked={checked}
                          onCheckedChange={(next) => {
                            field.onChange(
                              next
                                ? [...field.value, group]
                                : field.value.filter((item) => item !== group)
                            )
                          }}
                        />
                        <span>{group}</span>
                      </label>
                    )
                  })}
                </div>
                {groupOptions.length === 0 && (
                  <FormDescription>
                    {t('No groups are available yet. Add one below.')}
                  </FormDescription>
                )}
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='extraGroups'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Add group names')}</FormLabel>
                <FormControl>
                  <Textarea
                    rows={3}
                    placeholder={t('One group name per line')}
                    {...field}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Names here are added to the checked list when you save.'
                  )}
                </FormDescription>
              </FormItem>
            )}
          />
          <SettingsControlGroup>
            <FormField
              control={form.control}
              name='skipOpenAI'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>{t('Skip OpenAI models')}</FormLabel>
                    <FormDescription>
                      {t('Models whose names start with gpt- skip these checks.')}
                    </FormDescription>
                  </SettingsSwitchContent>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                </SettingsSwitchItem>
              )}
            />
          </SettingsControlGroup>
          <RuleField
            name='hardNeedles'
            label={t('hard_needles')}
            description={t('One phrase per line.')}
          />
          <RuleField
            name='patterns'
            label={t('patterns')}
            description={t(
              'One regular expression per line. A match is ignored when the text immediately before it is a direct negation.'
            )}
          />
          <RuleField
            name='fingerprints'
            label={t('fingerprints')}
            description={t('One phrase per line. Only the latest user text is scanned.')}
          />
          <RuleField
            name='needles'
            label={t('needles')}
            description={t('One phrase per line.')}
          />
          <div className='grid gap-2'>
            <Label>{t('structure')}</Label>
            <div className='border-input flex flex-col gap-4 rounded-lg border px-3 py-3'>
              <FormField
                control={form.control}
                name='minMaxTokens'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('min_max_tokens')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={0}
                        value={field.value}
                        onChange={(event) =>
                          field.onChange(event.target.valueAsNumber || 0)
                        }
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'Structure harvest also needs an empty tool list, a single turn, and a thinking request.'
                      )}
                    </FormDescription>
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='requireNoTools'
                render={({ field }) => (
                  <SettingsSwitchItem>
                    <SettingsSwitchContent>
                      <FormLabel>{t('require_no_tools')}</FormLabel>
                    </SettingsSwitchContent>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                      />
                    </FormControl>
                  </SettingsSwitchItem>
                )}
              />
              <FormField
                control={form.control}
                name='requireSingleTurn'
                render={({ field }) => (
                  <SettingsSwitchItem>
                    <SettingsSwitchContent>
                      <FormLabel>{t('require_single_turn')}</FormLabel>
                    </SettingsSwitchContent>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                      />
                    </FormControl>
                  </SettingsSwitchItem>
                )}
              />
              <FormField
                control={form.control}
                name='contestPattern'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('contest_pattern')}</FormLabel>
                    <FormControl>
                      <Textarea rows={3} {...field} />
                    </FormControl>
                  </FormItem>
                )}
              />
            </div>
          </div>
          <RuleField
            name='cotPatterns'
            label={t('Chain-of-thought extraction')}
            description={t(
              'One regular expression per line. A match is ignored when the text immediately before it is a direct negation.'
            )}
          />
        </SettingsForm>
      </Form>
    </PolicyPart>
  )
}

function RuleField(props: {
  name: 'hardNeedles' | 'patterns' | 'fingerprints' | 'needles' | 'cotPatterns'
  label: string
  description: string
}) {
  const { control } = useFormContext<DistillFormValues>()
  return (
    <FormField
      control={control}
      name={props.name}
      render={({ field }) => (
        <FormItem>
          <FormLabel>{props.label}</FormLabel>
          <FormControl>
            <Textarea rows={6} {...field} />
          </FormControl>
          <FormDescription>{props.description}</FormDescription>
          <FormMessage />
        </FormItem>
      )}
    />
  )
}
