import type { ReactNode } from 'react'

/** The heading and body every page starts from. */
export default function Page({ title, children }: { title: string; children?: ReactNode }) {
  return (
    <section>
      <h1 className="mb-4 text-2xl font-semibold tracking-tight">{title}</h1>
      {children}
    </section>
  )
}
