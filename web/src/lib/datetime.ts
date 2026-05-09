export function parseDateTime(value?: string | null) {
  const text = String(value || "").trim();
  if (!text) {
    return null;
  }
  const direct = new Date(text);
  if (!Number.isNaN(direct.getTime())) {
    return direct;
  }
  const normalized = text.replace(" ", "T") + (text.endsWith("Z") ? "" : "Z");
  const fallback = new Date(normalized);
  if (Number.isNaN(fallback.getTime())) {
    return null;
  }
  return fallback;
}
