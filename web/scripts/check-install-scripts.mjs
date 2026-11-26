import { readFileSync } from "node:fs";

const { allowScripts = {} } = JSON.parse(readFileSync(new URL("../package.json", import.meta.url), "utf8"));
const { packages } = JSON.parse(readFileSync(new URL("../package-lock.json", import.meta.url), "utf8"));

function excludesLinux(os = []) {
  const platforms = [os].flat();
  if (platforms.includes("!linux")) return true;

  const allowed = platforms.filter((platform) => !platform.startsWith("!"));
  return allowed.length > 0 && !allowed.includes("linux") &&
    !(platforms.length === 1 && platforms[0] === "any");
}

for (const [path, dependency] of Object.entries(packages)) {
  if (!path || dependency.hasInstallScript !== true) continue;
  if (dependency.optional === true && excludesLinux(dependency.os)) continue;

  // Lockfile aliases record the real package name separately from its path.
  const name = dependency.name ?? path.match(/(?:^|\/)node_modules\/((?:@[^/]+\/)?[^/]+)$/)?.[1];
  const approval = `${name}@${dependency.version}`;
  if (name && dependency.version && Object.hasOwn(allowScripts, approval) && allowScripts[approval] === true) continue;

  console.error(`Unapproved install script: ${name ? approval : path} (${path}). Review web/package.json's allowScripts.`);
  process.exitCode = 1;
}
