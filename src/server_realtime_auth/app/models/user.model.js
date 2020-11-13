const mongoose = require("mongoose");

const User = mongoose.model(
  "User",
  new mongoose.Schema({
    username: {type: String, default: "new user"},
    email: {type: String, default: ""},
    password: {type: String, default: ""},
    roles: [
      {
        type: mongoose.Schema.Types.ObjectId,
        ref: "Role"
      }
    ]
  }),
  "users"  
);

module.exports = User;