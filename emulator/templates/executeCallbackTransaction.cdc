import "UnsafeCallbackScheduler"

transaction(callbackID: UInt64) {
    execute {
        log("[system.execute_callback] executing callback ".concat(callbackID.toString()))
        UnsafeCallbackScheduler.executeCallback(ID: callbackID)
    }
}