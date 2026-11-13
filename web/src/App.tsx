import { lazy, Suspense, useEffect, useState } from 'react'
import { useTheme } from 'next-themes'
import { ArrowDown, ArrowRight, Cat as CatIcon, Check, ChevronLeft, ChevronRight, CircleHelp, Heart, LayoutGrid, List, Maximize2, Minimize2, Moon, PawPrint, Plus, RefreshCw, Search, ShieldCheck, SlidersHorizontal, Sun, Users, X } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectGroup, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { Skeleton } from '@/components/ui/skeleton'
import { Separator } from '@/components/ui/separator'
import { Empty, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle, EmptyContent } from '@/components/ui/empty'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Progress } from '@/components/ui/progress'
import { Toaster } from '@/components/ui/sonner'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { api, colors, dateLabel } from '@/lib/api'
import type { Cat, Stats } from '@/lib/api'
import { cn } from '@/lib/utils'
import { demoCats } from '@/lib/demo'
const CatForm = lazy(() => import('@/components/cat-form'))
const CatDetail = lazy(() => import('@/components/cat-detail'))
const demo = new URLSearchParams(location.search).get('demo') === '1'

function CatCard({ cat, onOpen }: { cat: Cat; onOpen: () => void }) {
  const [failed, setFailed] = useState(false)
  return <Card className="cat-card"><button className="cat-cover" onClick={onOpen} aria-label={`查看${cat.name}的档案`}>{failed ? <div className="image-fallback"><CatIcon /><span>照片暂时无法显示</span></div> : <img src={cat.photos[0]?.thumbnail} alt={cat.name} width={640} height={480} loading="lazy" decoding="async" onError={() => setFailed(true)} />}
    <Badge className="photo-status" variant={cat.neutered === '已绝育' ? 'secondary' : 'default'}>{cat.neutered === '已绝育' ? <Check /> : <Heart />}{cat.neutered}</Badge><span className="cover-action"><ArrowRight /></span></button>
    <CardHeader><div className="cat-title-row"><CardTitle><button onClick={onOpen}>{cat.name}</button></CardTitle><span className="cat-sex">{cat.sex === '公' ? '♂' : cat.sex === '母' ? '♀' : '?'}</span></div><CardDescription>{cat.color}<span className="description-dot">·</span>档案 #{String(cat.id).padStart(4, '0')}</CardDescription></CardHeader>
    <CardContent><p className="cat-description">{cat.description || '它的故事，等待下一次相遇。'}</p></CardContent>
    <CardFooter><span className="uploader-avatar">{cat.uploader.slice(0, 1)}</span><span>{cat.uploader}</span><time dateTime={cat.createdAt}>{dateLabel(cat.createdAt)}</time></CardFooter>
  </Card>
}

