import { Icon } from './Icon'

/**
 * COSCA FILE ICON — resolve um ícone semântico para um nó de arquivo/pasta.
 * ---------------------------------------------------------------------------
 * Fonte de verdade do tipo: NÃO adivinha pelo nome sozinho. Usa a tríade já
 * resolvida pelo backend (`is_dir`, `language`, `name`):
 *
 *   is_dir            → folder / folder-open        (não depende do nome)
 *   nome especial     → package.json / Dockerfile / README / .gitignore / .env / go.mod
 *   language          → typescript/go/tsx → file-code · json → file-json
 *                         markdown/yaml → file-text · css → file-type
 *                         sql → database · shell → terminal-file
 *                         image → file-image · video/audio/archive → específico
 *   fallback          → file
 *
 * Principio (o professor): a pasta parece pasta; go parece go; json parece json;
 * binário não finge ser código. Sem inventar centenas de tipos — começamos pelos
 * tipos realmente presentes no COSCA.
 */
export function FileIcon({ node, expanded = false, size = 14 }: {
  node: { is_dir?: boolean; name?: string; language?: string; ext?: string }
  expanded?: boolean
  size?: number
}) {
  if (node.is_dir) {
    return <Icon name={expanded ? 'folder-open' : 'folder'} size={size} />
  }

  const n = node.name ?? ''

  // NOMES ESPECIAIS (ex.: package.json, Dockerfile, go.mod, .env, README)
  if (n === 'package.json' || n === 'tsconfig.json' || n === 'jsconfig.json' ||
      n === 'composer.json' || n === 'cypress.json' || n === 'pnpm-workspace.yaml') {
    return <Icon name="file-json" size={size} />
  }
  if (n === 'Dockerfile' || n === '.gitignore' || n === '.env' || n === 'go.mod' ||
      n === 'Makefile' || n === '.dockerignore' || n === 'Cargo.toml') {
    return <Icon name="file-cog" size={size} />
  }
  if (/^readme/i.test(n) || n === 'CHANGELOG' || n === 'LICENSE') {
    return <Icon name="file-text" size={size} />
  }

  // POR LINGUAGEM (resolvida pelo backend `resolveLanguage`)
  const L = node.language ?? ''
  if (['typescript', 'javascript', 'tsx', 'jsx', 'go', 'rust', 'python', 'java',
       'c', 'cpp', 'csharp', 'html', 'swift', 'kotlin', 'php', 'ruby', 'dart'].includes(L)) {
    return <Icon name="file-code" size={size} />
  }
  if (L === 'json') return <Icon name="file-json" size={size} />
  if (L === 'yaml' || L === 'toml' || L === 'markdown' || L === 'xml') return <Icon name="file-text" size={size} />
  if (L === 'css') return <Icon name="file-type" size={size} />
  if (L === 'sql') return <Icon name="database" size={size} />
  if (L === 'shell' || L === 'powershell') return <Icon name="terminal-file" size={size} />
  if (L === 'image') return <Icon name="file-image" size={size} />
  if (L === 'video') return <Icon name="file-video" size={size} />
  if (L === 'audio') return <Icon name="file-audio" size={size} />
  if (L === 'archive') return <Icon name="file-archive" size={size} />
  if (L === 'env') return <Icon name="file-cog" size={size} />

  // FALLBACK por extensão simples
  const ext = (node.ext ?? '')
  if (ext === '.md') return <Icon name="file-text" size={size} />
  if (ext === '.json' || ext === '.tsbuildinfo') return <Icon name="file-json" size={size} />

  return <Icon name="file" size={size} />
}

export default FileIcon
