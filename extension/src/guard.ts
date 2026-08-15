/**
 * Main-world submission guard. Autofill dispatches untrusted input/change
 * events, so a page handler must not turn those events into a submission. A
 * trusted human activation authorizes one later action for its owning form.
 */

const formGuards = new WeakSet<object>();
// ponytail: 1000ms covers normal async validation; upgrade to an explicit page
// validation handshake if real flows routinely exceed this ceiling.
const ACTIVATION_AUTHORIZATION_MS = 1_000;

export interface SubmissionGuardOptions {
  isUserActivationActive?: () => boolean;
}

export function installSubmissionGuard(
  doc: Document,
  options: SubmissionGuardOptions = {},
): void {
  const view = doc.defaultView;
  if (!view || formGuards.has(view)) return;
  formGuards.add(view);

  const isUserActivationActive =
    options.isUserActivationActive ??
    (() => view.navigator.userActivation?.isActive === true);
  const authorizedForms = new Set<string>();
  const activationForms = new Map<string, number>();
  const allowedFormEvents = new Set<string>();
  const formTokens = new WeakMap<HTMLFormElement, string>();
  let nextFormToken = 0;
  const blocked = (): undefined => undefined;

  const formKey = (form: HTMLFormElement): string => {
    for (const attribute of ["id", "name"]) {
      const value = form.getAttribute(attribute);
      if (!value) continue;
      const matches = Array.from(doc.forms).filter(
        (candidate) => candidate.getAttribute(attribute) === value,
      );
      if (matches.length === 1) return `${attribute}:${value}`;
    }
    const existing = formTokens.get(form);
    if (existing) return existing;
    const token = `object:${nextFormToken++}`;
    formTokens.set(form, token);
    return token;
  };

  const formForTarget = (target: EventTarget | null): HTMLFormElement | null => {
    if (!target || typeof target !== "object") return null;
    const element = target as Element & { form?: HTMLFormElement | null };
    if (element.tagName?.toLowerCase() === "form") {
      return element as HTMLFormElement;
    }
    if (element.form) return element.form;
    if (typeof element.closest !== "function") return null;
    const form = element.closest("form");
    return form && form.tagName.toLowerCase() === "form"
      ? (form as HTMLFormElement)
      : null;
  };

  const submitControlForTarget = (
    target: EventTarget | null,
  ): { control: HTMLButtonElement | HTMLInputElement; form: HTMLFormElement } | null => {
    if (!target || typeof target !== "object") return null;
    const element = target as Element;
    const control = element.closest?.("button, input") as
      | HTMLButtonElement
      | HTMLInputElement
      | null;
    if (!control || control.disabled) return null;
    const tag = control.tagName.toLowerCase();
    const type = control.getAttribute("type")?.toLowerCase();
    const isSubmit =
      (tag === "button" && (type === null || type === "submit")) ||
      (tag === "input" && (type === "submit" || type === "image"));
    const form = isSubmit ? formForTarget(control) : null;
    return form ? { control, form } : null;
  };

  const activationControlForTarget = (
    target: EventTarget | null,
  ): { control: HTMLButtonElement | HTMLInputElement; form: HTMLFormElement } | null => {
    if (!target || typeof target !== "object") return null;
    const element = target as Element;
    const control = element.closest?.("button, input") as
      | HTMLButtonElement
      | HTMLInputElement
      | null;
    if (!control || control.disabled) return null;
    const tag = control.tagName.toLowerCase();
    const type = control.getAttribute("type")?.toLowerCase();
    if ((tag !== "button" && tag !== "input") || type !== "button") return null;
    const form = formForTarget(control);
    return form ? { control, form } : null;
  };

  const isImplicitSubmitControl = (target: EventTarget | null): HTMLFormElement | null => {
    if (!target || typeof target !== "object") return null;
    const submitter = submitControlForTarget(target);
    if (submitter) return submitter.form;
    const element = target as HTMLInputElement;
    if (element.tagName?.toLowerCase() !== "input" || element.disabled) return null;
    if (
      element.getAttribute("contenteditable") !== null ||
      element.closest?.("[contenteditable]") !== null
    ) {
      return null;
    }
    const type = (element.getAttribute("type") ?? "text").toLowerCase();
    const implicitTypes = new Set([
      "date",
      "datetime-local",
      "email",
      "month",
      "number",
      "password",
      "search",
      "tel",
      "text",
      "time",
      "url",
      "week",
    ]);
    return implicitTypes.has(type) ? formForTarget(element) : null;
  };

  const authorizeFromUserActivation = (event: Event): void => {
    if (!event.isTrusted) return;
    if (event.type !== "click") {
      for (const timer of activationForms.values()) view.clearTimeout(timer);
      activationForms.clear();
    }
    if (event.type === "click") {
      for (const timer of activationForms.values()) view.clearTimeout(timer);
      activationForms.clear();
      const submitter = submitControlForTarget(event.target);
      if (submitter) {
        authorizedForms.add(formKey(submitter.form));
      } else {
        const activationControl = activationControlForTarget(event.target);
        if (activationControl && isUserActivationActive()) {
          const key = formKey(activationControl.form);
          let timer = 0;
          timer = view.setTimeout(() => {
            if (activationForms.get(key) === timer) activationForms.delete(key);
          }, ACTIVATION_AUTHORIZATION_MS);
          activationForms.set(key, timer);
        }
      }
      return;
    }
    if (event.type === "keydown") {
      const key = event as KeyboardEvent;
      if (
        key.key !== "Enter" ||
        key.altKey ||
        key.ctrlKey ||
        key.metaKey ||
        key.shiftKey
      ) {
        return;
      }
      const form = isImplicitSubmitControl(event.target);
      if (form) {
        authorizedForms.add(formKey(form));
      }
    }
  };
  const consumeAuthorization = (form: HTMLFormElement): boolean => {
    const key = formKey(form);
    if (!authorizedForms.has(key)) return false;
    authorizedForms.delete(key);
    return true;
  };
  const allowNativeSubmission = (
    form: HTMLFormElement,
    invoke: () => unknown,
  ): unknown => {
    const authorized = consumeAuthorization(form);
    const key = formKey(form);
    if (!authorized) {
      const timer = activationForms.get(key);
      if (timer === undefined) {
        return blocked();
      }
      view.clearTimeout(timer);
      activationForms.delete(key);
    }
    allowedFormEvents.add(key);
    try {
      return invoke();
    } finally {
      allowedFormEvents.delete(key);
    }
  };

  doc.addEventListener("click", authorizeFromUserActivation, true);
  doc.addEventListener("keydown", authorizeFromUserActivation, true);
  const clearActivationOnTrustedInteraction = (event: Event): void => {
    if (!event.isTrusted) return;
    for (const timer of activationForms.values()) view.clearTimeout(timer);
    activationForms.clear();
  };
  for (const type of ["pointerdown", "mousedown", "touchstart", "input", "change"]) {
    doc.addEventListener(type, clearActivationOnTrustedInteraction, true);
  }
  doc.addEventListener(
    "submit",
    (event) => {
      const form = formForTarget(event.target);
      if (!form) {
        event.preventDefault();
        event.stopImmediatePropagation();
        return;
      }
      const key = formKey(form);
      if (allowedFormEvents.has(key)) {
        allowedFormEvents.delete(key);
        return;
      }
      if (!consumeAuthorization(form)) {
        event.preventDefault();
        event.stopImmediatePropagation();
        return;
      }
      if (event.isTrusted) {
        // Downstream validation runs after this capture listener.
        Promise.resolve().then(() => {
          if (event.defaultPrevented) authorizedForms.add(key);
        });
      }
    },
    true,
  );

  const formProto = view.HTMLFormElement.prototype as unknown as Record<string, unknown>;
  for (const key of ["submit", "requestSubmit"]) {
    const original = formProto[key];
    if (typeof original !== "function") continue;
    Object.defineProperty(formProto, key, {
      configurable: true,
      value: function (this: HTMLFormElement, ...args: unknown[]) {
        return allowNativeSubmission(this, () =>
          (original as (...values: unknown[]) => unknown).apply(this, args),
        );
      },
    });
  }

  const elementProto = view.HTMLElement.prototype as unknown as Record<string, unknown>;
  const originalClick = elementProto.click;
  if (typeof originalClick === "function") {
    Object.defineProperty(elementProto, "click", {
      configurable: true,
      value: function (this: HTMLElement, ...args: unknown[]) {
        const tag = this.tagName.toLowerCase();
        const type = this.getAttribute("type")?.toLowerCase();
        const submitter =
          (tag === "button" && (type === null || type === "submit")) ||
          (tag === "input" && (type === "submit" || type === "image"));
        const form = submitter ? formForTarget(this) : null;
        if (submitter && form) {
          return allowNativeSubmission(form, () =>
            (originalClick as (...values: unknown[]) => unknown).apply(this, args),
          );
        }
        return (originalClick as (...values: unknown[]) => unknown).apply(this, args);
      },
    });
  }
}
