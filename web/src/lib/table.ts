/** Slice list for client-side page (1-based). */
export function pageSlice<T>(list: T[] | null | undefined, page: number, pageSize: number): T[] {
  if (!list?.length) return []
  const p = Math.max(1, page)
  const size = Math.max(1, pageSize)
  const start = (p - 1) * size
  return list.slice(start, start + size)
}
