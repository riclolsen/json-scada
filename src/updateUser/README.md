# {json:scada} updateUser Tool

Command-line tool that creates a JSON-SCADA user or resets an existing user's
password. Passwords are stored as bcrypt hashes in the `users` collection.

## Usage

    cd src/updateUser
    npm install
    node updateUser.js username password [email] [config file name]

- If the user exists, the password (and the email, when given) is updated and
  every session of that user issued before ends (its `tokenVersion` is
  incremented; the server notices within 10 seconds).
- If the user does not exist, it is created with an empty `roles` array.
- Passwords need at least 8 characters and are hashed with bcrypt cost 10, as
  in the server. LDAP users are refused: their password lives in the directory.
- The exit code is 1 when nothing was changed because of an error.

With no config file argument the tool reads `../../conf/json-scada.json`; the
path can also be set with `JS_CONFIG_FILE`. The `JS_SERVER_UPDATE_USERINSTANCE`
and `JS_SERVER_UPDATE_USERLOGLEVEL` environment variables are accepted by the
shared config loader.

A new user cannot log in until an administrator assigns a role (Admin UI →
Users, or the `roles` field in the `users` collection).

The tool needs write access to the `users` collection. Run it on a host that can
reach the MongoDB server; no other JSON-SCADA process is required.
