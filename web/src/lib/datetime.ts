type LocalDateTimeOptions = {
  withSeconds?: boolean;
  locale?: string;
  fallback?: string;
};

const DATE_TIME_WITHOUT_TZ_RE =
  /^(\d{4})-(\d{2})-(\d{2})(?:[ T](\d{2}):(\d{2})(?::(\d{2})(?:\.(\d{1,9}))?)?)?$/;

function dateFromUTCParts(parts: RegExpMatchArray) {
  const year = Number(parts[1]);
  const month = Number(parts[2]) - 1;
  const day = Number(parts[3]);
  const hour = Number(parts[4] || "0");
  const minute = Number(parts[5] || "0");
  const second = Number(parts[6] || "0");
  const fraction = String(parts[7] || "");
  const millisecond = fraction ? Number((fraction + "000").slice(0, 3)) : 0;
  return new Date(Date.UTC(year, month, day, hour, minute, second, millisecond));
}

export function parseDateTime(value?: string | null): Date | null {
  const text = String(value || "").trim();
  if (!text) {
    return null;
  }

  const noTZMatched = text.match(DATE_TIME_WITHOUT_TZ_RE);
  if (noTZMatched) {
    const asUTC = dateFromUTCParts(noTZMatched);
    return Number.isNaN(asUTC.getTime()) ? null : asUTC;
  }

  const parsed = new Date(text);
  if (Number.isNaN(parsed.getTime())) {
    return null;
  }
  return parsed;
}

export function formatLocalDateTime(value?: string | null, options: LocalDateTimeOptions = {}) {
  const date = parseDateTime(value);
  if (!date) {
    return options.fallback ?? "-";
  }
  return new Intl.DateTimeFormat(options.locale || "zh-CN", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    ...(options.withSeconds ? { second: "2-digit" } : {}),
  }).format(date);
}
