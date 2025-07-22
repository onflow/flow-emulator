// todo: replace with flow-core-contracts implementation 
// issue: https://github.com/onflow/flow-emulator/issues/829

import "FlowCallbackScheduler"

transaction(callbackID: UInt64) {
    execute {
        log("[system.execute_callback] executing callback ".concat(callbackID.toString()))
        FlowCallbackScheduler.executeCallback(ID: callbackID)
    }
}