#!/usr/bin/env node
/**
 * Ralph Triage Script — Standalone CJS implementation
 *
 * ⚠️ SYNC NOTICE: This file ports triage logic from the SDK source:
 *   packages/squad-sdk/src/ralph/triage.ts
 *
 * Any changes to routing/triage logic MUST be applied to BOTH files.
 * The SDK module is the canonical implementation; this script exists
 * for zero-dependency use in GitHub Actions workflows.
 *
 * To verify parity: npm test -- test/ralph-triage.test.ts
 */
'use strict';

const fs = require('node:fs');
const path = require('node:path');
const https = require('node:https');
const { execSync } = require('node:child_process');

function parseArgs(argv) {
  let squadDir = '.squad';
  let output = 'triage-results.json';

  for (let i = 0; i < argv.length; i += 1) {
    const arg = argv[i];
    if (arg === '--squad-dir') {
      squadDir = argv[i + 1];
      i += 1;
      continue;
    }
    if (arg === '--output') {
      output = argv[i + 1];
      i += 1;
      continue;
    }
    if (arg === '--help' || arg === '-h') {
      printUsage();
      process.exit(0);
    }
    throw new Error(`Unknown argument: ${arg}`);
  }

  if (!squadDir) throw new Error('--squad-dir requires a value');
  if (!output) throw new Error('--output requires a value');

  return { squadDir, output };
}

function printUsage() {
  console.log('Usage: node .squad/templates/ralph-triage.js --squad-dir .squad --output triage-results.json');
}

function normalizeEol(content) {
  return content.replace(/\r\n/g, '\n').replace(/\r/g, '\n');
}

