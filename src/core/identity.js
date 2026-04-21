import { generateKeyPair, privateKeyFromProtobuf, privateKeyToProtobuf } from '@libp2p/crypto/keys'
import { mkdir, readFile, writeFile } from 'node:fs/promises'
import { dirname, join } from 'node:path'
import { homedir } from 'node:os'

export const getDefaultKeyPath = () => {
  return process.env.P2PTEST_KEY_PATH ?? join(homedir(), '.p2ptest', 'sender.key')
}

export const loadOrCreatePrivateKey = async (keyPath = getDefaultKeyPath()) => {
  try {
    const keyBytes = await readFile(keyPath)
    return {
      privateKey: privateKeyFromProtobuf(keyBytes),
      keyPath,
      created: false
    }
  } catch (err) {
    if (err.code !== 'ENOENT') {
      throw err
    }
  }

  const privateKey = await generateKeyPair('Ed25519')
  await mkdir(dirname(keyPath), { recursive: true })
  await writeFile(keyPath, privateKeyToProtobuf(privateKey), { mode: 0o600 })

  return {
    privateKey,
    keyPath,
    created: true
  }
}
