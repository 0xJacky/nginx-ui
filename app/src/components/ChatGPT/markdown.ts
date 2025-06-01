import hljs from 'highlight.js'
import nginx from 'highlight.js/lib/languages/nginx'
import { Marked } from 'marked'
import { markedHighlight } from 'marked-highlight'
import 'highlight.js/styles/vs2015.css'

// Register nginx language for highlight.js
hljs.registerLanguage('nginx', nginx)

// Markdown renderer
export const marked = new Marked(
  markedHighlight({
    langPrefix: 'hljs language-',
    highlight(code, lang) {
      const language = hljs.getLanguage(lang) ? lang : 'nginx'
      return hljs.highlight(code, { language }).value
    },
  }),
)

// Basic marked options
marked.setOptions({
  pedantic: false,
  gfm: true,
  breaks: false,
})
