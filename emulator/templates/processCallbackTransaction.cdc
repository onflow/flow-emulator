// todo: replace with flow-core-contracts implementation 
// issue: https://github.com/onflow/flow-emulator/issues/829

import "UnsafeCallbackScheduler"

transaction() {
    execute {
        log("[system.process_callbacks] processing callbacks")
        UnsafeCallbackScheduler.process()
    }
}