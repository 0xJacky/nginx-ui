import config from '@/api/config'

// Interface for included file information
interface IncludedFile {
  path: string
  content: string
  depth: number
}

// Interface for context building result
export interface LLMContext {
  mainFile: {
    path: string
    content: string
  }
  includedFiles: IncludedFile[]
  contextText: string
}

// Parse nginx config content to find include directives
function parseIncludeDirectives(content: string): string[] {
  // More flexible regex that handles include with or without semicolon
  const includePattern = /^\s*include\s+([^\s;]+)/g
  const includes: string[] = []
  let match

  // eslint-disable-next-line no-cond-assign
  while ((match = includePattern.exec(content)) !== null) {
    const includePath = match[1].trim()

    // Skip wildcards as they require special handling
    if (!includePath.includes('*') && !includePath.includes('?')) {
      includes.push(includePath)
    }
  }

  return includes
}

// Recursively load included files with depth limit to prevent infinite loops
async function loadIncludedFiles(
  includes: string[],
  visited: Set<string> = new Set(),
  depth: number = 0,
  maxDepth: number = 10,
): Promise<IncludedFile[]> {
  if (depth >= maxDepth) {
    console.warn('Maximum include depth reached, stopping recursion')
    return []
  }

  const includedFiles: IncludedFile[] = []

  for (const includePath of includes) {
    // Prevent circular includes
    if (visited.has(includePath)) {
      console.warn(`Circular include detected: ${includePath}`)
      continue
    }

    visited.add(includePath)

    try {
      // Use config.getItem to fetch the included file
      const response = await config.getItem(includePath)
      const fileContent = response.content

      const includedFile: IncludedFile = {
        path: includePath,
        content: fileContent,
        depth,
      }
      includedFiles.push(includedFile)

      // Recursively parse this file for more includes
      const nestedIncludes = parseIncludeDirectives(fileContent)
      if (nestedIncludes.length > 0) {
        const nestedFiles = await loadIncludedFiles(
          nestedIncludes,
          new Set(visited),
          depth + 1,
          maxDepth,
        )
        includedFiles.push(...nestedFiles)
      }
    }
    catch (error) {
      console.warn(`Failed to load included file: ${includePath}`, error)
      // Continue processing other includes even if one fails
    }

    visited.delete(includePath)
  }

  return includedFiles
}

// Build complete context including main file and all included files
export async function buildLLMContext(mainFilePath: string, mainFileContent: string): Promise<LLMContext> {
  const context: LLMContext = {
    mainFile: {
      path: mainFilePath,
      content: mainFileContent,
    },
    includedFiles: [],
    contextText: '',
  }

  try {
    // Parse include directives from main file
    const includes = parseIncludeDirectives(mainFileContent)

    if (includes.length > 0) {
      // Load all included files recursively
      context.includedFiles = await loadIncludedFiles(includes)
    }

    // Build the complete context text
    context.contextText = buildContextText(context)
  }
  catch (error) {
    console.error('Error building LLM context:', error)
    // Fallback to main file only
    context.contextText = `Main File: ${mainFilePath}\n\n${mainFileContent}`
  }

  return context
}

// Build formatted context text for LLM
function buildContextText(context: LLMContext): string {
  let contextText = `Main File: ${context.mainFile.path}\n\n${context.mainFile.content}`

  if (context.includedFiles.length > 0) {
    contextText += '\n\n--- INCLUDED FILES ---\n'

    for (const includedFile of context.includedFiles) {
      const indent = '  '.repeat(includedFile.depth)
      contextText += `\n${indent}Included File: ${includedFile.path}\n${indent}${includedFile.content.replace(/\n/g, `\n${indent}`)}\n`
    }
  }

  return contextText
}
