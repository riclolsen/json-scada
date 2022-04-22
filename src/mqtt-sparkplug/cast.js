const Long = require("long");

/**
 * MQTT sparkplug's numeric datatypes for metrics int8, int16, int32
 * are encoded using protobuf's uint32 type.
 * However tahu's sparkplug-payload library won't convert negative values back,
 * eg, if metric is {name: "metric", type: "Int8", value: -1}, the decoded value is 4294967295 instead of -1
 *
 * This does not "clamp" values
 *
 * 64bits return Long objects TODO should return a number?
 *
 * @param {string} type MQTT sparkplug metric datatype
 * @param {number|string|Long} value
 * @returns {number|Long|Date|string|Boolean}
 */
const castSparkplugValue = (type, value) => {
  switch (type.toLowerCase()) {
    case "int8":
    case "int16":
    case "int32":
      return new Int32Array([value])[0];

    case "int64":
      return Long.fromValue(value, false).toSigned();

    case "uint8":
    case "uint16":
    case "uint32":
      return new Uint32Array([value])[0];

    case "uint64":
      return Long.fromValue(value, true).toUnsigned();

    case "datetime":
      return new Date(Long.fromValue(value, true).toUnsigned().toNumber());

    case "boolean":
      return Boolean(value); // boolean false or true TODO return number 0 or 1 instead?
  }
  return value;
};

module.exports = { castSparkplugValue };
