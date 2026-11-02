import test from "node:test";
import assert from "node:assert/strict";
import { readFile, access } from "node:fs/promises";

const root = new URL("../../", import.meta.url);
const read = (path) => readFile(new URL(path, root), "utf8");

test("release version matches package metadata and public documentation", async () => {
  const version = (await read("VERSION")).trim();
  const manifest = JSON.parse(await read("web/package.json"));
  const lock = JSON.parse(await read("web/package-lock.json"));
  assert.match(version, /^\d+\.\d+\.\d+$/);
  assert.equal(manifest.version, version);
  assert.equal(lock.version, version);
  assert.equal(lock.packages[""].version, version);
  assert.deepEqual(lock.packages[""].dependencies, manifest.dependencies);
  assert.deepEqual(lock.packages[""].devDependencies, manifest.devDependencies);
  assert.ok((await read("DELIVERY.md")).includes("`" + version + "`"));
  assert.ok((await read("CHANGELOG.md")).includes("## Unreleased"));
  const offline = await read("compose.offline.yaml");
  assert.ok(offline.includes(`image: icloud-api:v${version}\n`));
  assert.ok(offline.includes(`image: icloud-api-postgres:v${version}\n`));
});

test("DeepFal branding keeps license attribution in the legal notices", async () => {
  const readme = await read("README.md");
  assert.ok(readme.includes("DeepFal/icloud-mail"));
  for (const match of readme.matchAll(/https:\/\/github\.com\/([^\s)]+)/g)) {
    assert.ok(match[1].startsWith("DeepFal/icloud-mail"));
  }
  assert.ok(readme.includes("[NOTICE](NOTICE.md)"));
  const license = await read("LICENSE");
  assert.ok(license.includes("Copyright (c) 2026 DeepFal"));
  const copyrights = new Set(license.match(/^Copyright .+$/gm));
  assert.ok(copyrights.size >= 2, "pre-existing copyright notices must remain");
  assert.ok(license.includes("The above copyright notice and this permission notice shall be included"));
  assert.ok((await read("NOTICE.md")).includes("## 第三方参考与依赖"));
});

test("default deployment pulls GHCR latest while source builds stay opt-in", async () => {
  const compose = await read("compose.yaml");
  assert.ok(compose.includes("ghcr.io/deepfal/icloud-mail:latest"));
  assert.ok(compose.includes("ghcr.io/deepfal/icloud-mail-postgres:latest"));
  assert.equal(compose.includes("    build:\n"), false);
  assert.equal((compose.match(/pull_policy: always/g) || []).length, 2);

  const build = await read("compose.build.yaml");
  assert.ok(build.includes("image: icloud-api:local"));
  assert.ok(build.includes("image: icloud-api-postgres:local"));
  assert.ok(build.includes("target: postgres-runtime"));

  const readme = await read("README.md");
  assert.ok(readme.includes("docker compose -f compose.yaml -f compose.build.yaml up -d --build --wait"));
  assert.equal(readme.includes("git clone --branch"), false);

  const workflow = await read(".github/workflows/publish-images.yml");
  assert.ok(workflow.includes("packages: write"));
  assert.ok(workflow.includes("platforms: linux/amd64,linux/arm64"));
  assert.ok(workflow.includes("target: postgres-runtime"));
  assert.ok(workflow.includes("type=raw,value=latest"));
  assert.ok(workflow.includes("${GITHUB_REPOSITORY_OWNER,,}"));
  assert.ok(workflow.includes("steps.namespace.outputs.owner"));
});

test("local links in release documentation resolve after reference extraction", async () => {
  for (const path of ["README.md", "NOTICE.md", "DELIVERY.md", "CHANGELOG.md", "docs/REFERENCE.md"]) {
    const source = await read(path);
    for (const match of source.matchAll(/\[[^\]]*\]\(([^)\s]+)\)/g)) {
      const target = match[1];
      if (/^(?:https?:|mailto:|#)/.test(target)) continue;
      const url = new URL(target, new URL(path, root));
      url.hash = "";
      await assert.doesNotReject(access(url), `${path}: ${target}`);
    }
  }
});
