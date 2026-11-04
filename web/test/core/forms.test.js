import { ApiError, NetworkError } from "horizon/core/api.js";
import { clearFieldErrors, errorMessage, formValues, showFieldErrors } from "horizon/core/forms.js";
import { createI18n } from "horizon/core/i18n.js";
import { assert, describe, flush, test } from "horizon-test/runner.js";

const i18n = createI18n(
  {
    "error.unexpected": "Unexpected",
    "error.network": "Offline",
    "error.invalid_credentials": "Wrong credentials",
    "error.field.required": "Required",
    "error.email.invalid": "Bad email",
  },
  { onMissing: () => {} },
);

/** A form of lds-fields in a fresh container; removed by the caller. */
async function fixture() {
  const host = document.createElement("div");
  host.innerHTML = `
    <form>
      <lds-field label="Email"><lds-input name="email" value="ada@example.com"></lds-input></lds-field>
      <lds-field label="Name"><lds-input name="name"></lds-input></lds-field>
      <lds-button type="submit">Go</lds-button>
    </form>`;
  document.body.append(host);
  await customElements.whenDefined("lds-input");
  await flush();
  return { host, form: host.querySelector("form") };
}

describe("forms", () => {
  test("formValues reads form-associated LDS controls", async () => {
    const { host, form } = await fixture();
    try {
      assert.deepEqual(formValues(form), { email: "ada@example.com", name: "" });
    } finally {
      host.remove();
    }
  });

  test("showFieldErrors fills error slots, translating field codes, and returns the rest", async () => {
    const { host, form } = await fixture();
    try {
      const rest = showFieldErrors(
        form,
        [{ field: "email", code: "invalid" }, { field: "name", code: "required" }, { field: "token", code: "expired" }],
        i18n.t,
        i18n.has,
      );
      const errors = [...form.querySelectorAll('[slot="error"]')].map((e) => [e.parentElement.getAttribute("label"), e.textContent]);
      assert.deepEqual(errors, [["Email", "Bad email"], ["Name", "Required"]]);
      assert.deepEqual(rest, [{ field: "token", code: "expired" }]);

      showFieldErrors(form, [{ field: "name", code: "required" }], i18n.t, i18n.has);
      assert.equal(form.querySelectorAll('[slot="error"]').length, 1, "previous errors cleared");
      clearFieldErrors(form);
      assert.equal(form.querySelectorAll('[slot="error"]').length, 0);
    } finally {
      host.remove();
    }
  });

  // Views rely on LDS (v0.0.6) for these; Horizon used to patch them in.
  test("lds-input submits its form on Enter and mirrors autocomplete", async () => {
    const { host, form } = await fixture();
    try {
      let submitted = 0;
      form.addEventListener("submit", (e) => {
        e.preventDefault();
        submitted++;
      });
      const input = form.querySelector('[name="email"]');
      input.setAttribute("autocomplete", "username");
      await flush();
      const inner = input.shadowRoot.querySelector("input");
      assert.equal(inner.autocomplete, "username");
      inner.dispatchEvent(new KeyboardEvent("keydown", { key: "Enter", bubbles: true, composed: true }));
      inner.dispatchEvent(new KeyboardEvent("keydown", { key: "Enter", shiftKey: true, bubbles: true, composed: true }));
      assert.equal(submitted, 1);
    } finally {
      host.remove();
    }
  });

  test("errorMessage translates codes, with network and generic fallbacks", () => {
    assert.equal(errorMessage(new ApiError(401, { type: "invalid_credentials" }), i18n), "Wrong credentials");
    assert.equal(errorMessage(new ApiError(500, {}), i18n), "Unexpected");
    assert.equal(errorMessage(new NetworkError(new Error("x")), i18n), "Offline");
  });
});
