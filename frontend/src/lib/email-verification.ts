const RESEND_STORAGE_PREFIX = "ipnu-ippnu-id:otp-resend-deadline:v1:";

export const OTP_RESEND_DELAY_SECONDS = 60;

function storageKey(email: string) {
  return `${RESEND_STORAGE_PREFIX}${encodeURIComponent(email.trim().toLowerCase())}`;
}

export function readOTPResendDeadline(email: string) {
  if (typeof window === "undefined" || !email.trim()) return 0;
  try {
    const value = Number(window.localStorage.getItem(storageKey(email)));
    if (!Number.isFinite(value) || value <= Date.now()) {
      window.localStorage.removeItem(storageKey(email));
      return 0;
    }
    return value;
  } catch {
    return 0;
  }
}

export function startOTPResendCooldown(email: string) {
  const deadline = Date.now() + OTP_RESEND_DELAY_SECONDS * 1000;
  if (typeof window !== "undefined" && email.trim()) {
    try {
      window.localStorage.setItem(storageKey(email), String(deadline));
    } catch {
      // Countdown tetap bekerja pada tab aktif meski penyimpanan browser dibatasi.
    }
  }
  return deadline;
}

export function clearOTPResendCooldown(email: string) {
  if (typeof window === "undefined" || !email.trim()) return;
  try {
    window.localStorage.removeItem(storageKey(email));
  } catch {
    // Tidak ada tindakan lanjutan jika penyimpanan browser dibatasi.
  }
}
