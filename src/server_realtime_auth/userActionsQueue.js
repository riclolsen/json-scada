const Queue = require('queue-fifo')

let UserActionsQueue = new Queue() // queue of user actions to write to mongo collection

module.exports = UserActionsQueue;