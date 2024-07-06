'use strict'

const bsonDouble = require('bson').Double;
const mongoose = require('mongoose')

class DoubleType extends Number {
  constructor(v) {
    super(v)
    this.value = v
    this._bsontype = 'Double'
  }

  toBSON() {
    return new bsonDouble(this.value)
  }
}

class Double extends mongoose.SchemaType {
  constructor(key, options) {
    super(key, options, 'Double')

    Object.assign(this.$conditionalHandlers, {
      $lt: (val) => this.castForQuery(val),
      $lte: (val) => this.castForQuery(val),
      $gt: (val) => this.castForQuery(val),
      $gte: (val) => this.castForQuery(val),
    })
  }

  cast(val) {
    if (val == null) {
      return val
    }
    if (val._bsontype === 'Double') {
      return new DoubleType(val.value)
    }

    const _val = Number(val)
    if (isNaN(_val)) {
      throw new mongoose.SchemaType.CastError(
        'Double',
        val + ' is not a valid double'
      )
    }
    return new DoubleType(_val)
  }
}

mongoose.Schema.Types.Double = Double
mongoose.Types.Double = DoubleType

module.exports = Double
