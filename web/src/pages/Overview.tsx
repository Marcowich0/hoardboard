import { type ReactNode, useState } from 'react'
import { useNavigate } from 'react-router'
import { api, type Overview as OverviewData } from '../api/index.ts'
import { useApi } from '../api/useApi.ts'
import DiskChart from '../charts/DiskChart.tsx'
import DonutChart from '../charts/DonutChart.tsx'
import { useParts } from '../charts/parts.ts'
import TrendChart from '../charts/TrendChart.tsx'
import { ShieldIcon, TrendIcon } from '../components/icons.tsx'
import { Card, ErrorNote, Segmented, Skeleton, Swatch } from '../components/ui.tsx'
import { formatBytes, formatChange, formatShare, formatTimeLeft } from '../lib/format.ts'
import { daysBetween } from '../lib/days.ts'

type Range = '90d' | '1y' | 'all'

const ranges: { value: Range; label: string }[] = [
  { value: '90d', label: '3M' },
  { value: '1y', label: '1Y' },
  { value: 'all', label: 'All' },
]

const monthYear = new Intl.DateTimeFormat('en-GB', { month: 'long', year: 'numeric', timeZone: 'UTC' })

export default function Overview() {
  const { data, error, reload } = useApi(() => api.overview(), [])

  if (error && !data) return <ErrorNote error={error} onRetry={reload} />
  if (!data) return <OverviewSkeleton />
  return <OverviewBody data={data} />
}

function OverviewBody({ data }: { data: OverviewData }) {
  const navigate = useNavigate()
  const [range, setRange] = useState<Range>('1y')
  const days = data.trend.days.length
  const from = range === 'all' ? 0 : Math.max(0, days - (range === '1y' ? 365 : 90))

  return (
    <div className="space-y-4 sm:space-y-5">
      <Summary data={data} />

      <div className="grid gap-4 sm:gap-5 lg:grid-cols-2">
        <Card title="Disks" subtitle="Used and free space per disk, live from the filesystems">
          <DiskChart disks={data.disks} />
          <div className="mt-4 space-y-2 border-t border-line pt-3">
            {data.parity.map((p) => (
              <div key={p.name} className="flex flex-wrap items-center gap-x-2 gap-y-1 text-[13px]">
                <ShieldIcon size={16} className={p.status === 'valid' ? 'text-good' : 'text-warn'} />
                <span className="font-medium">{p.name}</span>
                <span className="text-muted">
                  {formatBytes(p.size)} ·{' '}
                  {p.status === 'valid' ? 'Valid' : p.status === 'invalid' ? 'Invalid' : 'Missing'}
                  {p.lastCheck && ` · last check ${checkedAgo(p.lastCheck, data.today)}`}
                </span>
                <span className="ml-auto rounded-md bg-raised px-1.5 py-0.5 text-[11px] font-medium text-muted">
                  Protection only, not usable space
                </span>
              </div>
            ))}
          </div>
        </Card>

        <Card
          title="What uses the space"
          subtitle="Used space by category; shared bytes count once, for the higher Rank"
        >
          <DonutChart
            split={data.donut}
            onSelect={(part) => typeof part.key === 'number' && navigate(`/categories/${part.key}`)}
          />
        </Card>
      </div>

      <Card
        title="Used space over time"
        subtitle="Stacked by category in Rank order; the top edge is Used space"
        action={<Segmented label="Time range" value={range} options={ranges} onChange={setRange} />}
      >
        <TrendChart trend={data.trend} from={from} />
        <TrendLegend data={data} />
      </Card>
    </div>
  )
}

