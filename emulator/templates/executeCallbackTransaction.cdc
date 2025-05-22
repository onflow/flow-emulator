import "CallbackScheduler"

transaction(id: UInt64) {
    execute {
        CallbackScheduler.execute(id: id)
    }
}