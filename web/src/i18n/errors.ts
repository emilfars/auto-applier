import { ApiError } from "../api/http";
import { t, type Locale, type TranslationKey } from "./index";

/**
 * Turn a failed API call into a sentence that says what happened and what to
 * do next. `byStatus` lets a screen name a status in its own terms (a 401 on
 * the sign-in form is a wrong password, elsewhere it is an expired session).
 * Other 4xx keep the server's message, which names the specific field.
 */
export function errorMessage(
  locale: Locale,
  err: unknown,
  byStatus: Partial<Record<number, TranslationKey>> = {},
): string {
  if (!(err instanceof ApiError)) return t(locale, "error.network");
  const key =
    byStatus[err.status] ??
    (err.status === 401
      ? "error.session"
      : err.status === 429
        ? "error.rateLimit"
        : err.status >= 500 || err.status < 400
          ? "error.server"
          : undefined);
  return key ? t(locale, key) : err.message;
}
