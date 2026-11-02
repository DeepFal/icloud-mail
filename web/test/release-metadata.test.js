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
  assert.ok(offline.includes("image: ghcr.io/deepfal/icloud-mail:${ICLOUD_MAIL_VERSION:-v" + version + "}\n"));
  assert.ok(offline.includes("image: ghcr.io/deepfal/icloud-mail-postgres:${ICLOUD_MAIL_VERSION:-v" + version + "}\n"));
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

test("default deployment pins DeepFal releases while source builds stay opt-in", async () => {
  const version = (await read("VERSION")).trim();
  const compose = await read("compose.yaml");
  assert.ok(compose.includes("ghcr.io/deepfal/icloud-mail:${ICLOUD_MAIL_VERSION:-v" + version + "}"));
  assert.ok(compose.includes("ghcr.io/deepfal/icloud-mail-postgres:${ICLOUD_MAIL_VERSION:-v" + version + "}"));
  assert.equal(compose.includes("    build:\n"), false);
  assert.equal((compose.match(/pull_policy: always/g) || []).length, 2);
  assert.ok((await read(".env.example")).includes(`ICLOUD_MAIL_VERSION=v${version}\n`));

  const build = await read("compose.build.yaml");
  assert.ok(build.includes("image: ghcr.io/deepfal/icloud-mail:${ICLOUD_MAIL_VERSION:-v" + version + "}"));
  assert.ok(build.includes("image: ghcr.io/deepfal/icloud-mail-postgres:${ICLOUD_MAIL_VERSION:-v" + version + "}"));
  assert.ok(build.includes("target: runtime"));
  assert.ok(build.includes("target: postgres-runtime"));

  const readme = await read("README.md");
  assert.ok(readme.includes("docker compose -f compose.yaml -f compose.build.yaml up -d --build --wait"));
  assert.equal(readme.includes("git clone --branch"), false);

  const workflow = await read(".github/workflows/publish-images.yml");
  assert.ok(workflow.includes("packages: write"));
  assert.ok(workflow.includes("platforms: linux/amd64,linux/arm64"));
  assert.ok(workflow.includes("target: postgres-runtime"));
  assert.ok(workflow.includes("type=raw,value=latest"));
  assert.ok(workflow.includes("IMAGE_NAMESPACE: deepfal"));
  assert.ok(workflow.includes("${{ env.IMAGE_NAMESPACE }}"));
  assert.ok(workflow.includes("org.opencontainers.image.revision=${{ github.sha }}"));
  assert.ok(workflow.includes("org.opencontainers.image.version=${{ github.ref_name }}"));
  assert.ok(workflow.includes("org.opencontainers.image.source=https://github.com/DeepFal/icloud-mail"));
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
