import { mkdir, readdir, readFile, writeFile } from 'node:fs/promises'
import { basename, join } from 'node:path'

export const FILES_PROTOCOL = '/files/1.0.0'
export const FILES_PUSH_PROTOCOL = '/files/push/1.0.0'
export const OUTBOX_DIR = './outbox'
export const RECEIVED_DIR = './received'

export const safeFileName = (name) => {
  const clean = basename(String(name)).replaceAll(/[^a-zA-Z0-9._ -]/g, '_').trim()
  return clean === '' ? `file-${Date.now()}` : clean
}

export const listOutboxFiles = async () => {
  await mkdir(OUTBOX_DIR, { recursive: true })
  const entries = await readdir(OUTBOX_DIR, { withFileTypes: true })

  return entries
    .filter((entry) => entry.isFile())
    .filter((entry) => !entry.name.startsWith('.'))
    .map((entry) => entry.name)
    .sort((a, b) => a.localeCompare(b))
}

export const readOutboxPayload = async () => {
  const names = await listOutboxFiles()
  const files = []

  for (const name of names) {
    const bytes = await readFile(join(OUTBOX_DIR, name))
    files.push({
      name,
      data: bytes.toString('base64')
    })
  }

  return {
    files,
    createdAt: new Date().toISOString()
  }
}

export const saveReceivedPayload = async (payload) => {
  await mkdir(RECEIVED_DIR, { recursive: true })

  const saved = []
  for (const file of payload.files ?? []) {
    const name = safeFileName(file.name)
    const path = join(RECEIVED_DIR, name)
    await writeFile(path, Buffer.from(file.data, 'base64'))
    saved.push({ name, path })
  }

  return saved
}

export const listReceivedFiles = async () => {
  await mkdir(RECEIVED_DIR, { recursive: true })
  const entries = await readdir(RECEIVED_DIR, { withFileTypes: true })

  return entries
    .filter((entry) => entry.isFile())
    .filter((entry) => !entry.name.startsWith('.'))
    .map((entry) => entry.name)
    .sort((a, b) => a.localeCompare(b))
}

export const readStreamToBuffer = async (stream) => {
  const chunks = []

  for await (const chunk of stream) {
    chunks.push(Buffer.from(chunk.subarray ? chunk.subarray() : chunk))
  }

  return Buffer.concat(chunks)
}
