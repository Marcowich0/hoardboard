/** Escapes text for HTML. Names come from the disks: ECharts tooltips are HTML, so every name goes through here. */
export function escapeHtml(text: string): string {
  return text.replace(/[&<>"']/g, (c) => `&#${c.charCodeAt(0)};`)
}
