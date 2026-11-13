// Contract test double only: saves original uploads; serves a fixed WebP fixture.
// Does not implement conversion. Real oneimg image processing needs deployment QA.
// Never used by compose.yaml or the Docker image.
import http from 'node:http'
import { Readable } from 'node:stream'
import { mkdir, readFile, writeFile } from 'node:fs/promises'
import { basename, join } from 'node:path'
if (process.env.TNR_PREVIEW_MOCK !== '1') throw new Error('Set TNR_PREVIEW_MOCK=1 only for local QA')
if (!process.env.TNR_PREVIEW_FIXTURE) throw new Error('Set TNR_PREVIEW_FIXTURE to a local WebP test fixture')
const fixture = await readFile(process.env.TNR_PREVIEW_FIXTURE)
if (fixture.toString('ascii', 0, 4) !== 'RIFF' || fixture.toString('ascii', 8, 12) !== 'WEBP') throw new Error('Fixture must be WebP')
const dir = process.env.TNR_PREVIEW_IMAGES || '/tmp/tnr-preview-images'
await mkdir(dir, { recursive: true })
let next = Date.now()
const server = http.createServer(async (req, res) => {
  try {
    if (req.method === 'POST' && req.url === '/api/upload/images') {
      const input = new Request('http://127.0.0.1:8091/api/upload/images', { method: 'POST', headers: req.headers, body: Readable.toWeb(req), duplex: 'half' })
      const form = await input.formData()
      const file = form.get('images[]')
      if (!file || typeof file === 'string') { res.writeHead(400).end(); return }
      const id = ++next
      await writeFile(join(dir, `${id}-${basename(file.name)}`), Buffer.from(await file.arrayBuffer()))
      // Official local-storage uploads return relative paths. Configure TNR with
      // ONEIMG_PUBLIC_ORIGIN=http://127.0.0.1:8091 when using this test double.
      res.writeHead(200, { 'content-type': 'application/json' }).end(JSON.stringify({ code: 200, data: { files: [{ success: true, id, folder_id: Number(form.get('folder_id') || 0), mime_type: 'image/webp', width: 300, height: 300, url: `/images/${id}.webp`, thumbnail_url: `/images/${id}-thumb.webp` }] } }))
    } else if (req.method === 'GET' && /^\/images\/\d+(-thumb)?\.webp$/.test(req.url || '')) {
      res.writeHead(200, { 'content-type': 'image/webp', 'cache-control': 'public,max-age=3600' }).end(fixture)
    } else res.writeHead(404).end()
  } catch { res.writeHead(500).end() }
})
server.listen(8091, '127.0.0.1', () => console.log('Local image-host test double: http://127.0.0.1:8091'))