export default function App() {
  const { resolvedTheme, setTheme } = useTheme()
  const [entry, setEntry] = useState(false)
  const [selected, setSelected] = useState<Cat | null>(null)
  const [about, setAbout] = useState(false)
  const [screen, setScreen] = useState(false)
  const [view, setView] = useState('grid')
  const [query, setQuery] = useState('')
  const [debounced, setDebounced] = useState('')
  const [status, setStatus] = useState('all')
  const [color, setColor] = useState('all')
  const [sex, setSex] = useState('all')
  const [showFilters, setShowFilters] = useState(false)
  const [page, setPage] = useState(1)
  const [cats, setCats] = useState<Cat[]>([])
  const [total, setTotal] = useState(0)
  const [stats, setStats] = useState<Stats | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [revision, setRevision] = useState(0)
  useEffect(() => { const t = setTimeout(() => setDebounced(query), 250); return () => clearTimeout(t) }, [query])
  useEffect(() => { const onFullScreen = () => setScreen(!!document.fullscreenElement); document.addEventListener('fullscreenchange', onFullScreen); return () => document.removeEventListener('fullscreenchange', onFullScreen) }, [])
  useEffect(() => { if (demo) { setStats({ total: 8, neutered: 5, pending: 3, unknown: 0, contributors: 3 }); return }; const controller = new AbortController(); api<Stats>('/stats', { signal: controller.signal }).then(setStats).catch(() => {}); return () => controller.abort() }, [revision])
  useEffect(() => {
    const controller = new AbortController(); setLoading(true); setError('')
    if (demo) { const list = demoCats.filter(c => (status === 'all' || c.neutered === status) && (color === 'all' || c.color === color) && (sex === 'all' || c.sex === sex) && `${c.name}${c.uploader}${c.description}`.includes(debounced)); setCats(list); setTotal(list.length); setLoading(false); return }
    const params = new URLSearchParams({ page: String(page), q: debounced }); if (status !== 'all') params.set('status', status); if (color !== 'all') params.set('color', color); if (sex !== 'all') params.set('sex', sex)
    api<{ items: Cat[]; total: number }>(`/cats?${params}`, { signal: controller.signal }).then(r => { setCats(r.items); setTotal(r.total) }).catch(e => { if (!controller.signal.aborted) setError(e.message) }).finally(() => { if (!controller.signal.aborted) setLoading(false) }); return () => controller.abort()
  }, [debounced, status, color, sex, page, revision])
  // Display mode refreshes data without moving focus or interrupting an open record.
  useEffect(() => { if (!screen || demo) return; const timer = setInterval(() => setRevision(r => r + 1), 60000); return () => clearInterval(timer) }, [screen])
  function newEntry() { if (demo) { toast.info('当前为演示预览，请返回真实档案库后录入'); return }; setEntry(true) }
  async function fullscreen() { try { if (document.fullscreenElement) await document.exitFullscreen(); else await document.documentElement.requestFullscreen() } catch { toast.error('此浏览器暂不支持全屏，请使用浏览器的全屏功能') } }
  function resetFilters() { setQuery(''); setStatus('all'); setColor('all'); setSex('all'); setPage(1) }
  const rate = stats?.total ? Math.round(stats.neutered / stats.total * 100) : 0
  const filtered = query || status !== 'all' || color !== 'all' || sex !== 'all'
  return <div className={cn('app-shell', screen && 'display-mode')}>
    <header className="site-header"><div className="header-inner"><a href="/" className="brand" aria-label="猫咪档案首页"><span className="brand-mark"><PawPrint /></span><strong>猫咪档案</strong><span className="brand-subtitle">TNR Atlas</span></a><nav aria-label="主导航"><a href="#cats" className="nav-active">猫咪总览</a><button onClick={() => setAbout(true)}>关于 TNR</button><a href="/admin">管理后台</a></nav><div className="header-actions"><Button variant="ghost" size="icon" aria-label={resolvedTheme === 'dark' ? '切换浅色主题' : '切换深色主题'} onClick={() => setTheme(resolvedTheme === 'dark' ? 'light' : 'dark')}>{resolvedTheme === 'dark' ? <Sun /> : <Moon />}</Button><Button size="entry-sm" onClick={newEntry}><Plus data-icon="inline-start" />快速录入</Button></div></div></header>
    {demo && <div className="demo-banner">演示预览 · 以下为示例档案，照片来自 Unsplash<a href="/">返回真实档案库 <ArrowRight /></a></div>}
    <main className="main-container">
      <section className="intro"><div className="intro-copy"><Badge variant="outline"><span className="community-dot" />社区流浪猫关爱计划</Badge><h1>每一只猫，都值得被记住。</h1><p>记录相遇，连接善意。<br className="mobile-break" />一起为社区里的它们，留下一份温柔的档案。</p><div className="intro-actions"><Button size="entry" onClick={newEntry}><Plus data-icon="inline-start" />录入新猫咪</Button><Button variant="ghost" asChild><a href="#cats">看看我们的邻居<ArrowDown data-icon="inline-end" /></a></Button></div></div><div className="mission-note"><div className="mission-icon"><Heart /></div><p>从一次相遇<br />到长久的守护</p><span>Trap · Neuter · Return</span><button onClick={() => setAbout(true)}>了解 TNR 计划 <ArrowRight /></button><PawPrint className="mission-paw" /></div></section>
      <section className="stats-strip" aria-label="社区关爱数据"><div className="stat-item"><span className="stat-label"><CatIcon />已建档猫咪</span><strong>{stats?.total ?? '—'}<small>只</small></strong><p>每一个名字，都有一段故事</p></div><div className="stat-item"><span className="stat-label"><ShieldCheck />已完成绝育</span><strong>{stats?.neutered ?? '—'}<small>只</small></strong><p>让关爱以更长远的方式延续</p></div><div className="stat-item"><span className="stat-label"><Heart />等待绝育</span><strong>{stats?.pending ?? '—'}<small>只</small></strong><p>需要我们一起多一点关注</p></div><div className="stat-item progress-stat"><span className="stat-label"><Users />社区绝育进度</span><strong>{stats ? rate : '—'}<small>%</small></strong><Progress value={rate} aria-label={`社区绝育率 ${rate}%`} /><p>{stats?.contributors ?? '—'} 位记录者，让改变正在发生</p></div></section>
      <section id="cats" className="registry"><div className="section-heading"><div><h2>我们的猫咪邻居 <Badge variant="secondary">{stats?.total ?? '—'}</Badge></h2><p>在这里认识它们，也让更多人看见它们。</p></div><div className="overview-actions"><span className="data-hint">{screen ? '每分钟自动刷新' : '每一份记录都有意义'}</span><Button variant="outline" size="sm" onClick={fullscreen}>{screen ? <Minimize2 data-icon="inline-start" /> : <Maximize2 data-icon="inline-start" />}{screen ? '退出大屏' : '大屏总览'}</Button></div></div>
        <div className="toolbar"><ToggleGroup type="single" value={status} onValueChange={v => { if (v) { setStatus(v); setPage(1) } }} className="status-tabs" aria-label="按绝育状态筛选">{['all', '已绝育', '未绝育', '未知'].map(v => <ToggleGroupItem key={v} value={v}>{v === 'all' ? '全部猫咪' : v === '未知' ? '待确认' : v}</ToggleGroupItem>)}</ToggleGroup><div className="search-controls"><div className="search-input"><Search /><Input aria-label="搜索猫咪" placeholder="搜索名字、描述、上传人…" maxLength={100} value={query} onChange={e => { setQuery(e.target.value); setPage(1) }} /></div><Button variant={showFilters ? 'secondary' : 'outline'} size="icon" aria-label="更多筛选" aria-expanded={showFilters} onClick={() => setShowFilters(!showFilters)}><SlidersHorizontal /></Button><ToggleGroup type="single" variant="outline" value={view} onValueChange={v => { if (v) setView(v) }} className="view-toggle" aria-label="切换排列方式"><ToggleGroupItem value="grid" aria-label="网格视图"><LayoutGrid /></ToggleGroupItem><ToggleGroupItem value="list" aria-label="紧凑视图"><List /></ToggleGroupItem></ToggleGroup></div></div>
        {showFilters && <div className="filter-panel"><Select value={color} onValueChange={v => { setColor(v); setPage(1) }}><SelectTrigger aria-label="按毛色筛选"><SelectValue /></SelectTrigger><SelectContent><SelectGroup><SelectItem value="all">全部毛色</SelectItem>{colors.map(c => <SelectItem key={c} value={c}>{c}</SelectItem>)}</SelectGroup></SelectContent></Select><Select value={sex} onValueChange={v => { setSex(v); setPage(1) }}><SelectTrigger aria-label="按性别筛选"><SelectValue /></SelectTrigger><SelectContent><SelectGroup>{['all', '公', '母', '未知'].map(v => <SelectItem key={v} value={v}>{v === 'all' ? '全部性别' : v}</SelectItem>)}</SelectGroup></SelectContent></Select>{filtered && <Button variant="ghost" size="sm" onClick={resetFilters}><X data-icon="inline-start" />清除筛选</Button>}</div>}
        <div aria-live="polite" className="results-caption"><span>{loading ? '正在寻找猫咪邻居…' : `共 ${total} 份${filtered ? '符合条件的' : ''}档案`}</span><span>最近建档优先</span></div>
        {error ? <Alert variant="destructive"><CircleHelp /><AlertTitle>档案暂时无法加载</AlertTitle><AlertDescription>{error}<Button variant="outline" size="sm" onClick={() => setRevision(r => r + 1)}><RefreshCw data-icon="inline-start" />重新加载</Button></AlertDescription></Alert> : loading ? <div className="cat-grid" aria-label="正在加载档案">{Array.from({ length: 4 }, (_, i) => <Card key={i} className="cat-card"><Skeleton className="aspect-[4/3] w-full" /><CardHeader><Skeleton className="h-5 w-20" /><Skeleton className="h-4 w-32" /></CardHeader><CardContent><Skeleton className="h-4 w-full" /></CardContent><CardFooter><Skeleton className="h-5 w-full" /></CardFooter></Card>)}</div> : cats.length ? <div className={cn('cat-grid', view === 'list' && 'compact-grid')}>{cats.map(cat => <CatCard key={cat.id} cat={cat} onOpen={() => setSelected(cat)} />)}</div> : <Empty className="registry-empty"><EmptyHeader><EmptyMedia variant="icon"><PawPrint /></EmptyMedia><EmptyTitle>{filtered ? '还没有找到这位小邻居' : '第一份相遇，等你来记录'}</EmptyTitle><EmptyDescription>{filtered ? '试试其他名字，或清除筛选看看全部猫咪。' : '从一张照片开始，为社区里的猫咪建立第一份档案。'}</EmptyDescription></EmptyHeader><EmptyContent><Button onClick={filtered ? resetFilters : newEntry}>{filtered ? '清除筛选' : '记录第一只猫咪'}</Button>{!filtered && <Button variant="link" asChild><a href="/?demo=1">先看看演示预览</a></Button>}</EmptyContent></Empty>}
        {total > 24 && <div className="pagination"><Button variant="outline" disabled={page === 1} onClick={() => { setPage(page - 1); document.getElementById('cats')?.scrollIntoView() }}><ChevronLeft data-icon="inline-start" />上一页</Button><span>{page} / {Math.ceil(total / 24)}</span><Button variant="outline" disabled={page >= Math.ceil(total / 24)} onClick={() => { setPage(page + 1); document.getElementById('cats')?.scrollIntoView() }}>下一页<ChevronRight data-icon="inline-end" /></Button></div>}
      </section>
      <section className="care-note"><Heart /><p>我们记录的不只是信息，<strong>也是它们被看见、被照顾的开始。</strong></p><Button variant="ghost" size="sm" onClick={newEntry}>一起参与<ArrowRight data-icon="inline-end" /></Button></section>
    </main><footer className="site-footer"><Separator /><div className="footer-inner"><span><PawPrint />猫咪档案 <span className="text-muted-foreground">TNR Atlas</span></span><p>以记录连接关爱，让社区对它们更友好。</p><a href="/admin" className="footer-admin">管理后台 <ArrowRight /></a></div></footer>
    <Suspense fallback={null}>{entry && <CatForm open={entry} onOpenChange={setEntry} onSaved={() => { setPage(1); setRevision(r => r + 1) }} />}{selected && <CatDetail key={selected.id} cat={selected} onClose={() => setSelected(null)} />}</Suspense>
    <Dialog open={about} onOpenChange={setAbout}><DialogContent><DialogHeader><DialogTitle>让社区，对它们更友好</DialogTitle><DialogDescription>TNR：Trap · Neuter · Return</DialogDescription></DialogHeader><div className="about-content"><p>TNR 是捕捉、绝育、放归的简称。由有经验的志愿者与兽医合作，在保障动物福利的前提下，为社区猫实施绝育，并放归熟悉的生活环境。</p><p>这份档案库帮助大家识别猫咪、记录绝育状态，减少重复工作。未知的信息可以先标为「未知」，以后相遇时再确认。</p><Alert><Heart /><AlertTitle>温柔靠近，负责任地记录</AlertTitle><AlertDescription>请勿公开敏感位置、联系方式或个人信息。不要仅凭照片判断猫咪是否绝育，需要时请向当地志愿团队或兽医确认。</AlertDescription></Alert></div></DialogContent></Dialog><Toaster position="top-center" />
  </div>
}
