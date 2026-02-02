interface PaginationProps {
  page: number
  totalPages: number
  onPageChange: (page: number) => void
}

const btnClass = `px-4 py-2 text-sm font-medium rounded-lg
  bg-panel dark:bg-panel-dark border border-border dark:border-border-dark
  disabled:opacity-40 disabled:cursor-not-allowed
  hover:border-accent/30 transition-colors`

export function Pagination({ page, totalPages, onPageChange }: PaginationProps) {
  if (totalPages <= 1) return null

  return (
    <div className="flex items-center justify-between mt-6">
      <button
        onClick={() => onPageChange(Math.max(1, page - 1))}
        disabled={page <= 1}
        className={btnClass}
      >
        Previous
      </button>
      <span className="text-sm text-muted dark:text-muted-dark font-mono">
        {page} / {totalPages}
      </span>
      <button
        onClick={() => onPageChange(Math.min(totalPages, page + 1))}
        disabled={page >= totalPages}
        className={btnClass}
      >
        Next
      </button>
    </div>
  )
}
