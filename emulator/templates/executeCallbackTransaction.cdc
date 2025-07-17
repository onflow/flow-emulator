// todo: replace with flow-core-contracts implementation 
// issue: https://github.com/onflow/flow-emulator/issues/829

import "UnsafeCallbackScheduler"

transaction(callbackID: UInt64) {
    execute {
        log("[system.execute_callback] executing callback ".concat(callbackID.toString()))
        UnsafeCallbackScheduler.executeCallback(ID: callbackID)
    }
}