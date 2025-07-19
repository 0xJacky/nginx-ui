import FingerprintJS from '@fingerprintjs/fingerprintjs'

let fpPromise: Promise<string> | null = null

/**
 * Get browser fingerprint
 * Use caching mechanism to avoid duplicate calculations
 */
export async function getBrowserFingerprint(): Promise<string> {
  if (!fpPromise) {
    fpPromise = generateFingerprint()
  }
  return fpPromise
}

/**
 * Generate browser fingerprint
 */
async function generateFingerprint(): Promise<string> {
  try {
    // Initialize FingerprintJS
    const fp = await FingerprintJS.load()

    // Get fingerprint result
    const result = await fp.get()

    // Return fingerprint ID
    return result.visitorId
  }
  catch (error) {
    console.warn('Failed to generate browser fingerprint, fallback to User Agent:', error)
    // If fingerprint generation fails, fallback to User Agent
    return navigator.userAgent
  }
}

/**
 * Clear fingerprint cache
 * Force regenerate fingerprint
 */
export function clearFingerprintCache(): void {
  fpPromise = null
}
