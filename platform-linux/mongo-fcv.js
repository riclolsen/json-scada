// Raises the MongoDB feature compatibility version (FCV) to the running server version, so that the next
// MongoDB major can open the data. Only on a standalone server or a single-member replica set: with more
// members every member must run the new version first. Used by install-lib.sh (mongosh --file).
// Prints one line: FCV_CURRENT <v> | FCV_SET <old> -> <v> | FCV_SKIP <reason> | FCV_FAIL <reason>
(function () {
  var v = db.version().split(".");
  var want = v[0] + "." + v[1];
  var members = null;
  try {
    members = rs.conf().members.length;
  } catch (e) {
    try {
      if (!db.adminCommand({ hello: 1 }).setName) members = 1;
    } catch (e2) {}
  }
  var fcv = null;
  try {
    fcv = db.adminCommand({ getParameter: 1, featureCompatibilityVersion: 1 }).featureCompatibilityVersion.version;
  } catch (e) {}
  if (fcv === want) {
    print("FCV_CURRENT " + want);
    return;
  }
  if (members !== 1) {
    print("FCV_SKIP replica set members=" + members + ", fcv=" + fcv + ", server=" + want +
      "; after all members run " + want + " run: db.adminCommand({setFeatureCompatibilityVersion: '" + want + "', confirm: true})");
    return;
  }
  try {
    var r = db.adminCommand({ setFeatureCompatibilityVersion: want, confirm: true });
    print(r.ok ? "FCV_SET " + fcv + " -> " + want : "FCV_FAIL " + (r.errmsg || JSON.stringify(r)));
  } catch (e) {
    print("FCV_FAIL " + e.message);
  }
})();
