import { useEffect, useRef, useState } from 'react'
import type { FormEvent } from 'react'
import { Camera, Check, ImagePlus, LoaderCircle, LockKeyhole, Upload, X } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { Field, FieldDescription, FieldError, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Select, SelectContent, SelectGroup, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { api, colors } from '@/lib/api'
import type { Cat } from '@/lib/api'

type Props = { open: boolean; onOpenChange: (open: boolean) => void; onSaved: () => void }
export default function CatForm({ open, onOpenChange, onSaved }: Props) {
  const [authenticated, setAuthenticated] = useState(false)
  const [sessionLoading, setSessionLoading] = useState(true)
  const [ready, setReady] = useState(false)
  const [code, setCode] = useState('')
  const [loginMode,setLoginMode]=useState('volunteer')
  const [username,setUsername]=useState('')
  const [identity,setIdentity]=useState('')
  const [role,setRole]=useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [files, setFiles] = useState<File[]>([])
  const [previews, setPreviews] = useState<string[]>([])
  const [sex, setSex] = useState('')
  const [status, setStatus] = useState('')
  const [color, setColor] = useState('')
  const [invalid, setInvalid] = useState(false)
  const input = useRef<HTMLInputElement>(null)
  const form = useRef<HTMLFormElement>(null)
  useEffect(() => { const urls = files.map(f => URL.createObjectURL(f)); setPreviews(urls); return () => urls.forEach(URL.revokeObjectURL) }, [files])
  useEffect(() => { if (!open) return; let active = true; setSessionLoading(true); api<{ authenticated: boolean; uploadsReady: boolean;role:string;name:string }>('/session').then(s => { if (active) { setAuthenticated(s.authenticated); setReady(s.uploadsReady);setRole(s.role);setIdentity(s.name); setError('') } }).catch(e => { if (active) setError(e.message) }).finally(() => { if (active) setSessionLoading(false) }); return () => { active = false } }, [open])
  function addFiles(incoming: File[]) {
    setError('')
    if (files.length + incoming.length > 6) { setError('每份相册最多 6 张照片'); return }
    if (incoming.some(f => !['image/jpeg', 'image/png', 'image/webp'].includes(f.type) || f.size > 10 * 1024 * 1024)) { setError('请选择 10 MB 以内的 JPEG、PNG 或 WebP 照片'); return }
    if ([...files, ...incoming].reduce((n, f) => n + f.size, 0) > 50 * 1024 * 1024) { setError('相册总大小不能超过 50 MB'); return }
    setFiles([...files, ...incoming])
  }
  async function login(e: FormEvent) {
    e.preventDefault(); setBusy(true); setError('')
    try { const session=await api<{name:string}>('/session', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ code,username:loginMode==='volunteer'?username:'' }) }); setAuthenticated(true);setRole(loginMode==='volunteer'?'volunteer':'shared');setIdentity(session.name); setCode('') } catch (e) { setError((e as Error).message) } finally { setBusy(false) }
  }
  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault(); setInvalid(true); setError('')
    if (!sex || !status || !color || !files.length) { setError('请完成所有必填项，并添加至少一张照片'); return }
    setBusy(true)
    const data = new FormData(e.currentTarget);if(role==='volunteer')data.set('uploader',identity); data.set('sex', sex); data.set('neutered', status); data.set('color', color); files.forEach(f => data.append('photos', f))
    try { const cat = await api<Cat>('/cats', { method: 'POST', body: data }); toast.success(`${cat.name}的档案已建立`); form.current?.reset(); setSex(''); setStatus(''); setColor(''); setFiles([]); setInvalid(false); onOpenChange(false); onSaved() }
    catch (e) { const message=(e as Error).message;setError(message);if(message.includes('请先'))setAuthenticated(false) } finally { setBusy(false) }
  }
  return <Sheet open={open} onOpenChange={v => { if (!busy) onOpenChange(v) }}><SheetContent className="entry-sheet" onInteractOutside={e => { if (busy) e.preventDefault() }} onEscapeKeyDown={e => { if (busy) e.preventDefault() }}>
    <SheetHeader><div className="sheet-symbol"><Camera /></div><SheetTitle>认识一位新朋友</SheetTitle><SheetDescription>一张照片，一份记录，让每一份关爱有迹可循。</SheetDescription></SheetHeader>
    <div className="entry-body">
      {sessionLoading ? <div className="loading-inline"><LoaderCircle className="animate-spin size-5" />正在连接档案库…</div> : !authenticated ? <form onSubmit={login}><FieldGroup><Alert><LockKeyhole /><AlertTitle>志愿者入口</AlertTitle><AlertDescription>浏览档案无需登录。录入前，请使用管理员提供的志愿者账号或公共访问码。</AlertDescription></Alert><Field><FieldLabel id="login-method">录入方式</FieldLabel><ToggleGroup type="single" variant="outline" value={loginMode} aria-labelledby="login-method" onValueChange={v=>{if(v)setLoginMode(v)}}><ToggleGroupItem value="volunteer">志愿者账号</ToggleGroupItem><ToggleGroupItem value="shared">公共访问码</ToggleGroupItem></ToggleGroup></Field>{loginMode==='volunteer'&&<Field><FieldLabel htmlFor="volunteer-login">志愿者账号</FieldLabel><Input id="volunteer-login" autoComplete="username" value={username} onChange={e=>setUsername(e.target.value)} required maxLength={32}/></Field>}<Field data-invalid={!!error}><FieldLabel htmlFor="access-code">志愿者访问码</FieldLabel><Input id="access-code" type="password" autoComplete="current-password" required value={code} onChange={e => setCode(e.target.value)} aria-invalid={!!error} /></Field>{error && <FieldError role="alert">{error}</FieldError>}<Button disabled={busy} type="submit">{busy && <LoaderCircle data-icon="inline-start" className="animate-spin" />}进入快速录入</Button></FieldGroup></form> : <form ref={form} onSubmit={submit}><FieldGroup>
        {!ready && <Alert><AlertTitle>图床尚未配置</AlertTitle><AlertDescription>请联系管理员完成 oneimg 配置后再保存档案。</AlertDescription></Alert>}
        <Field><FieldLabel htmlFor="cat-name">猫咪名字 *</FieldLabel><Input id="cat-name" name="name" placeholder="给它一个温柔的名字" maxLength={40} required autoComplete="off" /></Field>
        <Field data-invalid={invalid && !sex}><FieldLabel id="sex-label">性别 *</FieldLabel><ToggleGroup type="single" variant="outline" value={sex} onValueChange={setSex} aria-labelledby="sex-label" aria-invalid={invalid && !sex}>{['公', '母', '未知'].map(s => <ToggleGroupItem key={s} value={s}>{s}</ToggleGroupItem>)}</ToggleGroup></Field>
        <Field data-invalid={invalid && !color}><FieldLabel htmlFor="cat-color">毛色 *</FieldLabel><Select value={color} onValueChange={setColor}><SelectTrigger id="cat-color" className="w-full" aria-invalid={invalid && !color}><SelectValue placeholder="选择它的毛色" /></SelectTrigger><SelectContent><SelectGroup>{colors.map(c => <SelectItem key={c} value={c}>{c}</SelectItem>)}</SelectGroup></SelectContent></Select><FieldDescription>没有合适的颜色？可以先选择「无法定义」。</FieldDescription></Field>
        <Field data-invalid={invalid && !status}><FieldLabel id="status-label">是否绝育 *</FieldLabel><ToggleGroup type="single" variant="outline" value={status} onValueChange={setStatus} aria-labelledby="status-label" aria-invalid={invalid && !status}>{['已绝育', '未绝育', '未知'].map(s => <ToggleGroupItem key={s} value={s}>{s}</ToggleGroupItem>)}</ToggleGroup></Field>
        <Field data-invalid={invalid && !files.length}><FieldLabel htmlFor="photos">相册 *</FieldLabel><input ref={input} id="photos" className="sr-only" type="file" multiple accept="image/jpeg,image/png,image/webp" onChange={e => { addFiles(Array.from(e.target.files || [])); e.target.value = '' }} />
          <button className="upload-zone" type="button" onClick={() => input.current?.click()} onDragOver={e => e.preventDefault()} onDrop={e => { e.preventDefault(); addFiles(Array.from(e.dataTransfer.files)) }}><ImagePlus /><strong>点击选择，或拖入照片</strong><span>JPEG、PNG、WebP · 每张最多 10 MB</span></button>
          {files.length > 0 && <div className="photo-previews">{files.map((f, i) => <div key={`${f.name}-${i}`}><img src={previews[i]} alt={`待上传照片 ${i + 1}`} /><Button type="button" variant="secondary" size="icon" aria-label={`移除照片 ${i + 1}`} onClick={() => setFiles(files.filter((_, n) => n !== i))}><X /></Button></div>)}</div>}
          <FieldDescription>已选 {files.length}/6 张。首张作为封面，照片会自动优化。</FieldDescription></Field>
        <Field><FieldLabel htmlFor="cat-description">补充描述 <span className="optional">选填</span></FieldLabel><Textarea id="cat-description" name="description" placeholder="它的性格、身体状况，或你想记录的小故事…" maxLength={2000} rows={3} /></Field>
        <Field><FieldLabel htmlFor="uploader">上传人 *</FieldLabel><Input id="uploader" name="uploader" placeholder="你的名字或志愿者昵称" maxLength={40} required autoComplete="nickname" defaultValue={role==='volunteer'?identity:''} readOnly={role==='volunteer'} /><FieldDescription>请使用昵称，上传人和描述会公开展示。</FieldDescription></Field>
        {error && <Alert variant="destructive" role="alert"><AlertTitle>暂时无法保存</AlertTitle><AlertDescription>{error}</AlertDescription></Alert>}
        <div className="entry-actions"><p>{busy ? '照片正在上传和处理，请保持页面打开…' : '让记录，成为照顾它的第一步。'}</p><Button className="w-full" size="lg" type="submit" disabled={busy || !ready}>{busy ? <LoaderCircle data-icon="inline-start" className="animate-spin" /> : <Check data-icon="inline-start" />}{busy ? '正在保存档案…' : '建立猫咪档案'}</Button><Button type="button" variant="ghost" disabled={busy} onClick={() => onOpenChange(false)}>稍后再录入</Button></div>
      </FieldGroup></form>}
      <div className="entry-note"><Upload />照片经过优化后保存到社区图床</div>{authenticated&&<Button className="mt-4 w-full" variant="ghost" size="sm" disabled={busy} onClick={async()=>{try{await api('/session',{method:'DELETE'});setAuthenticated(false)}catch(e){setError((e as Error).message)}}}>退出当前录入身份</Button>}
    </div>
  </SheetContent></Sheet>
}
