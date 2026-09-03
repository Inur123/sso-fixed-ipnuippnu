// Use Jakarta explicitly so server rendering and every browser agree.
export const APP_TIME_ZONE = "Asia/Jakarta";

const dateOptions: Intl.DateTimeFormatOptions = {
  timeZone: APP_TIME_ZONE,
  day: "2-digit",
  month: "short",
  year: "numeric",
};

const dateFormatter = new Intl.DateTimeFormat("id-ID", dateOptions);
const dateTimeFormatter = new Intl.DateTimeFormat("id-ID", {
  ...dateOptions,
  hour: "2-digit",
  minute: "2-digit",
  timeZoneName: "short",
});

type DateValue = string | Date | null | undefined;

function format(value: DateValue, formatter: Intl.DateTimeFormat, empty: string) {
  if (!value) return empty;
  const date = value instanceof Date ? value : new Date(value);
  return Number.isNaN(date.getTime()) ? "—" : formatter.format(date);
}

export function formatJakartaDate(value: DateValue, empty = "—") {
  return format(value, dateFormatter, empty);
}

export function formatJakartaDateTime(value: DateValue, empty = "—") {
  return format(value, dateTimeFormatter, empty);
}
