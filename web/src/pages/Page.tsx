import type { ReactNode } from 'react'

/** The heading and body every page starts from. */
export default function Page({
  title,
  actions,
  children,
}: {
  title: ReactNode
  actions?: ReactNode
  children?: ReactNode
}) {
  return (
    <section>
      <div className="mb-4 flex flex-wrap items-end justify-between gap-3 sm:mb-5">
        <h1 className="text-2xl font-semibold tracking-tight sm:text-[28px]">{title}</h1>
        {actions}
      </div>
      {children}
    </section>
  )
}
