import { useState } from 'react'
import { ArrowLeft, ArrowRight, CalendarDays, Camera, Heart, UserRound } from 'lucide-react'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import type { Cat } from '@/lib/api'
import { dateLabel } from '@/lib/api'
export default function CatDetail({ cat, onClose }: { cat: Cat; onClose: () => void }) {
  const [index, setIndex] = useState(0)
  const photo = cat.photos[index]
  return <Dialog open onOpenChange={v => { if (!v) onClose() }}><DialogContent className="detail-dialog"><div className="detail-photo"><img key={photo.url} src={photo.url} alt={`${cat.name}的照片 ${index + 1}`} width={photo.width} height={photo.height} />{cat.photos.length > 1 && <div className="gallery-controls"><Button size="icon" variant="secondary" aria-label="上一张照片" onClick={() => setIndex((index + cat.photos.length - 1) % cat.photos.length)}><ArrowLeft /></Button><Badge variant="secondary">{index + 1} / {cat.photos.length}</Badge><Button size="icon" variant="secondary" aria-label="下一张照片" onClick={() => setIndex((index + 1) % cat.photos.length)}><ArrowRight /></Button></div>}</div>
    <div className="detail-body"><DialogHeader><div className="detail-heading"><DialogTitle>{cat.name}</DialogTitle><Badge variant={cat.neutered === '已绝育' ? 'secondary' : 'outline'}>{cat.neutered === '已绝育' && <Heart />}{cat.neutered}</Badge></div><DialogDescription>档案 #{String(cat.id).padStart(4, '0')} · {cat.sex} · {cat.color}</DialogDescription></DialogHeader><Separator />{cat.neutered === '已绝育' && <div className="detail-neutered-date"><CalendarDays /><div><span>绝育日期</span>{cat.neuteredAt ? <time dateTime={cat.neuteredAt}>{cat.neuteredAt.replaceAll('-', ' / ')}</time> : <p>日期待补充</p>}</div></div>}<p className="detail-story">{cat.description || '关于它的故事，等待下一次相遇时补充。'}</p><div className="detail-meta"><span><UserRound />{cat.uploader}</span><span><CalendarDays />{dateLabel(cat.createdAt)}建档</span><span><Camera />{cat.photos.length} 张照片</span></div></div>
  </DialogContent></Dialog>
}
