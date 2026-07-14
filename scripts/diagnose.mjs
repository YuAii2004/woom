/* global console, process */

import { mkdir, readdir, readFile, stat, writeFile } from 'node:fs/promises'
import { extname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const textExtensions = new Set(['.log', '.txt', '.json', '.jsonl', '.md', '.yml', '.yaml'])
const sensitiveKey = /token|password|secret|authorization|cookie|api[_-]?key/i
const sensitiveValue = /Bearer\s+[^\s]+|eyJ[a-zA-Z0-9_-]+\.[a-zA-Z0-9_-]+\.[a-zA-Z0-9_-]+/g

const rules = [
  {
    code: 'dependency_connection_refused',
    severity: 'error',
    retryable: true,
    pattern: /(connection refused|actively refused|os error 10061|ECONNREFUSED)/i,
    cause: 'A required local dependency was not listening when the application tried to connect.',
    suggestions: ['检查 Redis 是否在目标端口持续运行', '确认依赖启动步骤与测试步骤处于同一进程生命周期', '检查 127.0.0.1 和端口配置是否一致']
  },
  {
    code: 'dependency_timeout',
    severity: 'error',
    retryable: true,
    pattern: /(timed out|timeout|ETIMEDOUT|超时)/i,
    cause: 'A dependency or browser operation did not respond before its deadline.',
    suggestions: ['检查依赖服务日志和端口健康状态', '确认 CI runner 的网络是否稳定', '增加超时前先确认服务是否真正 ready']
  },
  {
    code: 'browser_install_target_invalid',
    severity: 'error',
    retryable: false,
    pattern: /Invalid installation targets/i,
    cause: 'The Playwright browser installation target does not match a supported browser name.',
    suggestions: ['将 Edge 的 Playwright 安装目标映射为 msedge', '检查 playwright.config.ts 中的 browser project 配置']
  },
  {
    code: 'platform_image_unavailable',
    severity: 'error',
    retryable: false,
    pattern: /no matching manifest for/i,
    cause: 'The selected container image has no manifest for the runner operating system or architecture.',
    suggestions: ['确认镜像是否支持当前 runner 平台', '在桌面 runner 上改用原生依赖或使用兼容的容器平台']
  }
]

function sanitize(value) {
  if (Array.isArray(value)) return value.map(sanitize)
  if (value && typeof value === 'object') {
    return Object.fromEntries(Object.entries(value)
      .filter(([key]) => !sensitiveKey.test(key))
      .map(([key, item]) => [key, sanitize(item)]))
  }
  if (typeof value === 'string') return value.replace(sensitiveValue, '[REDACTED]')
  return value
}

function parseEvents(content) {
  return content.split(/\r?\n/).flatMap(line => {
    const trimmed = line.trim()
    if (!trimmed.startsWith('{') || !trimmed.endsWith('}')) return []
    try {
      const value = sanitize(JSON.parse(trimmed))
      return value && typeof value === 'object' && value.event && value.level ? [value] : []
    } catch {
      return []
    }
  })
}

function findingFromRule(rule, source) {
  return {
    code: rule.code,
    severity: rule.severity,
    retryable: rule.retryable,
    source,
    cause: rule.cause,
    suggestions: rule.suggestions
  }
}

export function analyzeLogs(inputs, environment = {}) {
  const events = inputs.flatMap(input => parseEvents(input.content).map(event => ({
    source: input.source,
    ...event
  })))
  const findings = []
  for (const input of inputs) {
    for (const rule of rules) {
      if (rule.pattern.test(input.content)) findings.push(findingFromRule(rule, input.source))
    }
  }
  for (const event of events) {
    const errorCode = event.error?.code
    const rule = rules.find(item => item.code === errorCode)
    if (rule && !findings.some(item => item.code === rule.code && item.source === event.source)) {
      findings.push(findingFromRule(rule, event.source))
    }
  }

  const uniqueFindings = findings.filter((finding, index, all) => all.findIndex(item => (
    item.code === finding.code && item.source === finding.source
  )) === index)

  return sanitize({
    generated_at: new Date().toISOString(),
    environment: {
      platform: environment.platform || process.env.RUNNER_OS || process.platform,
      browser: environment.browser || process.env.BROWSER || 'unknown',
      node: process.version
    },
    summary: {
      source_count: inputs.length,
      event_count: events.length,
      finding_count: uniqueFindings.length,
      highest_severity: uniqueFindings.some(item => item.severity === 'error') ? 'error' : uniqueFindings.length ? 'warning' : 'info'
    },
    events,
    findings: uniqueFindings
  })
}

export function renderMarkdown(report) {
  const environment = `${report.environment.platform} / ${report.environment.browser}`
  const lines = [
    '# Diagnostic Report',
    '',
    `- Environment: ${environment}`,
    `- Generated: ${report.generated_at}`,
    `- Sources: ${report.summary.source_count}`,
    `- Events: ${report.summary.event_count}`,
    `- Findings: ${report.summary.finding_count}`,
    '',
    '## Findings',
    ''
  ]

  if (!report.findings.length) {
    lines.push('No known failure pattern was detected.')
  } else {
    for (const finding of report.findings) {
      lines.push(`### ${finding.code}`, '', `- Severity: ${finding.severity}`, `- Retryable: ${finding.retryable}`, `- Source: ${finding.source}`, `- Cause: ${finding.cause}`, '- Suggestions:')
      for (const suggestion of finding.suggestions) lines.push(`  - ${suggestion}`)
      lines.push('')
    }
  }

  lines.push('## Structured Events', '')
  if (!report.events.length) {
    lines.push('No structured events were found.')
  } else {
    for (const event of report.events) {
      lines.push(`- \`${event.level}\` **${event.event}** from \`${event.source}\` (request: \`${event.request_id || 'n/a'}\`)`)
    }
  }
  return `${lines.join('\n')}\n`
}

async function collectFiles(paths) {
  const files = []
  async function visit(path) {
    let info
    try {
      info = await stat(path)
    } catch {
      return
    }
    if (info.isDirectory()) {
      for (const entry of await readdir(path)) await visit(join(path, entry))
      return
    }
    if (info.size > 4 * 1024 * 1024 || !textExtensions.has(extname(path).toLowerCase())) return
    try {
      files.push({ source: path, content: await readFile(path, 'utf8') })
    } catch {
      // Ignore files that disappear while a test runner is cleaning up.
    }
  }
  for (const path of paths) await visit(resolve(path))
  return files
}

function parseArguments(argv) {
  const inputs = []
  let outputDir = 'diagnostic-report'
  for (let index = 0; index < argv.length; index += 1) {
    if (argv[index] === '--input') inputs.push(argv[++index])
    else if (argv[index] === '--output-dir') outputDir = argv[++index]
    else if (argv[index] === '--help') {
      console.log('Usage: node scripts/diagnose.mjs [--input PATH]... [--output-dir PATH]')
      process.exit(0)
    }
  }
  return { inputs, outputDir }
}

async function main() {
  const { inputs, outputDir } = parseArguments(process.argv.slice(2))
  const defaultInputs = [
    'logs',
    'test-results',
    'playwright-report',
    process.env.RUNNER_TEMP ? join(process.env.RUNNER_TEMP, 'woom-native-deps') : ''
  ].filter(Boolean)
  const files = await collectFiles(inputs.length ? inputs : defaultInputs)
  const report = analyzeLogs(files, {
    platform: process.env.RUNNER_OS || process.platform,
    browser: process.env.BROWSER || 'unknown'
  })
  const destination = resolve(outputDir)
  await mkdir(destination, { recursive: true })
  await writeFile(join(destination, 'diagnostic-report.json'), `${JSON.stringify(report, null, 2)}\n`)
  await writeFile(join(destination, 'diagnostic-report.md'), renderMarkdown(report))
  console.log(`Generated diagnostic reports in ${destination}`)
}

if (process.argv[1] && resolve(process.argv[1]) === resolve(fileURLToPath(import.meta.url))) await main()
