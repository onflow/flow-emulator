import "UnsafeCallbackScheduler"

transaction() {
    execute {
        log("[system.process_callbacks] processing callbacks")
        UnsafeCallbackScheduler.process()
    }
}