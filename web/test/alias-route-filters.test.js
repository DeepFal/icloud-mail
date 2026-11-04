import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { effectScope, nextTick, reactive, ref, watch } from "vue";

const source = await readFile(new URL("../src/views/AliasesView.vue", import.meta.url), "utf8");
const filtersSource = source.slice(source.indexOf("// Filters live in the URL"), source.indexOf("const currentPage = ref(1)"));

function setup(query) {
  const route = reactive({ name: "aliases", query });
  const fields = Object.fromEntries(["keywordDraft", "appliedAliasQuery", "selectedAccountId", "selectedGroupFilter", "appliedGroupId", "selectedLatestMailFilter"].map((name) => [name, ref("")]));
  let reloads = 0, replacements = 0;
  const router = { replace({ query }) { replacements++; route.query = query; return Promise.resolve(); } };
  const scope = effectScope();
  scope.run(() => new Function("watch", "route", "router", "reloadAliasesForFilters", ...Object.keys(fields), filtersSource)(watch, route, router, () => reloads++, ...Object.values(fields)));
  return { route, fields, scope, reloads: () => reloads, replacements: () => replacements };
}

const values = (fields) => Object.fromEntries(Object.entries(fields).map(([name, value]) => [name, value.value]));

test("same-component alias navigation updates all four filters and clears them from the sidebar", async (t) => {
  const view = setup({ q: "first", account: "12", group: "3", mail: "none" });
  t.after(() => view.scope.stop());
  assert.equal(view.fields.appliedAliasQuery.value, "first");
  view.route.query = { q: " second ", account: "21", group: "none", mail: "yes" };
  await nextTick();
  assert.deepEqual(values(view.fields), {
    keywordDraft: "second", appliedAliasQuery: "second", selectedAccountId: 21,
    selectedGroupFilter: "none", appliedGroupId: "none", selectedLatestMailFilter: "yes",
  });
  assert.equal(view.reloads(), 1);
  view.route.query = {};
  await nextTick();
  assert.ok(Object.values(values(view.fields)).every((value) => value === ""));
  assert.equal(view.reloads(), 2);
  const replacements = view.replacements();
  await nextTick();
  assert.equal(view.replacements(), replacements, "route synchronization settles without a replace loop");
});

test("local URL writes do not reload twice and equivalent route values do not reload", async (t) => {
  const view = setup({ q: "first", account: "12" });
  t.after(() => view.scope.stop());
  view.fields.appliedAliasQuery.value = "second";
  await nextTick();
  assert.equal(view.route.query.q, "second");
  assert.equal(view.replacements(), 1);
  assert.equal(view.reloads(), 0, "local filter handlers already reload the list");
  view.route.query = { q: "second", account: "12", group: "", mail: "" };
  await nextTick();
  assert.equal(view.reloads(), 0);
  assert.equal(view.replacements(), 1);
  view.route.name = "accounts";
  view.route.query = { q: "ignored", account: "99", mail: "yes" };
  await nextTick();
  assert.equal(view.fields.appliedAliasQuery.value, "second");
  assert.equal(view.reloads(), 0);
});

test("invalid route filter values use existing defaults", async (t) => {
  const view = setup({ q: "first", account: "12", mail: "yes" });
  t.after(() => view.scope.stop());
  view.route.query = { q: ["one", "two"], account: "oops", group: ["1"], mail: "invalid" };
  await nextTick();
  assert.ok(Object.values(values(view.fields)).every((value) => value === ""));
  assert.equal(view.reloads(), 1);
});
