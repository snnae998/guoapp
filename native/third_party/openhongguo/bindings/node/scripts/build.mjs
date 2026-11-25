// build.mjs 构建 npm 包：编译 wasm、复制 Go 的 wasm_exec.js 和许可证、
// 重新生成 TypeScript 类型。需要本机装有 Go。
import { execFileSync } from 'node:child_process';
import { copyFileSync, existsSync, mkdirSync, writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const pkg = dirname(dirname(fileURLToPath(import.meta.url)));
const repo = join(pkg, '..', '..');
const dist = join(pkg, 'dist');
mkdirSync(dist, { recursive: true });

const go = (args, env = {}) =>
  execFileSync('go', args, { cwd: repo, env: { ...process.env, ...env }, encoding: 'utf8', stdio: ['ignore', 'pipe', 'inherit'] });

go(['build', '-trimpath', '-ldflags=-s -w', '-o', join(dist, 'openhongguo.wasm'), './bindings/node/wasm'], {
  GOOS: 'js',
  GOARCH: 'wasm',
});

// wasm_exec.js 必须和编译 wasm 的 Go 版本一致。Go 1.24 起放在 lib/wasm。
const goroot = go(['env', 'GOROOT']).trim();
const execJS = [join(goroot, 'lib', 'wasm', 'wasm_exec.js'), join(goroot, 'misc', 'wasm', 'wasm_exec.js')].find(existsSync);
if (!execJS) throw new Error(`wasm_exec.js not found under ${goroot}`);
copyFileSync(execJS, join(dist, 'wasm_exec.js'));
copyFileSync(join(repo, 'LICENSE'), join(pkg, 'LICENSE'));

writeFileSync(join(pkg, 'types.d.ts'), go(['run', './bindings/node/tsgen']));
console.log('built', join(dist, 'openhongguo.wasm'));
