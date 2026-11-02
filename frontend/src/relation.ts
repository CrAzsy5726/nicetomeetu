export function relationLevel(f: number): string {
  if (f === 0) return '陌生人'
  if (f < 5) return '初识'
  if (f < 15) return '熟络'
  if (f < 30) return '朋友'
  return '知己'
}

export function levelRank(level: string): number {
  return ['陌生人', '初识', '熟络', '朋友', '知己'].indexOf(level)
}

// 关系星座的点亮星数（0~5）
export function relationStars(f: number): number {
  const map = [0, 1, 2, 3, 5]
  return map[levelRank(relationLevel(f))] ?? 0
}