function parseRoutingRules(routingMd) {
  const table = parseTableSection(routingMd, /^##\s*work\s*type\s*(?:→|->)\s*agent\b/i);
  if (!table) return [];

  const workTypeIndex = findColumnIndex(table.headers, ['work type', 'type']);
  const agentIndex = findColumnIndex(table.headers, ['agent', 'route to', 'route']);
  const examplesIndex = findColumnIndex(table.headers, ['examples', 'example']);

  if (workTypeIndex < 0 || agentIndex < 0) return [];

  const rules = [];
  for (const row of table.rows) {
    const workType = cleanCell(row[workTypeIndex] || '');
    const agentName = cleanCell(row[agentIndex] || '');
    const keywords = splitKeywords(examplesIndex >= 0 ? row[examplesIndex] : '');
    if (!workType || !agentName) continue;
    rules.push({ workType, agentName, keywords });
  }

  return rules;
}

function parseModuleOwnership(routingMd) {
  const table = parseTableSection(routingMd, /^##\s*module\s*ownership\b/i);
  if (!table) return [];

  const moduleIndex = findColumnIndex(table.headers, ['module', 'path']);
  const primaryIndex = findColumnIndex(table.headers, ['primary']);
  const secondaryIndex = findColumnIndex(table.headers, ['secondary']);

  if (moduleIndex < 0 || primaryIndex < 0) return [];

  const modules = [];
  for (const row of table.rows) {
    const modulePath = normalizeModulePath(row[moduleIndex] || '');
    const primary = cleanCell(row[primaryIndex] || '');
    const secondaryRaw = cleanCell(secondaryIndex >= 0 ? row[secondaryIndex] || '' : '');
    const secondary = normalizeOptionalOwner(secondaryRaw);

    if (!modulePath || !primary) continue;
    modules.push({ modulePath, primary, secondary });
  }

  return modules;
}

function parseRoster(teamMd) {
  const table =
    parseTableSection(teamMd, /^##\s*members\b/i) ||
    parseTableSection(teamMd, /^##\s*team\s*roster\b/i);

  if (!table) return [];

  const nameIndex = findColumnIndex(table.headers, ['name']);
  const roleIndex = findColumnIndex(table.headers, ['role']);
  if (nameIndex < 0 || roleIndex < 0) return [];

  const excluded = new Set(['scribe', 'ralph']);
  const members = [];

  for (const row of table.rows) {
    const name = cleanCell(row[nameIndex] || '');
    const role = cleanCell(row[roleIndex] || '');
    if (!name || !role) continue;
    if (excluded.has(name.toLowerCase())) continue;

    members.push({
      name,
      role,
      label: `squad:${name.toLowerCase()}`,
    });
  }

  return members;
}

function triageIssue(issue, rules, modules, roster) {
  const issueText = `${issue.title}\n${issue.body || ''}`.toLowerCase();
  const normalizedIssueText = normalizeTextForPathMatch(issueText);

  const bestModule = findBestModuleMatch(normalizedIssueText, modules);
  if (bestModule) {
    const primaryMember = findMember(bestModule.primary, roster);
    if (primaryMember) {
      return {
        agent: primaryMember,
        reason: `Matched module path "${bestModule.modulePath}" to primary owner "${bestModule.primary}"`,
        source: 'module-ownership',
        confidence: 'high',
      };
    }

    if (bestModule.secondary) {
      const secondaryMember = findMember(bestModule.secondary, roster);
      if (secondaryMember) {
        return {
          agent: secondaryMember,
          reason: `Matched module path "${bestModule.modulePath}" to secondary owner "${bestModule.secondary}"`,
          source: 'module-ownership',
          confidence: 'medium',
        };
      }
    }
  }

  const bestRule = findBestRuleMatch(issueText, rules);
  if (bestRule) {
    const agent = findMember(bestRule.rule.agentName, roster);
    if (agent) {
      return {
        agent,
        reason: `Matched routing keyword(s): ${bestRule.matchedKeywords.join(', ')}`,
        source: 'routing-rule',
        confidence: bestRule.matchedKeywords.length >= 2 ? 'high' : 'medium',
      };
    }
  }

  const roleMatch = findRoleKeywordMatch(issueText, roster);
  if (roleMatch) {
    return {
      agent: roleMatch.agent,
      reason: roleMatch.reason,
      source: 'role-keyword',
      confidence: 'medium',
    };
  }

  const lead = findLeadFallback(roster);
  if (!lead) return null;

  return {
    agent: lead,
    reason: 'No module, routing, or role keyword match — routed to Lead/Architect',
    source: 'lead-fallback',
    confidence: 'low',
  };
}

function parseTableSection(markdown, sectionHeader) {
  const lines = normalizeEol(markdown).split('\n');
  let inSection = false;
  const tableLines = [];

  for (const line of lines) {
    const trimmed = line.trim();
    if (!inSection && sectionHeader.test(trimmed)) {
      inSection = true;
      continue;
    }
    if (inSection && /^##\s+/.test(trimmed)) break;
    if (inSection && trimmed.startsWith('|')) tableLines.push(trimmed);
  }

  if (tableLines.length === 0) return null;

  let headers = null;
  const rows = [];

  for (const line of tableLines) {
    const cells = parseTableLine(line);
    if (cells.length === 0) continue;
    if (cells.every((cell) => /^:?-{2,}:?$/.test(cell))) continue;

    if (!headers) {
      headers = cells;
      continue;
    }

    rows.push(cells);
  }

  if (!headers) return null;
  return { headers, rows };
}

function parseTableLine(line) {
  return line
    .replace(/^\|/, '')
    .replace(/\|$/, '')
    .split('|')
    .map((cell) => cell.trim());
}

function findColumnIndex(headers, candidates) {
  const normalizedHeaders = headers.map((header) => cleanCell(header).toLowerCase());
  for (const candidate of candidates) {
    const index = normalizedHeaders.findIndex((header) => header.includes(candidate));
    if (index >= 0) return index;
  }
  return -1;
}

function cleanCell(value) {
  return value
    .replace(/`/g, '')
    .replace(/\[([^\]]+)\]\([^)]+\)/g, '$1')
    .trim();
}

function splitKeywords(examplesCell) {
  if (!examplesCell) return [];
  return examplesCell
    .split(',')
    .map((keyword) => cleanCell(keyword))
    .filter((keyword) => keyword.length > 0);
}

function normalizeOptionalOwner(owner) {
  if (!owner) return null;
  if (/^[-—–]+$/.test(owner)) return null;
  return owner;
}

function normalizeModulePath(modulePath) {
  return cleanCell(modulePath).replace(/\\/g, '/').toLowerCase();
}

function normalizeTextForPathMatch(text) {
  return text.replace(/\\/g, '/').replace(/`/g, '');
}

function normalizeName(value) {
  return cleanCell(value)
    .toLowerCase()
    .replace(/[^\w@\s-]/g, '')
    .replace(/\s+/g, ' ')
    .trim();
}

function findMember(target, roster) {
  const normalizedTarget = normalizeName(target);
  if (!normalizedTarget) return null;

  for (const member of roster) {
    if (normalizeName(member.name) === normalizedTarget) return member;
  }

  for (const member of roster) {
    if (normalizeName(member.role) === normalizedTarget) return member;
  }

  for (const member of roster) {
    const memberName = normalizeName(member.name);
    if (normalizedTarget.includes(memberName) || memberName.includes(normalizedTarget)) {
      return member;
    }
  }

  for (const member of roster) {
    const memberRole = normalizeName(member.role);
    if (normalizedTarget.includes(memberRole) || memberRole.includes(normalizedTarget)) {
      return member;
    }
  }

  return null;
}

function findBestModuleMatch(issueText, modules) {
  let best = null;
  let bestLength = -1;

  for (const module of modules) {
    const modulePath = normalizeModulePath(module.modulePath);
    if (!modulePath) continue;
    if (!issueText.includes(modulePath)) continue;

    if (modulePath.length > bestLength) {
      best = module;
      bestLength = modulePath.length;
    }
  }

  return best;
}

function findBestRuleMatch(issueText, rules) {
  let best = null;
  let bestScore = 0;

  for (const rule of rules) {
    const matchedKeywords = rule.keywords
      .map((keyword) => keyword.toLowerCase())
      .filter((keyword) => keyword.length > 0 && issueText.includes(keyword));

    if (matchedKeywords.length === 0) continue;

    const score =
      matchedKeywords.length * 100 + matchedKeywords.reduce((sum, keyword) => sum + keyword.length, 0);
    if (score > bestScore) {
      best = { rule, matchedKeywords };
      bestScore = score;
    }
  }

  return best;
}

function findRoleKeywordMatch(issueText, roster) {
  for (const member of roster) {
    const role = member.role.toLowerCase();

    if (
      (role.includes('frontend') || role.includes('ui')) &&
      (issueText.includes('ui') || issueText.includes('frontend') || issueText.includes('css'))
    ) {
      return { agent: member, reason: 'Matched frontend/UI role keywords' };
    }

    if (
      (role.includes('backend') || role.includes('api') || role.includes('server')) &&
      (issueText.includes('api') || issueText.includes('backend') || issueText.includes('database'))
    ) {
      return { agent: member, reason: 'Matched backend/API role keywords' };
    }

    if (
      (role.includes('test') || role.includes('qa')) &&
      (issueText.includes('test') || issueText.includes('bug') || issueText.includes('fix'))
    ) {
      return { agent: member, reason: 'Matched testing/QA role keywords' };
    }
  }

  return null;
}

function findLeadFallback(roster) {
  return (
    roster.find((member) => {
      const role = member.role.toLowerCase();
      return role.includes('lead') || role.includes('architect');
    }) || null
  );
}

function parseOwnerRepoFromRemote(remoteUrl) {
  const sshMatch = remoteUrl.match(/^git@[^:]+:([^/]+)\/(.+?)(?:\.git)?$/);
  if (sshMatch) return { owner: sshMatch[1], repo: sshMatch[2] };

  if (remoteUrl.startsWith('http://') || remoteUrl.startsWith('https://') || remoteUrl.startsWith('ssh://')) {
    const parsed = new URL(remoteUrl);
    const parts = parsed.pathname.replace(/^\/+/, '').replace(/\.git$/, '').split('/');
    if (parts.length >= 2) {
      return { owner: parts[0], repo: parts[1] };
    }
  }

  throw new Error(`Unable to parse owner/repo from remote URL: ${remoteUrl}`);
}

function getOwnerRepoFromGit() {
  const remoteUrl = execSync('git remote get-url origin', { encoding: 'utf8' }).trim();
  return parseOwnerRepoFromRemote(remoteUrl);
}

function githubRequestJson(pathname, token) {
  return new Promise((resolve, reject) => {
    const req = https.request(
      {
        hostname: 'api.github.com',
        method: 'GET',
        path: pathname,
        headers: {
          Accept: 'application/vnd.github+json',
          Authorization: `Bearer ${token}`,
          'User-Agent': 'squad-ralph-triage',
          'X-GitHub-Api-Version': '2022-11-28',
        },
      },
      (res) => {
        let body = '';
        res.setEncoding('utf8');
        res.on('data', (chunk) => {
          body += chunk;
        });
        res.on('end', () => {
          if ((res.statusCode || 500) >= 400) {
            reject(new Error(`GitHub API ${res.statusCode}: ${body}`));
            return;
          }
          try {
            resolve(JSON.parse(body));
          } catch (error) {
            reject(new Error(`Failed to parse GitHub response: ${error.message}`));
          }
        });
      },
    );
    req.on('error', reject);
    req.end();
  });
}

async function fetchSquadIssues(owner, repo, token) {
  const all = [];
  let page = 1;
  const perPage = 100;

  for (;;) {
    const query = new URLSearchParams({
      state: 'open',
      labels: 'squad',
      per_page: String(perPage),
      page: String(page),
    });
    const issues = await githubRequestJson(`/repos/${owner}/${repo}/issues?${query.toString()}`, token);
    if (!Array.isArray(issues) || issues.length === 0) break;
    all.push(...issues);
    if (issues.length < perPage) break;
    page += 1;
  }

  return all;
}

function issueHasLabel(issue, labelName) {
  const target = labelName.toLowerCase();
  return (issue.labels || []).some((label) => {
    if (!label) return false;
    const name = typeof label === 'string' ? label : label.name;
    return typeof name === 'string' && name.toLowerCase() === target;
  });
}

function isUntriagedIssue(issue, memberLabels) {
  if (issue.pull_request) return false;
  if (!issueHasLabel(issue, 'squad')) return false;
  return !memberLabels.some((label) => issueHasLabel(issue, label));
}

async function main() {
  const args = parseArgs(process.argv.slice(2));
  const token = process.env.GITHUB_TOKEN;
  if (!token) {
    throw new Error('GITHUB_TOKEN is required');
  }

  const squadDir = path.resolve(process.cwd(), args.squadDir);
  const teamMd = fs.readFileSync(path.join(squadDir, 'team.md'), 'utf8');
  const routingMd = fs.readFileSync(path.join(squadDir, 'routing.md'), 'utf8');

  const roster = parseRoster(teamMd);
  const rules = parseRoutingRules(routingMd);
  const modules = parseModuleOwnership(routingMd);

  const { owner, repo } = getOwnerRepoFromGit();
  const openSquadIssues = await fetchSquadIssues(owner, repo, token);

  const memberLabels = roster.map((member) => member.label);
  const untriaged = openSquadIssues.filter((issue) => isUntriagedIssue(issue, memberLabels));

  const results = [];
  for (const issue of untriaged) {
    const decision = triageIssue(
      {
        number: issue.number,
        title: issue.title || '',
        body: issue.body || '',
        labels: [],
      },
      rules,
      modules,
      roster,
    );

    if (!decision) continue;
    results.push({
      issueNumber: issue.number,
      assignTo: decision.agent.name,
      label: decision.agent.label,
      reason: decision.reason,
      source: decision.source,
    });
  }

  const outputPath = path.resolve(process.cwd(), args.output);
  fs.mkdirSync(path.dirname(outputPath), { recursive: true });
  fs.writeFileSync(outputPath, `${JSON.stringify(results, null, 2)}\n`, 'utf8');
}

main().catch((error) => {
  console.error(error.message);
  process.exit(1);
});
