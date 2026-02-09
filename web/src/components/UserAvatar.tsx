interface UserAvatarProps {
  name: string
  thumbUrl: string
  size?: 'sm' | 'lg'
}

const sizeClasses = {
  sm: { img: 'w-8 h-8', initials: 'w-8 h-8 text-xs' },
  lg: { img: 'w-16 h-16', initials: 'w-16 h-16 text-xl' },
}

export function UserAvatar({ name, thumbUrl, size = 'sm' }: UserAvatarProps) {
  const s = sizeClasses[size]
  const initials = name.slice(0, 2).toUpperCase()

  if (thumbUrl) {
    return (
      <img
        src={thumbUrl}
        alt={name}
        className={`${s.img} rounded-full object-cover shrink-0`}
      />
    )
  }

  return (
    <div className={`${s.initials} rounded-full bg-accent/20 text-accent
                     flex items-center justify-center font-bold shrink-0`}>
      {initials}
    </div>
  )
}
