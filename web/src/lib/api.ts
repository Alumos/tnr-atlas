export type Photo = { url: string; thumbnail: string; width: number; height: number }
export type Cat = { albumKey:string; id: number; name: string; sex: string; color: string; neutered: string; description: string; uploader: string; photos: Photo[]; createdAt: string }
export type Stats = { total: number; neutered: number; pending: number; unknown: number; contributors: number }
export const colors = ['橘猫', '狸花', '三花', '奶牛', '黑猫', '白猫', '玳瑁', '灰猫', '橘白', '无法定义']
export async function api<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`/api${path}`, options)
  const data = await res.json().catch(() => ({ error: '服务响应异常，请稍后重试' }))
  if (!res.ok) throw new Error(data.error || '请求失败，请稍后重试')
  return data as T
}
export const dateLabel = (date: string) => new Intl.DateTimeFormat('zh-CN', { month: 'long', day: 'numeric' }).format(new Date(date))
