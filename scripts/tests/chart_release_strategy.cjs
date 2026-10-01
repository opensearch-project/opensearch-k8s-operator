// Exercise the actual Release Please version shipped by the pinned action.
const assert = require('node:assert/strict');
const {readFileSync} = require('node:fs');
const path = require('node:path');
const {test} = require('node:test');
const Ajv = require('ajv');
const yaml = require('yaml');
const {configSchema, manifestSchema} = require('release-please');
const {Helm} = require('release-please/build/src/strategies/helm');
const {parseConventionalCommits} = require('release-please/build/src/commit');
const {CommitSplit} = require('release-please/build/src/util/commit-split');
const {TagName} = require('release-please/build/src/util/tag-name');
const {Version} = require('release-please/build/src/version');
const {Merge} = require('release-please/build/src/plugins/merge');
const {setLogger} = require('release-please/build/src/util/logger');

const root = path.resolve(__dirname, '../..');
const config = JSON.parse(readFileSync(path.join(root, 'release-please-config.json')));
const versions = JSON.parse(readFileSync(path.join(root, '.release-please-manifest.json')));
// Keep test expectations independent of subsequent real chart releases.
const fixtureVersions = {
  'charts/opensearch-operator': '3.0.14',
  'charts/opensearch-cluster': '3.3.5',
};
const paths = Object.keys(config.packages);
const logger = {info() {}, warn() {}, error() {}, debug() {}};
setLogger(logger);

function commit(message, changedPaths) {
  return {sha: 'b'.repeat(40), message, files: changedPaths};
}

async function proposals(commits) {
  const split = new CommitSplit({packagePaths: paths}).split(commits);
  const results = {};
  const candidates = [];
  for (const chartPath of paths) {
    const component = config.packages[chartPath].component;
    const strategy = new Helm({
      path: chartPath,
      component,
      targetBranch: 'main',
      includeComponentInTag: config['include-component-in-tag'],
      includeVInTag: config['include-v-in-tag'],
      tagSeparator: config['tag-separator'],
      changelogSections: config['changelog-sections'],
      logger,
      github: {
        repository: {owner: 'opensearch-project', repo: 'opensearch-k8s-operator'},
        async getFileContents(file) {
          const content = readFileSync(path.join(root, file), 'utf8');
          return {parsedContent: content, content: Buffer.from(content).toString('base64'), sha: 'a'.repeat(40)};
        },
      },
    });
    const latest = {
      tag: new TagName(Version.parse(fixtureVersions[chartPath]), component, '-', false),
      sha: 'a'.repeat(40),
      notes: '',
    };
    const pr = await strategy.buildReleasePullRequest(
      parseConventionalCommits(split[chartPath] || []), latest
    );
    if (pr) {
      candidates.push({path: chartPath, pullRequest: pr, config: {releaseType: 'helm'}});
      const update = pr.updates.find(update => update.path === `${chartPath}/Chart.yaml`);
      const original = readFileSync(path.join(root, update.path), 'utf8');
      const chart = yaml.parse(update.updater.updateContent(original, logger));
      assert.equal(chart.appVersion, yaml.parse(original).appVersion);
      assert.ok(pr.updates.some(update => update.path === `${chartPath}/CHANGELOG.md`));
      results[chartPath] = chart.version;
    }
  }
  if (candidates.length) {
    const merger = new Merge({}, 'main', {}, {
      pullRequestTitlePattern: config['group-pull-request-title-pattern'],
    });
    merger.logger = logger;
    const merged = await merger.run(candidates);
    assert.equal(merged.length, 1);
    assert.equal(merged[0].pullRequest.headRefName, 'release-please--branches--main');
    assert.equal(merged[0].pullRequest.title.toString(), 'chore: release main');
  }
  return results;
}

test('configuration and manifest match the pinned Release Please schemas', () => {
  const ajv = new Ajv({allErrors: true});
  assert.ok(ajv.validate(configSchema, config), JSON.stringify(ajv.errors));
  assert.ok(ajv.validate(manifestSchema, versions), JSON.stringify(ajv.errors));
});

test('a fix releases only the changed chart and preserves appVersion', async () => {
  assert.deepEqual(await proposals([
    commit('fix: correct the operator deployment', [`${paths[0]}/templates/deployment.yaml`]),
  ]), {[paths[0]]: '3.0.15'});
});

test('a change touching both charts versions each component independently', async () => {
  assert.deepEqual(await proposals([
    commit('feat: add a chart option', paths.map(p => `${p}/values.yaml`)),
  ]), {[paths[0]]: '3.1.0', [paths[1]]: '3.4.0'});
});

test('breaking chart changes receive a major version', async () => {
  assert.deepEqual(await proposals([
    commit('feat(helm)!: remove an option', [`${paths[1]}/values.yaml`]),
  ]), {[paths[1]]: '4.0.0'});
});

test('all accepted non-feature chart changes produce a patch release', async () => {
  for (const section of config['changelog-sections'].filter(s => s.type !== 'feat')) {
    assert.deepEqual(await proposals([
      commit(`${section.type}: update the chart`, [`${paths[1]}/README.md`]),
    ]), {[paths[1]]: '3.3.6'}, section.type);
  }
});

test('Go-only changes and empty histories do not release charts', async () => {
  assert.deepEqual(await proposals([commit('fix: update reconciler', ['opensearch-operator/pkg/reconcilers/cluster.go'])]), {});
  assert.deepEqual(await proposals([]), {});
});

test('repeated preparation does not keep incrementing an unreleased version', async () => {
  const commits = [commit('fix: update the chart', [`${paths[1]}/values.yaml`])];
  assert.deepEqual(await proposals(commits), await proposals(commits));
});
