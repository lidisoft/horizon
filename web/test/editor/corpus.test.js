// The vectors in internal/document/testdata, shared with the Go
// implementation (internal/document/document_test.go): both must agree on
// every one.

import { schema } from "horizon/editor/model.js";
import { Mapping, rebase, stepFromJSON } from "horizon/editor/transform.js";
import { assert, describe, test } from "horizon-test/runner.js";

const load = (name) => fetch(`/test/corpus/${name}.json`).then((r) => r.json());
const show = (n) => JSON.stringify(n.toJSON());

function canonical(v) {
  if (Array.isArray(v)) return v.map(canonical);
  if (v && typeof v === "object") return Object.fromEntries(Object.keys(v).sort().map((k) => [k, canonical(v[k])]));
  return v;
}
const sameJSON = (a, b) => JSON.stringify(canonical(a)) === JSON.stringify(canonical(b));

describe("document corpus", () => {
  test("apply, invert and JSON round trip", async () => {
    const failures = [];
    for (const c of await load("apply")) {
      let doc = schema.nodeFromJSON(c.doc);
      const docs = [doc];
      let error = null;
      let steps = [];
      try {
        steps = c.steps.map((s) => stepFromJSON(s));
        for (const s of steps) {
          doc = s.apply(doc);
          docs.push(doc);
        }
      } catch (e) {
        error = e;
      }
      if (c.error) {
        if (!error) failures.push(`${c.name}: applied to ${show(doc)}`);
        continue;
      }
      if (error) {
        failures.push(`${c.name}: ${error.message}`);
        continue;
      }
      const want = schema.nodeFromJSON(c.result);
      if (!doc.eq(want)) failures.push(`${c.name}: got ${show(doc)}, want ${show(want)}`);
      for (let i = steps.length - 1; i >= 0; i--) doc = steps[i].invert(docs[i]).apply(doc);
      if (!doc.eq(docs[0])) failures.push(`${c.name}: inverted to ${show(doc)}`);
      for (const [i, s] of steps.entries()) {
        if (!sameJSON(s.toJSON(), c.steps[i])) failures.push(`${c.name}: step ${i} JSON ${JSON.stringify(s.toJSON())}`);
      }
    }
    assert.equal(failures.join("\n"), "");
  });

  test("mapping", async () => {
    const failures = [];
    for (const c of await load("mapping")) {
      let doc = schema.nodeFromJSON(c.doc);
      const m = new Mapping();
      for (const json of c.steps) {
        const s = stepFromJSON(json);
        doc = s.apply(doc);
        m.appendMap(s.getMap());
      }
      for (const p of c.positions) {
        const r = m.mapResult(p.pos, p.assoc);
        if (r.pos !== p.to || ("deleted" in p && r.deleted !== p.deleted)) {
          failures.push(`${c.name}: ${p.pos} (${p.assoc}) → ${r.pos} deleted ${r.deleted}; want ${p.to} ${p.deleted}`);
        }
      }
    }
    assert.equal(failures.join("\n"), "");
  });

  test("rebase", async () => {
    const failures = [];
    for (const c of await load("rebase")) {
      const { doc, steps } = rebase(schema.nodeFromJSON(c.doc), c.local.map((s) => stepFromJSON(s)), c.remote.map((s) => stepFromJSON(s)));
      const want = schema.nodeFromJSON(c.result);
      if (!doc.eq(want)) failures.push(`${c.name}: got ${show(doc)}, want ${show(want)}`);
      if (!sameJSON(steps.map((s) => s.toJSON()), c.rebased)) failures.push(`${c.name}: rebased ${JSON.stringify(steps.map((s) => s.toJSON()))}`);
    }
    assert.equal(failures.join("\n"), "");
  });

  test("schema", async () => {
    const failures = [];
    for (const c of await load("schema")) {
      let valid = true;
      let doc;
      try {
        doc = schema.nodeFromJSON(c.doc);
      } catch {
        valid = false;
      }
      if (valid !== c.valid) failures.push(`${c.name}: valid ${valid}, want ${c.valid}`);
      if (valid && !schema.nodeFromJSON(JSON.parse(show(doc))).eq(doc)) failures.push(`${c.name}: JSON round trip`);
    }
    assert.equal(failures.join("\n"), "");
  });
});
