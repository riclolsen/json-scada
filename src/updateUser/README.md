# {json:scada} updateUser Tool

Command-line tool that creates a JSON-SCADA user or resets an existing user's
password. Passwords are stored as bcrypt hashes in the `users` collection.

## Usage

    cd src/updateUser
    npm install
    node updateUser.js username password [email] [config file name]

- If the user exists, the password (and the email, when given) is updated.
- If the user does not exist, it is created with an empty `roles` array.

With no config file argument the tool reads `../../conf/json-scada.json`; the
path can also be set with `JS_CONFIG_FILE`. The `JS_SERVER_UPDATE_USERINSTANCE`
and `JS_SERVER_UPDATE_USERLOGLEVEL` environment variables are accepted by the
shared config loader.

A new user cannot log in until an administrator assigns a role (Admin UI →
Users, or the `roles` field in the `users` collection). Existing sessions keep
working until they expire.

The tool needs write access to the `users` collection. Run it on a host that can
reach the MongoDB server; no other JSON-SCADA process is required.
