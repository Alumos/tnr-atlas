import { readdir, readFile, writeFile } from 'node:fs/promises'
import { gzipSync } from 'node:zlib'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'
async function walk(dir) {
  for (const entry of await readdir(dir, { withFileTypes: true })) {
    const path = join(dir, entry.name)
    if (entry.isDirectory()) await walk(path)
    else if (/\.(js|css|html|svg)$/.test(path)) await writeFile(`${path}.gz`, gzipSync(await readFile(path), { level: 9 }))
  }
}
await walk(fileURLToPath(new URL('../dist/', import.meta.url)))
