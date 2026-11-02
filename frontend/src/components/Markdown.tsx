import type { ReactNode } from 'react'

// 极简 markdown 渲染：支持 **加粗**、*斜体*、`行内代码`，逐行渲染。
// 用 React 元素输出（无 dangerouslySetInnerHTML），天然无 XSS 风险。
export default function Markdown({ text }: { text: string }) {
  const renderInline = (s: string): ReactNode[] => {
    const parts: ReactNode[] = []
    const regex = /(\*\*[^*]+\*\*|\*[^*]+\*|`[^`]+`)/g
    let last = 0
    let key = 0
    let m: RegExpExecArray | null
    while ((m = regex.exec(s)) !== null) {
      if (m.index > last) parts.push(s.slice(last, m.index))
      const tok = m[0]
      if (tok.startsWith('**')) parts.push(<strong key={key++}>{tok.slice(2, -2)}</strong>)
      else if (tok.startsWith('`')) parts.push(<code key={key++}>{tok.slice(1, -1)}</code>)
      else parts.push(<em key={key++}>{tok.slice(1, -1)}</em>)
      last = m.index + tok.length
    }
    if (last < s.length) parts.push(s.slice(last))
    return parts
  }

  return (
    <>
      {text.split('\n').map((line, i) =>
        line === '' ? <br key={i} /> : <div key={i}>{renderInline(line)}</div>,
      )}
    </>
  )
}
