const mongoose = require('mongoose');
mongoose.Promise = global.Promise;

const db = {};

db.mongoose = mongoose;

db.user = require("./user.model");
db.role = require("./role.model");
db.tag = require("./tag.model");
db.protocolDriverInstance = require("./protocolDriverInstance.model");
db.protocolConnection = require("./protocolConnection.model");

// db.ROLES = ["user", "admin", "operator",  "viewer"];

module.exports = db;