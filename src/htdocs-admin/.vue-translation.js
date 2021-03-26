const path = require("path");
const { JSONAdapter } = require("vue-translation-manager");

module.exports = {
  srcPath: path.join(__dirname, "src/"),
  adapter: new JSONAdapter({
    path: path.join(__dirname, "src/locales/"),
  }),
  languages: ["en", "pt", "es", "de", "uk", "zh", "ru", "ar", "fa"],
};