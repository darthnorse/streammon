import { ReactNode } from 'react'

interface EmptyStateProps {
  icon: string
  title: string
  description?: string
  children?: ReactNode
}

export function EmptyState({ icon, title, description, children }: EmptyStateProps) {
  return (
    <div className="card p-12 text-center">
      <div className="text-4xl mb-3 opacity-30">{icon}</div>
      <p className="text-muted dark:text-muted-dark">{title}</p>
      {description && (
        <p className="text-sm text-muted dark:text-muted-dark mt-1">{description}</p>
      )}
      {children && <div className="mt-4">{children}</div>}
    </div>
  )
}
