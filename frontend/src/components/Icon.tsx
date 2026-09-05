import type { LucideIcon } from 'lucide-react'
import type { SVGProps } from 'react'
import {
  House,
  Folder,
  Bot,
  FolderTree,
  Terminal,
  Search,
  GitCompare,
  BrainCircuit,
  Network,
  LibraryBig,
  Brain,
  GitBranch,
  ShieldCheck,
  Shield,
  Activity,
  Hexagon,
  Sun,
  Moon,
  X,
  ChevronLeft,
  ChevronRight,
  ChevronUp,
  ChevronDown,
  Command,
  File,
  FileText,
  FileCode,
  FileJson,
  FileType,
  FileCog,
  FileImage,
  FileVideo,
  FileAudio,
  FileArchive,
  FolderOpen,
  Braces,
  Hash,
  Database,
  Cpu,
  Settings,
  Info,
  MessageSquare,
} from 'lucide-react'

/**
 * COSCA ICON PRIMITIVE
 * ---------------------------------------------------------------------------
 * Fonte única de ícones de UI do Desktop. Substitui os antigos símbolos
 * Unicode improvisados (◈ ◆ ◇ ▣ ❯ ⌕ ✦ ⧉ ◉ ⌘ ☀ 🌙) por ícones Lucide
 * stroke-based, minimalistas e consistentes.
 *
 * Características:
 *  - stroke-based por padrão (Lucide), `strokeWidth` global consistente (~1.75).
 *  - `color` e `stroke` NUNCA são hardcoded aqui: o SVG usa `currentColor` e
 *    herda os tokens `var(--...)` do contexto (`.rail-btn`, `.brand-mark`,
 *    `.iconbtn`, `.hero-mark`, badge etc.).
 *  - Um MAPA nome→componente para o rail, que usa strings `icon` no App.tsx.
 *
 * Uso:
 *   <Icon name="intelligence" size={16} />
 *   <Icon name="diff" size={14} strokeWidth={2} />
 */
export const ICONS = {
  // RAIL (sidebar)
  home: House,
  project: Folder,
  chat: MessageSquare,
  agent: Bot,
  files: FolderTree,
  terminal: Terminal,
  search: Search,
  diff: GitCompare,
  intelligence: BrainCircuit,
  forge: Network,
  knowledge: LibraryBig,
  memory: Brain,
  provenance: GitBranch,
  audit: ShieldCheck,
  sandbox: Shield,
  runtime: Activity,
  // HEADER / CHROME
  brand: Hexagon,
  sun: Sun,
  moon: Moon,
  close: X,
  settings: Settings,
  info: Info,
  // COMMAND PALETTE
  command: Command,
  // COLLAPSE / EXPAND (panes)
  'chevron-left': ChevronLeft,
  'chevron-right': ChevronRight,
  'chevron-up': ChevronUp,
  'chevron-down': ChevronDown,
  // FILE EXPLORER (árvore de arquivos)
  folder: Folder,
  'folder-open': FolderOpen,
  file: File,
  'file-code': FileCode,
  'file-json': FileJson,
  'file-text': FileText,
  'file-type': FileType,
  'file-cog': FileCog,
  'file-image': FileImage,
  'file-video': FileVideo,
  'file-audio': FileAudio,
  'file-archive': FileArchive,
  braces: Braces,
  hash: Hash,
  database: Database,
  cpu: Cpu,
  'terminal-file': Terminal,
} as const

export type IconName = keyof typeof ICONS

export type IconProps = {
  name: IconName
  /** Largura/altura em px (Lucide é quadrado). */
  size?: number
  /** Espessura do traço — global consistente (~1.75). */
  strokeWidth?: number
} & SVGProps<SVGSVGElement>

export function Icon({ name, size = 16, strokeWidth = 1.75, ...rest }: IconProps) {
  const Cmp = ICONS[name]
  if (!Cmp) return null
  // pointer-events none: ícone é decorativo dentro de botões — o hit-test vai ao
  // <button> pai (onClick), nunca ao svg. Evita "clique no ícone não dispara".
  return <Cmp size={size} strokeWidth={strokeWidth} aria-hidden="true" style={{ pointerEvents: 'none' }} {...rest} />
}

export default Icon
