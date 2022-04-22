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
 * @returns {number|Long|bigint|Date|string|Boolean}
 */
const castSparkplugValue = (type, value) => {
  switch (type.toLowerCase()) {
    case "int8":
    case "int16":
    case "int32":
      return new Int32Array([value])[0]; // number -1
      // return Long.fromValue(value, false).toSigned().toInt(); // number -1

    case "int64":
      return Long.fromValue(value, false).toSigned(); // object Long { low: -1, high: -1, unsigned: false }
      // return Long.fromValue(value, false).toSigned().toNumber(); // number -1
      // return Long.fromValue(value, false).toSigned().toString(); // string "-1"
      // return BigInt(Long.fromValue(value, false).toSigned().toString()); // bigint -1n

    case "uint8":
    case "uint16":
    case "uint32":
      return new Uint32Array([value])[0]; // number 4294967295
      // return Long.fromValue(value, true).toUnsigned().toInt(); // number 4294967295

    case "uint64":
      return Long.fromValue(value, true).toUnsigned(); // object Long { low: -1, high: -1, unsigned: true }
      // return Long.fromValue(value, true).toUnsigned().toNumber(); //         number  18446744073709552000  !!! warning number is truncated !!!
      // return Long.fromValue(value, true).toUnsigned().toString(); //         string "18446744073709551615"
      // return BigInt(Long.fromValue(value, true).toUnsigned().toString()); // bigint  18446744073709551615n

    case "datetime":
      return new Date(Long.fromValue(value, true).toUnsigned().toNumber()); // object Date

    case "boolean":
      return Boolean(value); // boolean false or true
      // return +Boolean(value); // number 0 or 1
  }
  return value;
};

module.exports = { castSparkplugValue };
