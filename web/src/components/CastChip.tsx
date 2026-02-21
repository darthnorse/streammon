import { TMDB_IMG } from '../lib/tmdb'

function getInitials(name: string): string {
  if (!name) return '?'
  return name.split(' ').filter(Boolean).map(n => n[0]).join('').slice(0, 2).toUpperCase()
}

interface CastChipProps {
  name: string
  character?: string
  profilePath?: string
  imgSrc?: string
  onClick?: () => void
}

export function CastChip({ name, character, profilePath, imgSrc, onClick }: CastChipProps) {
  const Wrapper = onClick ? 'button' : 'div'
  const resolvedImg = imgSrc ?? (profilePath ? `${TMDB_IMG}/w92${profilePath}` : undefined)
  return (
    <Wrapper
      onClick={onClick}
      className={`flex items-center gap-2 px-2 py-1.5 rounded-full bg-gray-100 dark:bg-white/10 shrink-0 text-left${
        onClick ? ' cursor-pointer hover:bg-gray-200 dark:hover:bg-white/15 transition-colors' : ''
      }`}
    >
      {resolvedImg ? (
        <img
          src={resolvedImg}
          alt={name}
          className="w-7 h-7 rounded-full object-cover bg-gray-300 dark:bg-white/20"
          loading="lazy"
        />
      ) : (
        <div className="w-7 h-7 rounded-full bg-gray-300 dark:bg-white/20 flex items-center justify-center text-[10px] font-medium text-gray-600 dark:text-gray-300">
          {getInitials(name)}
        </div>
      )}
      <div className="text-xs pr-1">
        <div className="font-medium text-gray-900 dark:text-gray-100">{name}</div>
        {character && (
          <div className="text-gray-500 dark:text-gray-400 text-[10px]">{character}</div>
        )}
      </div>
    </Wrapper>
  )
}
