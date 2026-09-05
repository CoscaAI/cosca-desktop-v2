// =========================================================================
// COSCA DESKTOP — MARKDOWN (FASE 3): render de mensagens do chat
// -------------------------------------------------------------------------
// Renderização read-only: marked (parse) → highlight.js (código) →
// DOMPurify (sanitiza). NUNCA reformata/alterar o conteúdo do usuário —
// apenas apresenta. Reusa a mesma paleta do CodeViewer (tokens --syntax-*)
// para o código; o corpo é markdown padrão. Identidade dark-first via design.css.
// =========================================================================
import { useMemo } from 'react'
import { marked, Renderer } from 'marked'
import hljs from 'highlight.js/lib/core'
import DOMPurify from 'dompurify'

// Registro seletivo de linguagens (mantém o bundle leve) — o MESMO conjunto
// do CodeViewer para o chat não divergir na identidade do código.
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

// Mapeia o rolê da linguagem da cerca (```lang) para um grammar hljs.
const LANG_ALIAS: Record<string, string> = {
  typescript: 'typescript', tsx: 'typescript',
  javascript: 'javascript', jsx: 'javascript',
  go: 'go', rust: 'rust', python: 'python', java: 'java',
  c: 'c', cpp: 'cpp', csharp: 'csharp',
  json: 'json', yaml: 'yaml', toml: 'ini', markdown: 'markdown',
  md: 'markdown', css: 'css', html: 'xml', xml: 'xml', sql: 'sql',
  shell: 'shell', sh: 'shell', bash: 'bash', powershell: 'powershell',
  ruby: 'ruby', php: 'php', swift: 'swift', kotlin: 'kotlin',
  dart: 'dart', ini: 'ini',
}

function escapeHtml(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
}

// Renderer customizado de fence de código → <div class="chat-code"> com a
// linguagem + <pre><code class="hljs"> destacado. DOMPurify sanitiza depois
// (o innerHTML destacado do hljs usa apenas tags/classes seguras).
const renderer = new Renderer()
renderer.code = ({ text, lang }) => {
  const alias = LANG_ALIAS[String(lang || '').toLowerCase()] || ''
  let code = ''
  if (alias && hljs.getLanguage(alias)) {
    try {
      code = hljs.highlight(text, { language: alias }).value
    } catch {
      code = escapeHtml(text)
    }
  } else {
    code = escapeHtml(text)
  }
  // Block válido (div > pre > code). Não injeta HTML cru do usuário — o texto
  // do código é sempre escapado ou destacado pelo hljs (classes seguras).
  const label = alias || 'plaintext'
  return `<div class="chat-code"><div class="chat-code-head"><span class="chat-code-lang">${label}</span></div><pre class="chat-code-block"><code class="hljs language-${label}">${code}</code></pre></div>`
}
marked.use({ renderer })

/** Renderiza markdown do chat → HTML sanitizado (read-only). */
export function renderMarkdown(content: string): string {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const raw = marked.parse(content || '', { async: false } as any) as unknown as string
  return DOMPurify.sanitize(raw)
}

export function Markdown({ content, inline }: { content: string; inline?: boolean }) {
  const html = useMemo(() => renderMarkdown(content), [content])
  // Código inline é renderizado como <code> simples (tokens .chat-inline-code).
  if (inline) {
    return <code className="chat-inline-code">{content}</code>
  }
  return <div className="chat-md" dangerouslySetInnerHTML={{ __html: html }} />
}

export default Markdown
