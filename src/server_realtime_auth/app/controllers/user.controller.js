const path = require('path')

exports.login = (req, res) => {
  res
    .status(200)
    .sendFile(path.join(__dirname + '../../../../htdocs-login/login.html'))
}

exports.userBoard = (req, res) => {
  res.status(200).send({ ok: true })
}

exports.adminBoard = (req, res) => {
  res.status(200).send({ ok: true })
}
