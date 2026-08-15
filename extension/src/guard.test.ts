import { describe, expect, it } from "vitest";
import { Event as HappyEvent, Window } from "happy-dom";
import { installSubmissionGuard } from "./guard.js";

function trustedEvent(
  window: Window,
  type: string,
  init: { key?: string; shiftKey?: boolean } = {},
): HappyEvent {
  const event =
    type === "keydown"
      ? new window.KeyboardEvent(type, { bubbles: true, cancelable: true, ...init })
      : type === "click"
        ? new window.MouseEvent(type, { bubbles: true, cancelable: true })
        : new window.Event(type, { bubbles: true, cancelable: true });
  Object.defineProperty(event, "isTrusted", { value: true });
  return event;
}

function dispatch(target: unknown, event: HappyEvent): void {
  (target as { dispatchEvent: (value: HappyEvent) => boolean }).dispatchEvent(event);
}

function formFixture(isUserActivationActive: () => boolean = () => false) {
  const window = new Window();
  window.document.body.innerHTML = `
    <form id="one"><input name="name"><button type="submit">Send</button><button type="button" id="js-backed">Run</button></form>
    <form id="two"><button type="submit">Send</button></form>
  `;
  Object.defineProperty(window.HTMLFormElement.prototype, "requestSubmit", {
    configurable: true,
    value: function (this: HTMLFormElement) {
      this.dispatchEvent(
        new window.Event("submit", {
          bubbles: true,
          cancelable: true,
        }) as unknown as Event,
      );
    },
  });
  installSubmissionGuard(window.document as unknown as Document, {
    isUserActivationActive,
  });
  return {
    window,
    one: window.document.querySelector("#one") as unknown as HTMLFormElement,
    two: window.document.querySelector("#two") as unknown as HTMLFormElement,
    input: window.document.querySelector("#one input") as unknown as HTMLInputElement,
    jsButton: window.document.querySelector("#js-backed") as unknown as HTMLButtonElement,
  };
}

