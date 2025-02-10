declare module '*.svg' {
  import type React from 'react'

  const content: React.FC<React.SVGProps<SVGElement>>
  export default content
}
