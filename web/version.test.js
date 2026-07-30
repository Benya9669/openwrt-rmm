const assert = require("node:assert/strict");
const { compareSemVer, parseSemVer } = require("./version.js");

assert.deepEqual(parseSemVer("v0.6.8"), {
  major: 0,
  minor: 6,
  patch: 8,
  prerelease: [],
});
assert.equal(compareSemVer("0.6.7", "0.6.8"), -1);
assert.equal(compareSemVer("0.6.8", "0.6.8"), 0);
assert.equal(compareSemVer("0.6.9", "0.6.8"), 1);
assert.equal(compareSemVer("0.7.0", "0.6.99"), 1);
assert.equal(compareSemVer("0.6.8-rc.1", "0.6.8"), -1);
assert.equal(compareSemVer("invalid", "0.6.8"), null);
