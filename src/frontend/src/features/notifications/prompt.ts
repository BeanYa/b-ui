const leadingPromptMarkerPattern = /^[\s\uFEFF]*(?:[。．、，：；•·・]\s*)+/

export const normalizePromptMessage = (message: unknown): string => {
  return String(message ?? '').replace(leadingPromptMarkerPattern, '').trimStart()
}
