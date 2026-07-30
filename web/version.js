(function exposeVersionHelpers(root, factory) {
  const helpers = factory();
  if (typeof module === "object" && module.exports) module.exports = helpers;
  if (root) root.RMMVersions = helpers;
})(typeof globalThis !== "undefined" ? globalThis : this, function createVersionHelpers() {
  function parseSemVer(value) {
    const match = String(value || "").trim().match(/^v?(\d+)\.(\d+)\.(\d+)(?:-([0-9A-Za-z.-]+))?(?:\+[0-9A-Za-z.-]+)?$/);
    if (!match) return null;
    return {
      major: Number(match[1]),
      minor: Number(match[2]),
      patch: Number(match[3]),
      prerelease: match[4] ? match[4].split(".") : [],
    };
  }

  function comparePrerelease(left, right) {
    if (!left.length && !right.length) return 0;
    if (!left.length) return 1;
    if (!right.length) return -1;
    const length = Math.max(left.length, right.length);
    for (let index = 0; index < length; index += 1) {
      const a = left[index];
      const b = right[index];
      if (a === undefined) return -1;
      if (b === undefined) return 1;
      if (a === b) continue;
      const aNumber = /^\d+$/.test(a);
      const bNumber = /^\d+$/.test(b);
      if (aNumber && bNumber) return Number(a) < Number(b) ? -1 : 1;
      if (aNumber !== bNumber) return aNumber ? -1 : 1;
      return a < b ? -1 : 1;
    }
    return 0;
  }

  function compareSemVer(leftValue, rightValue) {
    const left = parseSemVer(leftValue);
    const right = parseSemVer(rightValue);
    if (!left || !right) return null;
    for (const key of ["major", "minor", "patch"]) {
      if (left[key] !== right[key]) return left[key] < right[key] ? -1 : 1;
    }
    return comparePrerelease(left.prerelease, right.prerelease);
  }

  return { parseSemVer, compareSemVer };
});
