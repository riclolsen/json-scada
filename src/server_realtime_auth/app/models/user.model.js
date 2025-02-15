const mongoose = require('mongoose')

const User = mongoose.model(
  'User',
  new mongoose.Schema({
    username: { type: String, default: 'new user' },
    email: { type: String, default: '' },
    password: { type: String, default: '' },
    roles: [
      {
        type: mongoose.Schema.Types.ObjectId,
        ref: 'Role',
      },
    ],
    isLDAPUser: { type: Boolean, default: false },
    ldapDN: { type: String, default: '' }, // Distinguished Name from LDAP
    lastLDAPSync: { type: Date, default: null }, // Last time user was synced with LDAP
  }),
  'users'
)

module.exports = User