describe("submission guard", () => {
  it("allows delayed human authorization once and consumes it", async () => {
    const { window, one } = formFixture();
    let submitCalls = 0;
    one.addEventListener("submit", () => {
      submitCalls++;
    });

    const input = window.document.querySelector("#one input")!;
    const enter = trustedEvent(window, "keydown", { key: "Enter" });
    enter.preventDefault();
    dispatch(input, enter);
    await new Promise((resolve) => setTimeout(resolve, 120));
    (one as unknown as { requestSubmit: () => void }).requestSubmit();
    (one as unknown as { requestSubmit: () => void }).requestSubmit();

    expect(submitCalls).toBe(1);
  });

  it("does not let unrelated clicks or keys authorize a form", () => {
    const { window, one, input } = formFixture();
    let submitCalls = 0;
    one.addEventListener("submit", () => {
      submitCalls++;
    });

    dispatch(input, trustedEvent(window, "click"));
    dispatch(input, new window.KeyboardEvent("keydown", { key: "Enter", bubbles: true }));
    dispatch(input, trustedEvent(window, "keydown", { key: "Enter", shiftKey: true }));
    dispatch(input, new window.Event("input", { bubbles: true }));
    dispatch(input, new window.Event("change", { bubbles: true }));
    (one as unknown as { requestSubmit: () => void }).requestSubmit();

    expect(submitCalls).toBe(0);
  });

  it("allows a trusted submit-control activation once", () => {
    const { window, one } = formFixture();
    let oneSubmits = 0;
    one.addEventListener("submit", () => {
      oneSubmits++;
    });

    const button = one.querySelector("button")!;
    const click = trustedEvent(window, "click");
    click.preventDefault();
    dispatch(button, click);
    (one as unknown as { requestSubmit: () => void }).requestSubmit();
    (one as unknown as { requestSubmit: () => void }).requestSubmit();

    expect(oneSubmits).toBe(1);
  });

  it("allows one synchronous programmatic submission from an active JS-backed button", () => {
    const { window, one, jsButton } = formFixture(() => true);
    let submitCalls = 0;
    one.addEventListener("submit", () => {
      submitCalls++;
    });
    jsButton.addEventListener("click", () => {
      (one as unknown as { requestSubmit: () => void }).requestSubmit();
      (one as unknown as { requestSubmit: () => void }).requestSubmit();
    });

    dispatch(jsButton, trustedEvent(window, "click"));

    expect(submitCalls).toBe(1);
  });

  it("allows async JS-backed validation within the bounded window", async () => {
    const { window, one, jsButton } = formFixture(() => true);
    let submitCalls = 0;
    one.addEventListener("submit", () => {
      submitCalls++;
    });
    jsButton.addEventListener("click", () => {
      window.setTimeout(() => {
        (one as unknown as { requestSubmit: () => void }).requestSubmit();
      }, 120);
    });

    dispatch(jsButton, trustedEvent(window, "click"));
    await new Promise((resolve) => setTimeout(resolve, 150));

    expect(submitCalls).toBe(1);
  });

  it("expires an unused JS-backed authorization", async () => {
    const { window, one, jsButton } = formFixture(() => true);
    let submitCalls = 0;
    one.addEventListener("submit", () => {
      submitCalls++;
    });

    dispatch(jsButton, trustedEvent(window, "click"));
    await new Promise((resolve) => setTimeout(resolve, 1_050));
    (one as unknown as { requestSubmit: () => void }).requestSubmit();

    expect(submitCalls).toBe(0);
  });

  it("cancels a pending authorization after an unrelated trusted interaction", () => {
    const { window, one, jsButton, input } = formFixture(() => true);
    let submitCalls = 0;
    one.addEventListener("submit", () => {
      submitCalls++;
    });

    dispatch(jsButton, trustedEvent(window, "click"));
    dispatch(input, trustedEvent(window, "click"));
    (one as unknown as { requestSubmit: () => void }).requestSubmit();

    expect(submitCalls).toBe(0);
  });

  it("blocks inactive or untrusted autofill-triggered programmatic submission", () => {
    const { window, one, jsButton, input } = formFixture(() => false);
    let submitCalls = 0;
    one.addEventListener("submit", () => {
      submitCalls++;
    });
    jsButton.addEventListener("click", () => {
      (one as unknown as { requestSubmit: () => void }).requestSubmit();
    });
    input.addEventListener("input", () => {
      (one as unknown as { requestSubmit: () => void }).requestSubmit();
    });

    dispatch(jsButton, new window.MouseEvent("click", { bubbles: true }));
    dispatch(input, new window.Event("input", { bubbles: true }));

    expect(submitCalls).toBe(0);
  });

  it("restores a canceled trusted submit for one later programmatic attempt", async () => {
    const { window, one } = formFixture();
    let submitCalls = 0;
    one.addEventListener("submit", (event) => {
      submitCalls++;
      if (event.isTrusted) event.preventDefault();
    });

    const button = one.querySelector("button")!;
    const click = trustedEvent(window, "click");
    click.preventDefault();
    dispatch(button, click);
    dispatch(one, trustedEvent(window, "submit"));
    await Promise.resolve();

    (one as unknown as { requestSubmit: () => void }).requestSubmit();
    (one as unknown as { requestSubmit: () => void }).requestSubmit();

    expect(submitCalls).toBe(2);
  });

  it("does not preserve authorization when the trusted submit is not canceled", async () => {
    const { window, one } = formFixture();
    let submitCalls = 0;
    one.addEventListener("submit", () => {
      submitCalls++;
    });

    const button = one.querySelector("button")!;
    dispatch(button, trustedEvent(window, "click"));
    dispatch(one, trustedEvent(window, "submit"));
    await Promise.resolve();

    (one as unknown as { requestSubmit: () => void }).requestSubmit();

    expect(submitCalls).toBe(1);
  });

  it("rejects untrusted submit-control events and textarea/contenteditable Enter", () => {
    const { window, one } = formFixture();
    one.insertAdjacentHTML(
      "beforeend",
      '<textarea name="notes"></textarea><div contenteditable="true"></div>',
    );
    const textarea = one.querySelector("textarea")!;
    const editable = one.querySelector("[contenteditable]")!;

    let submitCalls = 0;
    one.addEventListener("submit", () => {
      submitCalls++;
    });
    dispatch(one.querySelector("button"), new window.MouseEvent("click", { bubbles: true }));
    dispatch(textarea, trustedEvent(window, "keydown", { key: "Enter" }));
    dispatch(editable, trustedEvent(window, "keydown", { key: "Enter" }));
    (one as unknown as { requestSubmit: () => void }).requestSubmit();

    expect(submitCalls).toBe(0);
  });
});
