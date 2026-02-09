const leftPath = 'M12.79 5.23a.75.75 0 01-.02 1.06L8.832 10l3.938 3.71a.75.75 0 11-1.04 1.08l-4.5-4.25a.75.75 0 010-1.08l4.5-4.25a.75.75 0 011.06.02z'
const rightPath = 'M7.21 14.77a.75.75 0 01.02-1.06L11.168 10 7.23 6.29a.75.75 0 111.04-1.08l4.5 4.25a.75.75 0 010 1.08l-4.5 4.25a.75.75 0 01-1.06-.02z'

export function ChevronIcon({ direction, className }: { direction: 'left' | 'right'; className?: string }) {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" className={className ?? 'w-5 h-5'}>
      <path fillRule="evenodd" d={direction === 'left' ? leftPath : rightPath} clipRule="evenodd" />
    </svg>
  )
}
