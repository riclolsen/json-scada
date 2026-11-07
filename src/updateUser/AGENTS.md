# DOX: src/updateUser — User Management Utility

## Purpose

Node.js command-line utility for JSON-SCADA user accounts: creates a user or resets an existing user's password (bcrypt hash) and email. Roles are not assigned by this tool; an administrator does that in the Admin UI.

## Ownership

- updateUser owns the user management CLI tool

## Local Contracts

- **Language:** Node.js
- **Main entry:** `updateUser.js`
- **Pattern:** standard Node.js app (`app-defs.js`, `load-config.js`, `simple-logger.js`)
- **Config:** JSON-SCADA config file (`../../conf/json-scada.json` or `JS_CONFIG_FILE`)
- **Execution:** Command-line only (not a service)

## Work Guidance

- Usage: `node updateUser.js username password [email] [config file name]`
- Creates the user with an empty `roles` array when missing; updates password (and email when given) otherwise
- Operates directly on the `users` collection
- Reports that an administrator must assign a role before the new user can log in

## Verification

- Create a user, verify the bcrypt hash in `users`, then assign a role and log in
