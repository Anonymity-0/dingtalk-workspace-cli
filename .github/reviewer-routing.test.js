'use strict';

const assert = require('node:assert/strict');
const {resolveReviewRouting} = require('./reviewer-routing');

function route(files, author = 'author', latestPusher = author) {
  return resolveReviewRouting({files: files.map((filename) => ({filename})), author, latestPusher});
}

{
  const result = route(['internal/helpers/chat_toolbar.go']);
  assert.deepEqual(result.reviewers, ['wxianfeng']);
  assert.equal(result.requiredReviewers, 1);
  assert.deepEqual(result.modules.map((module) => module.id), ['product:chat']);
}

{
  const result = route(['internal/helpers/chat_toolbar.go', 'internal/helpers/doc_style.go']);
  assert.deepEqual(result.reviewers, ['wxianfeng', 'typefield']);
  assert.equal(result.requiredReviewers, 2);
}

{
  const result = route(['.github/workflows/ci.yml']);
  assert.deepEqual(result.reviewers, ['haofeng0705', 'wxianfeng']);
  assert.equal(result.requiredReviewers, 2);
  assert.equal(result.reason, 'cross_or_sensitive');
}

{
  const result = route(['internal/auth/login.go'], 'hlzjsong');
  assert.deepEqual(result.reviewers, ['typefield', 'wxianfeng']);
  assert.equal(result.requiredReviewers, 2);
}

{
  const result = route(['internal/upgrade/downloader.go']);
  assert.deepEqual(result.reviewers, ['haofeng0705', 'wxianfeng']);
  assert.equal(result.requiredReviewers, 2);
}

{
  const result = route(['internal/app/upgrade.go', 'scripts/dev/test-release.sh']);
  assert.deepEqual(result.reviewers, ['haofeng0705', 'wxianfeng']);
  assert.equal(result.requiredReviewers, 2);
}

{
  const result = route(['pkg/edition/edition.go']);
  assert.deepEqual(result.reviewers, ['hlzjsong', 'typefield']);
  assert.equal(result.requiredReviewers, 2);
}

{
  const result = route(['internal/shortcut/chat/compatibility_coverage_test.go']);
  assert.deepEqual(result.reviewers, ['wxianfeng', 'typefield']);
  assert.equal(result.requiredReviewers, 2);
  assert.deepEqual(result.modules.map((module) => module.id), ['product:chat', 'compatibility']);
}

{
  const result = route(['internal/helpers/leaf_dispatch.go']);
  assert.deepEqual(result.reviewers, ['wxianfeng', 'typefield']);
  assert.equal(result.requiredReviewers, 2);
}

{
  const result = route(['docs/unknown-area.md']);
  assert.deepEqual(result.reviewers, []);
  assert.equal(result.reason, 'unknown_paths');
}

console.log('reviewer routing policy tests passed');
