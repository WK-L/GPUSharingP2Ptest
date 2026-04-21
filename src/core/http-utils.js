export const sendJson = (res, statusCode, value) => {
  const body = JSON.stringify(value)
  res.writeHead(statusCode, {
    'content-type': 'application/json; charset=utf-8',
    'content-length': Buffer.byteLength(body)
  })
  res.end(body)
}

export const sendHtml = (res, html) => {
  res.writeHead(200, { 'content-type': 'text/html; charset=utf-8' })
  res.end(html)
}

export const readJsonBody = async (req, maxBytes = 100 * 1024 * 1024) => {
  const chunks = []
  let size = 0

  for await (const chunk of req) {
    size += chunk.length
    if (size > maxBytes) {
      throw new Error('Request body is too large')
    }

    chunks.push(chunk)
  }

  if (chunks.length === 0) {
    return {}
  }

  return JSON.parse(Buffer.concat(chunks).toString('utf8'))
}
