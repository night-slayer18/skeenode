import { useState, useEffect, useCallback, useRef } from 'react';

interface UseFetchOptions<T> {
  initialData?: T;
  enabled?: boolean;
  refetchInterval?: number;
  retryCount?: number;
  retryDelay?: number;
  onSuccess?: (data: T) => void;
  onError?: (error: Error) => void;
}

interface UseFetchResult<T> {
  data: T | undefined;
  error: Error | null;
  isLoading: boolean;
  isError: boolean;
  refetch: () => Promise<void>;
  isFetching: boolean;
}

/**
 * Custom hook for data fetching with retry, refetch intervals, and visibility awareness
 */
export function useFetch<T>(
  url: string,
  options: UseFetchOptions<T> = {}
): UseFetchResult<T> {
  const {
    initialData,
    enabled = true,
    refetchInterval,
    retryCount = 3,
    retryDelay = 1000,
    onSuccess,
    onError,
  } = options;

  const [data, setData] = useState<T | undefined>(initialData);
  const [error, setError] = useState<Error | null>(null);
  const [isLoading, setIsLoading] = useState(!initialData);
  const [isFetching, setIsFetching] = useState(false);
  
  const retryCountRef = useRef(0);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const fetchData = useCallback(async () => {
    if (!enabled) return;

    setIsFetching(true);
    
    try {
      const response = await fetch(url);
      
      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
      }
      
      const result = await response.json();
      setData(result);
      setError(null);
      retryCountRef.current = 0;
      onSuccess?.(result);
    } catch (err) {
      const error = err instanceof Error ? err : new Error(String(err));
      
      // Retry logic with exponential backoff
      if (retryCountRef.current < retryCount) {
        retryCountRef.current++;
        const delay = retryDelay * Math.pow(2, retryCountRef.current - 1);
        setTimeout(fetchData, delay);
        return;
      }
      
      setError(error);
      onError?.(error);
    } finally {
      setIsLoading(false);
      setIsFetching(false);
    }
  }, [url, enabled, retryCount, retryDelay, onSuccess, onError]);

  // Initial fetch
  useEffect(() => {
    fetchData();
  }, [fetchData]);

  // Refetch interval with visibility awareness
  useEffect(() => {
    if (!refetchInterval || !enabled) return;

    const handleVisibilityChange = () => {
      if (document.hidden) {
        // Clear interval when tab is hidden
        if (intervalRef.current) {
          clearInterval(intervalRef.current);
          intervalRef.current = null;
        }
      } else {
        // Resume when tab becomes visible
        fetchData();
        intervalRef.current = setInterval(fetchData, refetchInterval);
      }
    };

    intervalRef.current = setInterval(fetchData, refetchInterval);
    document.addEventListener('visibilitychange', handleVisibilityChange);

    return () => {
      if (intervalRef.current) {
        clearInterval(intervalRef.current);
      }
      document.removeEventListener('visibilitychange', handleVisibilityChange);
    };
  }, [refetchInterval, enabled, fetchData]);

  return {
    data,
    error,
    isLoading,
    isError: error !== null,
    refetch: fetchData,
    isFetching,
  };
}

/**
 * Wrapper for multiple fetches with combined loading state
 */
export function useMultiFetch<T extends Record<string, unknown>>(
  urls: Record<keyof T, string>,
  options: UseFetchOptions<unknown> = {}
): { data: T; isLoading: boolean; error: Error | null } {
  const results = Object.entries(urls).map(([key, url]) => ({
    key,
    // eslint-disable-next-line react-hooks/rules-of-hooks
    ...useFetch(url, options),
  }));

  const data = results.reduce(
    (acc, { key, data }) => ({ ...acc, [key]: data }),
    {} as T
  );

  const isLoading = results.some((r) => r.isLoading);
  const error = results.find((r) => r.error)?.error ?? null;

  return { data, isLoading, error };
}
