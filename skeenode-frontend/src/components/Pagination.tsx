interface PaginationProps {
  currentPage: number;
  totalPages: number;
  onPageChange: (page: number) => void;
  pageSize?: number;
  onPageSizeChange?: (size: number) => void;
  pageSizeOptions?: number[];
  loading?: boolean;
}

/**
 * Pagination component with page size selection
 */
export function Pagination({
  currentPage,
  totalPages,
  onPageChange,
  pageSize = 10,
  onPageSizeChange,
  pageSizeOptions = [10, 25, 50, 100],
  loading = false,
}: PaginationProps) {
  const handlePrevious = () => {
    if (currentPage > 1) {
      onPageChange(currentPage - 1);
    }
  };

  const handleNext = () => {
    if (currentPage < totalPages) {
      onPageChange(currentPage + 1);
    }
  };

  // Generate page numbers to show
  const getPageNumbers = (): (number | string)[] => {
    const pages: (number | string)[] = [];
    const showPages = 5;
    
    if (totalPages <= showPages + 2) {
      for (let i = 1; i <= totalPages; i++) pages.push(i);
    } else {
      pages.push(1);
      
      if (currentPage > 3) pages.push('...');
      
      const start = Math.max(2, currentPage - 1);
      const end = Math.min(totalPages - 1, currentPage + 1);
      
      for (let i = start; i <= end; i++) pages.push(i);
      
      if (currentPage < totalPages - 2) pages.push('...');
      
      pages.push(totalPages);
    }
    
    return pages;
  };

  if (totalPages <= 1) return null;

  return (
    <div className="pagination">
      <div className="pagination-controls">
        <button
          onClick={handlePrevious}
          disabled={currentPage === 1 || loading}
          className="pagination-btn"
          aria-label="Previous page"
        >
          ←
        </button>
        
        {getPageNumbers().map((page, idx) =>
          typeof page === 'number' ? (
            <button
              key={idx}
              onClick={() => onPageChange(page)}
              disabled={loading}
              className={`pagination-btn ${page === currentPage ? 'active' : ''}`}
              aria-label={`Page ${page}`}
              aria-current={page === currentPage ? 'page' : undefined}
            >
              {page}
            </button>
          ) : (
            <span key={idx} className="pagination-ellipsis">
              {page}
            </span>
          )
        )}
        
        <button
          onClick={handleNext}
          disabled={currentPage === totalPages || loading}
          className="pagination-btn"
          aria-label="Next page"
        >
          →
        </button>
      </div>
      
      {onPageSizeChange && (
        <div className="pagination-size">
          <label htmlFor="page-size">Per page:</label>
          <select
            id="page-size"
            value={pageSize}
            onChange={(e) => onPageSizeChange(Number(e.target.value))}
            disabled={loading}
          >
            {pageSizeOptions.map((size) => (
              <option key={size} value={size}>
                {size}
              </option>
            ))}
          </select>
        </div>
      )}
    </div>
  );
}

export default Pagination;
