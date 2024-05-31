var express = require('express')

const { postgraphile } = require('postgraphile')

var app = express()
app.use(
  postgraphile(
    process.env.DATABASE_URL || 'postgres://postgres@127.0.0.1:5432/json_scada',
    'public',
    {
      watchPg: true,
      graphiql: true,
      enhanceGraphiql: true,
    }
  )
)
app.listen(4000, () =>
  console.log('go to for playground graphiql http://localhost:4000/graphiql')
)
