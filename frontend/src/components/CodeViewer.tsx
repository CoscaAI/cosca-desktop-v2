import { useMemo } from 'react'
import hljs from 'highlight.js/lib/core'
import DOMPurify from 'dompurify'

// Registro seletivo de linguagens (mantém o bundle leve). Só as realmente
// presentes no COSCA + as que fazem sentido abrir no viewer.
import go from 'highlight.js/lib/languages/go'
import typescript from 'highlight.js/lib/languages/typescript'
import javascript from 'highlight.js/lib/languages/javascript'
import python from 'highlight.js/lib/languages/python'
import rust from 'highlight.js/lib/languages/rust'
import java from 'highlight.js/lib/languages/java'
import cpp from 'highlight.js/lib/languages/cpp'
import c from 'highlight.js/lib/languages/c'
import csharp from 'highlight.js/lib/languages/csharp'
import json from 'highlight.js/lib/languages/json'
import yaml from 'highlight.js/lib/languages/yaml'
import markdown from 'highlight.js/lib/languages/markdown'
import css from 'highlight.js/lib/languages/css'
import xml from 'highlight.js/lib/languages/xml'
import sql from 'highlight.js/lib/languages/sql'
import shell from 'highlight.js/lib/languages/shell'
import bash from 'highlight.js/lib/languages/bash'
import powershell from 'highlight.js/lib/languages/powershell'
import ruby from 'highlight.js/lib/languages/ruby'
import php from 'highlight.js/lib/languages/php'
import swift from 'highlight.js/lib/languages/swift'
import kotlin from 'highlight.js/lib/languages/kotlin'
import dart from 'highlight.js/lib/languages/dart'
import ini from 'highlight.js/lib/languages/ini'

hljs.registerLanguage('go', go)
hljs.registerLanguage('typescript', typescript)
hljs.registerLanguage('javascript', javascript)
hljs.registerLanguage('python', python)
hljs.registerLanguage('rust', rust)
hljs.registerLanguage('java', java)
hljs.registerLanguage('cpp', cpp)
hljs.registerLanguage('c', c)
hljs.registerLanguage('csharp', csharp)
hljs.registerLanguage('json', json)
hljs.registerLanguage('yaml', yaml)
hljs.registerLanguage('markdown', markdown)
hljs.registerLanguage('css', css)
hljs.registerLanguage('xml', xml)
hljs.registerLanguage('sql', sql)
hljs.registerLanguage('shell', shell)
hljs.registerLanguage('bash', bash)
hljs.registerLanguage('powershell', powershell)
hljs.registerLanguage('ruby', ruby)
hljs.registerLanguage('php', php)
hljs.registerLanguage('swift', swift)
hljs.registerLanguage('kotlin', kotlin)
hljs.registerLanguage('dart', dart)
hljs.registerLanguage('ini', ini)

// Mapeia o rótulo semântico do backend (`node.language`) para um grammar hljs.
// tsx→typescript (o grammar TS lida com JSX), jsx→javascript, toml→ini
// (não existe grammar toml dedicado no core — ini é o mais próximo p/ config).
const LANG_ALIAS: Record<string, string> = {
  typescript: 'typescript',
  tsx: 'typescript',
  javascript: 'javascript',
  jsx: 'javascript',
  go: 'go',
  rust: 'rust',
  python: 'python',
  java: 'java',
  c: 'c',
  cpp: 'cpp',
  csharp: 'csharp',
  json: 'json',
  yaml: 'yaml',
  toml: 'ini',
  markdown: 'markdown',
  css: 'css',
  html: 'xml',
  sql: 'sql',
  xml: 'xml',
  shell: 'shell',
  powershell: 'powershell',
  ruby: 'ruby',
  php: 'php',
  swift: 'swift',
  kotlin: 'kotlin',
  dart: 'dart',
  ini: 'ini',
}

function escapeHtml(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
}

function highlight(content: string, language: string): { html: string; lang: string } {
  const name = LANG_ALIAS[language] ?? ''
  if (name && hljs.getLanguage(name)) {
    try {
      return { html: hljs.highlight(content, { language: name }).value, lang: name }
    } catch {
      return { html: escapeHtml(content), lang: name }
    }
  }
  return { html: escapeHtml(content), lang: name }
}

function fmtSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

const LARGE_THRESHOLD = 1_000_000 // 1 MB — acima disso, preview truncado
const PREVIEW_LIMIT = 200_000 // ~200 KB exibidos

export type CodeViewerProps = {
  content: string
  language: string
  fileName: string
  size: number
  modifiedAt?: string
  isBinary?: boolean
}

/**
 * COSCA CODE VIEWER — renderização read-only com qualidade de IDE.
 * ---------------------------------------------------------------------------
 * Leitura ≠ edição: o viewer NUNCA reformata, NUNCA altera indentação/espaços,
 * NUNCA salva. Apenas apresenta. Usa highlight.js (BSD, leve) registrando só as
 * linguagens úteis, com line-numbers e `<pre>` preservando whitespace/tabs.
 *
 * Binário/imagem/vídeo: NÃO passam pelo code viewer — mostram um card honesto de
 * metadata (Type/Size/Modified). Arquivos grandes: aviso + `preview truncado`.
 */
export function CodeViewer({ content, language, fileName, size, modifiedAt, isBinary }: CodeViewerProps) {
  const nonCode = isBinary || ['image', 'video', 'audio', 'archive', 'pdf'].includes(language)

  // Arquivos não-código → card de metadata (não fingir ser código).
  if (nonCode) {
    return (
      <div className="cv-binary" data-state="binary">
        <div className="cv-binary__head"><span className="cv-binary__mark"><span /></span><span>{fileName}</span></div>
        <dl className="cv-binary__meta">
          <div><dt>Type</dt><dd>{language === 'image' ? 'image' : language === 'video' ? 'video' : language === 'audio' ? 'audio' : language === 'archive' ? 'archive' : 'binary'}</dd></div>
          <div><dt>Size</dt><dd>{fmtSize(size)}</dd></div>
          {modifiedAt && <div><dt>Modified</dt><dd>{new Date(modifiedAt).toLocaleString()}</dd></div>}
        </dl>
        <p className="cv-binary__note">Preview de conteúdo não disponível via binding atual.</p>
      </div>
    )
  }

  const truncated = size > LARGE_THRESHOLD
  const shown = truncated ? content.slice(0, PREVIEW_LIMIT) : content
  const { html, lang } = useMemo(() => highlight(shown, language), [shown, language])
  const lines = countLines(shown)

  return (
    <div className="cv" data-state="viewer">
      <div className="cv-bar">
        <span className="cv-bar__name">{fileName}</span>
        <span className="cv-bar__lang">{lang || 'plaintext'}</span>
        {truncated && <span className="cv-bar__warn">arquivo grande — preview truncado</span>}
      </div>
      <div className="cv-scroll">
        <pre className="cv-pre"><code className="hljs" dangerouslySetInnerHTML={{ __html: DOMPurify.sanitize(html) }} /></pre>
        <div className="cv-gutter" aria-hidden="true">
          {Array.from({ length: lines }, (_, i) => (
            <div key={i} className="cv-gutter__ln">{i + 1}</div>
          ))}
        </div>
      </div>
    </div>
  )
}

function countLines(s: string): number {
  if (!s) return 1
  let n = 1
  for (let i = 0; i < s.length; i++) if (s[i] === '\n') n++
  return n
}

export default CodeViewer
