export function parseDateTime(value?: string | null) {
  const text = String(value || "").trim();
  if (!text) {
    return null;
  }
  const hasTimezone = /(?:z|[+-]\d{2}:?\d{2})$/i.test(text);
  const normalized = hasTimezone ? text.replace(" ", "T") : `${text.replace(" ", "T")}Z`;
  const direct = new Date(normalized);
  if (!Number.isNaN(direct.getTime())) {
    return direct;
  }
  const fallback = new Date(text);
  if (Number.isNaN(fallback.getTime())) {
    return null;
  }
  return fallback;
}

export function formatBeijingDateTime(value?: string | null) {
  const date = parseDateTime(value);
  if (!date) {
    return String(value || "-").trim() || "-";
  }
  const parts = new Intl.DateTimeFormat("zh-CN", {
    timeZone: "Asia/Shanghai",
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
  })
    .formatToParts(date)
    .reduce<Record<string, string>>((acc, part) => {
      if (part.type !== "literal") acc[part.type] = part.value;
      return acc;
    }, {});
  return `${parts.year}-${parts.month}-${parts.day} ${parts.hour}:${parts.minute}:${parts.second}`;
}
