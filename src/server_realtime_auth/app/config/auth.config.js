// Configuration for JWT and LDAP authentication

module.exports = {
  // JWT secret key
  secret: 'json-scada-secret-key',
  
  // LDAP Configuration
  ldap: {
    enabled: true, // Enable/disable LDAP authentication

    // Example for MS Active Directory Server
    url: 'ldap://192.168.0.26:389',
    bindDN: 'cn=Administrator,cn=Users,dc=ad,dc=gpfs,dc=net', // Admin bind DN
    bindCredentials: 'secret', // Admin bind password
    searchBase: 'ou=JSON-SCADA,dc=ad,dc=gpfs,dc=net', // Search base for users
    searchFilter: '(sAMAccountName={{username}})', // Search filter, {{username}} will be replaced with the login username
    attributes: {
      username: 'sAMAccountName',
      email: 'mail',
      displayName: 'cn',
      memberOf: 'memberOf',
    },
    // Group search base (optional) - if not provided, the search base will be used
    groupSearchBase: 'ou=JSON-SCADA,dc=ad,dc=gpfs,dc=net',
    // Group mapping (optional) - maps LDAP groups to local roles
    groupMapping: {
      'CN=captains,ou=JSON-SCADA,dc=ad,dc=gpfs,dc=net': 'admin',
      'cn=pilots,ou=JSON-SCADA,dc=ad,dc=gpfs,dc=net': 'user'
    },

//    // Example for other LDAP server like Apache Directory Server
//    url: 'ldap://ldap.forumsys.com:389',
//    bindDN: 'cn=read-only-admin,dc=example,dc=com', // Admin bind DN
//    bindCredentials: 'password', // Admin bind password
//    searchBase: 'dc=example,dc=com', // Search base for users
//    searchFilter: '(uid={{username}})', // Search filter, {{username}} will be replaced with the login username
//    // User attributes mapping
//    attributes: {
//      username: 'uid',
//      email: 'mail',
//      displayName: 'cn'
//    },
//    // Group search base (optional) - if not provided, the search base will be used
//    groupSearchBase: 'ou=groups,dc=example,dc=com',
//    // Group mapping (optional) - maps LDAP groups to local roles
//    groupMapping: {
//      'cn=admins,dc=example,dc=com': 'admin',
//      'cn=users,dc=example,dc=com': 'user'
//    },

    // Default role for LDAP users (should match a role name defined in your MongoDB)
    defaultRole: 'user',
    tlsOptions: {
      rejectUnauthorized: true, // Set to true if you want to reject unauthorized certificates
      minVersion: 'TLSv1.2', // Minimum TLS version to accept
      maxVersion: 'TLSv1.3', // Maximum TLS version to accept
      //ca: [fs.readFileSync('/path/to/ca.crt')],
      //cert: fs.readFileSync('/path/to/client.crt'),
      //key: fs.readFileSync('/path/to/client.key'),
      //passphrase: '',
      //// Or you can add a pfx file containing both the client certificate and key
      //pfx: fs.readFileSync('/path/to/client.pfx'),
      //// You can add a CRL file to check against revoked certificates
      //crl: fs.readFileSync('/path/to/crl.pem'),
      //ciphers: 'DEFAULT:!aNULL:!eNULL:!LOW:!EXPORT:!SSLv2:!MD5', // Cipher suite to use
      //secureProtocol: 'TLSv1_2_method', // TLS version to use
    }
  }
}
