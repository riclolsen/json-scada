const path = require('path')

exports.login = (req, res) => {
  res
    .status(200)
    .sendFile(path.join(__dirname + '../../../../AdminUI/dist/index.html'))
}

// session check of the AdminUI (every 15 s): also answers the current boolean
// rights, so menu entries follow role changes made after sign-in (the
// json-scada-user cookie is written only at sign-in)
exports.userBoard = (req, res) => {
  const rights = {}
  for (const [k, v] of Object.entries(req.rights || {}))
    if (typeof v === 'boolean') rights[k] = v
  res.status(200).send({ ok: true, rights })
}

exports.adminBoard = (req, res) => {
  res.status(200).send({ ok: true })
}
