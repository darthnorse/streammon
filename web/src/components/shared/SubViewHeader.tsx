import { BackButton } from './BackButton'

interface SubViewHeaderProps {
  icon?: string
  title: string
  subtitle: string
  onBack: () => void
}

export function SubViewHeader({ icon, title, subtitle, onBack }: SubViewHeaderProps) {
  return (
    <div className="flex items-center gap-4">
      <BackButton onClick={onBack} />
      <div className="flex items-center gap-3">
        {icon && <span className="text-2xl">{icon}</span>}
        <div>
          <h1 className="text-2xl font-semibold">{title}</h1>
          <p className="text-sm text-muted dark:text-muted-dark">{subtitle}</p>
        </div>
      </div>
    </div>
  )
}