/** The hero: Used space, a bar of what it holds, Free space, growth and the forecast. */
function Summary({ data }: { data: OverviewData }) {
  const parts = useParts(data.donut)
  const { totals, forecast } = data
  const fullOn = forecast?.fullOn ?? null
  const usedShare = totals.size > 0 ? totals.used / totals.size : 0

  return (
    <Card bodyClassName="p-4 sm:p-6">
      <div className="grid gap-6 lg:grid-cols-[1fr_auto] lg:items-end">
        <div className="min-w-0">
          <p className="text-[13px] font-medium text-muted">Used space</p>
          <p className="mt-1 flex flex-wrap items-baseline gap-x-3">
            <span className="text-5xl font-semibold tracking-tight">{formatBytes(totals.used)}</span>
            <span className="text-lg text-muted">
              of {formatBytes(totals.size)} · {formatShare(totals.used, totals.size)}
            </span>
          </p>
          {/* The whole array as one bar: each part of Used space, then Free space. */}
          <div
            className="mt-4 flex h-3 w-full gap-[2px] overflow-hidden rounded-full bg-track"
            role="img"
            aria-label={`${formatShare(totals.used, totals.size)} used`}
          >
            {parts
              .filter((p) => p.bytes > 0)
              .map((p) => (
                <span
                  key={String(p.key)}
                  title={`${p.name}: ${formatBytes(p.bytes)}`}
                  className="h-full first:rounded-l-full"
                  style={{
                    width: `${(p.bytes / totals.size) * 100}%`,
                    backgroundColor: p.colour,
                    backgroundImage: p.hatch
                      ? 'repeating-linear-gradient(135deg, rgba(0,0,0,0.28) 0 1.5px, transparent 1.5px 4px)'
                      : undefined,
                  }}
                />
              ))}
          </div>
          <p className="mt-2 text-[13px] text-muted">
            Data disks and the cache pool{usedShare >= 0.9 ? ' · getting full' : ''}
          </p>
        </div>

        <dl className="grid grid-cols-3 gap-3 sm:gap-8 lg:flex">
          <Figure
            label="Free space"
            value={formatBytes(totals.free)}
            detail={`${formatShare(totals.free, totals.size)} left`}
          />
          <Figure
            label="Growth"
            icon={<TrendIcon size={14} />}
            value={forecast ? `${formatChange(forecast.bytesPerMonth)} / month` : '—'}
            detail={forecast ? `last ${data.forecastDays} days` : 'Needs a week of history'}
          />
          <Figure
            label="Full in"
            value={fullOn ? formatTimeLeft(fullOn, data.today) : '—'}
            detail={
              fullOn
                ? `around ${monthYear.format(new Date(`${fullOn}T00:00:00Z`))}`
                : forecast
                  ? 'No end in sight'
                  : 'Needs a week of history'
            }
          />
        </dl>
      </div>
    </Card>
  )
}

function Figure({ label, value, detail, icon }: { label: string; value: string; detail: string; icon?: ReactNode }) {
  return (
    <div className="min-w-0">
      <dt className="flex items-center gap-1 text-[13px] font-medium text-muted">
        {icon}
        {label}
      </dt>
      <dd className="mt-1 text-lg font-semibold tracking-tight whitespace-nowrap sm:text-2xl">{value}</dd>
      <dd className="mt-0.5 text-[12px] leading-snug text-muted sm:text-[13px]">{detail}</dd>
    </div>
  )
}

function TrendLegend({ data }: { data: OverviewData }) {
  const parts = useParts(data.donut)
  return (
    <ul className="mt-3 flex flex-wrap gap-x-4 gap-y-1.5 text-[13px] text-ink-2">
      {parts.map((p) => (
        <li key={String(p.key)} className="flex items-center gap-1.5">
          <Swatch colour={p.colour} hatch={p.hatch} />
          {p.name}
        </li>
      ))}
    </ul>
  )
}

function OverviewSkeleton() {
  return (
    <div className="space-y-5" aria-busy="true" aria-label="Loading">
      <Skeleton className="h-44 rounded-2xl" />
      <div className="grid gap-5 lg:grid-cols-5">
        <Skeleton className="h-80 rounded-2xl lg:col-span-3" />
        <Skeleton className="h-80 rounded-2xl lg:col-span-2" />
      </div>
      <Skeleton className="h-96 rounded-2xl" />
    </div>
  )
}

function checkedAgo(day: string, today: string): string {
  const days = daysBetween(day, today)
  return days <= 0 ? 'today' : days === 1 ? 'yesterday' : `${days} days ago`
}
