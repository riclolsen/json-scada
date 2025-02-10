// Configuration for JWT and LDAP authentication

module.exports = {
  // JWT secret key
  secret: 'json-scada-secret-key',
  
  // LDAP Configuration
  ldap: {
    enabled: false, // Enable/disable LDAP authentication
    url: 'ldap://ldap.forumsys.com:389',
    bindDN: 'cn=read-only-admin,dc=example,dc=com', // Admin bind DN
    bindCredentials: 'password', // Admin bind password
    searchBase: 'dc=example,dc=com', // Search base for users
    searchFilter: '(uid={{username}})', // Search filter, {{username}} will be replaced with the login username
    // User attributes mapping
    attributes: {
      username: 'uid',
      email: 'mail',
      displayName: 'cn'
    },
    // Default role for LDAP users (should match a role name defined in your MongoDB)
    defaultRole: 'user',
    // Group mapping (optional) - maps LDAP groups to local roles
    groupMapping: {
      'cn=admins,dc=example,dc=com': 'admin',
      'cn=users,dc=example,dc=com': 'user'
    }
  }
}
