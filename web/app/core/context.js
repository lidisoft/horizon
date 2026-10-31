// Views get their services (api, store, router, i18n, shortcuts) from the
// nearest provider instead of globals (ADR-0018). In standalone mode hzn-app
// provides them; a future shell adapter would provide the same object.

const EVENT = "hzn-context";

/** Makes host answer context requests from its descendants. */
export function provideContext(host, context) {
  host.addEventListener(EVENT, (event) => {
    event.stopPropagation();
    event.detail.context = context;
  });
}

/** Returns the services for el, which must be connected. */
export function requestContext(el) {
  const detail = {};
  el.dispatchEvent(new CustomEvent(EVENT, { bubbles: true, composed: true, detail }));
  if (!detail.context) {
    throw new Error(`${el.localName}: no context provider above this element`);
  }
  return detail.context;
}
