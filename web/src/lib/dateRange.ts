export type DateRangePreset = 'today' | '7d' | '30d' | 'all' | 'custom'

/** 数据库里 started_at 存的是 UTC 的 RFC3339，所以本地日期必须先转成 UTC 时刻。 */
export function dateInputToRFC3339(dateStr: string, endOfDay: boolean): string {
  if (!dateStr) return ''
  const [y, m, d] = dateStr.split('-').map(Number)
  if (!y || !m || !d) return ''
  const dt = endOfDay
    ? new Date(y, m - 1, d, 23, 59, 59, 999)
    : new Date(y, m - 1, d, 0, 0, 0, 0)
  return dt.toISOString()
}

/** 把预设区间换算成 from/to；'all' 返回两个空串，即不筛选。 */
export function presetRange(
  preset: DateRangePreset,
  now: Date = new Date(),
): { from: string; to: string } {
  if (preset === 'all' || preset === 'custom') return { from: '', to: '' }
  const start = new Date(now.getFullYear(), now.getMonth(), now.getDate(), 0, 0, 0, 0)
  if (preset === '7d') start.setDate(start.getDate() - 6)
  if (preset === '30d') start.setDate(start.getDate() - 29)
  return { from: start.toISOString(), to: '' }
}
