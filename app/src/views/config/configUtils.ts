// List of protected directories that cannot be deleted
const PROTECTED_DIRS = ['sites-enabled', 'sites-available', 'streams-enabled', 'streams-available', 'conf.d']

/**
 * Check if a file/directory name is protected and cannot be deleted
 * @param name - The name of the file or directory
 * @returns true if the item is protected, false otherwise
 */
export function isProtectedPath(name: string): boolean {
  return PROTECTED_DIRS.includes(name)
}

/**
 * Get the list of protected directories
 * @returns Array of protected directory names
 */
export function getProtectedDirs(): string[] {
  return [...PROTECTED_DIRS]
}
